# go-web-base

A [Copier](https://copier.readthedocs.io) template for a Go API and a
SvelteKit PWA with Google sign-in and Casbin RBAC, whose
server code lives in the `core/` Go module of this repository, which a
generated project imports instead of copying. The code began as
`andrewmyhre/huddle-go` at `v0.1.0` (commit `6028ffc`).

## What a generated project has

The Go code below is `core`, imported at the version pinned in the project's
`go.mod`. The project itself holds a small `cmd/<slug>/main.go` that calls
`app.Main` with its name, env prefix and cookie prefix, tests for the files it
owns, docs, deploy manifests, CI and the UI. See [core/README.md](core/README.md).

- A Go API on chi and Huma. Every route goes through `server.Register` with a
  typed `server.Permission`, `server.Public` or `server.Authenticated`, and an
  unchecked route fails `Build`.
- Google OIDC sign-in: browser-bound state taken with `GETDEL`, PKCE, a nonce,
  the ID token verified with go-oidc, and sessions in Redis rotated at sign-in.
- Casbin RBAC with project domains plus `global`, the example role graph in
  `roles.go` and `docs/roles.md` (six roles, edit them per project; core
  ships none), an admin bootstrapped from config, and users with no binding
  refused at sign-in.
- Optional feature modules (`app.Options.Features`) that add routes,
  resources and role grants, migrations, settings and a log scope. See
  [features/example](features/example/README.md).
- `GET /me` and `POST /auth/logout`.
- A SvelteKit 2 / Svelte 5 PWA in `ui/`, whose API client is generated from the
  API's OpenAPI document (drift fails CI). Its reusable parts (account menu,
  avatar, side menu, API helpers, service worker policy) come from the
  `@andrewmyhre/web-base-ui` package in `packages/web-base-ui/`, pinned by
  URL in `ui/package.json`. See [packages/web-base-ui/README.md](packages/web-base-ui/README.md).
- Postgres via pgx, sqlc and goose. Redis via go-redis.
- zap logging with scopes `api`, `rbac`, `queue` and `agent`, for Loki, with a
  Grafana dashboard in `deploy/grafana/`.
- A Woodpecker pipeline: tests, lint, UI checks, BuildKit image builds, and
  `deploy-dev` with a `/health` and `/` smoke check.

## Use

For how projects are meant to build on the base over time (versioned
packages, feature toggles, what belongs where), read
[docs/consuming.md](docs/consuming.md).

Copier is pinned to **9.19.1**. Run it through `uvx`; nothing needs installing
beyond [uv](https://docs.astral.sh/uv/).

```sh
uvx copier@9.19.1 copy gh:andrewmyhre/go-web-base my-app
```

Later, to merge template changes into the project (a three-way merge against
the answers in `.copier-answers.yml`; commit first):

```sh
cd my-app
uvx copier@9.19.1 update
```

### Questions

| Question | Default | Used for |
|---|---|---|
| `project_slug` | none | binary, images, Deployments, ServiceAccount, dashboard |
| `project_name` | slug in title case | UI title, PWA manifest, prose |
| `github_repo` | `andrewmyhre/<slug>` | references to the repo |
| `go_module` | `github.com/<github_repo>` | `go.mod` and every import |
| `env_prefix` | slug in upper case, `-` as `_` | every `PREFIX_*` variable |
| `dev_host` | `<slug>-dev.immutablesoftware.dev` | ingress, redirect URI, docs |
| `dev_namespace` | `<slug>-dev` | dev environment |
| `registry` | `192.168.0.10:30500` | image names |
| `admin_sub` | empty | the global Admin's Google sub |

Two values are derived and never asked: the lower case of `env_prefix` names
the cookies (`<ident>_session`, `<ident>_oauth_state`), the Postgres user and
database, the Redis key prefixes and the root Go package; the first label of
`dev_host` is the Cloudflare subdomain.

## Per-project setup checklist

Nothing in a generated project works on the cluster until these are done.
Each one needs a human or an admin kubeconfig. `<slug>`, `<prefix>` and so on
are your answers.

1. **GitHub and Woodpecker.** Create the repo, push the generated project,
   and enable it in Woodpecker. Mark it trusted: `deploy-dev` sets
   `backend_options`.
2. **Google OAuth Web client.**
   - Authorized origin: `https://<dev_host>`.
   - Redirect URI: `https://<dev_host>/auth/google/callback`.
   - The OAuth consent screen's authorized domains must include the host's
     domain.
3. **Seal the secrets** with `kubeseal` (controller
   `sealed-secrets-controller` in `kube-system`). The commands are in the
   generated `deploy/dev/README.md`:
   - `dev-services` (`postgres-password`, `redis-password`) in
     `<dev_namespace>`;
   - `<slug>-google-oauth` (`client-id`, `client-secret`) in
     `<dev_namespace>`.

   The template never ships a SealedSecret: each is encrypted for one cluster
   and namespace.
4. **One-time applies** from `deploy/dev`: the
   namespace, `rbac.yaml`, Postgres and Redis, then the first app rollout
   (`deploy/dev/README.md`, step 3).
5. **Cloudflare route, DNS and Access app.** Needs Andrew's yes. The default
   zone is `immutablesoftware.dev`:

   ```sh
   python3 ~/.claude-test/skills/cloudflare-zero-trust/scripts/cf_ops.py \
     add-service "<project_name> Dev" <dev_subdomain>
   ```

6. **The admin.** Set `<prefix>_ADMIN_SUB` in `deploy/dev/api.yaml` to the
   owner's Google sub (digits, never an email). If you left `admin_sub` empty,
   nobody can sign in yet, so `/me` cannot tell you. Sign in once anyway: the
   refusal logs an `rbac` deny line whose `subject` is the sub.

   ```sh
   kubectl -n <dev_namespace> logs deploy/<slug>-api | grep '"decision":"deny"'
   ```

## Developing the template

- `copier.yml` holds the questions. `template/` is what gets rendered.
- Only files where a value varies are templated, and only those carry the
  `.jinja` suffix. Paths are templated too (`cmd/[[ project_slug ]]/`).
- The delimiters are `[[ ]]`, `[[% %]]` and `[[# #]]`, not Jinja's defaults:
  Svelte's `{#if}` is a Jinja comment opener, and Go uses `[%q]`. A YAML flow
  list of one templated value needs quotes (`["[[ project_slug ]]"]`), or
  `[[[` confuses the lexer.
- A generated project's tests need no Postgres or Redis: those tests live in
  `core/` and run against Woodpecker `services:` in this repository's own CI.
  The generated pipeline therefore has no test services and no secrets.
- A generated project's `go.mod` requires a published core release, and
  `template/go.sum` carries its hash. Both are set by the release script below,
  because a release has to exist before it can be hashed.

### Releasing

Core, the UI package and the template share one version number. Core is
tagged `core/vX.Y.Z` (the Go convention for a module in a subdirectory), the UI
package is a GitHub release `ui/vX.Y.Z`, and the template is tagged `vX.Y.Z`,
one commit later, once it pins both. From a clean, green `main`:

```sh
scripts/release.sh v0.3.0
```

The script tags and pushes `core/v0.3.0`, waits until `proxy.golang.org`
serves it, then builds, tests and packs `packages/web-base-ui` at `0.3.0`
(`pnpm pack`, from a copy, so `main` stays at version `0.0.0`) and creates the
GitHub release `ui/v0.3.0` with `andrewmyhre-web-base-ui-0.3.0.tgz` attached,
using your `gh` login. No registry or token is involved: generated projects
depend on the tarball's download URL,
`https://github.com/andrewmyhre/go-web-base/releases/download/ui%2Fv0.3.0/andrewmyhre-web-base-ui-0.3.0.tgz`.
It then pushes a `release/v0.3.0` branch that pins the template to core and
that URL, and records the template's `go.sum` and `ui/pnpm-lock.yaml` (the
lockfile is made by installing a rendered sample against the real URL, then
checked with `--frozen-lockfile`). Open a PR from that branch, merge it, and
tag the merge commit `v0.3.0`. The tag's pipeline runs `release-check`, which
builds and tests a sample against the published core with no `replace` and a
read-only `go.sum`, and `release-check-ui`, which installs the sample's UI
from the release URL with a frozen lockfile and runs its checks.

Re-running the script skips what exists: the core tag and the `ui/vX.Y.Z`
release. Until the first release, the template pins the `ui/v0.2.0` URL, which
does not exist yet, and its `ui/pnpm-lock.yaml` does not match; the pipeline's
`ui` step does not use either (below), and `release.sh` fixes both.

Semver applies to core's exported Go API and to the `app.Options` and feature
contract. Before `v1`, a minor version may break them; say how in the release
notes. Feature modules (`features/<name>/`) are tagged on their own
(`features/<name>/vX.Y.Z`).

### Template CI

`.woodpecker.yml` renders `sample/` from the commit under test (with
`--vcs-ref HEAD`, since Copier otherwise renders the latest tag), then:

- sweeps it for `huddle` and for unrendered `[[ ]]` tags, with a control
  render using huddle answers that must bring the string back;
- in `sample/`, runs `go mod edit -replace github.com/andrewmyhre/go-web-base/core=../core`
  and `go mod tidy`, so the template is tested against the core in the same
  commit, then `gofmt`, `go vet` and `go test`;
- in `ui-package`, installs, builds, tests, type-checks and packs
  `packages/web-base-ui` (committed version `0.0.0`);
- in `ui`, points the sample's `@andrewmyhre/web-base-ui` dependency at that
  local tarball (`pnpm pkg set` to a `file:` path, then a non-frozen install,
  since the pinned release URL may not exist yet), so the template is tested
  against the UI package in the same commit, then runs `check:client`, `test`,
  `check` and `build`;
- on a `v*` tag, `release-check-ui` runs the same UI checks in the untouched
  sample with the pinned URL and a frozen lockfile;
- tests `core/` and `features/example` against real Postgres and Redis
  services.

The sample's own image build and deploy steps never run: Woodpecker reads only
the root `.woodpecker.yml`.

To run the same checks locally (the replace path is absolute because the
sample is outside this checkout):

```sh
uvx copier@9.19.1 copy --vcs-ref HEAD --defaults --trust . /tmp/sample \
  -d project_slug=sample-app -d github_repo=example/sample-app
cd /tmp/sample
go mod edit -replace github.com/andrewmyhre/go-web-base/core=/path/to/go-web-base/core
go mod tidy
test -z "$(gofmt -l .)" && go vet ./... && go test ./...
(cd /path/to/go-web-base/packages/web-base-ui && pnpm install --frozen-lockfile \
  && pnpm build && pnpm pack --pack-destination /tmp)   # makes /tmp/andrewmyhre-web-base-ui-0.0.0.tgz
cd ui
pnpm pkg set "dependencies.@andrewmyhre/web-base-ui=file:/tmp/andrewmyhre-web-base-ui-0.0.0.tgz"
pnpm install --no-frozen-lockfile
pnpm run check:client && pnpm test && pnpm check && pnpm build
```

### Regression control

The control render uses huddle answers (`project_slug=huddle-go`,
`env_prefix=HUDDLE`). It no longer reproduces `huddle-go` `v0.1.0`, because
that code moved into `core/`. What it proves now is that the sweep works: the
render must contain the string `huddle`, so a sweep that finds nothing in the
sample means the template is clean, not that the sweep is blind.

### Updating from an older template

A project generated at `v0.1.x` carries its own `config`, `db`, `server` and
the other packages. `copier update` to a core-based version deletes the files
the template no longer renders, then the project needs `go mod tidy` once
(and a `go.sum` line for core, once core is tagged). Edits the project made to
those packages are lost with them: move them upstream into `core/` first.

A project already on core `v0.2.0` also gains `roles.go` when it updates to
the template that adds feature support, and its `main.go` must pass
`Roles` and `Resources` to `app.Options`, because core no longer ships a role
graph. Copier renders `roles.go` and merges `main.go`; check the merge and run
`go test ./...`. `TestRolesMatchDocs` now compares `docs/roles.md` with
`roles.go`.
