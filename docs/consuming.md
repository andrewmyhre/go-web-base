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
`@andrewmyhre/web-base-ui` package. Dependabot and the feature UI packages are
not done yet. Projects generated at `v0.1.x` still carry
their own copies; `copier update` moves them onto core. Each section says which parts are **available now** and which are
**planned**, with the issue that delivers them.

## The model

go-web-base reaches your project through two channels, each for a different
kind of content.

| | Versioned packages | Copier template |
|---|---|---|
| Carries | shared code: server, sign-in, RBAC, logging, Redis, Postgres, config, features, reusable UI | your project's own files: `main.go` wiring, config values, deploy manifests, CI, docs |
| Lives in your repo as | a version in `go.mod`, and a release URL in `ui/package.json` | real files you own and edit |
| You update with | `uvx copier@9.19.1 update` for the UI package; `go get -u` or (later) a Dependabot PR for Go | `uvx copier@9.19.1 update` |
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
and the template's `ui/package.json` depends on that file's URL, so it needs no
token. Because the URL is pinned by the template, UI updates arrive through
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
(or `copier update`, which also moves the pin). **Planned (IMM-2078):**
Dependabot opens a grouped PR for new Go module versions. Your CI runs on it
like any other PR; read the changelog for a major version before you merge.

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

**Planned: IMM-2075.** Each optional feature is a boolean Copier question. To
turn one on in an existing project:

```sh
uvx copier@9.19.1 update -d agent_client=true
```

That renders only the glue: the import and `Install` call in `main.go`, the
feature's config and env vars in `deploy/dev/api.yaml`, and any UI wiring. The
feature's code arrives as its module and package. A new project can answer
the question in `copier copy`.

To turn a feature off, run the same update with `=false`, which should remove
the glue; IMM-2075 verifies that. The feature's database tables are not
dropped, so remove them with a migration of your own if you need to.

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
question that renders that glue is planned (IMM-2075). Its migrations run
after core's and in the order the features are listed, before the project's own.

If it has UI, it ships a matching npm package (planned). Feature modules are
versioned independently of `core`. Each one states the range of `core`
versions it supports; the example requires `core v0.2.0` and, in this
repository, builds against `../../core` through a `replace`.

## Versions and compatibility

**Planned: IMM-2070.**

- Go modules are tagged per module: `core/v0.3.0`, `agent/v0.1.0`.
- The UI package is released as the GitHub release `ui/v0.3.0` (available
  now), with the same number as core.
- The template is tagged `v0.3.0`.
- All follow semver. A breaking change to an exported Go API, the `Install`
  contract or a package's exports is a major bump. Before `v1`, a minor bump may
  break, and the changelog says how.

## Private access

**Planned: IMM-2069.** The modules and packages are private, so every machine
that builds a project needs access:

- Go: `GOPRIVATE=github.com/andrewmyhre/*`, plus a `.netrc` for `github.com`.
- npm: nothing, while go-web-base is public. The UI package is a release
  tarball downloaded by URL. If the repository ever goes private, the URL
  needs an authenticated download and this section is the place to add it.
- In Woodpecker, the generated pipeline mints a short-lived kit-pai-app token
  in the step and writes the `.netrc` from it. Never store the token.

## Working on the base and a project together

**Planned: IMM-2079.** To try an unreleased change to go-web-base in your
project without publishing it:

```sh
go work init . ../go-web-base/core
# ui/package.json: "@andrewmyhre/web-base-ui": "link:../../go-web-base/packages/web-base-ui"
```

Remove the `go.work` file and the `link:` before you commit. CI must build
against published versions.

## Where to look next

- [README](../README.md): template questions, the setup checklist, and how the
  template and its CI work.
- Epic [IMM-2066](https://linear.app/immutablesoftware/issue/IMM-2066): the
  plan and the order of the work.
