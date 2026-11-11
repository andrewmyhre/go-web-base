# Building on go-web-base

This is for anyone, human or agent, who starts or maintains a project built on
go-web-base. It explains what your project gets from the base, how
improvements reach it, how to turn optional features on, and what belongs in
your project versus in the base.

## Status

The model below is the target design, tracked in epic
[IMM-2066](https://linear.app/immutablesoftware/issue/IMM-2066). Today, at
template `v0.1.1`, only the Copier half exists: a generated project carries
its own copy of every package, and `copier update` is the only way to pick up
changes. Each section says which parts are **available now** and which are
**planned**, with the issue that delivers them.

## The model

go-web-base reaches your project through two channels, each for a different
kind of content.

| | Versioned packages | Copier template |
|---|---|---|
| Carries | shared code: server, sign-in, RBAC, logging, Redis, Postgres, config, features, reusable UI | your project's own files: `main.go` wiring, config values, deploy manifests, CI, docs |
| Lives in your repo as | a version number in `go.mod` and `ui/package.json` | real files you own and edit |
| You update with | a Dependabot PR, or `go get -u` and `pnpm update` | `uvx copier@9.19.1 update` |
| Conflicts | none: you never edit package code | a three-way merge with your edits |

The rule behind the split: **code you would never edit is a package; code you
are meant to edit is a template file.** A fix to sign-in then reaches every
project as a version bump, with no merge. Your route table and manifests stay
yours, and the template only changes them when its glue changes.

The Go modules are:

- `github.com/andrewmyhre/go-web-base/core`: the server, sign-in, sessions,
  RBAC, logging, Redis and Postgres access, and config.
- One module per optional feature, such as `.../agent`.

The UI packages, on GitHub Packages, are `@andrewmyhre/web-base-ui` plus one
per feature that has UI, such as `@andrewmyhre/web-base-agent-ui`.
(**Planned:** IMM-2067, IMM-2068, IMM-2076, IMM-2077.)

## Starting a project

**Available now.**

```sh
uvx copier@9.19.1 copy gh:andrewmyhre/go-web-base my-app
```

Copier asks for the slug, names, Go module path, env prefix, dev host,
namespaces, registry, two free test NodePorts and the admin's Google sub.
Then work through the per-project setup checklist in the
[README](../README.md#per-project-setup-checklist): nothing works on the
cluster until it is done.

Commit `.copier-answers.yml`. It records your answers and the template version,
and `copier update` needs it.

## Keeping up to date

**Package updates (planned: IMM-2078).** Dependabot opens one grouped PR when
go-web-base publishes new module or package versions. Your CI runs on it like
any other PR. Read the changelog for a major version before you merge: a major
bump means a breaking change you may have to adapt to.

**Template updates (available now).** When the template's glue changes, run
from a clean working tree:

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

Until IMM-2068 lands, the base's code is copied into your project, so a fix
made in your copy does not reach go-web-base or the other projects. Make the
fix in go-web-base, then pick it up with `copier update`.

## Roles and permissions

**Available now:** your role graph is `docs/roles.md`, and `TestRolesMatchPlan`
fails until the table in `db/rbac.go` agrees with it. Edit both together.

**Planned: IMM-2071.** Your app registers its role graph with core at startup.
Features add their own resources and grant them to your roles. Every route
still declares a permission through `server.Register`, and an unguarded route
still fails at startup.

## Writing a feature

**Planned: IMM-2074, with an `example` feature module to copy.** A feature is a
Go module that exposes one entry point:

```go
func Install(app *core.App) error
```

Through `app` it can contribute:

- routes, registered with `server.Register` so each declares a permission;
- resources, and grants of them to roles;
- goose migrations, embedded in the module with their own version table
  (IMM-2072);
- a config section, bound to env vars under the project's prefix, such as
  `PREFIX_AGENT_*` (IMM-2073);
- a log scope.

If it has UI, it ships a matching npm package. It also gets a Copier question
and the glue that question renders. Feature modules are versioned
independently of `core`. Each one states the range of `core` versions it
supports.

## Versions and compatibility

**Planned: IMM-2070.**

- Go modules are tagged per module: `core/v0.3.0`, `agent/v0.1.0`.
- UI packages are tagged `ui/v0.3.0`.
- The template is tagged `v0.3.0`.
- All follow semver. A breaking change to an exported Go API, the `Install`
  contract or a package's exports is a major bump. Before `v1`, a minor bump may
  break, and the changelog says how.

## Private access

**Planned: IMM-2069.** The modules and packages are private, so every machine
that builds a project needs access:

- Go: `GOPRIVATE=github.com/andrewmyhre/*`, plus a `.netrc` for `github.com`.
- npm: an `.npmrc` that maps `@andrewmyhre` to GitHub Packages with a token.
- In Woodpecker, the generated pipeline mints a short-lived kit-pai-app token
  in the step and writes both files from it. Never store the token.

## Working on the base and a project together

**Planned: IMM-2079.** To try an unreleased change to go-web-base in your
project without publishing it:

```sh
go work init . ../go-web-base/core
# ui/package.json: "@andrewmyhre/web-base-ui": "link:../../go-web-base/ui"
```

Remove the `go.work` file and the `link:` before you commit. CI must build
against published versions.

## Where to look next

- [README](../README.md): template questions, the setup checklist, and how the
  template and its CI work.
- Epic [IMM-2066](https://linear.app/immutablesoftware/issue/IMM-2066): the
  plan and the order of the work.
