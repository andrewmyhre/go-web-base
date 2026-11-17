package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// This file is the ADR 3.7 item 2 scenario: an app that owns its decisions
// through the hooks alone (Guard, Middleware, Params, NoteRequestUser) and
// reproduces huddle-go's: a project route decided in the project, then the
// organization, then globally; a bearer token refused on an organization
// route; a blocked session refused on an Authenticated route; a slug turned
// into the canonical project id. Open sign-up is the SignIn scenario in
// core/server (TestOpenSignUpAdmitsAnUnboundSubjectAndRefusesABlockedOne).

type scenarioKey struct{}

// scenarioWorld is the app's own data: who holds what where.
type scenarioWorld struct {
	grants  map[string]bool   // "sub|domain|Resource|Verb"
	blocked map[string]bool   // blocked subjects
	slugs   map[string]string // project slug to canonical id
	orgOf   map[string]string // canonical project id to organization
	tokens  map[string]string // bearer token to the project id it is for
}

type scenarioAuth struct {
	token, tokenProject, sub string
}

// guard is huddle-go's shape: token first, then session, then the chain.
func (w *scenarioWorld) guard(Deps) server.Guard { return scenarioGuard{w} }

type scenarioGuard struct{ w *scenarioWorld }

func (g scenarioGuard) Check(ctx context.Context, req server.GuardRequest) server.GuardResult {
	p := req.Permission
	if p.IsPublic() {
		return server.GuardResult{}
	}
	if req.SessionLookupFailed {
		return server.GuardResult{Status: http.StatusServiceUnavailable, Detail: "the session store is unavailable"}
	}
	if a, ok := ctx.Value(scenarioKey{}).(scenarioAuth); ok && a.token != "" {
		// A project token: only project routes, only its own project.
		if p.Scope != server.ScopeProject {
			return server.GuardResult{Status: http.StatusForbidden, Detail: "a project token cannot use this route"}
		}
		id := a.tokenProject // a token names no project in the path on routes without one
		if slug := req.Param("project"); slug != "" {
			id = g.w.slugs[slug]
		}
		if id == "" || id != a.tokenProject {
			return server.GuardResult{Status: http.StatusForbidden, Detail: "token is for another project"}
		}
		return server.GuardResult{Params: map[string]string{"project": id}}
	}
	s, ok := server.SessionFromContext(ctx)
	if !ok {
		return server.GuardResult{Status: http.StatusUnauthorized, Detail: "not signed in"}
	}
	if g.w.blocked[s.UserID] {
		return server.GuardResult{Status: http.StatusForbidden, Detail: "blocked"}
	}
	if p.IsAuthenticated() {
		return server.GuardResult{}
	}
	has := func(domain string) bool {
		return g.w.grants[s.UserID+"|"+domain+"|"+string(p.Resource)+"|"+string(p.Verb)]
	}
	switch p.Scope {
	case server.ScopeProject:
		id := g.w.slugs[req.Param("project")]
		if id == "" {
			return server.GuardResult{Status: http.StatusNotFound, Detail: "no such project"}
		}
		if has("project:"+id) || has("org:"+g.w.orgOf[id]) || has(db.Global) {
			return server.GuardResult{Params: map[string]string{"project": id}}
		}
	case server.ScopeOrganization:
		if has("org:"+req.Param("org")) || has(db.Global) {
			return server.GuardResult{}
		}
	default:
		if has(db.Global) {
			return server.GuardResult{}
		}
	}
	return server.GuardResult{Status: http.StatusForbidden, Detail: "forbidden"}
}

func (w *scenarioWorld) bearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
			project, known := w.tokens[tok]
			if !known {
				http.Error(rw, "bad token", http.StatusUnauthorized)
				return
			}
			server.NoteRequestUser(r.Context(), "token:"+project)
			r = r.WithContext(context.WithValue(r.Context(), scenarioKey{}, scenarioAuth{token: tok, tokenProject: project}))
		}
		next.ServeHTTP(rw, r)
	})
}

type scenarioProjectIn struct {
	Project string `path:"project"`
}
type scenarioOrgIn struct {
	Org string `path:"org"`
}
type scenarioNoIn struct{}

func scenarioOut(s string) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = s
	return out, nil
}

func TestScenarioReproducesHuddleGoDecisions(t *testing.T) {
	const alice, bob, carol, admin, blocked = "alice", "bob", "carol", "admin", "blocked"
	w := &scenarioWorld{
		grants: map[string]bool{
			alice + "|project:p-1|Projects|Read": true, // project grant
			bob + "|org:o-1|Projects|Read":       true, // organization grant
			admin + "|global|Projects|Read":      true, // global grant
			admin + "|global|Orgs|Read":          true,
			bob + "|org:o-1|Orgs|Read":           true,
			blocked + "|global|Projects|Read":    true,
		},
		blocked: map[string]bool{blocked: true},
		slugs:   map[string]string{"alpha": "p-1", "beta": "p-2"},
		orgOf:   map[string]string{"p-1": "o-1", "p-2": "o-2"},
		tokens:  map[string]string{"tok-p1": "p-1"},
	}
	store := newGoldenStore()
	for _, sub := range []string{alice, bob, carol, admin, blocked} {
		store.data[fmt.Sprintf("%064x", []byte(sub))] = map[string]string{"sub": sub, "name": sub}
	}
	cookie := func(sub string) string { return fmt.Sprintf("%064x", []byte(sub)) }

	o := testOpts
	o.Resources = []server.ResourceDef{{Name: "Projects"}, {Name: "Orgs"}}
	o.Roles = []db.RoleDef{{Name: db.AdminRole, Permissions: []string{"Projects: Read"}}}
	o.Guard = w.guard
	o.Middleware = []func(http.Handler) http.Handler{w.bearer}
	o.Routes = func(r *server.Router, _ Deps) error {
		perm := func(res server.Resource, sc server.ScopeKind) server.Permission {
			return server.Permission{Resource: res, Verb: server.Read}.In(sc)
		}
		op := func(id, path string, p server.Permission) server.Operation {
			return server.Operation{OperationID: id, Method: http.MethodGet, Path: path, Summary: id, Permission: p}
		}
		if err := server.Register(r, op("get-project", "/projects/{project}", perm("Projects", server.ScopeProject)),
			func(_ context.Context, in *scenarioProjectIn) (*HealthOutput, error) {
				return scenarioOut("project=" + in.Project)
			}); err != nil {
			return err
		}
		if err := server.Register(r, op("get-org", "/orgs/{org}", perm("Orgs", server.ScopeOrganization)),
			func(_ context.Context, in *scenarioOrgIn) (*HealthOutput, error) { return scenarioOut("org=" + in.Org) }); err != nil {
			return err
		}
		plain := func(context.Context, *scenarioNoIn) (*HealthOutput, error) { return scenarioOut("ok") }
		if err := server.Register(r, op("get-orgs", "/orgs", perm("Orgs", server.ScopeGlobal)), plain); err != nil {
			return err
		}
		if err := server.Register(r, op("get-account", "/account", server.Authenticated), plain); err != nil {
			return err
		}
		return nil
	}
	var logs bytes.Buffer
	log := zap.New(zapcore.NewCore(zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), zapcore.AddSync(&logs), zapcore.DebugLevel))
	router, err := testRouter(t, o, Deps{Sessions: store, Roles: goldenAuthz{}, Log: log})
	if err != nil {
		t.Fatal(err)
	}
	h, err := server.New(config.Default(), log, router).Handler()
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		name, path, sub, bearer string
		status                  int
		body                    string
	}{
		{"project grant, slug becomes the id", "/projects/alpha", alice, "", 200, "project=p-1"},
		{"project grant does not reach another project", "/projects/beta", alice, "", 403, ""},
		{"organization grant reaches its project", "/projects/alpha", bob, "", 200, "project=p-1"},
		{"organization grant stops at the organization", "/projects/beta", bob, "", 403, ""},
		{"global grant reaches every project", "/projects/beta", admin, "", 200, "project=p-2"},
		{"no grant", "/projects/alpha", carol, "", 403, ""},
		{"unknown project", "/projects/nope", admin, "", 404, ""},
		{"no session", "/projects/alpha", "", "", 401, ""},
		{"org route, organization grant", "/orgs/o-1", bob, "", 200, "org=o-1"},
		{"org route, other organization", "/orgs/o-2", bob, "", 403, ""},
		{"org route, global grant", "/orgs/o-2", admin, "", 200, "org=o-2"},
		{"global route, global grant", "/orgs", admin, "", 200, ""},
		{"global route, organization grant is not enough", "/orgs", bob, "", 403, ""},
		{"token on its own project", "/projects/alpha", "", "tok-p1", 200, "project=p-1"},
		{"token on another project", "/projects/beta", "", "tok-p1", 403, ""},
		{"token refused on an organization route", "/orgs/o-1", "", "tok-p1", 403, ""},
		{"token refused on a global route", "/orgs", "", "tok-p1", 403, ""},
		{"token refused on an Authenticated route", "/account", "", "tok-p1", 403, ""},
		{"unknown token", "/projects/alpha", "", "nope", 401, ""},
		{"token wins over a valid cookie", "/orgs/o-1", bob, "tok-p1", 403, ""},
		{"Authenticated route, signed in", "/account", carol, "", 200, ""},
		{"blocked session refused on an Authenticated route", "/account", blocked, "", 403, ""},
		{"blocked session refused despite a global grant", "/projects/alpha", blocked, "", 403, ""},
		{"Authenticated route, no session", "/account", "", "", 401, ""},
	} {
		logs.Reset()
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		if c.sub != "" {
			req.AddCookie(&http.Cookie{Name: o.names().SessionCookie, Value: cookie(c.sub)})
		}
		if c.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+c.bearer)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.status || (c.body != "" && !strings.Contains(rec.Body.String(), c.body)) {
			t.Errorf("%s: GET %s = %d %s, want %d containing %q", c.name, c.path, rec.Code, strings.TrimSpace(rec.Body.String()), c.status, c.body)
		}
		if c.bearer == "tok-p1" && rec.Code != 401 {
			for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
				var e map[string]any
				if json.Unmarshal([]byte(line), &e) == nil && e["scope"] == "api" && e["user_sub"] != "token:p-1" {
					t.Errorf("%s: access log user_sub = %v, want token:p-1", c.name, e["user_sub"])
				}
			}
		}
		for _, line := range strings.Split(strings.TrimSpace(logs.String()), "\n") {
			var e map[string]any
			if json.Unmarshal([]byte(line), &e) == nil && e["scope"] == "rbac" {
				t.Errorf("%s: core logged an rbac line although the Guard decides", c.name)
			}
		}
	}

	// A session store outage is a 503 from the app's Guard, not a 401.
	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	req.AddCookie(&http.Cookie{Name: o.names().SessionCookie, Value: goldenBrokenCookie})
	store.data[goldenBrokenCookie] = nil
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("store outage = %d, want 503", rec.Code)
	}
}
