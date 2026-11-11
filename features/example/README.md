# example

The smallest feature module that uses every hook of core's feature contract
(`app.Feature`). Copy it to start a real feature. It is its own Go module,
`github.com/andrewmyhre/go-web-base/features/example`, requiring `core v0.2.0`;
this repository's `go.mod` replaces core with `../../core` so it builds against
the core in the same commit.

## What it adds

| Hook | In this feature |
|---|---|
| Route | `GET /example/hello`, returning `{"greeting": "..."}`. Tag `example`, operation id `get-example-hello`. |
| Resource and grant | Registers `ExampleGreetings` and grants `ExampleGreetings: Read` to the role the app names in `example.New(role)`. The route needs that permission. |
| Migration | `migrations/00001_create_example_greetings.sql` creates `example_greetings`. Recorded in `goose_example_version`. |
| Setting | `greeting`, default `Hello from the example feature`. Flag `--example-greeting`, env `<PREFIX>_EXAMPLE_GREETING`. |
| Log scope | `scope=example` on its log lines (`Installer.Log`). |

## Install it

In the app's `main.go`, name a role of the app's role graph:

```go
import "github.com/andrewmyhre/go-web-base/features/example"

app.Options{
	Roles:    roles, // must define "Contributor"
	Features: []app.Feature{example.New("Contributor")},
	// ...
}
```

Then regenerate the app's `ui/openapi.json`, since the route is in the document.
Every role that inherits `Contributor` can call the route. A role the app does
not define makes startup fail with the role named. The grant is not in the
app's `docs/roles.md`, because it comes from the feature.

## How it is written

`example.go` is the whole feature. `Install` declares the resource, grant,
migrations and setting. The route is registered inside the function passed to
`i.Routes`, which core calls when it builds the router, after the settings are
resolved. That split is the contract: `Install` runs before flags are parsed, so
it declares; `Routes` runs at startup, so it reads.

## Tests

From `features/example/`:

```sh
gofmt -l .
go vet ./...
CORE_SKIP_SERVICE_TESTS=1 go test ./...
```

The OpenAPI and flag tests need no services. `TestServesTheRouteByRole` runs the
real `serve` command against Postgres and Redis, with sessions created in
Redis, and checks that the migration ran, the route is allowed for the granted
role (and for `Admin`, which inherits it) and denied for another role, and the
three ways of setting the greeting (default, flag, env). It uses the same
variables as core, and fails rather than skips when they are unset:

| Variable | Meaning |
|---|---|
| `CORE_TEST_DATABASE_URL` | Postgres DSN. Each test creates and drops its own schema. |
| `CORE_TEST_REDIS_ADDR` | Redis `host:port`. |
| `CORE_TEST_REDIS_PASSWORD` | Redis password, if any. |
| `CORE_SKIP_SERVICE_TESTS` | `1` skips the tests that need the two services. |

See [core/README.md](../../core/README.md#tests) for starting the services. The
root `.woodpecker.yml` `core` step runs these tests against the same services.
