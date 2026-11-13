# Building on go-web-base

This is for anyone, human or agent, who starts or maintains a project built on
go-web-base. It explains what your project gets from the base, how
improvements reach it, how to turn optional features on, and what belongs in
your project versus in the base.

## Status

The model below is the target design, tracked in epic
[IMM-2066](https://linear.app/immutablesoftware/issue/IMM-2066). Today the
shared server code is the `core` Go module in this repository, and a project
generated from the current template imports it, and the reusable UI is the
`@andrewmyhre/web-base-ui` package. Dependabot for Go modules and local
development across the base and a project are available now; the feature UI
packages are not done yet. Projects generated at `v0.1.x` still carry
their own copies; `copier update` moves them onto core. Each section says which parts are **available now** and which are
**planned**, with the issue that delivers them.

The repository is public, so consumers, CI and image builds need no GitHub
credential to fetch the Go modules, the UI package or the template.

## The model

go-web-base reaches your project through two channels, each for a different
kind of content.

| | Versioned packages | Copier template |
|---|---|---|
| Carries | shared code: server, sign-in, RBAC, logging, Redis, Postgres, config, features, reusable UI | your project's own files: `main.go` wiring, config values, deploy manifests, CI, docs |
| Lives in your repo as | a version in `go.mod`, and a release URL in `ui/package.json` | real files you own and edit |
| You update with | `uvx copier@9.19.1 update` for the UI package; `go get -u` or a Dependabot PR for Go | `uvx copier@9.19.1 update` |
| Conflicts | none: you never edit package code | a three-way merge with your edits |

The rule behind the split: **code you would never edit is a package; code you
are meant to edit is a template file.** A fix to sign-in then reaches every
project as a version bump, with no merge. Your route table and manifests stay
yours, and the template only changes them when its glue changes.

The Go modules are:

- `github.com/andrewmyhre/go-web-base/core`: the server, sign-in, sessions,
  RBAC, logging, Redis and Postgres access, and config.
- One module per optional feature, such as `.../agent`.

The UI package is `@andrewmyhre/web-base-ui` (**available now**, source in
`packages/web-base-ui/`): the account menu, avatar, side menu, user badge,
typed API client helpers and the service worker policy. It is not on an npm
registry. Each release attaches its tarball to a GitHub release `ui/vX.Y.Z`,
and the template's `ui/package.json` depends on that file's URL. Because the URL is pinned by the template, UI updates arrive through
`copier update`. Feature UI packages, such as `@andrewmyhre/web-base-agent-ui`,
are **planned** (IMM-2076, IMM-2077).

## Starting a project

**Available now.**

```sh
uvx copier@9.19.1 copy gh:andrewmyhre/go-web-base my-app
```

Copier asks for the slug, names, Go module path, env prefix, dev host,
namespaces, registry and the admin's Google sub.
Then work through the per-project setup checklist in the
[README](../README.md#per-project-setup-checklist): nothing works on the
cluster until it is done.

Commit `.copier-answers.yml`. It records your answers and the template version,
and `copier update` needs it.

## Keeping up to date

**UI package updates (available now).** The UI package version is a URL pinned
in `ui/package.json`, so a new version arrives through `copier update`
(below), which moves the URL and the lockfile together. Read the release notes
of a minor version before `v1` and of any major version: they say how exports
or CSS class names changed.

**Go module updates.** Bump `core` with `go get github.com/andrewmyhre/go-web-base/core@vX.Y.Z`
(or `copier update`, which also moves the pin).

**Dependabot (available now).** A generated project has
`.github/dependabot.yml`. GitHub runs Dependabot version updates for any
repository that holds this file on its default branch, so there is nothing to
switch on in the repository settings. (Dependabot alerts and security updates
are separate settings under Settings, Code security, and are worth enabling.)
Weekly, it opens at most two PRs for Go:

- one that moves every `github.com/andrewmyhre/go-web-base/*` module together.
  Core and the features share a release number and must be bumped as a set;
- one that moves all other Go dependencies.

Your CI runs on them like any other PR; read the release notes for a major
version before you merge, and run `copier update` as well when they say the
template glue changed.

The file does not cover npm, on purpose. The UI package is pinned by a release
URL that Dependabot cannot bump, and the rest of `ui/` (its dependency
versions and `ui/pnpm-lock.yaml`) comes from the template, so a Dependabot bump
would conflict with the next `copier update` merge. UI and template changes
arrive through `copier update`. If you would rather own the UI dependencies,
add an `npm` entry with `directory: /ui` and accept that merge noise.

**Template updates (available now).** When the template's glue changes, and
for every release of the UI package, run from a clean working tree:

```sh
uvx copier@9.19.1 update
```

Copier re-renders the template at the new version with your recorded answers
and three-way merges it with your edits. Resolve any conflicts, run your
checks, and commit. Once modules are released (IMM-2070), a module's release
notes will say when a bump also needs a template update, for example when a
new version of `core` changes how `main.go` must call it.

## Enabling a feature

**Available now.** Each optional feature is a boolean Copier question named
`feature_<name>`, default `false`. The only one today is `feature_example`
(the [example feature](../features/example/README.md)). A new project answers
it in `copier copy` (`-d feature_example=true`). To turn it on in an existing
project, from a clean working tree:

```sh
uvx copier@9.19.1 update -d feature_example=true
```

That renders only the glue, and nothing else changes:

- `cmd/<slug>/main.go`: the import and `Features: []app.Feature{example.New("Contributor")}`.
  The feature is granted to `Contributor`, so every role that inherits it
  (`Project Manager`, `Manager`, `Admin`) can call `GET /example/hello`. Change
  the role name in `main.go` if your role graph differs.
- `go.mod`: a `require` of `github.com/andrewmyhre/go-web-base/features/example`
  at the template's release version (run `go mod tidy` once after the update to
  fetch it and record `go.sum`).
- `deploy/dev/api.yaml`: a commented `<PREFIX>_EXAMPLE_GREETING` entry.
- `deploy/dev/ingress.yaml` and `ui/src/service-worker.ts`: the `/example`
  prefix, so the Ingress sends the feature's routes to the API and the
  service worker never serves them from its cache.
- `README.md` and `llms.txt`: a line each.
- `ui/openapi.json` and `ui/src/lib/api/openapi.d.ts`: the route and its
  response schema, so the OpenAPI drift test and `check:client` pass with the
  feature on. Both files are templates with the feature's sections wrapped in
  conditions, so a project's own routes sit outside them and merge cleanly.
  Only the sections for features you enabled are rendered. If you add your own
  routes, regenerate both with `pnpm run generate` as usual, and the feature's
  routes stay in the output.

The feature's code arrives as its module, not as files in your repo.

To turn it off, run the same update with `=false`. Verified on a scratch
project (render with the feature off, commit, update to on, commit, update to
off): the update to `true` adds exactly the glue above, and the update back to
`false` returns every file to its pre-feature content, byte for byte, with no
conflicts. The feature's database tables are not dropped, so remove them with
a migration of your own if you need to.

Copier does a three-way merge, so an edit of yours that touches lines next to
the glue can conflict. A project edit on a line beside `Features:` in `main.go`
or beside the end of the `env:` list in `deploy/dev/api.yaml` produced ordinary
conflict markers on both updates, which you resolve by hand. Edits elsewhere in
the same files (a version bump, an env var added before `ADMIN_SUB`) merged
without conflict. If you have added a second entry to `Features` yourself,
expect to resolve that line when you turn the example off.

Enable a feature only where the project needs it. Each one adds routes,
permissions and config that must be set up and secured.

## What belongs where

Put a change in **your project** when it is about your product: your routes,
your domain logic, your role graph, your UI pages, your deployment values.

Put it in **go-web-base** when a second project would want it unchanged:

- a fix or hardening of sign-in, sessions, RBAC, logging or the pipeline;
- a capability several projects need, which becomes a feature module;
- a change to how projects are wired or deployed, which changes the template.

When unsure, build it in your project first. Move it into go-web-base when the
second project needs it. Do not edit code that came from a package: fork the
change upstream instead, or it disappears at the next bump.

The base's server code comes from `core`, imported rather than copied, so
there is no copy of it in your project to edit. Make the fix in go-web-base's
`core/` and bump the version in your `go.mod`. Projects still on the old
copied layout (template `v0.1.x`) get core by running `copier update`.

## Admitting new users (first sign-in)

**Available now** (core `v0.3.0`). Core refuses a Google account that has no
role binding. An app that wants to admit new people, for example one that
creates an account on a person's first sign-in, sets one or both of these in
`app.Options` (in `cmd/<slug>/main.go`):

- `DefaultRole`: the role every new subject is bound to, globally. It must be a
  role in `Roles`, and not `Admin`.
- `OnFirstSignIn(ctx, server.Identity) (role string, err error)`: runs once per
  new subject, with the Google `Sub`, the verified `Email` (empty if
  unverified), `Name` and `Picture`. Create your own account row there and
  return the role (empty means `DefaultRole`). An error refuses the sign-in
  (`server.ErrSignInRefused` for a 403, anything else a retryable 503).

Make the hook idempotent (an upsert keyed on `Sub`): it can run again for the
same subject after a failure or on another replica. Set neither and nothing
changes, so an existing project is unaffected by upgrading. See
[core/README.md](../core/README.md#first-sign-in) for the details.

## Background work and the database pool

**Available now** (core `v0.4.0`). An app that runs workers in the API process,
for example queue consumers, sets `Options.Run` in `cmd/<slug>/main.go`:

- `Run(ctx context.Context, d app.RunDeps) error` starts under `serve` after the
  database, Redis and migrations are ready and before the listener, with the
  signal context. `RunDeps` has `Pool` (`*pgxpool.Pool`), `Authorizer` and
  `Log`. It never runs under `openapi`.
- Shutdown cancels the context, stops the listener and waits for `Run` to
  return before it closes the pool, so `Run` must return when the context is
  cancelled.
- A non-nil error from `Run` (other than `context.Canceled`), or a panic, is
  fatal: the listener stops and the process exits non-zero. Retry inside `Run`
  if a failure should be survivable. A nil return before shutdown is logged and
  the API keeps serving.

Route handlers reach the same pool through `Deps.Pool` in `Options.Routes`
(`nil` without a database and under `openapi`, so only capture it, never use it
while registering). Set neither and nothing changes. See
[core/README.md](../core/README.md#background-work) for the details.

## Roles and permissions

**Available now.** Core ships no roles and no resources. Your project defines
its role graph in `roles.go` at the project root (`Roles()` and `Resources()`),
and `cmd/<slug>/main.go` passes them to core as `app.Options.Roles` and
`app.Options.Resources`. `docs/roles.md` documents the graph, and
`TestRolesMatchDocs` in your project fails until the two agree.

Features add their own resources and grant them to your roles, so you decide
who can use what a feature adds. Every route still declares a permission
through `server.Register`. Two guarantees hold at startup, before anything
connects:

- a permission that names a resource or verb nobody registered is refused,
  whether it is on a route or in a role;
- a route with no declared permission cannot be served.

A feature grant to a role you do not have is refused as well, with the
feature and role named. The admin bootstrap and "no binding, no sign-in" work
as before: your graph must define `Admin`. See
[core/README.md](../core/README.md#roles-and-resources) for the details.

## Writing a feature

**Available now.** A feature is a Go module with one type that implements
`app.Feature`:

```go
type Feature interface {
	Name() string                // lower case id: log scope, migration table, config prefix
	Install(i *app.Installer) error
}
```

`Install` only declares. Through the `Installer` it can contribute:

- routes (`Routes`), registered with `server.Register` so each declares a
  permission;
- resources (`Resource`), and grants of them to roles (`Grant`);
- goose migrations (`Migrations`), embedded in the module, each module in its
  own version table, `goose_<name>_version`;
- settings (`String`, `Int`, `Bool`, `Secret`), bound to flags and to env vars
  under the project's prefix, such as `PREFIX_AGENT_*`; secrets are env only;
- a log scope (`Log`), so every line carries `scope=<name>`.

`Install` runs before the command line is parsed, because settings become
flags, so it must not read a setting or open a connection. The function
passed to `Routes` runs at startup with the settings resolved, and is where
the feature reads them.

[features/example](../features/example/README.md) uses every hook and has its
own end to end test. Copy it to start a feature. A project installs one by
importing the module and listing it in `Features` in `main.go`; the Copier
question that renders that glue is described in
[Enabling a feature](#enabling-a-feature). Its migrations run
after core's and in the order the features are listed, before the project's own.

If it has UI, it ships a matching npm package (planned). Feature modules are
released in lockstep with `core` (same version number). Each one states the
`core` it requires in its `go.mod`; the example's is updated by the release
script, and in this repository it builds against `../../core` through a
`replace`.

## Versions and compatibility

**Planned: IMM-2070.**

- Go modules are tagged per module, all with the release's version:
  `core/v0.3.0`, `features/example/v0.3.0`.
- The UI package is released as the GitHub release `ui/v0.3.0` (available
  now), with the same number as core.
- The template is tagged `v0.3.0`.
- Releases so far: `v0.2.0` and `v0.2.1`; `v0.3.0` adds the first sign-in
  options above, an additive change to `app.Options`, `server.Auth`,
  `server.CasbinAuthorizer` and `db`; `v0.4.0` adds `app.Options.Run`,
  `app.RunDeps` and `app.Deps.Pool`, additive changes to `app`. An additive change to exported API is a
  minor bump while we are before `v1`, a fix with no API change is a patch.
  The release notes of the GitHub release `ui/vX.Y.Z` say what changed.
- All follow semver. A breaking change to an exported Go API, the `Install`
  contract or a package's exports is a major bump. Before `v1`, a minor bump may
  break, and the changelog says how.

## Access

go-web-base is a public repository, so consumers, CI and image builds need no
GitHub credential: the Go proxy serves the modules and the UI package is a
public release download.

## Working on the base and a project together

**Available now.** To try an unreleased change to go-web-base in your project
without publishing it, check both repositories out side by side. The paths
below assume `~/src/go-web-base` and a project in `~/src/my-app`; use your own.

**Go.** From the project root, add a workspace that uses the local core (and
each feature module you have enabled):

```sh
cd ~/src/my-app
go work init . ~/src/go-web-base/core
go work use ~/src/go-web-base/features/example   # only if feature_example is on
go build ./... && go test ./...
```

The workspace replaces the version in `go.mod`, so edits under
`go-web-base/core` take effect on the next build with nothing to publish.
`go list -m github.com/andrewmyhre/go-web-base/core` prints no version while
it is active. Tried on a project rendered from this branch: with the pinned
core release published, `go work init . <path>/core` built with no other
change. While the release a project pins does not exist yet (a fresh template
before its first release), Go cannot read that version's `go.mod` and the
build fails, so cut the release first. The workspace also puts a `go.work` in
the project; the template's `.gitignore` and `.dockerignore` list `go.work` and
`go.work.sum`, so neither is committed nor sent to an image build. To undo it,
delete both files: `rm go.work go.work.sum`.

**UI.** Build the package, then link it into the project's UI:

```sh
cd ~/src/go-web-base/packages/web-base-ui
pnpm install --frozen-lockfile && pnpm build
cd ~/src/my-app/ui
pnpm pkg set "dependencies.@andrewmyhre/web-base-ui=link:$HOME/src/go-web-base/packages/web-base-ui"
pnpm install --no-frozen-lockfile
pnpm check && pnpm build
```

`link:` symlinks the directory, so after each change run `pnpm build` in the
package again and the project picks the new `dist/` up. Tried: `pnpm check` and
`pnpm build` passed in the project, and an edit to `AccountMenu.svelte`,
rebuilt in the package, showed up in the project's next `pnpm build` with no
reinstall. A packed tarball also works, with `pnpm pack` in the package and
`file:<path>/andrewmyhre-web-base-ui-0.0.0.tgz` as the dependency, which is
what the template's CI does. It installs a copy, though, so every change needs
a new pack and install. Prefer `link:` for day to day work.

**Undo before you commit.** `pnpm pkg set` and the install rewrite
`ui/package.json` and `ui/pnpm-lock.yaml`. Restore them, then reinstall:

```sh
cd ~/src/my-app
git checkout ui/package.json ui/pnpm-lock.yaml
pnpm --dir ui install --frozen-lockfile
rm -f go.work go.work.sum
```

**How CI catches a forgotten one.** A committed `link:` dependency does not
match what the pinned lockfile expects, and a CI install is frozen, so
`pnpm install --frozen-lockfile` fails (tried: it reported that the lockfile
specifiers do not match `package.json`). A `go.work` is ignored by git, so it
cannot be committed by accident; if one is force-added, its paths do not exist
on the CI machine, so `go build` should fail there (not tried). Either way the pipeline builds
against published versions only.

## Where to look next

- [README](../README.md): template questions, the setup checklist, and how the
  template and its CI work.
- Epic [IMM-2066](https://linear.app/immutablesoftware/issue/IMM-2066): the
  plan and the order of the work.
