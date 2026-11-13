// Package db is the durable storage for the API.
//
// It owns the PostgreSQL connection and all data access, the generated
// queries, and the domain services that sit on top of them. It does not
// know about HTTP: the server package adapts the results it returns to the
// wire.
package db

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Global is the special Casbin domain. A role binding made in this domain
// applies in every project; a binding made in a project's own domain applies
// only in that project. Role definitions also live here: a binding in any
// domain draws the permissions of the role it is bound to, whatever that
// domain is, so a binding in the global domain is a wildcard the authorizer
// checks in addition to the request's project.
//
// A project must never be named "global": a binding in that project's
// domain would be a global binding. Project IDs are generated, not chosen.
const Global = "global"

// AdminRole is the role the configured admin user is bound to in the global
// domain. It is the most privileged role: it carries, through inheritance,
// every permission the other roles grant.
const AdminRole = "Admin"

// Errors returned for subjects that cannot be authorized or bootstrapped.
var (
	// ErrSubjectIsRole is returned when a subject is the name of a role.
	// Casbin treats every name as linked to itself, so a request whose
	// subject is "Admin" would otherwise draw the Admin role's permissions
	// with no binding at all.
	ErrSubjectIsRole = errors.New("subject is a role name")
	// ErrEmptySubject is returned when a subject is empty.
	ErrEmptySubject = errors.New("subject is empty")
	// ErrAdminSubIsEmail is returned when the configured admin is an email
	// address. The bootstrap binds the OIDC subject, never an email.
	ErrAdminSubIsEmail = errors.New("admin subject looks like an email address, want the OIDC sub")
)

// rbacModelString is the API's RBAC-with-domains model. The domain (r.dom) scopes
// a role binding to one project or to the global domain. It is deliberately
// not required to equal the domain of the role definition (p.dom), which is
// always Global: the binding's domain is where the user draws their
// permissions from, so scoping is enforced by the binding, and the authorizer
// also checks the global domain so a wildcard binding applies everywhere.
// Because p.dom is not matched, NewEnforcer removes every p rule that is not
// a role definition from the code below, so no stray rule can apply.
const rbacModelString = `
[request_definition]
r = sub, dom, obj, act

[policy_definition]
p = sub, obj, act, dom

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.obj == p.obj && r.act == p.act
`

// RoleDef is one role of the graph: the roles it inherits from and the
// permissions it grants on its own, as "Resource: Verb" strings (the form
// docs/roles.md uses for its bullets, and server.Permission.String gives).
//
// Inheritance is a graph, not a ladder: a role may inherit several roles,
// and a role's effective permissions are the union of its own and every role
// it inherits. Core ships no roles: the app supplies the graph in
// app.Options.Roles, and features add grants to its roles.
type RoleDef struct {
	Name        string
	Inherits    []string
	Permissions []string
}

// Errors returned for a role graph that cannot be enforced.
var (
	// ErrBadRoles is wrapped by every ValidateRoles failure.
	ErrBadRoles = errors.New("invalid role graph")
)

// ValidateRoles checks the structure of a role graph: every role is named
// once, inherits only roles that exist (with no cycle), has well formed
// permissions, and ends up with at least one effective permission; and the
// graph holds AdminRole, which the bootstrap binds the configured admin to.
// It does not know which resources exist: the caller checks each permission
// against the server's registry (server.Resources.CheckString).
func ValidateRoles(roles []RoleDef) error {
	seen := map[string]bool{}
	for _, r := range roles {
		if r.Name == "" {
			return fmt.Errorf("%w: a role has no name", ErrBadRoles)
		}
		if seen[r.Name] {
			return fmt.Errorf("%w: role %q is defined twice", ErrBadRoles, r.Name)
		}
		seen[r.Name] = true
		for _, p := range r.Permissions {
			if _, _, ok := strings.Cut(p, ": "); !ok {
				return fmt.Errorf("%w: role %q permission %q is not in the form \"Resource: Verb\"", ErrBadRoles, r.Name, p)
			}
		}
	}
	if !seen[AdminRole] {
		return fmt.Errorf("%w: no %q role, which the admin bootstrap needs", ErrBadRoles, AdminRole)
	}
	for _, r := range roles {
		for _, parent := range r.Inherits {
			if !seen[parent] {
				return fmt.Errorf("%w: role %q inherits unknown role %q", ErrBadRoles, r.Name, parent)
			}
		}
	}
	for _, r := range roles {
		if cyclic(roles, r.Name) {
			return fmt.Errorf("%w: role %q inherits itself", ErrBadRoles, r.Name)
		}
		if len(flatten(roles, r.Name)) == 0 {
			return fmt.Errorf("%w: role %q grants nothing", ErrBadRoles, r.Name)
		}
	}
	return nil
}

// cyclic reports whether name can reach itself through inherits.
func cyclic(roles []RoleDef, name string) bool {
	visited := map[string]bool{}
	var walk func(string) bool
	walk = func(n string) bool {
		for _, parent := range roleByName(roles, n).Inherits {
			if parent == name {
				return true
			}
			if visited[parent] {
				continue
			}
			visited[parent] = true
			if walk(parent) {
				return true
			}
		}
		return false
	}
	return walk(name)
}

func roleByName(roles []RoleDef, name string) *RoleDef {
	for i := range roles {
		if roles[i].Name == name {
			return &roles[i]
		}
	}
	return nil
}

// flatten returns name's full permission set: its own permissions plus the
// permissions of every role it (transitively) inherits, de-duplicated, own
// first then inherited. Collapsing the role graph into per-role permissions
// (instead of relying on Casbin to walk role-to-role links) is what lets a
// binding in any project draw the full set: Casbin's domain-scoped role
// inheritance would only reach links stored in the same domain, but a role's
// permissions are a fixed set regardless of where its members sit.
func flatten(roles []RoleDef, name string) []string {
	visited := map[string]bool{}
	seen := map[string]bool{}
	var out []string
	var visit func(string)
	visit = func(r string) {
		if visited[r] {
			return
		}
		visited[r] = true
		role := roleByName(roles, r)
		if role == nil {
			return
		}
		for _, p := range role.Permissions {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
		for _, parent := range role.Inherits {
			visit(parent)
		}
	}
	visit(name)
	return out
}

// RolePermissions returns every role's flattened permissions as
// "Resource: Verb" strings, sorted, keyed by role name. It is the role graph
// as the enforcer sees it, for checks outside this package.
func RolePermissions(roles []RoleDef) map[string][]string {
	out := make(map[string][]string, len(roles))
	for _, r := range roles {
		perms := append([]string(nil), flatten(roles, r.Name)...)
		sort.Strings(perms)
		out[r.Name] = perms
	}
	return out
}

// CloneRoles returns a deep copy of roles, so a caller can add grants
// without touching the original.
func CloneRoles(roles []RoleDef) []RoleDef {
	out := make([]RoleDef, len(roles))
	for i, r := range roles {
		out[i] = RoleDef{
			Name:        r.Name,
			Inherits:    append([]string(nil), r.Inherits...),
			Permissions: append([]string(nil), r.Permissions...),
		}
	}
	return out
}

// NewEnforcer builds the Casbin enforcer over the Casbin adapter for pool:
// the RBAC-with-domains model, the role definitions in roles (checked with
// ValidateRoles first), and, when
// adminSub is not empty, the binding of adminSub to the Admin role in the
// global domain.
//
// The role definitions in the database are reconciled to the code: missing
// ones are added and any other p rule is removed, so a permission dropped
// from a role stops applying on the next start. Running it again over the
// same pool (a restart) writes nothing. The admin binding is only ever
// added: changing adminSub does not revoke the previous admin.
func NewEnforcer(ctx context.Context, pool *pgxpool.Pool, adminSub string, roles []RoleDef) (*casbin.Enforcer, error) {
	if err := ValidateRoles(roles); err != nil {
		return nil, err
	}
	if adminSub != "" {
		if err := checkAdminSub(roles, adminSub); err != nil {
			return nil, fmt.Errorf("bootstrap admin %q: %w", adminSub, err)
		}
	}
	m, err := model.NewModelFromString(rbacModelString)
	if err != nil {
		return nil, fmt.Errorf("build rbac model: %w", err)
	}
	e, err := casbin.NewEnforcer(m, NewCasbinAdapter(pool))
	if err != nil {
		return nil, fmt.Errorf("new casbin enforcer: %w", err)
	}
	// casbin.NewEnforcer has already loaded the stored policy, so the
	// reconcile below writes only the difference: a restart is a no-op.
	if err := loadRoleDefinitions(e, roles); err != nil {
		return nil, fmt.Errorf("load role definitions: %w", err)
	}
	if adminSub != "" {
		if err := bindAdmin(e, adminSub); err != nil {
			return nil, fmt.Errorf("bootstrap admin %q: %w", adminSub, err)
		}
	}
	return e, nil
}

// checkAdminSub refuses an admin subject that cannot be an OIDC sub.
func checkAdminSub(roles []RoleDef, sub string) error {
	if roleByName(roles, sub) != nil {
		return ErrSubjectIsRole
	}
	if strings.Contains(sub, "@") {
		return ErrAdminSubIsEmail
	}
	return nil
}

// wantedPolicies is every role's flattened permissions as p rules, all in
// the global domain.
func wantedPolicies(roles []RoleDef) [][]string {
	var out [][]string
	for _, r := range roles {
		for _, p := range flatten(roles, r.Name) {
			res, verb, _ := strings.Cut(p, ": ")
			out = append(out, []string{r.Name, res, verb, Global})
		}
	}
	return out
}

func ruleKey(rule []string) string { return strings.Join(rule, "\x00") }

// loadRoleDefinitions reconciles e's p rules to wantedPolicies: it removes
// every p rule not in that set and adds every one missing from e.
func loadRoleDefinitions(e *casbin.Enforcer, roles []RoleDef) error {
	wanted := wantedPolicies(roles)
	want := make(map[string]bool, len(wanted))
	for _, rule := range wanted {
		want[ruleKey(rule)] = true
	}
	have, err := e.GetPolicy()
	if err != nil {
		return fmt.Errorf("read policy: %w", err)
	}
	for _, rule := range have {
		if want[ruleKey(rule)] {
			continue
		}
		if _, err := e.RemovePolicy(rule); err != nil {
			return fmt.Errorf("remove stale %v: %w", rule, err)
		}
	}
	for _, rule := range wanted {
		if _, err := e.AddPolicy(rule); err != nil {
			return fmt.Errorf("add %v: %w", rule, err)
		}
	}
	return nil
}

// bindAdmin binds adminSub to the Admin role in the global domain.
func bindAdmin(e *casbin.Enforcer, adminSub string) error {
	if _, err := e.AddGroupingPolicy(adminSub, AdminRole, Global); err != nil {
		return err
	}
	return nil
}

// refuseRoleName returns ErrSubjectIsRole when sub is a role of the loaded
// graph, which is any name the policy defines permissions for (ValidateRoles
// guarantees every role grants something).
func refuseRoleName(e *casbin.Enforcer, sub string) error {
	rules, err := e.GetFilteredPolicy(0, sub)
	if err != nil {
		return fmt.Errorf("read role definitions: %w", err)
	}
	if len(rules) > 0 {
		return fmt.Errorf("%w: %q", ErrSubjectIsRole, sub)
	}
	return nil
}

// locks holds one RWMutex per enforcer. A casbin.Enforcer is not safe for a
// write concurrent with a read, and BindRole writes at sign-in while every
// request reads, so Authorize, HasRoleBinding, RoleBindingsOf and BindRole
// take the enforcer's lock. A write made directly on the enforcer (the
// bootstrap, before serving) bypasses it, which is fine before any request.
var locks sync.Map // *casbin.Enforcer -> *sync.RWMutex

func lockOf(e *casbin.Enforcer) *sync.RWMutex {
	l, _ := locks.LoadOrStore(e, &sync.RWMutex{})
	return l.(*sync.RWMutex)
}

// ErrUnknownRole is returned by BindRole for a role the loaded graph does
// not define.
var ErrUnknownRole = errors.New("role is not defined")

// BindRole binds sub to role in the global domain, so sub passes the
// sign-in gate and holds the role everywhere. It is idempotent: binding a
// subject that already holds the role changes nothing and returns nil, so
// two concurrent first sign-ins of one subject both succeed and leave one
// binding. It refuses an empty subject, a subject that is a role name, and
// a role the graph does not define.
func BindRole(e *casbin.Enforcer, sub, role string) error {
	if sub == "" {
		return ErrEmptySubject
	}
	l := lockOf(e)
	l.Lock()
	defer l.Unlock()
	if err := refuseRoleName(e, sub); err != nil {
		return err
	}
	rules, err := e.GetFilteredPolicy(0, role)
	if err != nil {
		return fmt.Errorf("read role %q: %w", role, err)
	}
	if role == "" || len(rules) == 0 {
		return fmt.Errorf("%w: %q", ErrUnknownRole, role)
	}
	if _, err := e.AddGroupingPolicy(sub, role, Global); err != nil {
		return fmt.Errorf("bind %s to %s: %w", sub, role, err)
	}
	return nil
}

// Authorize reports whether sub may act on object with action in project. A
// binding in project's own domain applies there; a binding in the global
// domain applies in every project, so the global domain is checked in
// addition. It denies, with an error, an empty subject or one that is a role
// name. Every error is a denial.
func Authorize(e *casbin.Enforcer, sub, project, object, action string) (bool, error) {
	l := lockOf(e)
	l.RLock()
	defer l.RUnlock()
	if sub == "" {
		return false, ErrEmptySubject
	}
	if err := refuseRoleName(e, sub); err != nil {
		return false, err
	}
	ok, err := e.Enforce(sub, project, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce %s on %s/%s in %s: %w", sub, object, action, project, err)
	}
	if ok {
		return true, nil
	}
	ok, err = e.Enforce(sub, Global, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce %s on %s/%s in global: %w", sub, object, action, err)
	}
	return ok, nil
}

// HasRoleBinding reports whether sub is bound to any role in any domain:
// the global domain or a project's. It is the sign-in gate: a subject with
// no binding at all is refused before a session is created. It reads the
// enforcer's loaded policy, the same policy Authorize decides from. It
// refuses, with an error, an empty subject or one that is a role name, so
// a role can never sign in as itself. Every error is a refusal.
func HasRoleBinding(e *casbin.Enforcer, sub string) (bool, error) {
	l := lockOf(e)
	l.RLock()
	defer l.RUnlock()
	if sub == "" {
		return false, ErrEmptySubject
	}
	if err := refuseRoleName(e, sub); err != nil {
		return false, err
	}
	rules, err := e.GetFilteredGroupingPolicy(0, sub)
	if err != nil {
		return false, fmt.Errorf("read bindings of %s: %w", sub, err)
	}
	return len(rules) > 0, nil
}

// Binding is one role a subject is bound to, and the domain it is bound in:
// Global or a project's ID.
type Binding struct {
	Role   string
	Domain string
}

// RoleBindingsOf returns every role sub is bound to, in every domain,
// sorted by domain and then role. It reads the enforcer's loaded policy,
// the same policy Authorize decides from. A subject with no binding gets an
// empty, non-nil slice. Like HasRoleBinding it refuses, with an error, an
// empty subject or one that is a role name.
func RoleBindingsOf(e *casbin.Enforcer, sub string) ([]Binding, error) {
	l := lockOf(e)
	l.RLock()
	defer l.RUnlock()
	if sub == "" {
		return nil, ErrEmptySubject
	}
	if err := refuseRoleName(e, sub); err != nil {
		return nil, err
	}
	rules, err := e.GetFilteredGroupingPolicy(0, sub)
	if err != nil {
		return nil, fmt.Errorf("read bindings of %s: %w", sub, err)
	}
	out := make([]Binding, 0, len(rules))
	for _, r := range rules {
		// A g rule is (subject, role, domain).
		if len(r) < 3 {
			return nil, fmt.Errorf("binding of %s has %d fields, want 3: %v", sub, len(r), r)
		}
		out = append(out, Binding{Role: r[1], Domain: r[2]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Domain != out[j].Domain {
			return out[i].Domain < out[j].Domain
		}
		return out[i].Role < out[j].Role
	})
	return out, nil
}
