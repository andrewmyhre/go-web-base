package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// rbacLines returns the rbac-scope entries logged into buf.
func rbacLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	if buf.Len() == 0 {
		return nil
	}
	for _, e := range logEntries(t, buf) {
		if e["scope"] == "rbac" {
			out = append(out, e)
		}
	}
	return out
}

func request(h http.Handler, method, path, cookie string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookie, Value: cookie})
	}
	h.ServeHTTP(rec, req)
	return rec
}

func TestBuildRefusesRoutesItCannotAuthorize(t *testing.T) {
	cases := map[string]struct {
		router *Router
		perm   Permission
	}{
		"authenticated, no session store": {NewRouter("t", "0"), Authenticated},
		"role, no session store":          {NewRouter("t", "0"), projectsRead},
		"role, no authorizer":             {NewRouter("t", "0").WithAuth(newMemStore(), nil), projectsRead},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Register(c.router, op(http.MethodGet, "/x", c.perm), okHandler); err != nil {
				t.Fatalf("Register: %v", err)
			}
			if _, err := c.router.Build(nil); !errors.Is(err, ErrNoAuth) {
				t.Fatalf("Build error = %v, want ErrNoAuth", err)
			}
		})
	}
	// Public needs nothing, and Authenticated needs no authorizer.
	r := NewRouter("t", "0")
	if _, err := r.Build(nil); err != nil {
		t.Fatalf("Build of a public-only router: %v", err)
	}
	r = NewRouter("t", "0").WithAuth(newMemStore(), nil)
	if err := Register(r, op(http.MethodGet, "/x", Authenticated), okHandler); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Build(nil); err != nil {
		t.Fatalf("Build of an Authenticated route without an authorizer: %v", err)
	}
}

func TestAuthenticatedRoute(t *testing.T) {
	var buf bytes.Buffer
	store := newMemStore()
	r := NewRouter("t", "0").WithAuth(store, nil)
	if err := Register(r, op(http.MethodGet, "/me", Authenticated), NewMe(nil, nil)); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, bufferLogger(&buf))

	// No cookie, a malformed one and an unknown one are all 401, with a
	// deny line naming no subject.
	for name, cookie := range map[string]string{
		"none":      "",
		"malformed": "not-a-session-id",
		"unknown":   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	} {
		buf.Reset()
		rec := request(h, http.MethodGet, "/me", cookie)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s cookie: GET /me = %d, want 401: %s", name, rec.Code, rec.Body)
		}
		lines := rbacLines(t, &buf)
		if len(lines) != 1 || lines[0]["decision"] != "deny" || lines[0]["subject"] != "" ||
			lines[0]["object"] != authenticatedObject || lines[0]["domain"] != "global" {
			t.Fatalf("%s cookie: rbac lines = %v, want one deny with no subject", name, lines)
		}
		if lines[0]["request_id"] != rec.Header().Get(RequestIDHeader) {
			t.Fatalf("rbac request_id = %v, want the response's %q", lines[0]["request_id"], rec.Header().Get(RequestIDHeader))
		}
	}

	buf.Reset()
	rec := request(h, http.MethodGet, "/me", testSessionID)
	if rec.Code != http.StatusOK {
		t.Fatalf("signed in: GET /me = %d, want 200: %s", rec.Code, rec.Body)
	}
	want := `{"id":"` + testSub + `","name":"Test User","email":"test@example.com","avatar":"https://example.com/a.png","roles":[]}`
	if got := bytes.TrimSpace(rec.Body.Bytes()); string(got) != want {
		t.Fatalf("GET /me body = %s, want %s", got, want)
	}
	lines := rbacLines(t, &buf)
	if len(lines) != 1 || lines[0]["decision"] != "allow" || lines[0]["subject"] != testSub {
		t.Fatalf("rbac lines = %v, want one allow for %s", lines, testSub)
	}

	// A store outage is a 503, never a 500 and never "signed out".
	store.fail = true
	if rec := request(h, http.MethodGet, "/me", testSessionID); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("store down: GET /me = %d, want 503", rec.Code)
	}
}

func TestRoleRouteAsksTheAuthorizer(t *testing.T) {
	var buf bytes.Buffer
	g := &grants{allowed: map[string]bool{}}
	r := NewRouter("t", "0").WithAuth(newMemStore(), g)
	if err := Register(r, op(http.MethodGet, "/projects", projectsRead), okHandler); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, bufferLogger(&buf))

	if rec := request(h, http.MethodGet, "/projects", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous: %d, want 401", rec.Code)
	}
	if len(g.calls) != 0 {
		t.Fatalf("authorizer asked about an anonymous request: %v", g.calls)
	}

	buf.Reset()
	if rec := request(h, http.MethodGet, "/projects", testSessionID); rec.Code != http.StatusForbidden {
		t.Fatalf("no grant: %d, want 403", rec.Code)
	}
	if want := testSub + "|global|Projects|Read"; len(g.calls) != 1 || g.calls[0] != want {
		t.Fatalf("authorizer calls = %v, want [%s]", g.calls, want)
	}
	lines := rbacLines(t, &buf)
	if len(lines) != 1 || lines[0]["decision"] != "deny" || lines[0]["subject"] != testSub ||
		lines[0]["object"] != "Projects" || lines[0]["action"] != "Read" {
		t.Fatalf("rbac lines = %v, want one deny of Projects Read for %s", lines, testSub)
	}

	buf.Reset()
	g.allowed[testSub+"|Projects|Read"] = true
	if rec := request(h, http.MethodGet, "/projects", testSessionID); rec.Code != http.StatusOK {
		t.Fatalf("granted: %d, want 200", rec.Code)
	}
	if lines := rbacLines(t, &buf); len(lines) != 1 || lines[0]["decision"] != "allow" {
		t.Fatalf("rbac lines = %v, want one allow", lines)
	}

	buf.Reset()
	g.err = errors.New("enforcer broke")
	if rec := request(h, http.MethodGet, "/projects", testSessionID); rec.Code != http.StatusForbidden {
		t.Fatalf("authorizer error: %d, want 403", rec.Code)
	}
	if lines := rbacLines(t, &buf); len(lines) != 1 || lines[0]["decision"] != "deny" {
		t.Fatalf("rbac lines = %v, want one deny", lines)
	}
}

func TestPlainHandlerRouteIsGuardedToo(t *testing.T) {
	r := NewRouter("t", "0").WithAuth(newMemStore(), nil)
	reached := false
	if err := RegisterHandler(r, op(http.MethodGet, "/plain", Authenticated), http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		reached = true
	})); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, zap.NewNop())
	rec := request(h, http.MethodGet, "/plain", "")
	if rec.Code != http.StatusUnauthorized || reached {
		t.Fatalf("anonymous: %d (handler reached %v), want 401 without reaching the handler", rec.Code, reached)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	if rec := request(h, http.MethodGet, "/plain", testSessionID); rec.Code != http.StatusOK || !reached {
		t.Fatalf("signed in: %d (reached %v), want 200", rec.Code, reached)
	}
}
