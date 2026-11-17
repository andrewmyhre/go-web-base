package server

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// recordingGuard is a Guard that records every request it was asked about
// and answers with fn.
type recordingGuard struct {
	seen []GuardRequest
	fn   func(ctx context.Context, req GuardRequest) GuardResult
}

func (g *recordingGuard) Check(ctx context.Context, req GuardRequest) GuardResult {
	g.seen = append(g.seen, req)
	if g.fn == nil {
		return GuardResult{}
	}
	return g.fn(ctx, req)
}

type guardIDInput struct {
	ID string `path:"id"`
}

type guardIDOutput struct {
	Body struct {
		ID   string `json:"id"`
		Note string `json:"note"`
	}
}

type noteKey struct{}

// guardIDHandler echoes the path parameter and any note the Guard put on the
// context.
func guardIDHandler(ctx context.Context, in *guardIDInput) (*guardIDOutput, error) {
	out := &guardIDOutput{}
	out.Body.ID = in.ID
	out.Body.Note, _ = ctx.Value(noteKey{}).(string)
	return out, nil
}

// guardRouter has a Huma route and a plain chi route, both taking {id}, and
// the given Guard.
func guardRouter(t *testing.T, g Guard, scoped Permission) (http.Handler, *bytes.Buffer) {
	t.Helper()
	var logs bytes.Buffer
	r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie).WithGuard(g)
	if err := Register(r, op(http.MethodGet, "/things/{id}", scoped), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	if err := RegisterHandler(r, op(http.MethodGet, "/plain/{id}", scoped), http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		note, _ := req.Context().Value(noteKey{}).(string)
		_, _ = w.Write([]byte(chiParam(req, "id") + "|" + note))
	})); err != nil {
		t.Fatal(err)
	}
	return mustBuild(t, r, bufferLogger(&logs)), &logs
}

var projectScoped = Permission{Resource: Projects, Verb: Read}.In(ScopeProject)

// TestGuardReplacesCoresCheck: the Guard decides every route, including a
// Public one, and core's own check does not run (no rbac line).
func TestGuardReplacesCoresCheck(t *testing.T) {
	g := &recordingGuard{fn: func(context.Context, GuardRequest) GuardResult {
		return GuardResult{Status: http.StatusTeapot, Detail: "app says no"}
	}}
	h, logs := guardRouter(t, g, Public)
	for _, path := range []string{"/things/1", "/plain/1"} {
		rec := request(h, http.MethodGet, path, testSessionID)
		if rec.Code != http.StatusTeapot || !strings.Contains(rec.Body.String(), "app says no") {
			t.Errorf("GET %s = %d %s, want the Guard's 418 and detail", path, rec.Code, rec.Body)
		}
	}
	if len(g.seen) != 2 {
		t.Fatalf("Guard asked %d times, want 2 (a Public route is still the Guard's)", len(g.seen))
	}
	if n := len(rbacLines(t, logs)); n != 0 {
		t.Errorf("core's check logged %d rbac lines although a Guard decides", n)
	}
}

// TestGuardReceivesTheRequestFacts: method, matched pattern, parameters,
// header, query and the permission, for a Huma route and a chi route.
func TestGuardReceivesTheRequestFacts(t *testing.T) {
	g := &recordingGuard{}
	h, _ := guardRouter(t, g, projectScoped)
	for _, c := range []struct{ path, pattern string }{
		{"/things/abc?q=1", "/things/{id}"},
		{"/plain/abc?q=1", "/plain/{id}"},
	} {
		g.seen = nil
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		req.Header.Set("X-Probe", "yes")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || len(g.seen) != 1 {
			t.Fatalf("GET %s = %d with %d Guard calls: %s", c.path, rec.Code, len(g.seen), rec.Body)
		}
		got := g.seen[0]
		if got.Method != http.MethodGet || got.Path != c.pattern || got.Param("id") != "abc" || got.Param("nope") != "" ||
			got.Header("X-Probe") != "yes" || got.Query("q") != "1" || got.Permission != projectScoped || got.SessionLookupFailed {
			t.Errorf("GuardRequest for %s = {Method:%q Path:%q id:%q header:%q query:%q perm:%v failed:%v}",
				c.path, got.Method, got.Path, got.Param("id"), got.Header("X-Probe"), got.Query("q"), got.Permission, got.SessionLookupFailed)
		}
	}
}

// TestGuardSeesASessionStoreFailure: SessionLookupFailed is true when the
// session store failed, and false for an unknown cookie.
func TestGuardSeesASessionStoreFailure(t *testing.T) {
	store := newMemStore()
	g := &recordingGuard{}
	r := testRouter("t", "0").WithAuth(store, nil, testNames.SessionCookie).WithGuard(g)
	if err := Register(r, op(http.MethodGet, "/things/{id}", Public), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, nil)
	unknown := strings.Repeat("e", 64)
	request(h, http.MethodGet, "/things/1", unknown)
	store.fail = true
	request(h, http.MethodGet, "/things/1", testSessionID)
	if len(g.seen) != 2 || g.seen[0].SessionLookupFailed || !g.seen[1].SessionLookupFailed {
		t.Fatalf("SessionLookupFailed over (unknown cookie, store down) = %v, want [false true]", []bool{g.seen[0].SessionLookupFailed, g.seen[1].SessionLookupFailed})
	}
}

// TestGuardStatusesAreServed: 0 passes, anything else is written as a
// problem with the Guard's status and detail, and the handler does not run.
func TestGuardStatusesAreServed(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusServiceUnavailable} {
		g := &recordingGuard{fn: func(context.Context, GuardRequest) GuardResult {
			return GuardResult{Status: status, Detail: "detail " + http.StatusText(status), Params: map[string]string{"id": "never"}}
		}}
		h, _ := guardRouter(t, g, Authenticated)
		for _, path := range []string{"/things/1", "/plain/1"} {
			rec := request(h, http.MethodGet, path, testSessionID)
			if rec.Code != status || !strings.Contains(rec.Body.String(), "detail "+http.StatusText(status)) ||
				!strings.Contains(rec.Header().Get("Content-Type"), "problem+json") {
				t.Errorf("GET %s with Guard status %d = %d %q (%s)", path, status, rec.Code, rec.Body, rec.Header().Get("Content-Type"))
			}
			if strings.Contains(rec.Body.String(), "never") {
				t.Errorf("GET %s: Params were applied to a refused request", path)
			}
		}
	}
}

// TestGuardContextReachesTheHandler: what the Guard puts on its returned
// context is what the handler sees, and the permission note core made
// before is still there.
func TestGuardContextReachesTheHandler(t *testing.T) {
	g := &recordingGuard{fn: func(ctx context.Context, _ GuardRequest) GuardResult {
		if _, ok := PermissionFromContext(ctx); !ok {
			t.Error("the context given to the Guard lacks the route's permission")
		}
		return GuardResult{Ctx: context.WithValue(ctx, noteKey{}, "resolved")}
	}}
	h, _ := guardRouter(t, g, Authenticated)
	if rec := request(h, http.MethodGet, "/things/1", testSessionID); !strings.Contains(rec.Body.String(), `"note":"resolved"`) {
		t.Errorf("Huma handler body = %s, want the Guard's note", rec.Body)
	}
	if rec := request(h, http.MethodGet, "/plain/1", testSessionID); rec.Body.String() != "1|resolved" {
		t.Errorf("chi handler body = %q, want 1|resolved", rec.Body)
	}
}

// TestGuardRewritesPathParameters: a slug becomes the canonical id before
// the handler, for a Huma operation and a plain chi route. A parameter the
// route lacks is ignored.
func TestGuardRewritesPathParameters(t *testing.T) {
	g := &recordingGuard{fn: func(context.Context, GuardRequest) GuardResult {
		return GuardResult{Params: map[string]string{"id": "canonical-7", "absent": "x"}}
	}}
	h, _ := guardRouter(t, g, Authenticated)
	if rec := request(h, http.MethodGet, "/things/my-slug", testSessionID); !strings.Contains(rec.Body.String(), `"id":"canonical-7"`) {
		t.Errorf("Huma handler body = %s, want the rewritten id", rec.Body)
	}
	if rec := request(h, http.MethodGet, "/plain/my-slug", testSessionID); rec.Body.String() != "canonical-7|" {
		t.Errorf("chi handler body = %q, want the rewritten id", rec.Body)
	}
}

// TestNoGuardKeepsCoresCheck: with no Guard a refusal still comes from
// core's own check.
func TestNoGuardKeepsCoresCheck(t *testing.T) {
	r := testRouter("t", "0").WithAuth(newMemStore(), denyAll{}, testNames.SessionCookie)
	if err := Register(r, op(http.MethodGet, "/things/{id}", projectsRead), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, nil)
	if rec := request(h, http.MethodGet, "/things/1", testSessionID); rec.Code != http.StatusForbidden {
		t.Errorf("with no Guard a denying authorizer gave %d, want 403", rec.Code)
	}
}

// TestScopedPermissionWithoutGuardFailsBuild: a route that declares a scope
// core cannot check is refused at Build, never decided globally.
func TestScopedPermissionWithoutGuardFailsBuild(t *testing.T) {
	for _, scope := range []ScopeKind{ScopeProject, ScopeOrganization} {
		r := testRouter("t", "0").WithAuth(newMemStore(), allowAll{}, testNames.SessionCookie)
		if err := Register(r, op(http.MethodGet, "/things/{id}", Permission{Resource: Projects, Verb: Read}.In(scope)), guardIDHandler); err != nil {
			t.Fatal(err)
		}
		_, err := r.Build(nil)
		if !errors.Is(err, ErrScopedWithoutGuard) {
			t.Errorf("Build with scope %d and no Guard = %v, want ErrScopedWithoutGuard", scope, err)
		}
		if _, err := r.WithGuard(&recordingGuard{}).Build(nil); err != nil {
			t.Errorf("Build with scope %d and a Guard = %v, want nil", scope, err)
		}
	}
}

// TestGlobalPermissionIsNotScoped: the zero scope is global, and a scope
// on Public or Authenticated is malformed.
func TestGlobalPermissionIsNotScoped(t *testing.T) {
	if (Permission{Resource: Projects, Verb: Read}).IsScoped() || !projectScoped.IsScoped() {
		t.Error("IsScoped must be false for the zero scope and true for ScopeProject")
	}
	for _, p := range []Permission{Public.In(ScopeProject), Authenticated.In(ScopeOrganization)} {
		if err := p.Validate(); !errors.Is(err, ErrUnknownResource) {
			t.Errorf("%v.Validate() = %v, want ErrUnknownResource", p, err)
		}
	}
	if projectScoped.String() != "Projects: Read" {
		t.Errorf("a scoped permission prints as %q; the access log must not change", projectScoped.String())
	}
}

// TestGuardMakesAnAuthorizerUnnecessary: with a Guard a role route builds
// without an authorizer (the Guard decides), where without one it is
// refused with ErrNoAuth.
func TestGuardMakesAnAuthorizerUnnecessary(t *testing.T) {
	r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie)
	if err := Register(r, op(http.MethodGet, "/things/{id}", projectsRead), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Build(nil); !errors.Is(err, ErrNoAuth) {
		t.Fatalf("Build without authorizer or Guard = %v, want ErrNoAuth", err)
	}
	if _, err := r.WithGuard(&recordingGuard{}).Build(nil); err != nil {
		t.Fatalf("Build with a Guard = %v, want nil", err)
	}
}

// TestMiddlewareRunsAfterSessionAndBeforeTheCheck: in the order given, with
// the request id and the session already on the context, and able to answer
// the request itself before the check runs.
func TestMiddlewareRunsAfterSessionAndBeforeTheCheck(t *testing.T) {
	var order []string
	g := &recordingGuard{fn: func(context.Context, GuardRequest) GuardResult {
		order = append(order, "guard")
		return GuardResult{}
	}}
	mw := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s, ok := SessionFromContext(r.Context())
				order = append(order, name+":"+map[bool]string{true: "session", false: "anon"}[ok && s.UserID == testSub]+":"+
					map[bool]string{true: "rid", false: "norid"}[RequestIDFromContext(r.Context()) != ""])
				if r.Header.Get("X-Stop") == name {
					http.Error(w, "stopped by "+name, http.StatusTeapot)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}
	r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie).WithGuard(g).
		WithMiddleware(mw("one"), nil, mw("two")).WithMiddleware(mw("three"))
	if err := Register(r, op(http.MethodGet, "/things/{id}", Public), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, nil)

	request(h, http.MethodGet, "/things/1", testSessionID)
	want := "one:session:rid two:session:rid three:session:rid guard"
	if got := strings.Join(order, " "); got != want {
		t.Errorf("order = %q, want %q", got, want)
	}

	order = nil
	req := httptest.NewRequest(http.MethodGet, "/things/1", nil)
	req.Header.Set("X-Stop", "two")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot || strings.Contains(strings.Join(order, " "), "guard") || strings.Contains(strings.Join(order, " "), "three") {
		t.Errorf("a middleware that answers = %d with order %v; want 418 and neither three nor the Guard to run", rec.Code, order)
	}
}

// TestOpenAPIPathCanBePrefixed: the spec moves to the given path and is no
// longer at the root.
func TestOpenAPIPathCanBePrefixed(t *testing.T) {
	r := testRouter("t", "0").WithOpenAPIPath("/api/v1/openapi.json").WithOpenAPIPath("")
	h := mustBuild(t, r, nil)
	if rec := request(h, http.MethodGet, "/api/v1/openapi.json", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"openapi"`) {
		t.Errorf("GET /api/v1/openapi.json = %d %.60s", rec.Code, rec.Body)
	}
	if rec := request(h, http.MethodGet, OpenAPIPath, ""); rec.Code != http.StatusNotFound {
		t.Errorf("GET %s = %d after moving the spec, want 404", OpenAPIPath, rec.Code)
	}
}

func chiParam(r *http.Request, name string) string { return chi.URLParam(r, name) }

// denyAll is an Authorizer that denies every check.
type denyAll struct{}

func (denyAll) Authorize(string, string, string, string) (bool, error) { return false, nil }

// TestGuardStatusOutside400To599IsA500: a 2xx, 3xx, 1xx or negative status
// from the Guard is its bug, answered 500 with no handler run and no Params
// applied; 400 and 599 pass as given.
func TestGuardStatusOutside400To599IsA500(t *testing.T) {
	for status, want := range map[int]int{200: 500, 204: 500, 302: 500, 99: 500, -1: 500, 399: 500, 600: 500, 400: 400, 599: 599} {
		ran := false
		g := &recordingGuard{fn: func(context.Context, GuardRequest) GuardResult {
			return GuardResult{Status: status, Detail: "d", Params: map[string]string{"id": "x"}}
		}}
		var logs bytes.Buffer
		r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie).WithGuard(g)
		if err := RegisterHandler(r, op(http.MethodGet, "/plain/{id}", Public), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { ran = true })); err != nil {
			t.Fatal(err)
		}
		if err := Register(r, op(http.MethodGet, "/things/{id}", Public), func(ctx context.Context, in *guardIDInput) (*guardIDOutput, error) {
			ran = true
			return &guardIDOutput{}, nil
		}); err != nil {
			t.Fatal(err)
		}
		h := mustBuild(t, r, bufferLogger(&logs))
		for _, path := range []string{"/plain/1", "/things/1"} {
			ran = false
			rec := request(h, http.MethodGet, path, "")
			if rec.Code != want || (want == 500 && ran) {
				t.Errorf("Guard status %d on %s = %d (handler ran: %v), want %d", status, path, rec.Code, ran, want)
			}
			if want == 500 && strings.Contains(rec.Body.String(), `"d"`) {
				t.Errorf("Guard status %d leaked its detail on a 500: %s", status, rec.Body)
			}
		}
	}
}

// TestNoteRequestUserAttributesABearerRequest: a Middleware (or Guard) that
// authenticated a bearer token names the user, and the access log carries it;
// without the call the line is unattributed, and a session user is replaced
// by the later call.
func TestNoteRequestUserAttributesABearerRequest(t *testing.T) {
	var logs bytes.Buffer
	bearer := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "Bearer good" {
				NoteRequestUser(r.Context(), "token-subject")
			}
			next.ServeHTTP(w, r)
		})
	}
	r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie).WithGuard(&recordingGuard{}).WithMiddleware(bearer)
	if err := Register(r, op(http.MethodGet, "/things/{id}", Public), guardIDHandler); err != nil {
		t.Fatal(err)
	}
	h := mustBuild(t, r, bufferLogger(&logs))
	for _, c := range []struct{ auth, cookie, want string }{
		{"Bearer good", "", "token-subject"},
		{"", "", ""},
		{"", testSessionID, testSub},
		{"Bearer good", testSessionID, "token-subject"},
	} {
		logs.Reset()
		req := httptest.NewRequest(http.MethodGet, "/things/1", nil)
		if c.auth != "" {
			req.Header.Set("Authorization", c.auth)
		}
		if c.cookie != "" {
			req.AddCookie(&http.Cookie{Name: testNames.SessionCookie, Value: c.cookie})
		}
		h.ServeHTTP(httptest.NewRecorder(), req)
		var got any = "no api line"
		for _, e := range logEntries(t, &logs) {
			if e["scope"] == "api" {
				got = e["user_sub"]
			}
		}
		if got != c.want {
			t.Errorf("auth %q cookie %q: user_sub = %v, want %q", c.auth, c.cookie, got, c.want)
		}
	}
	NoteRequestUser(context.Background(), "x") // outside a request: a no-op
}
