package example_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/andrewmyhre/go-web-base/core/app"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/redis"
	"github.com/andrewmyhre/go-web-base/core/server"
	"github.com/andrewmyhre/go-web-base/features/example"
)

// The app under test names the role the feature grants to: Greeter has no
// permission of its own, so it holds only what the feature grants it.
// Outsider holds an unrelated permission, and Admin inherits both.
func options(f app.Feature) app.Options {
	return app.Options{
		Name: "testex", Version: "0.0.1", EnvPrefix: "TESTEX", CookiePrefix: "testex",
		Resources: []server.ResourceDef{{Name: "Things"}},
		Roles: []db.RoleDef{
			{Name: "Greeter"},
			{Name: "Outsider", Permissions: []string{"Things: Read"}},
			{Name: db.AdminRole, Inherits: []string{"Greeter", "Outsider"}},
		},
		Features: []app.Feature{f},
	}
}

func TestOpenAPIDocumentsTheRoute(t *testing.T) {
	spec, err := app.OpenAPISpec(options(example.New("Greeter")))
	if err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string   `json:"operationId"`
			Tags        []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	op := doc.Paths[example.HelloPath]["get"]
	if op.OperationID != "get-example-hello" || len(op.Tags) != 1 || op.Tags[0] != "example" {
		t.Errorf("GET %s in the spec = %+v, want get-example-hello tagged example", example.HelloPath, op)
	}

	// The openapi command prints the same document.
	var out bytes.Buffer
	root := app.NewRootCmd(options(example.New("Greeter")))
	root.SetOut(&out)
	root.SetArgs([]string{"openapi"})
	if err := root.Execute(); err != nil {
		t.Fatalf("openapi command: %v", err)
	}
	if !bytes.Equal(out.Bytes(), spec) {
		t.Error("the openapi command prints a different document than OpenAPISpec")
	}
}

func TestFeatureFlagAndHelp(t *testing.T) {
	root := app.NewRootCmd(options(example.New("Greeter")))
	serve, _, err := root.Find([]string{"serve"})
	if err != nil {
		t.Fatal(err)
	}
	fl := serve.Flags().Lookup("example-greeting")
	if fl == nil {
		t.Fatal("serve has no --example-greeting flag")
	}
	if fl.DefValue != example.DefaultGreeting || !strings.Contains(fl.Usage, "TESTEX_EXAMPLE_GREETING") {
		t.Errorf("flag default %q usage %q, want the default greeting and the env var", fl.DefValue, fl.Usage)
	}
}

func TestGrantToUnknownRoleFailsStartup(t *testing.T) {
	_, err := app.OpenAPISpec(options(example.New("Ghost")))
	if err == nil || !strings.Contains(err.Error(), "Ghost") {
		t.Fatalf("err = %v, want a refusal naming the role Ghost", err)
	}
}

// TestServesTheRouteByRole runs the real serve command against Postgres and
// Redis: the migration is applied, and GET /example/hello is allowed for a
// user bound to the granted role (and to Admin, which inherits it), denied
// for one bound to another role, and refused without a session.
func TestServesTheRouteByRole(t *testing.T) {
	for name, tc := range map[string]struct {
		args []string
		env  string
		want string
	}{
		"default": {want: example.DefaultGreeting},
		"flag":    {args: []string{"--example-greeting=Howdy"}, want: "Howdy"},
		"env":     {env: "From env", want: "From env"},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv("TESTEX_EXAMPLE_GREETING", tc.env)
			}
			base, conn := startApp(t, tc.args...)

			if !tableExists(t, conn, "example_greetings") || !tableExists(t, conn, "goose_example_version") {
				t.Error("the feature's table or its goose version table is missing: its migration was not applied")
			}
			if !tableExists(t, conn, "goose_db_version") || !tableExists(t, conn, "casbin_rule") {
				t.Error("core's migration is missing")
			}

			for _, c := range []struct {
				who    string
				status int
			}{
				{"alice", http.StatusOK},         // Greeter
				{"admin-sub", http.StatusOK},     // Admin inherits Greeter
				{"bob", http.StatusForbidden},    // Outsider
				{"", http.StatusUnauthorized},    // no session
				{"nobody", http.StatusForbidden}, // a session but no binding
			} {
				status, body := get(t, base+example.HelloPath, sessionCookie(t, c.who))
				if status != c.status {
					t.Errorf("%q: GET %s = %d %s, want %d", c.who, example.HelloPath, status, body, c.status)
				}
				if c.status == http.StatusOK && !strings.Contains(body, `"greeting":"`+tc.want+`"`) {
					t.Errorf("%q: body %s, want greeting %q", c.who, body, tc.want)
				}
			}
		})
	}
}

// --- service plumbing, following core's CORE_TEST_* convention ---

var sessions *redis.Store

func requireServices(t *testing.T) (dsn, redisAddr string) {
	t.Helper()
	if os.Getenv("CORE_SKIP_SERVICE_TESTS") == "1" {
		t.Skip("CORE_SKIP_SERVICE_TESTS=1: skipping a test that needs the test Postgres and Redis")
	}
	dsn = os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("CORE_TEST_DATABASE_URL is not set; see features/example/README.md")
	}
	redisAddr = os.Getenv("CORE_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("CORE_TEST_REDIS_ADDR is not set; see features/example/README.md")
	}
	return dsn, redisAddr
}

// startApp migrates a fresh schema far enough to seed role bindings, then
// runs `testex serve` over it and returns its base URL and a pool on the
// schema. The schema is dropped and the server stopped when the test ends.
func startApp(t *testing.T, extraArgs ...string) (string, *pgx.Conn) {
	t.Helper()
	ctx := context.Background()
	dsn, redisAddr := requireServices(t)

	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		t.Fatal(err)
	}
	schema := "t_" + hex.EncodeToString(buf)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		_ = admin.Close(context.Background())
	})
	schemaDSN := dsn + "&search_path=" + schema
	if !strings.Contains(dsn, "?") {
		schemaDSN = dsn + "?search_path=" + schema
	}

	// Seed bindings the way an admin tool would, before the server loads
	// its policy: core's migration creates the table.
	if err := db.Migrate(ctx, schemaDSN); err != nil {
		t.Fatalf("migrate core: %v", err)
	}
	conn, err := pgx.Connect(ctx, schemaDSN)
	if err != nil {
		t.Fatalf("connect to schema: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	for sub, role := range map[string]string{"alice": "Greeter", "bob": "Outsider"} {
		if _, err := conn.Exec(ctx, "INSERT INTO casbin_rule (ptype, v0, v1, v2) VALUES ('g', $1, $2, $3)", sub, role, db.Global); err != nil {
			t.Fatalf("bind %s: %v", sub, err)
		}
	}
	if tableExists(t, conn, "example_greetings") {
		t.Fatal("the feature's table exists before the app started")
	}

	// Sessions for the users, in the Redis the app will use.
	client, err := redis.NewClient(ctx, redis.Config{Addr: redisAddr, Password: os.Getenv("CORE_TEST_REDIS_PASSWORD")})
	if err != nil {
		t.Fatalf("redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	sessions = redis.NewStore(client, "testex:session:")

	addr := freeAddr(t)
	runCtx, cancel := context.WithCancel(ctx)
	root := app.NewRootCmd(options(example.New("Greeter")))
	root.SilenceErrors = true
	args := []string{"serve", "--addr", addr, "--database-dsn", schemaDSN, "--redis-addr", redisAddr, "--admin-sub", "admin-sub", "--log-level", "error"}
	root.SetArgs(append(args, extraArgs...))
	done := make(chan error, 1)
	go func() { done <- root.ExecuteContext(runCtx) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("serve returned %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("serve did not stop")
		}
	})

	base := "http://" + addr
	deadline := time.Now().Add(15 * time.Second)
	for {
		select {
		case err := <-done:
			t.Fatalf("serve exited early: %v", err)
		default:
		}
		resp, err := http.Get(base + "/health")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become healthy: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	return base, conn
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

func tableExists(t *testing.T, conn *pgx.Conn, table string) bool {
	t.Helper()
	var ok bool
	// to_regclass resolves against the connection's search_path.
	if err := conn.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok); err != nil {
		t.Fatalf("look up %s: %v", table, err)
	}
	return ok
}

// sessionCookie signs who in directly in Redis and returns the cookie, or
// nil for "" (no session).
func sessionCookie(t *testing.T, who string) *http.Cookie {
	t.Helper()
	if who == "" {
		return nil
	}
	id, err := sessions.Create(context.Background(), time.Hour, map[string]string{"sub": who, "name": who})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return &http.Cookie{Name: server.NewNames("testex", "testex").SessionCookie, Value: id}
}

func get(t *testing.T, url string, c *http.Cookie) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c != nil {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}
