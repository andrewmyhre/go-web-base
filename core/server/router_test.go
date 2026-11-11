package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type okOutput struct {
	Body struct {
		Status string `json:"status"`
	}
}

func okHandler(context.Context, *struct{}) (*okOutput, error) {
	out := &okOutput{}
	out.Body.Status = "ok"
	return out, nil
}

var (
	projectsRead   = Permission{Resource: Projects, Verb: Read}
	projectsCreate = Permission{Resource: Projects, Verb: Create}
)

func op(method, path string, permission Permission) Operation {
	return Operation{
		OperationID: strings.ToLower(method) + strings.ReplaceAll(path, "/", "-"),
		Method:      method,
		Path:        path,
		Permission:  permission,
	}
}

// newTestRouter is a router whose session store holds one session,
// testSessionID for testSub, and whose authorizer allows everything, so
// tests of routing itself can serve non-public routes through serve.
func newTestRouter() *Router {
	return testRouter("test", "0.0.0").WithAuth(newMemStore(), allowAll{}, testNames.SessionCookie)
}

// serve sends a request carrying the test session's cookie.
func serve(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
	h.ServeHTTP(rec, req)
	return rec
}

func mustBuild(t *testing.T, r *Router, log *zap.Logger) http.Handler {
	t.Helper()
	h, err := r.Build(log)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return h
}

// bufferLogger returns a JSON zap logger writing to buf.
func bufferLogger(buf *bytes.Buffer) *zap.Logger {
	return zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(buf), zapcore.DebugLevel))
}

func logEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("log line is not JSON: %v\n%s", err, line)
		}
		out = append(out, entry)
	}
	return out
}

func TestRegisterRefusesRouteWithoutPermission(t *testing.T) {
	router := newTestRouter()
	before := len(router.Routes())
	err := Register(router, op(http.MethodGet, "/open", Permission{}), okHandler)
	if err == nil {
		t.Fatal("Register accepted a route with the zero Permission")
	}
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("Register error = %v, want ErrNoPermission", err)
	}
	if got := len(router.Routes()); got != before {
		t.Fatalf("router holds %d routes after a refused registration, want %d", got, before)
	}
}

func TestBuildRefusesRouteWithoutPermission(t *testing.T) {
	router := newTestRouter()
	// A route that bypassed Register: startup must still refuse it.
	router.regs = append(router.regs, registration{
		method: http.MethodGet,
		path:   "/open",
		mount:  func(huma.API, chi.Router, *guard) {},
	})
	h, err := router.Build(zap.NewNop())
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("Build() error = %v, want ErrNoPermission", err)
	}
	if h != nil {
		t.Fatalf("Build() returned a handler alongside the refusal")
	}
}

func TestBuildRefusesRouteMountedOutsideACheckedRegistration(t *testing.T) {
	router := newTestRouter()
	// A registration whose mount adds a second, undeclared route as well as
	// its own: the walk at the end of Build must catch it.
	if err := router.add(registration{
		method:     http.MethodGet,
		path:       "/declared",
		permission: projectsRead,
		mount: func(_ huma.API, mux chi.Router, _ *guard) {
			mux.Get("/declared", func(http.ResponseWriter, *http.Request) {})
			mux.Get("/smuggled", func(http.ResponseWriter, *http.Request) {})
		},
	}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := router.Build(zap.NewNop()); !errors.Is(err, ErrUncheckedRoute) {
		t.Fatalf("Build() error = %v, want ErrUncheckedRoute", err)
	}
}

// Huma registers OpenAPI, docs and schema routes itself, outside Register.
// They are switched off; if a config change turned them back on, startup
// must refuse rather than serve them unchecked.
func TestBuildRefusesHumaAutomaticRoutes(t *testing.T) {
	defer func(orig func(string, string) huma.Config) { humaConfig = orig }(humaConfig)
	humaConfig = func(title, version string) huma.Config {
		return huma.DefaultConfig(title, version)
	}
	if _, err := newTestRouter().Build(zap.NewNop()); !errors.Is(err, ErrUncheckedRoute) {
		t.Fatalf("Build() error = %v, want ErrUncheckedRoute", err)
	}
}

func TestHumaAutomaticRoutesAreNotServed(t *testing.T) {
	h := mustBuild(t, newTestRouter(), zap.NewNop())
	for _, path := range []string{"/openapi", "/openapi.yaml", "/openapi-3.0.json", "/docs", "/schemas/Thing.json"} {
		if rec := serve(t, h, http.MethodGet, path); rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestBuildServesOperationsWithPermissions(t *testing.T) {
	router := newTestRouter()
	var seen Permission
	var seenOK bool
	if err := Register(router, op(http.MethodGet, "/things", projectsRead),
		func(ctx context.Context, in *struct{}) (*okOutput, error) {
			seen, seenOK = PermissionFromContext(ctx)
			return okHandler(ctx, in)
		}); err != nil {
		t.Fatalf("Register: %v", err)
	}

	rec := serve(t, mustBuild(t, router, zap.NewNop()), http.MethodGet, "/things")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /things = %d, want 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "ok" {
		t.Fatalf("GET /things body = %s, want {\"status\":\"ok\"}", rec.Body.String())
	}
	if _, ok := body["$schema"]; ok {
		t.Fatalf("body links a $schema, but schema routes are not served: %s", rec.Body.String())
	}
	if !seenOK || seen != projectsRead {
		t.Fatalf("handler saw permission %v (ok %v), want %v", seen, seenOK, projectsRead)
	}
}

func TestBuildDoesNotServeUnregisteredMethod(t *testing.T) {
	router := newTestRouter()
	if err := Register(router, op(http.MethodPost, "/things", projectsCreate), okHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h := mustBuild(t, router, zap.NewNop())
	if rec := serve(t, h, http.MethodGet, "/things"); rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET /things = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if rec := serve(t, h, http.MethodGet, "/nowhere"); rec.Code != http.StatusNotFound {
		t.Fatalf("GET /nowhere = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestBuildHandlerDoesNotExposeRouterOrAPI(t *testing.T) {
	h := mustBuild(t, newTestRouter(), zap.NewNop())
	if _, ok := h.(chi.Router); ok {
		t.Fatal("Build returned a chi.Router; callers could add unchecked routes")
	}
	if _, ok := h.(huma.API); ok {
		t.Fatal("Build returned a huma.API; callers could add unchecked operations")
	}
}

func TestRegisterRefusesDuplicateRoute(t *testing.T) {
	router := newTestRouter()
	if err := Register(router, op(http.MethodGet, "/things", projectsRead), okHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := Register(router, op(http.MethodGet, "/things", projectsRead), okHandler); err == nil {
		t.Fatal("Register accepted a duplicate route")
	}
	if err := Register(router, op(http.MethodGet, OpenAPIPath, projectsRead), okHandler); err == nil {
		t.Fatal("Register accepted a route shadowing the OpenAPI spec")
	}
	dupID := op(http.MethodPost, "/other", projectsCreate)
	dupID.OperationID = "get-things"
	if err := Register(router, dupID, okHandler); err == nil {
		t.Fatal("Register accepted a duplicate operation id")
	}
	// The same path under another method is a different route.
	if err := Register(router, op(http.MethodPost, "/things", projectsCreate), okHandler); err != nil {
		t.Fatalf("Register POST /things: %v", err)
	}
}

type idInput struct {
	ID string `path:"id"`
}

func idHandler(context.Context, *idInput) (*okOutput, error) { return &okOutput{}, nil }

func TestRegisterRefusesConflictingPattern(t *testing.T) {
	router := newTestRouter()
	if err := Register(router, op(http.MethodGet, "/things/{id}", projectsRead), idHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}
	other := op(http.MethodGet, "/things/{name}", projectsRead)
	other.OperationID = "get-things-by-name"
	if err := Register(router, other, idHandler); err == nil {
		t.Fatal("Register accepted /things/{name} beside /things/{id}; chi would silently replace one")
	}
}

// chi silently replaces a route whose pattern conflicts with an earlier
// one. A conflict that got past Register must still stop startup.
func TestBuildRefusesRouteReplacedByAConflict(t *testing.T) {
	router := newTestRouter()
	for _, path := range []string{"/things/{id}", "/things/{name}"} {
		path := path
		router.regs = append(router.regs, registration{
			method: http.MethodGet, path: path, permission: projectsRead,
			mount: func(_ huma.API, mux chi.Router, _ *guard) {
				mux.Get(path, func(http.ResponseWriter, *http.Request) {})
			},
		})
	}
	if _, err := router.Build(zap.NewNop()); err == nil {
		t.Fatal("Build accepted two conflicting routes, one of which chi replaced")
	}
}

func TestBuildReturnsErrorInsteadOfPanic(t *testing.T) {
	cases := map[string]func(*Router) error{
		// chi panics on an unclosed parameter.
		"chi": func(r *Router) error {
			return Register(r, op(http.MethodGet, "/broken/{id", projectsRead), idHandler)
		},
		// Huma panics on an input type that is not a struct.
		"huma": func(r *Router) error {
			return Register(r, op(http.MethodGet, "/scalar", projectsRead),
				func(context.Context, *int) (*okOutput, error) { return &okOutput{}, nil })
		},
	}
	for name, register := range cases {
		t.Run(name, func(t *testing.T) {
			router := newTestRouter()
			if err := register(router); err != nil {
				t.Fatalf("Register: %v", err)
			}
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("Build panicked: %v", p)
				}
			}()
			_, err := router.Build(zap.NewNop())
			if err == nil {
				t.Fatal("Build accepted a malformed route")
			}
			t.Logf("Build error: %v", err)
		})
	}
}

func TestOpenAPISpecIsServedUnderItsPermission(t *testing.T) {
	var buf bytes.Buffer
	router := newTestRouter()
	if err := Register(router, op(http.MethodGet, "/things", projectsRead), okHandler); err != nil {
		t.Fatalf("Register: %v", err)
	}
	var specPermission Permission
	for _, r := range router.Routes() {
		if r.Method == http.MethodGet && r.Path == OpenAPIPath {
			specPermission = r.Permission
		}
	}
	if !specPermission.IsPublic() {
		t.Fatalf("spec route permission = %v, want Public", specPermission)
	}

	rec := serve(t, mustBuild(t, router, bufferLogger(&buf)), http.MethodGet, OpenAPIPath)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200", OpenAPIPath, rec.Code)
	}
	var spec struct {
		OpenAPI string                    `json:"openapi"`
		Paths   map[string]map[string]any `json:"paths"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
		t.Fatalf("spec is not JSON: %v", err)
	}
	if spec.OpenAPI == "" || spec.Paths["/things"]["get"] == nil {
		t.Fatalf("spec lacks GET /things: %s", rec.Body.String())
	}
	entries := logEntries(t, &buf)
	if len(entries) != 1 {
		t.Fatalf("access log = %v, want one entry", entries)
	}
	if got := entries[0]; got["scope"] != "api" || got["permission"] != "public" {
		t.Fatalf("access log = %v, want scope \"api\" and permission \"public\"", got)
	}
	assertRequestID(t, entries[0], rec)
}

// assertRequestID pins the api line's request_id field and the response
// header that carries it: both must be present and equal, so the id pairs
// the line with its caller and any rbac lines of the same request.
func assertRequestID(t *testing.T, entry map[string]any, rec *httptest.ResponseRecorder) {
	t.Helper()
	header := rec.Header().Get(RequestIDHeader)
	if header == "" {
		t.Fatalf("%s response header is empty", RequestIDHeader)
	}
	if got, ok := entry["request_id"].(string); !ok || got != header {
		t.Fatalf("request_id = %v (header %q), want the header's value", entry["request_id"], header)
	}
}

func TestRecoveryAndAccessLogUseZap(t *testing.T) {
	var buf bytes.Buffer
	router := newTestRouter()
	if err := Register(router, op(http.MethodGet, "/boom", projectsRead),
		func(context.Context, *struct{}) (*okOutput, error) { panic("kaboom") }); err != nil {
		t.Fatalf("Register: %v", err)
	}
	h := mustBuild(t, router, bufferLogger(&buf))
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.RemoteAddr = "10.1.2.3:4567"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("X-Real-IP", "203.0.113.9")
	req.AddCookie(&http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET /boom = %d, want 500", rec.Code)
	}

	entries := logEntries(t, &buf)
	if len(entries) != 3 || entries[0]["msg"] != "rbac decision" || entries[1]["msg"] != "handler panicked" || entries[2]["msg"] != "request" {
		t.Fatalf("log entries = %v, want [rbac decision, handler panicked, request]", entries)
	}
	request := entries[2]
	if request["user_sub"] != testSub {
		t.Fatalf("user_sub = %v, want the session's sub %q", request["user_sub"], testSub)
	}
	if request["scope"] != "api" || request["status"] != float64(500) || request["permission"] != "Projects: Read" {
		t.Fatalf("request entry = %v, want scope \"api\", status 500 and permission \"Projects: Read\"", request)
	}
	if _, ok := request["route"]; !ok {
		t.Fatalf("request entry lacks the route field: %v", request)
	}
	if _, ok := request["user_sub"]; !ok {
		t.Fatalf("request entry lacks the user_sub field: %v", request)
	}
	if _, ok := request["duration_ms"]; !ok {
		t.Fatalf("request entry lacks the duration_ms field: %v", request)
	}
	if request["client_ip"] != "10.1.2.3" {
		t.Fatalf("client_ip = %v, want the peer 10.1.2.3 (proxy headers must not be trusted)", request["client_ip"])
	}
	if request["method"] != "GET" || request["path"] != "/boom" {
		t.Fatalf("method/path = %v %v, want GET /boom", request["method"], request["path"])
	}
	assertRequestID(t, request, rec)
}

func TestRegisterHandlerRequiresAPermission(t *testing.T) {
	router := newTestRouter()
	before := len(router.Routes())
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	err := RegisterHandler(router, op(http.MethodGet, "/open", Permission{}), h)
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("RegisterHandler error = %v, want ErrNoPermission", err)
	}
	if got := len(router.Routes()); got != before {
		t.Fatalf("router holds %d routes after a refused registration, want %d", got, before)
	}
}

func TestRegisterHandlerRefusesMissingHandler(t *testing.T) {
	router := newTestRouter()
	if err := RegisterHandler(router, op(http.MethodGet, "/open", projectsRead), nil); err == nil {
		t.Fatal("RegisterHandler accepted a nil handler")
	}
}

func TestRegisterHandlerRefusesADuplicate(t *testing.T) {
	router := newTestRouter()
	h := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	if err := RegisterHandler(router, op(http.MethodGet, "/ping", projectsRead), h); err != nil {
		t.Fatalf("first RegisterHandler: %v", err)
	}
	if err := RegisterHandler(router, op(http.MethodGet, "/ping", projectsRead), h); err == nil {
		t.Fatal("second RegisterHandler for GET /ping: expected a refusal, got nil")
	}
}

func TestRegisterHandlerServesWithTheDeclaredPermission(t *testing.T) {
	router := newTestRouter()
	if err := RegisterHandler(router, Operation{
		OperationID: "get-ping",
		Method:      http.MethodGet,
		Path:        "/ping",
		Permission:  projectsRead,
	}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("pong"))
	})); err != nil {
		t.Fatalf("RegisterHandler: %v", err)
	}

	found := false
	for _, r := range router.Routes() {
		if r.Method == http.MethodGet && r.Path == "/ping" {
			found = true
			if r.OperationID != "get-ping" {
				t.Errorf("route /ping operation id = %q, want get-ping", r.OperationID)
			}
			if r.Permission != projectsRead {
				t.Errorf("route /ping permission = %v, want %v", r.Permission, projectsRead)
			}
		}
	}
	if !found {
		t.Fatal("Routes() does not list GET /ping")
	}

	h := mustBuild(t, router, zap.NewNop())
	rec := serve(t, h, http.MethodGet, "/ping")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("GET /ping = %d, want 418", rec.Code)
	}
	if got := rec.Body.String(); got != "pong" {
		t.Fatalf("GET /ping body = %q, want pong", got)
	}
}
