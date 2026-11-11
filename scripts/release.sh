#!/usr/bin/env bash
# Cut a go-web-base release: tag core, wait for the Go module proxy to serve
# it, then prepare a release branch that requires that core in each feature
# module (tagging each one with the same version), pins the template to core
# and the features, and records its go.sum. After that branch merges, tag the
# merge commit with the same version for the template.
#
#   scripts/release.sh v0.2.0
#
# Run it from a clean checkout of main whose CI is green. It pushes the core
# tag and the release branch with your git credentials.
set -euo pipefail

VERSION=${1:?usage: scripts/release.sh vX.Y.Z}
[[ $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like v1.2.3" >&2; exit 2; }
MODULE=github.com/andrewmyhre/go-web-base/core
COPIER="${UVX:-uvx} copier@9.19.1"
ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

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

# 4. Release branch: pin the features and the template to the new versions and
#    record the template's go.sum (rendered with every feature on, so it has
#    the hashes of all of them).
git switch -q -c "release/$VERSION"

# --- Feature modules (lockstep versions) ------------------------------------
# Each feature in FEATURES is released with the same version as core. A feature
# tag has to sit on a commit whose go.mod requires this core version, and that
# commit cannot exist before core is published, so it is made here on the
# release branch and tagged from there. Merge the release PR with a merge
# commit (not squash or rebase) so the tagged commit stays in main's history.
FEATURES=(example)
FEATURE_BASE=github.com/andrewmyhre/go-web-base/features
for f in "${FEATURES[@]}"; do
  sed -i -E "s#^(\s*$MODULE) v[0-9.]+#\1 $VERSION#" "features/$f/go.mod"
  # The in-repo replace stays, so tidy needs no published core.
  ( cd "features/$f" && go mod tidy )
  git add "features/$f/go.mod" "features/$f/go.sum"
  git commit -q -m "features/$f: require core $VERSION" || true
  if git rev-parse -q --verify "refs/tags/features/$f/$VERSION" >/dev/null; then
    echo "features/$f/$VERSION already exists; skipping the tag"
  else
    git tag -a "features/$f/$VERSION" -m "features/$f $VERSION"
    git push -q origin "features/$f/$VERSION"
    echo "pushed features/$f/$VERSION"
  fi
  for i in $(seq 1 30); do
    if GOPROXY=https://proxy.golang.org GOFLAGS=-mod=mod go list -m "$FEATURE_BASE/$f@$VERSION" >/dev/null 2>&1; then
      echo "proxy.golang.org serves $FEATURE_BASE/$f@$VERSION"
      break
    fi
    [[ $i == 30 ]] && { echo "the proxy does not serve $FEATURE_BASE/$f@$VERSION after 5 minutes" >&2; exit 1; }
    sleep 10
  done
  sed -i -E "s#($FEATURE_BASE/$f) v[0-9.]+#\1 $VERSION#" template/go.mod.jinja
done
# --- end of feature modules ---------------------------------------------------

sed -i -E "s#^(\s*$MODULE) v[0-9.]+#\1 $VERSION#" template/go.mod.jinja
git commit -q -am "Pin the template to core and features $VERSION" || true
SAMPLE=$(mktemp -d)
$COPIER copy --vcs-ref HEAD --defaults --trust --quiet . "$SAMPLE" \
  -d project_slug=release-check -d github_repo=example/release-check \
  -d feature_example=true
( cd "$SAMPLE" && GOFLAGS=-mod=mod go mod tidy && go build ./... && go vet ./... )
cp "$SAMPLE/go.sum" template/go.sum
rm -rf "$SAMPLE"
git add template/go.sum
git commit -q -m "Release $VERSION" || echo "go.sum unchanged"
git push -q -u origin "release/$VERSION"

cat <<EOF

Pushed release/$VERSION. Next:
  1. Open a PR from release/$VERSION to main and wait for CI. Merge it with a
     merge commit: features/*/$VERSION tags sit on its commits.
  2. Merge it, then tag the merge commit for the template:
       git switch main && git pull --ff-only
       git tag -a $VERSION -m "go-web-base $VERSION" && git push origin $VERSION
  3. The tag's pipeline runs release-check: a sample built against the
     published core with no replace. Confirm it is green.
EOF
