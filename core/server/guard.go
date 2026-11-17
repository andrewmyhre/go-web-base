package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ErrScopedWithoutGuard is returned by Build when a route declares a scoped
// permission (ScopeProject or ScopeOrganization) and the router has no
// Guard. Core's own check only knows the global domain, so such a route
// would silently be decided there.
var ErrScopedWithoutGuard = errors.New("route declares a scoped permission but the router has no Guard")

// A Guard is the app's replacement for core's per-route check. When a
// router has one (Router.WithGuard, app.Options.Guard) it decides every
// route, built-ins and feature routes included, so it must handle Public,
// Authenticated and global permissions as core's own check does, and answer
// 503 when GuardRequest.SessionLookupFailed is set. Without one, core's
// check runs, unchanged.
type Guard interface {
	// Check returns Status 0 to let the request through, or the status and
	// detail to refuse it with. A status outside 400-599 other than 0 is a
	// bug in the Guard and is answered with 500.
	Check(ctx context.Context, req GuardRequest) GuardResult
}

// GuardRequest is what core knows about a request when it asks the Guard.
type GuardRequest struct {
	// Permission is the one the route declared.
	Permission Permission
	// Param reads a path parameter of the matched route; "" when absent.
	Param func(name string) string
	// Method is the HTTP method.
	Method string
	// Path is the matched route pattern as registered, for example
	// /api/v1/projects/{projectID}.
	Path string
	// Header reads a request header.
	Header func(name string) string
	// Query reads a query parameter.
	Query func(name string) string
	// SessionLookupFailed reports that the session cookie could not be
	// resolved because the session store failed, as opposed to being
	// unknown. The Guard must answer 503 for it, as core's check does, or
	// an outage reads as signed out.
	SessionLookupFailed bool
}

// GuardResult is the Guard's decision.
type GuardResult struct {
	// Ctx, when not nil, replaces the request context for the handler. It
	// must derive from the context passed to Check. The Guard uses it to
	// carry what it resolved (the project, the auth scope).
	Ctx context.Context
	// Status is 0 to let the request through, or the HTTP status to refuse
	// it with.
	Status int
	// Detail is the problem detail of a refusal.
	Detail string
	// Params rewrites path parameters before the handler runs, for example
	// a slug into its canonical id. It is applied only when Status is 0, and
	// only to parameters the matched route has.
	Params map[string]string
}

// decision is what a route's check came to, whichever check ran.
type decision struct {
	ctx    context.Context
	status int
	detail string
	params map[string]string
}

// decide runs the router's check for a route: the Guard when there is one,
// otherwise core's own.
func (g *guard) decide(ctx context.Context, p Permission, routePath string, r requestView) decision {
	if g.custom == nil {
		status, detail := g.check(ctx, p)
		return decision{ctx: ctx, status: status, detail: detail}
	}
	res := g.custom.Check(ctx, GuardRequest{
		Permission:          p,
		Param:               r.param,
		Method:              r.method,
		Path:                routePath,
		Header:              r.header,
		Query:               r.query,
		SessionLookupFailed: sessionLookupFailed(ctx),
	})
	d := decision{ctx: ctx, status: res.Status, detail: res.Detail, params: res.Params}
	if res.Status != 0 && (res.Status < 400 || res.Status > 599) {
		// A status outside 400-599 would let the request through with no
		// handler run (a 2xx or 3xx) or be unwritable. It is the Guard's
		// bug, and refusing is the safe reading.
		g.log.Error("guard returned a status outside 400-599; answering 500", zap.Int("status", res.Status), zap.String("path", routePath), zap.String("request_id", RequestIDFromContext(ctx)))
		d.status, d.detail, d.params = http.StatusInternalServerError, "internal error", nil
	}
	if res.Ctx != nil {
		d.ctx = res.Ctx
	}
	return d
}

// requestView is the part of a request a Guard may read, from either a Huma
// context or a plain http.Request.
type requestView struct {
	method string
	param  func(string) string
	header func(string) string
	query  func(string) string
}

func viewOfHuma(ctx huma.Context) requestView {
	return requestView{method: ctx.Method(), param: ctx.Param, header: ctx.Header, query: ctx.Query}
}

func viewOfRequest(r *http.Request) requestView {
	return requestView{
		method: r.Method,
		param:  func(name string) string { return chi.URLParam(r, name) },
		header: r.Header.Get,
		query:  func(name string) string { return r.URL.Query().Get(name) },
	}
}

// rewriteParams replaces the values of path parameters the matched route
// has, in both places a handler may read them from: chi's route context
// (chi.URLParam) and the request's path values (Request.PathValue, which
// Huma reads). chi reads the last entry of a key, so the last entry is the
// one rewritten. A name the route lacks is ignored.
func rewriteParams(r *http.Request, params map[string]string) {
	rc := chi.RouteContext(r.Context())
	if rc == nil {
		return
	}
	for name, value := range params {
		for i := len(rc.URLParams.Keys) - 1; i >= 0; i-- {
			if rc.URLParams.Keys[i] == name {
				rc.URLParams.Values[i] = value
				r.SetPathValue(name, value)
				break
			}
		}
	}
}
