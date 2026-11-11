#!/usr/bin/env bash
# Cut a go-web-base release: tag core, wait for the Go module proxy to serve
# it, build and pack the UI package and attach it to a GitHub release
# ui/vX.Y.Z, then prepare a release branch that pins the template to both and
# records the template's go.sum and ui/pnpm-lock.yaml. After that branch
# merges, tag the merge commit with the same version for the template.
#
#   scripts/release.sh v0.2.0
#
# Run it from a clean checkout of main whose CI is green. It pushes the core
# tag and the release branch with your git credentials, and creates the UI
# release with your gh login (it needs pnpm, node 20.19 or later, and a gh
# that can write releases in this repository).
set -euo pipefail

VERSION=${1:?usage: scripts/release.sh vX.Y.Z}
[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like v1.2.3" >&2; exit 2; }
MODULE=github.com/andrewmyhre/go-web-base/core
COPIER="${UVX:-uvx} copier@9.19.1"
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"
REPO=andrewmyhre/go-web-base
NUM=${VERSION#v}
UI_TAG=ui/$VERSION
UI_TARBALL=andrewmyhre-web-base-ui-$NUM.tgz
# GitHub serves the asset under the tag name with its slash encoded.
UI_URL=https://github.com/$REPO/releases/download/ui%2F$VERSION/$UI_TARBALL
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0
command -v gh >/dev/null || { echo "gh is required to create the UI release" >&2; exit 1; }
command -v pnpm >/dev/null || { echo "pnpm is required to pack the UI package" >&2; exit 1; }

# 1. Preconditions: main, clean, in sync with origin.
[[ $(git branch --show-current) == main ]] || { echo "check out main first" >&2; exit 1; }
[[ -z $(git status --porcelain --untracked-files=no) ]] || { echo "working tree is not clean" >&2; exit 1; }
git fetch -q origin main --tags
[[ $(git rev-parse HEAD) == $(git rev-parse origin/main) ]] || { echo "main is not in sync with origin/main" >&2; exit 1; }
if git rev-parse -q --verify "refs/tags/core/$VERSION" >/dev/null; then
  echo "core/$VERSION already exists; skipping the tag"
else
  # 2. Tag core and push the tag.
  git tag -a "core/$VERSION" -m "core $VERSION"
  git push -q origin "core/$VERSION"
  echo "pushed core/$VERSION"
fi

# 3. Wait until the public module proxy serves it (it fetches on first request).
for i in $(seq 1 30); do
  if GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod go list -m "$MODULE@$VERSION" >/dev/null 2>&1; then
    echo "proxy.golang.org serves $MODULE@$VERSION"
    break
  fi
  [[ $i == 30 ]] && { echo "the proxy does not serve $MODULE@$VERSION after 5 minutes" >&2; exit 1; }
  sleep 10
done

# 4. The UI package: pack it at this version and attach it to the release
# ui/vX.Y.Z. It is packed from a copy of the committed source, so main's
# working tree is never edited to set the version (the committed version is
# 0.0.0 and only the copy gets the real one). gh creates the ui/vX.Y.Z tag on
# this commit. If the release already exists, nothing is rebuilt or uploaded.
if gh release view "$UI_TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "release $UI_TAG already exists; skipping the UI build"
else
  PACK=$(mktemp -d)
  git archive HEAD packages/web-base-ui | tar -x -C "$PACK"
  ( cd "$PACK/packages/web-base-ui" \
    && pnpm install --frozen-lockfile \
    && pnpm pkg set "version=$NUM" \
    && pnpm build && pnpm test && pnpm check \
    && pnpm pack --pack-destination "$PACK" )
  [[ -f $PACK/$UI_TARBALL ]] || { echo "pnpm pack did not produce $UI_TARBALL" >&2; exit 1; }
  gh release create "$UI_TAG" "$PACK/$UI_TARBALL" --repo "$REPO" --target "$(git rev-parse HEAD)" \
    --title "UI package $VERSION" \
    --notes "@andrewmyhre/web-base-ui $NUM. Generated projects pin this file's URL in ui/package.json and take new versions with copier update."
  rm -rf "$PACK"
  echo "created release $UI_TAG"
fi
# The asset has to be downloadable before the template pins it.
curl -fsSIL -o /dev/null "$UI_URL" || { echo "$UI_URL is not downloadable" >&2; exit 1; }

# 5. Release branch: pin the template to the new core and UI package, and
# record go.sum and ui/pnpm-lock.yaml from a rendered sample. The lockfile
# must be made against the real release URL: release-check-ui installs with
# --frozen-lockfile, which fails when package.json and the lockfile disagree
# or the tarball's integrity hash is missing.
git switch -q -c "release/$VERSION"
sed -i -E "s#^(\s*$MODULE) v[0-9.]+#\1 $VERSION#" template/go.mod.jinja
sed -i -E "s#(\"@andrewmyhre/web-base-ui\": )\"[^\"]*\"#\1\"$UI_URL\"#" template/ui/package.json.jinja
grep -q "$UI_URL" template/ui/package.json.jinja || { echo "could not pin the UI dependency" >&2; exit 1; }
git commit -q -am "Pin the template to core and the UI package $VERSION" || true
SAMPLE=$(mktemp -d)
$COPIER copy --vcs-ref HEAD --defaults --trust --quiet . "$SAMPLE" \
  -d project_slug=release-check -d github_repo=example/release-check
( cd "$SAMPLE" && GOFLAGS=-mod=mod go mod tidy && go build ./... && go vet ./... )
# Install against the real URL (rewriting the lockfile), then prove the
# frozen install and the UI checks pass with the result.
( cd "$SAMPLE/ui" \
  && pnpm install --no-frozen-lockfile \
  && pnpm install --frozen-lockfile \
  && pnpm run check:client && pnpm test && pnpm check && pnpm build )
cp "$SAMPLE/go.sum" template/go.sum
cp "$SAMPLE/ui/pnpm-lock.yaml" template/ui/pnpm-lock.yaml
rm -rf "$SAMPLE"
git add template/go.sum template/ui/pnpm-lock.yaml
git commit -q -m "Release $VERSION" || echo "go.sum and pnpm-lock.yaml unchanged"
git push -q -u origin "release/$VERSION"

cat <<EOF

Pushed release/$VERSION and created the GitHub release $UI_TAG. Next:
  1. Open a PR from release/$VERSION to main and wait for CI.
  2. Merge it, then tag the merge commit for the template:
       git switch main && git pull --ff-only
       git tag -a $VERSION -m "go-web-base $VERSION" && git push origin $VERSION
  3. The tag's pipeline runs release-check (a sample built against the
     published core with no replace) and release-check-ui (the sample's UI
     installed from the release URL with a frozen lockfile). Confirm both
     are green.
EOF
