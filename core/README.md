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
| `app` | The program: cobra root with `serve` and `openapi`, config loading, Redis and Postgres connections, migrations, the Casbin enforcer and admin bootstrap, the built-in routes, serving. |
| `server` | chi and Huma router. Every route goes through `server.Register` with a declared permission (`server.Public`, `server.Authenticated` or a role permission); an unchecked route fails `Build`. Google OIDC sign-in, sessions, logout, `/me`. |
| `config` | Viper configuration and flags. The env prefix is an argument. |
| `db` | pgx pool, goose migrations, sqlc queries, the Casbin adapter and the role table (`db.Roles()`). |
| `redis` | go-redis client, health check and the session store. |
| `logger`, `logging` | zap logger with a `service` field, and the typed log helpers behind the log scopes. |

## The `app.Options` contract

```go
package main

import (
	"github.com/andrewmyhre/go-web-base/core/app"
	"github.com/andrewmyhre/go-web-base/core/server"
)

func main() {
	app.Main(app.Options{
		Name:         "my-app",  // slug
		Version:      "0.1.0",
		EnvPrefix:    "MY_APP",
		CookiePrefix: "my_app",
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
| `Routes` | no | The app's own routes, registered after the built-ins. It gets `app.Deps` (session store, authorizer, role reader, auth, logger). Every route must go through `server.Register` or `server.RegisterHandler` with a permission, or `Build` refuses it. |

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

### Roles

The role graph is fixed in core for now. `db.Roles()` returns it as
`[]db.RoleDef{Name, Inherits, Permissions}` where each permission is a
`"Resource: Verb"` string, so a project can test its own `docs/roles.md`
against it.

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
