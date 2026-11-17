package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// stubGuard decides with fn and records what it was built with.
type stubGuard struct {
	fn func(server.GuardRequest) server.GuardResult
}

func (g stubGuard) Check(_ context.Context, req server.GuardRequest) server.GuardResult {
	return g.fn(req)
}

// hookRouterHandler builds the handler for o with d, as serve does.
func hookRouterHandler(t *testing.T, o Options, d Deps) http.Handler {
	t.Helper()
	router, err := testRouter(t, o, d)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	h, err := server.New(config.Default(), zap.NewNop(), router).Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

func get(h http.Handler, path string, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: testOpts.names().SessionCookie, Value: cookie})
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// TestGuardOptionIsBuiltFromDepsAndDecidesEveryRoute: Options.Guard gets
// the Deps the routes get, and its answer reaches built-in routes too.
func TestGuardOptionIsBuiltFromDepsAndDecidesEveryRoute(t *testing.T) {
	enforcer := mustEnforcer(t)
	store := newGoldenStore()
	var built Deps
	var asked []string
	o := testOpts
	o.Guard = func(d Deps) server.Guard {
		built = d
		return stubGuard{fn: func(req server.GuardRequest) server.GuardResult {
			asked = append(asked, req.Method+" "+req.Path)
			if req.Path == "/me" {
				return server.GuardResult{Status: http.StatusTeapot, Detail: "guard says so"}
			}
			return server.GuardResult{}
		}}
	}
	h := hookRouterHandler(t, o, Deps{Sessions: store, Enforcer: enforcer})
	if built.Sessions != server.SessionStore(store) || built.Enforcer != enforcer {
		t.Errorf("Guard built with %+v, want the Deps the routes get", built)
	}
	if rec := get(h, "/me", goldenSession); rec.Code != http.StatusTeapot {
		t.Errorf("GET /me = %d, want the Guard's 418 on a built-in route", rec.Code)
	}
	if rec := get(h, "/health", ""); rec.Code != http.StatusOK {
		t.Errorf("GET /health = %d, want 200", rec.Code)
	}
	if strings.Join(asked, ",") != "GET /me,GET /health" {
		t.Errorf("Guard asked about %v, want /me and /health", asked)
	}
}

// TestScopedRouteWithoutGuardOptionRefusesToStart: the app cannot ship a
// scoped permission and forget the Guard.
func TestScopedRouteWithoutGuardOptionRefusesToStart(t *testing.T) {
	o := testOpts
	o.Routes = func(r *server.Router, _ Deps) error {
		return server.Register(r, server.Operation{
			OperationID: "scoped", Method: http.MethodGet, Path: "/p/{id}", Summary: "s",
			Permission: server.Permission{Resource: "Projects", Verb: server.Read}.In(server.ScopeProject),
		}, health2)
	}
	router, err := testRouter(t, o, Deps{Sessions: newGoldenStore(), Authorizer: goldenAuthz{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := server.New(config.Default(), zap.NewNop(), router).Handler(); err == nil || !strings.Contains(err.Error(), server.ErrScopedWithoutGuard.Error()) {
		t.Fatalf("Handler with a scoped route and no Guard = %v, want ErrScopedWithoutGuard", err)
	}
	o.Guard = func(Deps) server.Guard {
		return stubGuard{fn: func(server.GuardRequest) server.GuardResult { return server.GuardResult{} }}
	}
	hookRouterHandler(t, o, Deps{Sessions: newGoldenStore(), Authorizer: goldenAuthz{}})
}

func health2(context.Context, *struct{}) (*HealthOutput, error) { return health(nil, nil) }

// TestMiddlewareOptionRunsOnEveryRequest: Options.Middleware wraps the
// built-ins, and sees the session.
func TestMiddlewareOptionRunsOnEveryRequest(t *testing.T) {
	var sawSub string
	o := testOpts
	o.Middleware = []func(http.Handler) http.Handler{func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if s, ok := server.SessionFromContext(r.Context()); ok {
				sawSub = s.UserID
			}
			w.Header().Set("X-Mw", "ran")
			next.ServeHTTP(w, r)
		})
	}}
	h := hookRouterHandler(t, o, Deps{Sessions: newGoldenStore()})
	rec := get(h, "/health", goldenSession)
	if rec.Header().Get("X-Mw") != "ran" || sawSub != goldenSub {
		t.Errorf("middleware ran=%q saw %q, want ran and the session's sub", rec.Header().Get("X-Mw"), sawSub)
	}
}

// TestAPIPrefixMovesMeLogoutAndTheSpec: those three move under the prefix,
// /auth/google, its callback and /health stay at the root.
func TestAPIPrefixMovesMeLogoutAndTheSpec(t *testing.T) {
	o := testOpts
	o.APIPrefix = "/api/v1"
	google, err := server.NewGoogle(server.GoogleConfig{ClientID: "id", ClientSecret: "s", RedirectURL: "https://x.test/auth/google/callback"})
	if err != nil {
		t.Fatal(err)
	}
	store := newGoldenStore()
	d := Deps{Sessions: store, Roles: goldenAuthz{}, Auth: server.NewAuth(google, store, newGoldenStore(), goldenAuthz{}, nil, "s", o.names())}
	h := hookRouterHandler(t, o, d)
	for path, want := range map[string]int{
		"/api/v1/me": http.StatusOK, "/me": http.StatusNotFound,
		"/api/v1/openapi.json": http.StatusOK, "/openapi.json": http.StatusNotFound,
		"/health": http.StatusOK, "/api/v1/health": http.StatusNotFound,
		"/auth/google": http.StatusFound,
	} {
		if rec := get(h, path, goldenSession); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", path, rec.Code, want)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("POST /api/v1/auth/logout = %d, want 204", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /auth/logout = %d, want it gone from the root", rec.Code)
	}
}

// TestAPIPrefixReachesTheOpenAPICommand: OpenAPISpec renders the document
// of a prefixed app, with the prefixed paths.
func TestAPIPrefixReachesTheOpenAPICommand(t *testing.T) {
	o := testOpts
	o.APIPrefix = "/api/v1"
	spec, err := OpenAPISpec(o)
	if err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	s := string(spec)
	if !strings.Contains(s, `"/api/v1/me"`) || !strings.Contains(s, `"/api/v1/auth/logout"`) || !strings.Contains(s, `"/health"`) || strings.Contains(s, `"/me"`) {
		t.Errorf("spec paths are not prefixed as specified:\n%s", s)
	}
}

// TestSkipBuiltinsLetsTheAppServeMe: with "me" skipped core registers no
// /me, so the app's own can take its place.
func TestSkipBuiltinsLetsTheAppServeMe(t *testing.T) {
	o := testOpts
	o.APIPrefix = "/api/v1"
	o.SkipBuiltins = []string{"me"}
	o.Routes = func(r *server.Router, _ Deps) error {
		return server.Register(r, server.Operation{
			OperationID: "app-me", Method: http.MethodGet, Path: "/api/v1/me", Summary: "mine", Permission: server.Authenticated,
		}, health2)
	}
	spec, err := OpenAPISpec(o)
	if err != nil {
		t.Fatalf("OpenAPISpec with the app's own /me: %v", err)
	}
	if strings.Contains(string(spec), "get-me") || !strings.Contains(string(spec), "app-me") {
		t.Error("the spec still has core's get-me, or lacks the app's")
	}
	o.SkipBuiltins = nil
	if _, err := OpenAPISpec(o); err == nil {
		t.Error("registering a second /me without SkipBuiltins must conflict")
	}
}

// TestNewOptionsAreValidated: each new option refuses what cannot work.
func TestNewOptionsAreValidated(t *testing.T) {
	for name, c := range map[string]struct {
		mutate func(*Options)
		want   string
	}{
		"prefix without slash":   {func(o *Options) { o.APIPrefix = "api" }, "APIPrefix"},
		"prefix with trailing /": {func(o *Options) { o.APIPrefix = "/api/" }, "APIPrefix"},
		"unknown builtin":        {func(o *Options) { o.SkipBuiltins = []string{"health"} }, "SkipBuiltins"},
		"SignIn with default": {func(o *Options) {
			o.SignIn = func(context.Context, server.Identity) error { return nil }
			o.DefaultRole = "R"
		}, "SignIn"},
		"SignIn with first": {func(o *Options) {
			o.SignIn = func(context.Context, server.Identity) error { return nil }
			o.OnFirstSignIn = func(context.Context, server.Identity) (string, error) { return "", nil }
		}, "SignIn"},
		"negative timeout": {func(o *Options) { o.PolicyWriteTimeout = -time.Second }, "PolicyWriteTimeout"},
	} {
		o := testOpts
		o.Roles = append([]db.RoleDef{{Name: "R"}}, o.Roles...)
		c.mutate(&o)
		if _, err := newInstallation(o); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: newInstallation = %v, want an error naming %s", name, err, c.want)
		}
	}
	o := testOpts
	o.APIPrefix, o.SkipBuiltins, o.PolicyWriteTimeout = "/api/v1", []string{"me"}, time.Second
	o.SignIn = func(context.Context, server.Identity) error { return nil }
	if _, err := newInstallation(o); err != nil {
		t.Errorf("valid new options refused: %v", err)
	}
}

// TestRunGetsTheEnforcer: Options.Run sees the enforcer connect opened.
func TestRunGetsTheEnforcer(t *testing.T) {
	enforcer := mustEnforcer(t)
	got := make(chan *casbin.Enforcer, 1)
	o := testOpts
	o.Run = func(ctx context.Context, d RunDeps) error {
		got <- d.Enforcer
		<-ctx.Done()
		return nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- serveWith(ctx, mustInstall(t, o), zap.NewNop(), Deps{Enforcer: enforcer}, blockUntilDone)
	}()
	if e := waitFor(t, got, "Run"); e != enforcer {
		t.Errorf("RunDeps.Enforcer = %p, want %p", e, enforcer)
	}
	cancel()
	waitFor(t, done, "serve")
}

// mustEnforcer is a Casbin enforcer with no adapter, enough to be passed
// around and locked.
func mustEnforcer(t *testing.T) *casbin.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString("[request_definition]\nr = sub, obj\n[policy_definition]\np = sub, obj\n[policy_effect]\ne = some(where (p.eft == allow))\n[matchers]\nm = r.sub == p.sub && r.obj == p.obj\n")
	if err != nil {
		t.Fatal(err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// settingsOptions declares unprefixed settings the way huddle-go does.
func settingsOptions() Options {
	o := testOpts
	o.Settings = func(i *Installer) error {
		if err := i.String("runtime_driver", "docker", "the runtime"); err != nil {
			return err
		}
		if err := i.Int("pool_size", 3, "pool size"); err != nil {
			return err
		}
		if err := i.Bool("lab", false, "lab mode"); err != nil {
			return err
		}
		return i.Secret("api_key")
	}
	return o
}

// TestAppSettingsKeepTheirUnprefixedNames: the variable is
// <EnvPrefix>_<KEY> and the flag --<key>, with no feature name in between.
func TestAppSettingsKeepTheirUnprefixedNames(t *testing.T) {
	t.Setenv("TESTAPP_RUNTIME_DRIVER", "kube")
	t.Setenv("TESTAPP_API_KEY", "s3cret")
	inst := mustInstall(t, settingsOptions())
	if got := inst.v.GetString("runtime_driver"); got != "kube" {
		t.Errorf("runtime_driver = %q, want the env value kube", got)
	}
	if got := inst.v.GetInt("pool_size"); got != 3 {
		t.Errorf("pool_size = %d, want its default 3", got)
	}
	if inst.v.GetString("api_key") != "s3cret" {
		t.Error("the secret was not read from TESTAPP_API_KEY")
	}
	cmd := newServeCmd(settingsOptions(), inst, nil)
	if f := cmd.Flags().Lookup("runtime-driver"); f == nil || !strings.Contains(f.Usage, "TESTAPP_RUNTIME_DRIVER") {
		t.Errorf("flag --runtime-driver = %v, want one naming TESTAPP_RUNTIME_DRIVER", f)
	}
	if cmd.Flags().Lookup("api-key") != nil {
		t.Error("a Secret must have no flag")
	}
	if err := cmd.ParseFlags([]string{"--pool-size=9", "--lab"}); err != nil {
		t.Fatal(err)
	}
	if inst.v.GetInt("pool_size") != 9 || !inst.v.GetBool("lab") {
		t.Errorf("flags did not win: pool_size=%d lab=%v", inst.v.GetInt("pool_size"), inst.v.GetBool("lab"))
	}
}

// TestAppSettingsCannotClash: a key core owns, a duplicate, or a feature's
// prefixed key is refused, and so is Installer.Migrations.
func TestAppSettingsCannotClash(t *testing.T) {
	for name, declare := range map[string]func(i *Installer) error{
		"core key":  func(i *Installer) error { return i.String("redis_addr", "", "") },
		"duplicate": func(i *Installer) error { _ = i.String("x", "", ""); return i.String("x", "", "") },
		"feature":   func(i *Installer) error { return i.String("widgets_color", "", "") },
		"migrations": func(i *Installer) error {
			return i.Migrations(os.DirFS("."))
		},
	} {
		o := testOpts
		o.Features = []Feature{goldenFeature{}}
		o.Settings = declare
		_, err := newInstallation(o)
		if err == nil {
			t.Errorf("%s: newInstallation accepted it", name)
		}
		if name == "migrations" && (err == nil || !strings.Contains(err.Error(), "Options.Migrations")) {
			t.Errorf("migrations on the app installer: %v, want the pointer to Options.Migrations", err)
		}
	}
}

// TestAppSettingsReadThroughTheInstaller: the Settings getters of the app's
// installer read the unprefixed keys.
func TestAppSettingsReadThroughTheInstaller(t *testing.T) {
	t.Setenv("TESTAPP_RUNTIME_DRIVER", "kube")
	var read string
	o := testOpts
	o.Settings = func(i *Installer) error {
		if err := i.String("runtime_driver", "docker", "the runtime"); err != nil {
			return err
		}
		s := i.Settings()
		read = s.String("runtime_driver")
		return nil
	}
	mustInstall(t, o)
	if read != "kube" {
		t.Errorf("Settings().String(runtime_driver) = %q, want kube", read)
	}
}

// serviceSchemaDSN returns a DSN for a fresh schema in the test Postgres.
func serviceSchemaDSN(t *testing.T) string {
	t.Helper()
	if os.Getenv("CORE_SKIP_SERVICE_TESTS") == "1" {
		t.Skip("CORE_SKIP_SERVICE_TESTS=1: skipping a test that needs the test Postgres")
	}
	dsn := os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("CORE_TEST_DATABASE_URL is not set; see core/README.md")
	}
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	schema := "t_" + hex.EncodeToString(buf)
	admin, err := pgx.Connect(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	if _, err := admin.Exec(t.Context(), "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("options", "-csearch_path="+schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// TestConnectWiresTheEnforcerTimeoutAndSignInHooks (service test): connect
// puts the enforcer in Deps, hands PolicyWriteTimeout to its adapter and
// gives the sign-in flow the app's hooks.
func TestConnectWiresTheEnforcerTimeoutAndSignInHooks(t *testing.T) {
	dsn := serviceSchemaDSN(t)
	redisAddr := os.Getenv("CORE_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Fatal("CORE_TEST_REDIS_ADDR is not set; see core/README.md")
	}
	cfg := config.Default()
	cfg.DatabaseDSN = dsn
	cfg.RedisAddr = redisAddr
	cfg.RedisPassword = os.Getenv("CORE_TEST_REDIS_PASSWORD")
	cfg.GoogleClientID, cfg.GoogleClientSecret = "id", "secret"
	cfg.GoogleRedirectURI = "https://x.test/auth/google/callback"

	o := testOpts
	o.SignIn = func(context.Context, server.Identity) error { return nil }
	o.OnSignedIn = func(context.Context, server.Identity) {}
	d, cleanup, err := connect(t.Context(), mustInstall(t, o), cfg, zap.NewNop())
	defer cleanup()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if d.Enforcer == nil || db.LockOf(d.Enforcer) == nil {
		t.Fatal("connect left Deps.Enforcer nil although a database is configured")
	}
	auth := reflect.ValueOf(d.Auth).Elem()
	if auth.FieldByName("signIn").IsNil() || auth.FieldByName("onSignedIn").IsNil() {
		t.Error("connect did not give the sign-in flow the app's SignIn and OnSignedIn")
	}

	plain := testOpts
	d2, cleanup2, err := connect(t.Context(), mustInstall(t, plain), cfg, zap.NewNop())
	defer cleanup2()
	if err != nil {
		t.Fatalf("connect without hooks: %v", err)
	}
	auth2 := reflect.ValueOf(d2.Auth).Elem()
	if !auth2.FieldByName("signIn").IsNil() || !auth2.FieldByName("onSignedIn").IsNil() {
		t.Error("an app with no sign-in hooks got some")
	}

	// A fresh schema again: a 1ns write timeout makes the first policy
	// write (the role definitions) fail, so the option reached the adapter.
	slow := testOpts
	slow.PolicyWriteTimeout = time.Nanosecond
	cfg.DatabaseDSN = serviceSchemaDSN(t)
	_, cleanup3, err := connect(t.Context(), mustInstall(t, slow), cfg, zap.NewNop())
	defer cleanup3()
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("connect with a 1ns PolicyWriteTimeout = %v, want a deadline error", err)
	}
}
