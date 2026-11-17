package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/redis"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// The golden files in testdata/golden were produced by this test run against
// core/v0.4.0, before any hook existed (GOLDEN_UPDATE=1 in a checkout of that
// tag). This test then runs the same app, which sets none of the hooks,
// against the tree under test and requires every byte to match: the routes,
// every status and body of the built-in routes, the rbac log lines, and the
// OpenAPI document. A change that alters default behaviour fails here.
//
// One deliberate change since: the site settings routes (/admin/settings and
// /admin/invites, guarded by SiteSettings and SiteInvites permissions that
// only Admin holds) are added to the route list and the OpenAPI document.
// The golden files were regenerated for them and nothing else in them moved.
//
// It uses only API that exists at v0.4.0, so it can be copied into that tag
// unchanged to regenerate the files.

const (
	goldenSession      = "00000000000000000000000000000000000000000000000000000000000000a1"
	goldenBrokenCookie = "00000000000000000000000000000000000000000000000000000000000000b2"
	goldenSub          = "100000000000000000001"
)

// goldenFeature is a feature that registers a resource, a grant, a setting
// and a route, as an installed feature does.
type goldenFeature struct{}

func (goldenFeature) Name() string { return "widgets" }
func (goldenFeature) Install(i *Installer) error {
	if err := i.Resource("Widgets", "Spin"); err != nil {
		return err
	}
	if err := i.Grant("Contributor", server.Permission{Resource: "Widgets", Verb: "Read"}); err != nil {
		return err
	}
	if err := i.String("color", "red", "widget color"); err != nil {
		return err
	}
	i.Routes(func(r *server.Router, d Deps) error {
		return server.Register(r, server.Operation{
			OperationID: "list-widgets", Method: http.MethodGet, Path: "/widgets",
			Summary: "List widgets", Tags: []string{"widgets"},
			Permission: server.Permission{Resource: "Widgets", Verb: "Read"},
		}, goldenList)
	})
	return nil
}

func goldenList(context.Context, *struct{}) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = "widgets"
	return out, nil
}

type goldenProjectInput struct {
	ID string `path:"id"`
}

func goldenOptions() Options {
	return Options{
		Name: "goldenapp", Version: "9.9.9", EnvPrefix: "GOLDENAPP", CookiePrefix: "goldenapp",
		Roles: []db.RoleDef{
			{Name: db.AdminRole, Inherits: []string{"Contributor"}, Permissions: []string{"Projects: Delete"}},
			{Name: "Contributor", Permissions: []string{"Projects: Read", "Projects: Create"}},
		},
		Resources: []server.ResourceDef{{Name: "Projects"}},
		Features:  []Feature{goldenFeature{}},
		Routes: func(r *server.Router, d Deps) error {
			for _, op := range []struct {
				id, method, path string
				perm             server.Permission
			}{
				{"list-projects", http.MethodGet, "/projects", server.Permission{Resource: "Projects", Verb: server.Read}},
				{"create-project", http.MethodPost, "/projects", server.Permission{Resource: "Projects", Verb: server.Create}},
				{"delete-project", http.MethodDelete, "/projects/{id}", server.Permission{Resource: "Projects", Verb: server.Delete}},
			} {
				if err := server.Register(r, server.Operation{
					OperationID: op.id, Method: op.method, Path: op.path, Summary: op.id, Permission: op.perm,
				}, func(_ context.Context, in *goldenProjectInput) (*HealthOutput, error) {
					out := &HealthOutput{}
					out.Body.Status = "id=" + in.ID
					return out, nil
				}); err != nil {
					return err
				}
			}
			if err := server.Register(r, server.Operation{
				OperationID: "get-account", Method: http.MethodGet, Path: "/account", Summary: "account",
				Permission: server.Authenticated,
			}, goldenList); err != nil {
				return err
			}
			return server.RegisterHandler(r, server.Operation{
				OperationID: "plain-page", Method: http.MethodGet, Path: "/page", Permission: server.Authenticated,
			}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("page")) }))
		},
	}
}

// goldenStore is an in-memory session store. goldenBrokenCookie fails on Get.
type goldenStore struct {
	mu   sync.Mutex
	data map[string]map[string]string
}

func newGoldenStore() *goldenStore {
	return &goldenStore{data: map[string]map[string]string{
		goldenSession: {"sub": goldenSub, "name": "Golden User", "email": "golden@example.com", "avatar": "https://example.com/a.png"},
	}}
}

func (s *goldenStore) Create(_ context.Context, _ time.Duration, d map[string]string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := strings.Repeat("c", 64)
	s.data[id] = d
	return id, nil
}

func (s *goldenStore) Get(_ context.Context, id string) (map[string]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == goldenBrokenCookie {
		return nil, errors.New("store is down")
	}
	d, ok := s.data[id]
	if !ok {
		return nil, redis.ErrNotFound
	}
	return d, nil
}

func (s *goldenStore) Take(ctx context.Context, id string) (map[string]string, error) {
	d, err := s.Get(ctx, id)
	if err == nil {
		s.mu.Lock()
		delete(s.data, id)
		s.mu.Unlock()
	}
	return d, err
}

func (s *goldenStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	return nil
}

// goldenAuthz allows the subject Projects: Read and Create, and nothing else.
type goldenAuthz struct{}

func (goldenAuthz) Authorize(sub, domain, object, action string) (bool, error) {
	if sub != goldenSub || domain != db.Global {
		return false, nil
	}
	return object == "Projects" && (action == "Read" || action == "Create"), nil
}
func (goldenAuthz) HasRoleBinding(sub string) (bool, error) { return sub == goldenSub, nil }
func (goldenAuthz) RoleBindingsOf(sub string) ([]db.Binding, error) {
	if sub != goldenSub {
		return nil, nil
	}
	return []db.Binding{{Role: "Contributor", Domain: db.Global}}, nil
}

type goldenRequest struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Cookie string `json:"cookie,omitempty"`
}

type goldenResponse struct {
	goldenRequest
	Status      int               `json:"status"`
	ContentType string            `json:"content_type,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
}

type goldenDoc struct {
	Routes    []string          `json:"routes"`
	Responses []goldenResponse  `json:"responses"`
	LogLines  []map[string]any  `json:"log_lines"`
	Settings  map[string]string `json:"settings"`
	Flags     []string          `json:"serve_flags"`
}

func buildGolden(t *testing.T) (goldenDoc, []byte) {
	t.Helper()
	o := goldenOptions()
	inst, err := newInstallation(o)
	if err != nil {
		t.Fatalf("newInstallation: %v", err)
	}
	google, err := server.NewGoogle(server.GoogleConfig{
		ClientID: "id", ClientSecret: "secret", RedirectURL: "https://golden.test/auth/google/callback",
	})
	if err != nil {
		t.Fatal(err)
	}
	store := newGoldenStore()
	var logs bytes.Buffer
	log := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zapcore.DebugLevel))
	auth := server.NewAuth(google, store, newGoldenStore(), goldenAuthz{}, log, "secret", o.names())
	d := Deps{Sessions: store, Authorizer: goldenAuthz{}, Roles: goldenAuthz{}, Auth: auth, Log: log}
	router, err := newRouter(inst, d)
	if err != nil {
		t.Fatalf("newRouter: %v", err)
	}
	var doc goldenDoc
	for _, r := range router.Routes() {
		doc.Routes = append(doc.Routes, strings.Join([]string{r.Method, r.Path, r.OperationID, r.Permission.String()}, " | "))
	}
	h, err := server.New(config.Default(), log, router).Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	for _, rq := range []goldenRequest{
		{"GET", "/health", ""},
		{"POST", "/health", ""},
		{"GET", "/nope", ""},
		{"GET", "/me", ""},
		{"GET", "/me", goldenSession},
		{"GET", "/me", goldenBrokenCookie},
		{"GET", "/me", strings.Repeat("d", 64)},
		{"GET", "/openapi.json", ""},
		{"GET", "/projects", ""},
		{"GET", "/projects", goldenSession},
		{"POST", "/projects", goldenSession},
		{"DELETE", "/projects/7", goldenSession},
		{"DELETE", "/projects/7", goldenBrokenCookie},
		{"GET", "/widgets", goldenSession},
		{"GET", "/account", goldenSession},
		{"GET", "/account", goldenBrokenCookie},
		{"GET", "/page", ""},
		{"GET", "/page", goldenSession},
		{"GET", "/auth/google", ""},
		{"GET", "/auth/google/callback", ""},
		{"POST", "/auth/logout", goldenSession},
		{"POST", "/auth/logout", ""},
		{"GET", "/api/v1/me", goldenSession},
	} {
		req := httptest.NewRequest(rq.Method, rq.Path, nil)
		if rq.Cookie != "" {
			req.AddCookie(&http.Cookie{Name: o.names().SessionCookie, Value: rq.Cookie})
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		resp := goldenResponse{goldenRequest: rq, Status: rec.Code, ContentType: rec.Header().Get("Content-Type"), Body: rec.Body.String()}
		if rec.Code == http.StatusFound {
			resp.Body = "<redirect body carries a random nonce and challenge>"
		}
		resp.Headers = map[string]string{}
		for _, name := range []string{"Cache-Control", "Set-Cookie", "Location"} {
			if v := rec.Header().Get(name); v != "" {
				if name == "Location" || name == "Set-Cookie" {
					if i := strings.IndexAny(v, "?;"); i >= 0 && name == "Location" {
						v = v[:i]
					}
					v = normalizeVolatile(v)
				}
				resp.Headers[name] = v
			}
		}
		if rec.Header().Get(server.RequestIDHeader) == "" {
			resp.Headers["X-Request-Id"] = "missing"
		}
		if len(resp.Headers) == 0 {
			resp.Headers = nil
		}
		doc.Responses = append(doc.Responses, resp)
	}
	for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		for _, k := range []string{"ts", "request_id", "duration_ms", "duration", "latency_ms", "stacktrace"} {
			delete(m, k)
		}
		doc.LogLines = append(doc.LogLines, m)
	}
	doc.Settings = map[string]string{}
	for _, k := range inst.v.AllKeys() {
		doc.Settings[k] = inst.v.GetString(k)
	}
	cmd := newServeCmd(o, inst, nil)
	cmd.Flags().VisitAll(func(f *pflag.Flag) { doc.Flags = append(doc.Flags, f.Name+"="+f.DefValue+" | "+f.Usage) })
	sort.Strings(doc.Flags)

	spec, err := OpenAPISpec(o)
	if err != nil {
		t.Fatalf("OpenAPISpec: %v", err)
	}
	return doc, spec
}

// normalizeVolatile blanks the parts of a header that change per run.
func normalizeVolatile(v string) string {
	parts := strings.Split(v, "; ")
	for i, p := range parts {
		if strings.HasPrefix(p, "Expires=") {
			parts[i] = "Expires=<time>"
		}
	}
	return strings.Join(parts, "; ")
}

func TestDefaultBehaviourIsByteIdenticalToV040(t *testing.T) {
	doc, spec := buildGolden(t)
	got, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	dir := filepath.Join("testdata", "golden")
	files := map[string][]byte{"default-behaviour.json": got, "openapi.json": spec}
	if os.Getenv("GOLDEN_UPDATE") == "1" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, b := range files {
			if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		t.Skip("golden files rewritten")
	}
	for name, b := range files {
		want, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if !bytes.Equal(want, b) {
			t.Errorf("%s differs from the v0.4.0 golden file.\n--- want ---\n%s\n--- got ---\n%s", name, want, b)
		}
	}
}
