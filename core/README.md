# core

The shared server code of go-web-base as one Go module,
`github.com/andrewmyhre/go-web-base/core`. A project's `main` calls
`app.Main` with its own values instead of carrying a copy of this code.

Core holds no project-specific value. The app supplies the env prefix, the
service name, the cookie names, the Redis key prefixes, the sign-in refusal
text and the OpenAPI title through `app.Options`.

## Packages

| Package | What it is |
|---|---|
| `app` | The program: cobra root with `serve` and `openapi`, config loading, Redis and Postgres connections, migrations, the Casbin enforcer and admin bootstrap, the built-in routes, serving. Also the feature contract (`app.Feature`, `app.Installer`). |
| `server` | chi and Huma router. Every route goes through `server.Register` with a declared permission (`server.Public`, `server.Authenticated` or a permission on a registered resource); an unchecked route fails `Build`. The resource registry (`server.Resources`). Google OIDC sign-in, sessions, logout, `/me`. |
| `config` | Viper configuration and flags. The env prefix is an argument. |
| `db` | pgx pool, per-module goose migrations, sqlc queries, the Casbin adapter, and the role graph type (`db.RoleDef`) with its validation. Core ships no roles. |
| `redis` | go-redis client, health check and the session store. |
| `logger`, `logging` | zap logger with a `service` field, and the typed log helpers behind the log scopes. |

## The `app.Options` contract

```go
package main

import (
	"github.com/andrewmyhre/go-web-base/core/app"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

func main() {
	app.Main(app.Options{
		Name:         "my-app",  // slug
		Version:      "0.1.0",
		EnvPrefix:    "MY_APP",
		CookiePrefix: "my_app",
		Resources:    []server.ResourceDef{{Name: "Notes"}},
		Roles: []db.RoleDef{
			{Name: db.AdminRole, Permissions: []string{"Notes: Read"}},
		},
		Features: nil, // e.g. []app.Feature{example.New("Admin")}
		Routes: func(r *server.Router, d app.Deps) error {
			// the app's own routes, through server.Register
			return nil
		},
	})
}
```

| Field | Required | Used for |
|---|---|---|
| `Name` | yes | Binary name in help, the `service` field on every log line, the OpenAPI title, and the refusal "you cannot sign in to `<Name>`". |
| `Version` | no (`0.0.0`) | The version in the OpenAPI document. |
| `EnvPrefix` | yes | Every env var: `MY_APP_ADDR`, `MY_APP_DATABASE_DSN`, and so on. It is also named in flag help and in startup errors. |
| `CookiePrefix` | yes | Cookies `<prefix>_session` and `<prefix>_oauth_state`, and Redis keys `<prefix>:session:` and `<prefix>:signin:`. Use a cookie-safe token. |
| `Roles` | yes | The role graph, `[]db.RoleDef{Name, Inherits, Permissions}` with permissions as `"Resource: Verb"`. Core ships none. It must define `db.AdminRole` (`"Admin"`), which `<PREFIX>_ADMIN_SUB` is bound to. See below. |
| `Resources` | no | The app's resources, `[]server.ResourceDef{Name, Verbs}`. Every permission in `Roles` and on a route must name a registered resource. |
| `Migrations` | no | The app's own goose migrations, an `fs.FS` with `*.sql` at its root. Run last, in `goose_app_version`. |
| `Features` | no | Optional modules, `[]app.Feature`, installed in order. See below. |
| `Routes` | no | The app's own routes, registered after the built-ins and the features' routes. It gets `app.Deps` (session store, authorizer, role reader, auth, logger). Every route must go through `server.Register` or `server.RegisterHandler` with a permission, or `Build` refuses it. |

Also exported: `app.NewRootCmd(o)` for a custom CLI, and
`app.OpenAPISpec(o)` for the exact bytes `my-app openapi` prints. An app
tests that its committed `ui/openapi.json` equals `OpenAPISpec(o)`.

`Routes` also runs for `openapi`, with stand-in stores, so it must only
register routes.

### Cookie names and the OpenAPI document

The logout operation takes no cookie parameter in its Go input type, because
a Huma `cookie:"..."` struct tag is fixed at compile time. The session
middleware, which already knows the configured cookie name, passes the
presented session id to `Logout` through the request context. The document
is unchanged: that cookie was always hidden from it.

### Roles and resources

Core ships no roles and no resources. The only permissions core's own routes
use are `server.Public` and `server.Authenticated`, which need no role.
Everything else is registered by the app and its features at startup:

- A **resource** is a name (letters and digits) with the verbs `Read`,
  `Create`, `Patch` and `Delete`, plus any extra verbs it lists
  (`server.ResourceDef{Name: "Workflows", Verbs: []server.Verb{"Run"}}`). The
  app registers its own through `Options.Resources` and a feature through
  `Installer.Resource`. The registry belongs to the router
  (`Router.Resources()`); there is no package-level list.
- A **permission** is a verb on a registered resource: `server.Permission{Resource, Verb}`,
  written `"Resource: Verb"` in a role.
- The **role graph** is `Options.Roles`. A feature adds permissions to a role
  with `Installer.Grant(role, perms...)`, and a role grants whatever its
  inheritors inherit. `db.ValidateRoles` refuses a graph with no `Admin`, a
  duplicate or unnamed role, a parent that does not exist, a cycle, or a role
  that grants nothing.

Two checks keep the old guarantees. At startup, before anything connects, every
permission in the assembled graph must name a registered resource and a verb
that applies to it, or the app fails with the role and permission. And
`Router.Build` re-checks every route: a permission naming an unregistered
resource or verb, or none at all, cannot be served. A route may be registered
before the resource it names, so long as the resource exists by `Build`.

The admin bootstrap and sign-in are unchanged: `<PREFIX>_ADMIN_SUB` is bound
to `Admin` in the global domain, and a user with no role binding cannot sign in.

### Migrations

Core, each feature and the app ship goose migrations as an `fs.FS`, each with
its own version table, so two modules can both have a version 1:

| Module | Version table | Order |
|---|---|---|
| core | `goose_db_version` | first |
| feature `<name>` | `goose_<name>_version` | in `Options.Features` order |
| app | `goose_app_version` | last |

Core keeps goose's default table, `goose_db_version`, which is the table every
database migrated by an earlier core already uses. Those databases are recorded
as migrated and nothing is re-run; a feature's or the app's table is created
the first time it has migrations. `db.Migrate(ctx, dsn, mods...)` applies core
and then `mods` (`db.Migration{Name, Table, FS}`); the app runs it at startup
when a database is configured. A failing module stops the run and the error
names it. "app" and "core" are not valid feature names.

### Features

A feature is a Go module that implements:

```go
type Feature interface {
	Name() string                 // lower case id: log scope, migration table, config prefix
	Install(i *Installer) error   // declare what the feature contributes
}
```

`Install` runs once, when `app.NewRootCmd` builds the command tree, before any
flag is parsed (and again for each `app.OpenAPISpec`). That is why it only
declares: settings become flags of `serve`, and flags must exist before cobra
parses. The `Installer` offers:

| Method | Contributes |
|---|---|
| `Resource(name, verbs...)` | a resource for routes and grants |
| `Grant(role, perms...)` | permissions added to a role of the app's graph |
| `Migrations(fsys)` | goose migrations, in the feature's own version table |
| `Routes(fn)` | routes; `fn(r, d)` runs when the router is built, with settings resolved |
| `String`, `Int`, `Bool`, `Secret` | settings |
| `Settings()` | the reader for those settings |
| `Log(d)` | a logger with `scope=<name>` |

Work that needs a resolved setting, a store or the router goes in the `Routes`
function. Routes still go through `server.Register`, so each declares a
permission and an unguarded one fails `Build`. The feature's routes register
after core's built-ins and before the app's `Routes`, and they are in the
OpenAPI document. `features/example` is a complete worked example.

### Config

A feature declares a setting with a default and help; `String("greeting", "Hello", "...")` in
feature `example` becomes the flag `--example-greeting` on `serve` and the
variable `<PREFIX>_EXAMPLE_GREETING`. A flag wins over the environment, which
wins over the default, as for core's own settings. `Secret(key)` declares a
setting read from the environment only, with no flag, like the Google client
secret. A feature reads values through `Settings()` (`String`, `Int`, `Bool`)
inside its `Routes` function. A key that would collide with another setting
(feature `redis`, key `addr` is core's `redis_addr`) is refused, and reading a
key the feature never declared panics.

Both `serve` and `openapi` run `Install`; if it fails, both print the error.

## Tests

From `core/`:

```sh
gofmt -l .
go vet ./...
```

Without services (what a laptop usually has): the Postgres and Redis tests
skip, everything else runs.

```sh
CORE_SKIP_SERVICE_TESTS=1 go test ./...
```

With services: start Postgres and Redis, then point the tests at them.
Without `CORE_SKIP_SERVICE_TESTS=1`, an unset variable fails the test
rather than skipping it.

```sh
docker run -d --rm --name core-pg -e POSTGRES_PASSWORD=ci -e POSTGRES_DB=core_test -p 127.0.0.1:25432:5432 postgres:18.6-alpine
docker run -d --rm --name core-redis -p 127.0.0.1:26379:6379 redis:8.10-alpine

CORE_TEST_DATABASE_URL='postgres://postgres:ci@127.0.0.1:25432/core_test?sslmode=disable' \
CORE_TEST_REDIS_ADDR=127.0.0.1:26379 \
go test ./...
```

| Variable | Meaning |
|---|---|
| `CORE_TEST_DATABASE_URL` | Postgres DSN. Each test creates and drops its own schema. |
| `CORE_TEST_REDIS_ADDR` | Redis `host:port`. Keys are random per test. |
| `CORE_TEST_REDIS_PASSWORD` | Redis password, if any. |
| `CORE_SKIP_SERVICE_TESTS` | `1` skips the tests that need the two services. Anything else does not. |

CI runs the `core` step in the root `.woodpecker.yml` against Woodpecker
`services:` (`postgres:18.6-alpine`, `redis:8.10-alpine`), with no skip
switch set.

## sqlc

`sqlc.yaml` generates `db/sqlc` from `db/migrations` and `db/query`. Run
`sqlc generate` in `core/` after changing either.
