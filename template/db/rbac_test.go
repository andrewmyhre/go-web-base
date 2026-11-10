package db

import (
	"bufio"
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/casbin/casbin/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

// hasPerm reports whether perms grants action on resource.
func hasPerm(perms []permission, resource, action string) bool {
	for _, p := range perms {
		if p.resource == resource && p.action == action {
			return true
		}
	}
	return false
}

// planRole is one role as docs/roles.md writes it.
type planRole struct {
	inherits []string
	perms    []string
}

// parsePlanRoles reads docs/roles.md: each
// "## Name (scope)" heading, its "Everything a X and a Y has" line, and its
// "- Resource: Verb" bullets.
func parsePlanRoles(t *testing.T) map[string]*planRole {
	t.Helper()
	f, err := os.Open("../docs/roles.md")
	if err != nil {
		t.Fatalf("open plan: %v", err)
	}
	defer func() { _ = f.Close() }()
	out := map[string]*planRole{}
	var cur *planRole
	in := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "# Roles":
			in = true
			continue
		case strings.HasPrefix(line, "# "):
			in = false
		}
		if !in {
			continue
		}
		if name, ok := strings.CutPrefix(line, "## "); ok {
			if i := strings.Index(name, " ("); i >= 0 {
				name = name[:i]
			}
			cur = &planRole{}
			out[name] = cur
			continue
		}
		if cur == nil {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "Everything a "); ok {
			if i := strings.Index(rest, " ha"); i >= 0 {
				rest = rest[:i]
			}
			rest = strings.ReplaceAll(rest, " and ", ", ")
			for _, n := range strings.Split(rest, ", ") {
				cur.inherits = append(cur.inherits, strings.TrimPrefix(strings.TrimSpace(n), "a "))
			}
			continue
		}
		if p, ok := strings.CutPrefix(line, "- "); ok {
			cur.perms = append(cur.perms, p)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read plan: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("parsed no roles in docs/roles.md; the parser is blind, not the plan empty")
	}
	return out
}

func sorted(s []string) []string {
	c := append([]string(nil), s...)
	sort.Strings(c)
	return c
}

// TestRolesMatchPlan proves the role table is docs/roles.md: the same roles,
// each with the same own permissions and the same parents. edit docs/roles.md
// and roles together.
func TestRolesMatchPlan(t *testing.T) {
	plan := parsePlanRoles(t)
	if len(plan) != len(roles) {
		t.Errorf("plan has %d roles, code has %d", len(plan), len(roles))
	}
	for _, r := range roles {
		p, ok := plan[r.name]
		if !ok {
			t.Errorf("role %q is not in the plan", r.name)
			continue
		}
		var own []string
		for _, perm := range r.permissions {
			own = append(own, perm.resource+": "+perm.action)
		}
		if got, want := sorted(own), sorted(p.perms); !reflect.DeepEqual(got, want) {
			t.Errorf("%s own permissions:\n got %v\nwant %v", r.name, got, want)
		}
		if got, want := sorted(r.inherits), sorted(p.inherits); !reflect.DeepEqual(got, want) {
			t.Errorf("%s inherits %v, plan says %v", r.name, got, want)
		}
	}
}

// TestFlatten pins inheritance per edge of the graph, and that a role does
// not reach up or across it.
func TestFlatten(t *testing.T) {
	cases := []struct {
		role, resource, verb string
		want                 bool
	}{
		{"Contributor", "Projects", "Read", true},
		{"Contributor", "Secrets", "Create", false},
		{"Contributor", "UserInvitation", "Create", false},
		{"Secrets Manager", "Secrets", "Delete", true},
		{"Secrets Manager", "Projects", "Read", false},
		{"Project Manager", "Projects", "Read", true},         // from Contributor
		{"Project Manager", "Secrets", "Patch", true},         // from Secrets Manager
		{"Project Manager", "UserInvitation", "Delete", true}, // own
		{"Project Manager", "Projects", "Create", false},
		{"Workflow Manager", "WorkflowLink", "Create", true},
		{"Workflow Manager", "AgentMessages", "Create", false},
		{"Workflow Manager", "Workflows", "Run", false},
		{"Manager", "UserInvitation", "Create", true}, // from Project Manager
		{"Manager", "Secrets", "Create", true},        // via Project Manager
		{"Manager", "Projects", "Delete", true},       // own
		{"Manager", "Workflows", "Create", false},     // not a Workflow Manager
		{"Manager", "Users", "Read", false},
		{"Admin", "Workflows", "Create", true}, // from Workflow Manager
		{"Admin", "Projects", "Delete", true},  // from Manager
		{"Admin", "Providers", "Delete", true}, // own
		{"Admin", "AccessReviews", "Create", true},
		{"Admin", "Secrets", "Read", false}, // no role reads secret values
	}
	for _, c := range cases {
		if got := hasPerm(flatten(c.role), c.resource, c.verb); got != c.want {
			t.Errorf("%s has %s: %s = %v, want %v", c.role, c.resource, c.verb, got, c.want)
		}
	}
}

func mustEnforcer(t *testing.T, pool *pgxpool.Pool, adminSub string) *casbin.Enforcer {
	t.Helper()
	e, err := NewEnforcer(context.Background(), pool, adminSub)
	if err != nil {
		t.Fatalf("NewEnforcer(%q): %v", adminSub, err)
	}
	return e
}

func bind(t *testing.T, e *casbin.Enforcer, sub, role, dom string) {
	t.Helper()
	if _, err := e.AddGroupingPolicy(sub, role, dom); err != nil {
		t.Fatalf("bind %s to %s in %s: %v", sub, role, dom, err)
	}
}

func wantAuth(t *testing.T, e *casbin.Enforcer, sub, proj, obj, act string, want bool) {
	t.Helper()
	ok, err := Authorize(e, sub, proj, obj, act)
	if err != nil {
		t.Errorf("Authorize(%s, %s, %s, %s): %v", sub, proj, obj, act, err)
		return
	}
	if ok != want {
		t.Errorf("Authorize(%s, %s, %s, %s) = %v, want %v", sub, proj, obj, act, ok, want)
	}
}

func casbinRows(t *testing.T, pool *pgxpool.Pool) (p, g int) {
	t.Helper()
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FILTER (WHERE ptype = 'p'), count(*) FILTER (WHERE ptype = 'g') FROM casbin_rule`).Scan(&p, &g)
	if err != nil {
		t.Fatalf("count casbin_rule: %v", err)
	}
	return p, g
}

// TestRBACScoping proves a project binding applies only in its own project.
func TestRBACScoping(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "")
	bind(t, e, "alice", "Contributor", "proj-1")
	bind(t, e, "bob", "Contributor", "proj-2")

	wantAuth(t, e, "alice", "proj-1", "AgentConversations", "Read", true)
	wantAuth(t, e, "alice", "proj-2", "AgentConversations", "Read", false)
	wantAuth(t, e, "alice", Global, "AgentConversations", "Read", false)
	wantAuth(t, e, "bob", "proj-2", "AgentConversations", "Read", true)
	wantAuth(t, e, "bob", "proj-1", "AgentConversations", "Read", false)
}

// TestRBACGlobalBinding proves a global binding of an ordinary role applies
// in every project.
func TestRBACGlobalBinding(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "")
	bind(t, e, "erin", "Contributor", Global)
	for _, proj := range []string{"proj-1", "proj-2", Global} {
		wantAuth(t, e, "erin", proj, "ProjectFiles", "Patch", true)
		wantAuth(t, e, "erin", proj, "Secrets", "Create", false)
	}
}

// TestRBACInheritance proves a bound user draws inherited permissions, but
// only in the project they are bound in.
func TestRBACInheritance(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "")
	bind(t, e, "carol", "Project Manager", "proj-9")

	wantAuth(t, e, "carol", "proj-9", "Requests", "Patch", true)        // Contributor
	wantAuth(t, e, "carol", "proj-9", "Secrets", "Delete", true)        // Secrets Manager
	wantAuth(t, e, "carol", "proj-9", "UserInvitation", "Create", true) // own
	wantAuth(t, e, "carol", "proj-9", "Projects", "Create", false)      // Manager's
	wantAuth(t, e, "carol", "proj-8", "Requests", "Patch", false)
}

// TestRBACDefaultDeny proves an unbound user, an unknown resource and an
// unknown verb are all denied.
func TestRBACDefaultDeny(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "admin-sub")
	wantAuth(t, e, "nobody", "proj-1", "Projects", "Read", false)
	wantAuth(t, e, "admin-sub", "proj-1", "Nonsense", "Read", false)
	wantAuth(t, e, "admin-sub", "proj-1", "Projects", "Nonsense", false)
	wantAuth(t, e, "admin-sub", "proj-1", "Secrets", "Read", false)
}

// TestAuthorizeRefusesRoleSubject proves a subject that is a role name gets
// nothing. Casbin links every name to itself, so without the guard a caller
// whose subject is "Admin" holds Admin with no binding at all.
func TestAuthorizeRefusesRoleSubject(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "")
	for _, r := range roles {
		ok, err := Authorize(e, r.name, "proj-1", "Projects", "Read")
		if ok || !errors.Is(err, ErrSubjectIsRole) {
			t.Errorf("Authorize(%q) = %v, %v; want false, ErrSubjectIsRole", r.name, ok, err)
		}
	}
	if ok, err := Authorize(e, "", "proj-1", "Projects", "Read"); ok || !errors.Is(err, ErrEmptySubject) {
		t.Errorf("Authorize(\"\") = %v, %v; want false, ErrEmptySubject", ok, err)
	}
}

// TestRBACGlobalAdmin proves the bootstrapped admin acts in every project.
func TestRBACGlobalAdmin(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "admin-sub")
	for _, proj := range []string{"proj-1", "proj-2", Global} {
		wantAuth(t, e, "admin-sub", proj, "Workflows", "Create", true)
		wantAuth(t, e, "admin-sub", proj, "Users", "Delete", true)
		wantAuth(t, e, "admin-sub", proj, "ProjectFiles", "Delete", true)
	}
}

// TestBootstrapRefusesBadAdminSub proves the bootstrap binds only an OIDC
// sub: not an email and not a role name.
func TestBootstrapRefusesBadAdminSub(t *testing.T) {
	pool, _ := testAdapter(t)
	for sub, want := range map[string]error{
		"andrew@example.com": ErrAdminSubIsEmail,
		"Admin":              ErrSubjectIsRole,
		"Contributor":        ErrSubjectIsRole,
	} {
		if _, err := NewEnforcer(context.Background(), pool, sub); !errors.Is(err, want) {
			t.Errorf("NewEnforcer(%q) err = %v, want %v", sub, err, want)
		}
	}
	if _, g := casbinRows(t, pool); g != 0 {
		t.Errorf("refused bootstraps left %d g rules, want 0", g)
	}
}

// TestBootstrapEmptySkips proves an empty admin subject binds nobody.
func TestBootstrapEmptySkips(t *testing.T) {
	pool, _ := testAdapter(t)
	mustEnforcer(t, pool, "")
	if _, g := casbinRows(t, pool); g != 0 {
		t.Errorf("g rules = %d, want 0", g)
	}
}

// TestRBACIdempotent proves rebuilding the enforcer over the same database
// writes nothing: the row counts do not move.
func TestRBACIdempotent(t *testing.T) {
	pool, _ := testAdapter(t)
	mustEnforcer(t, pool, "admin-sub")
	p1, g1 := casbinRows(t, pool)
	wantP := len(wantedPolicies())
	if p1 != wantP || g1 != 1 {
		t.Fatalf("first start: p=%d g=%d, want p=%d g=1", p1, g1, wantP)
	}
	e2 := mustEnforcer(t, pool, "admin-sub")
	if p2, g2 := casbinRows(t, pool); p2 != p1 || g2 != g1 {
		t.Errorf("restart: p=%d g=%d, want p=%d g=%d", p2, g2, p1, g1)
	}
	wantAuth(t, e2, "admin-sub", "proj-7", "Workflows", "Create", true)
}

// TestRoleDefinitionsReconcile proves a p rule the code no longer defines
// (a permission removed from a role, or one written by hand) is deleted on
// the next start, so it stops granting anything.
func TestRoleDefinitionsReconcile(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "")
	for _, rule := range [][]string{
		{"Contributor", "Users", "Delete", Global},
		{"Contributor", "Secrets", "Create", "proj-1"},
	} {
		if _, err := e.AddPolicy(rule); err != nil {
			t.Fatalf("seed stale %v: %v", rule, err)
		}
	}
	e2 := mustEnforcer(t, pool, "")
	bind(t, e2, "dave", "Contributor", "proj-1")
	wantAuth(t, e2, "dave", "proj-1", "Users", "Delete", false)
	wantAuth(t, e2, "dave", "proj-1", "Secrets", "Create", false)
	wantAuth(t, e2, "dave", "proj-1", "Projects", "Read", true)
	if p, _ := casbinRows(t, pool); p != len(wantedPolicies()) {
		t.Errorf("p rows = %d, want %d", p, len(wantedPolicies()))
	}
}

// TestHasRoleBinding proves the sign-in gate: a binding in any domain, a
// project's or the global one, counts, and a subject with none does not.
// The bootstrapped admin, bound only globally, has one. A subject that
// shares a prefix with a bound one, or that is a role name or empty, has
// none.
func TestHasRoleBinding(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "admin-sub")
	bind(t, e, "alice", "Contributor", "proj-a")
	bind(t, e, "erin", "Contributor", Global)

	for sub, want := range map[string]bool{
		"admin-sub": true, // the bootstrap binding, global
		"alice":     true, // project-scoped only
		"erin":      true, // global, ordinary role
		"nobody":    false,
		"alic":      false,
		"alice2":    false,
	} {
		got, err := HasRoleBinding(e, sub)
		if err != nil {
			t.Errorf("HasRoleBinding(%q): %v", sub, err)
			continue
		}
		if got != want {
			t.Errorf("HasRoleBinding(%q) = %v, want %v", sub, got, want)
		}
	}
	for _, r := range roles {
		if ok, err := HasRoleBinding(e, r.name); ok || !errors.Is(err, ErrSubjectIsRole) {
			t.Errorf("HasRoleBinding(%q) = %v, %v; want false, ErrSubjectIsRole", r.name, ok, err)
		}
	}
	if ok, err := HasRoleBinding(e, ""); ok || !errors.Is(err, ErrEmptySubject) {
		t.Errorf("HasRoleBinding(\"\") = %v, %v; want false, ErrEmptySubject", ok, err)
	}
}

// TestHasRoleBindingAfterRestart proves the gate reads bindings stored in
// Postgres, not only ones added to this enforcer in memory: a second
// enforcer over the same pool still finds them.
func TestHasRoleBindingAfterRestart(t *testing.T) {
	pool, _ := testAdapter(t)
	bind(t, mustEnforcer(t, pool, "admin-sub"), "alice", "Contributor", "proj-a")
	e := mustEnforcer(t, pool, "")
	for sub, want := range map[string]bool{"admin-sub": true, "alice": true, "nobody": false} {
		if got, err := HasRoleBinding(e, sub); err != nil || got != want {
			t.Errorf("after restart HasRoleBinding(%q) = %v, %v; want %v", sub, got, err, want)
		}
	}
}

// TestRoleBindingsOf proves /me reads roles from the bindings: every role a
// subject holds, in every domain, sorted by domain then role; none for an
// unbound subject or one sharing a prefix; and a refusal for an empty or
// role-name subject.
func TestRoleBindingsOf(t *testing.T) {
	pool, _ := testAdapter(t)
	e := mustEnforcer(t, pool, "admin-sub")
	bind(t, e, "alice", "Contributor", "proj-b")
	bind(t, e, "alice", "Manager", "proj-a")
	bind(t, e, "alice", "Contributor", "proj-a")
	bind(t, e, "erin", "Contributor", Global)

	for sub, want := range map[string][]Binding{
		"admin-sub": {{AdminRole, Global}},
		"alice":     {{"Contributor", "proj-a"}, {"Manager", "proj-a"}, {"Contributor", "proj-b"}},
		"erin":      {{"Contributor", Global}},
		"nobody":    {},
		"alic":      {},
	} {
		got, err := RoleBindingsOf(e, sub)
		if err != nil {
			t.Errorf("RoleBindingsOf(%q): %v", sub, err)
			continue
		}
		if got == nil || !reflect.DeepEqual(got, want) {
			t.Errorf("RoleBindingsOf(%q) = %#v, want %#v", sub, got, want)
		}
	}
	for _, r := range roles {
		if got, err := RoleBindingsOf(e, r.name); got != nil || !errors.Is(err, ErrSubjectIsRole) {
			t.Errorf("RoleBindingsOf(%q) = %v, %v; want nil, ErrSubjectIsRole", r.name, got, err)
		}
	}
	if got, err := RoleBindingsOf(e, ""); got != nil || !errors.Is(err, ErrEmptySubject) {
		t.Errorf("RoleBindingsOf(\"\") = %v, %v; want nil, ErrEmptySubject", got, err)
	}
}
