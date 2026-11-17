package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
)

type ctxKey int

const (
	permissionKey ctxKey = iota
	requestInfoKey
	sessionKey
)

// PermissionFromContext returns the declared permission of the route
// handling the request. ok is false outside a route.
func PermissionFromContext(ctx context.Context) (p Permission, ok bool) {
	p, ok = ctx.Value(permissionKey).(Permission)
	return p, ok
}

// requestInfo is filled in by inner layers for the access log.
type requestInfo struct {
	permission Permission
	set        bool
	userSub    string
}

func noteRequestPermission(ctx context.Context, p Permission) context.Context {
	if info, ok := ctx.Value(requestInfoKey).(*requestInfo); ok {
		info.permission, info.set = p, true
	}
	return context.WithValue(ctx, permissionKey, p)
}

// withPermission is the Huma middleware every operation carries: it puts
// the declared permission on the request context, where the access log can
// read it, and then runs the authorization check, answering 401, 403 or 503
// itself when the check refuses the request. path is the route pattern as
// registered.
func withPermission(api huma.API, g *guard, p Permission, path string) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		rctx := noteRequestPermission(ctx.Context(), p)
		d := g.decide(rctx, p, path, viewOfHuma(ctx))
		if d.status != 0 {
			_ = huma.WriteErr(api, ctx, d.status, d.detail)
			return
		}
		if len(d.params) > 0 {
			r, _ := humachi.Unwrap(ctx)
			rewriteParams(r, d.params)
		}
		next(huma.WithContext(ctx, d.ctx))
	}
}

// setPermission is the chi equivalent, for routes that are not Huma
// operations (the OpenAPI spec and the sign-in redirects).
func setPermission(g *guard, p Permission, path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := noteRequestPermission(r.Context(), p)
			d := g.decide(ctx, p, path, viewOfRequest(r))
			if d.status != 0 {
				writeProblem(w, d.status, d.detail)
				return
			}
			rewriteParams(r, d.params)
			next.ServeHTTP(w, r.WithContext(d.ctx))
		})
	}
}

// noteRequestSession puts the session on the request context and records
// the signed-in user's account ID for the access log.
func noteRequestSession(ctx context.Context, s *Session) context.Context {
	if info, ok := ctx.Value(requestInfoKey).(*requestInfo); ok {
		info.userSub = s.UserID
	}
	return context.WithValue(ctx, sessionKey, s)
}

// NoteRequestUser records sub as the user of the request in the access log
// (user_sub). The session middleware does it for a cookie session; app
// Middleware or a Guard calls it for a bearer token, which core cannot
// resolve itself. It does nothing outside a request that core is serving, and
// the last call wins. It does not authenticate anyone.
func NoteRequestUser(ctx context.Context, sub string) {
	if info, ok := ctx.Value(requestInfoKey).(*requestInfo); ok {
		info.userSub = sub
	}
}

// SessionFromContext returns the session of the request, if a signed-in
// user's cookie was presented and resolved.
func SessionFromContext(ctx context.Context) (*Session, bool) {
	s, ok := ctx.Value(sessionKey).(*Session)
	return s, ok && s != nil
}

// sessionErrorKey marks a request whose session cookie could not be
// resolved because the store failed, as opposed to an unknown cookie.
type sessionErrorKey struct{}

func noteSessionError(ctx context.Context) context.Context {
	return context.WithValue(ctx, sessionErrorKey{}, true)
}

func sessionLookupFailed(ctx context.Context) bool {
	failed, _ := ctx.Value(sessionErrorKey{}).(bool)
	return failed
}
