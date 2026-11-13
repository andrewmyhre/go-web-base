package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/casbin/casbin/v2"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/logging"
)

// Authorizer decides whether a signed-in subject (the Google account ID)
// may act on object with action in domain. Every error is a denial.
type Authorizer interface {
	Authorize(sub, domain, object, action string) (bool, error)
}

// CasbinAuthorizer is the Authorizer over the Casbin enforcer in db.
type CasbinAuthorizer struct{ Enforcer *casbin.Enforcer }

// Authorize implements Authorizer with db.Authorize.
func (a CasbinAuthorizer) Authorize(sub, domain, object, action string) (bool, error) {
	return db.Authorize(a.Enforcer, sub, domain, object, action)
}

// RoleBindings reports whether a subject is bound to any role in any
// domain. Sign-in asks it before minting a session: a subject with no
// binding is refused. An error is a refusal.
type RoleBindings interface {
	HasRoleBinding(sub string) (bool, error)
}

// HasRoleBinding implements RoleBindings with db.HasRoleBinding.
func (a CasbinAuthorizer) HasRoleBinding(sub string) (bool, error) {
	return db.HasRoleBinding(a.Enforcer, sub)
}

// RoleBinder binds a subject to a role. First sign-in asks it to give a new
// subject the role the app chose. It must be idempotent: binding a subject
// that already holds the role is not an error.
type RoleBinder interface {
	BindRole(sub, role string) error
}

// BindRole implements RoleBinder with db.BindRole.
func (a CasbinAuthorizer) BindRole(sub, role string) error {
	return db.BindRole(a.Enforcer, sub, role)
}

// RoleReader lists the roles a subject is bound to, in every domain. GET
// /me reads it so the account view shows the roles the user really holds.
type RoleReader interface {
	RoleBindingsOf(sub string) ([]db.Binding, error)
}

// RoleBindingsOf implements RoleReader with db.RoleBindingsOf.
func (a CasbinAuthorizer) RoleBindingsOf(sub string) ([]db.Binding, error) {
	return db.RoleBindingsOf(a.Enforcer, sub)
}

// authenticatedObject is the object an rbac line names for an
// Authenticated route, which needs a session but no role.
const authenticatedObject = "authenticated"

// guard is the authorization check every route runs before its handler.
// It logs one rbac-scope line for every route that is not Public, allow or
// deny, so a refused request is always visible in Loki.
type guard struct {
	authorizer Authorizer
	log        *zap.Logger
}

// check returns 0 to let the request through, or the status and detail to
// refuse it with:
//
//   - Public: always through, no rbac line.
//   - the session store failed: 503, so an outage never reads as signed out.
//   - no session: 401, and an rbac deny line with an empty subject.
//   - Authenticated: through, with an rbac allow line.
//   - a role permission: the authorizer decides in the global domain (no
//     route is project-scoped yet); a denial or an error is 403.
func (g *guard) check(ctx context.Context, p Permission) (int, string) {
	if p.IsPublic() {
		return 0, ""
	}
	if sessionLookupFailed(ctx) {
		return http.StatusServiceUnavailable, "the session store is unavailable; try again"
	}
	object, action := string(p.Resource), string(p.Verb)
	if p.IsAuthenticated() {
		object, action = authenticatedObject, ""
	}
	decision := logging.RBACDecision{
		Domain:    db.Global,
		Object:    object,
		Action:    action,
		Decision:  logging.Deny,
		RequestID: RequestIDFromContext(ctx),
	}
	rbac := logging.New(g.log)

	s, ok := SessionFromContext(ctx)
	if !ok {
		rbac.Check(decision)
		return http.StatusUnauthorized, "not signed in"
	}
	decision.Subject = s.UserID
	if p.IsAuthenticated() {
		decision.Decision = logging.Allow
		rbac.Check(decision)
		return 0, ""
	}
	allowed, err := g.authorizer.Authorize(s.UserID, db.Global, object, action)
	if err != nil {
		g.log.Warn("rbac check failed", zap.String("request_id", decision.RequestID), zap.Error(err))
	}
	if err != nil || !allowed {
		rbac.Check(decision)
		return http.StatusForbidden, "you do not have " + p.String()
	}
	decision.Decision = logging.Allow
	rbac.Check(decision)
	return 0, ""
}

// writeProblem writes an RFC 9457 problem body, the shape Huma uses for
// its own errors, for routes that are not Huma operations.
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Title  string `json:"title"`
		Status int    `json:"status"`
		Detail string `json:"detail"`
	}{http.StatusText(status), status, detail})
}
