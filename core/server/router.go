// Package server implements the the API HTTP API on chi and Huma.
//
// Every route in the API declares the permission a caller needs to invoke
// it. A route without a declared permission is refused: it cannot be
// registered and the server will not start with one, so an unprotected
// route cannot exist.
//
// The package-level Register function is the only way to add an operation.
// The chi router and the huma.API are created inside Router.Build and are
// never handed to callers (Build returns a plain http.Handler that does not
// expose them), so nothing outside this package can register an unchecked
// route. Huma's own automatic routes (OpenAPI, docs, schemas) are switched
// off; the OpenAPI spec is served at GET /openapi.json as an ordinary
// registration declared server.Public. As a last check,
// Build walks the chi router and refuses to start if it holds any route
// that did not come through a permission-checked registration.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

// ErrUncheckedRoute is returned by Build when the router holds a route that
// did not come through a permission-checked registration, for example one
// Huma registered automatically.
var ErrUncheckedRoute = errors.New("route was not registered through the permission check")

// ErrNoSessionCookie is returned by Build when the router has a session
// store but no cookie name to find the session id in.
var ErrNoSessionCookie = errors.New("router has a session store but no session cookie name")

// ErrNoAuth is returned by Build when a route needs a signed-in user (any
// permission but Public) and the router has no session store, or needs a
// role and the router has no authorizer. Such a route could not be
// checked, so the server refuses to start rather than serve it.
var ErrNoAuth = errors.New("route needs authorization but the router has no session store or authorizer")

// OpenAPIPath serves the generated OpenAPI 3.1 spec as JSON. It is Public:
// the spec describes the API, and every operation in it is still checked.
const OpenAPIPath = "/openapi.json"

// MetadataPermission is the huma.Operation Metadata key holding an
// operation's declared Permission value, so it is visible to Huma hooks.
const MetadataPermission = "permission"

// Operation describes one API operation. Permission is what a caller must
// hold to invoke it, or Public. It is required: an operation without a
// valid one is refused and never served.
type Operation struct {
	OperationID string
	Method      string
	Path        string
	Summary     string
	Description string
	Tags        []string
	Permission  Permission
}

// registration is one checked route waiting to be mounted by Build.
type registration struct {
	method, path, operationID string
	permission                Permission
	// mount adds the route to the API being built. g is the authorization
	// check the route must run before its handler.
	mount func(api huma.API, mux chi.Router, g *guard)
}

// Router collects the API's operations.
type Router struct {
	title, version string
	regs           []registration
	// sessions resolves session cookies; authorizer decides role
	// permissions. See WithAuth.
	sessions      SessionStore
	authorizer    Authorizer
	sessionCookie string
	// resources are the resources a route's permission may name. Build
	// checks every route against it.
	resources *Resources
}

// NewRouter returns a router holding only the OpenAPI spec route. title and
// version describe the API in its OpenAPI document.
func NewRouter(title, version string) *Router {
	r := &Router{title: title, version: version, resources: NewResources()}
	r.regs = append(r.regs, specRegistration())
	return r
}

// Resources is the registry of resources this router accepts permissions
// for. Register a resource before Build, in any order relative to the routes
// that use it: Build refuses a route whose permission names a resource or
// verb that is not registered.
func (r *Router) Resources() *Resources { return r.resources }

// WithResources replaces the router's registry with rs, for an app that
// assembles its resources (and checks its role graph against them) before it
// has a router. A nil rs is ignored.
func (r *Router) WithResources(rs *Resources) *Router {
	if rs != nil {
		r.resources = rs
	}
	return r
}

// WithAuth sets the session store the session cookie is resolved against
// and the authorizer that decides role permissions. A router without a
// session store can serve Public routes only, and one without an
// authorizer cannot serve a route that needs a role: Build refuses either
// with ErrNoAuth. A nil argument leaves that half unset. sessionCookie is
// the name of the cookie that carries the session id; a session store
// without one is refused by Build with ErrNoSessionCookie.
func (r *Router) WithAuth(sessions SessionStore, authorizer Authorizer, sessionCookie string) *Router {
	r.sessions = sessions
	r.sessionCookie = sessionCookie
	r.authorizer = authorizer
	return r
}

// Register adds an operation, served by handler, to r. Registration is
// refused for an operation that does not declare a permission, so an
// unprotected route can never exist in the router.
func Register[I, O any](r *Router, op Operation, handler func(context.Context, *I) (*O, error)) error {
	reg := registration{
		method:      op.Method,
		path:        op.Path,
		permission:  op.Permission,
		operationID: op.OperationID,
	}
	if handler == nil {
		return fmt.Errorf("route %s %s: missing handler", op.Method, op.Path)
	}
	permission := op.Permission
	reg.mount = func(api huma.API, _ chi.Router, g *guard) {
		huma.Register(api, huma.Operation{
			OperationID: op.OperationID,
			Method:      op.Method,
			Path:        op.Path,
			Summary:     op.Summary,
			Description: op.Description,
			Tags:        op.Tags,
			Metadata:    map[string]any{MetadataPermission: permission},
			Middlewares: huma.Middlewares{withPermission(api, g, permission)},
		}, handler)
	}
	return r.add(reg)
}

// RegisterHandler adds a plain http.Handler route. It is for routes that
// are not Huma operations: redirects and cookies, like the Google sign-in
// flow. Like Register, registration is refused unless op declares a valid
// permission, so an unprotected route can never exist.
func RegisterHandler(r *Router, op Operation, h http.Handler) error {
	if h == nil {
		return fmt.Errorf("route %s %s: missing handler", op.Method, op.Path)
	}
	reg := registration{
		method:      op.Method,
		path:        op.Path,
		permission:  op.Permission,
		operationID: op.OperationID,
	}
	reg.mount = func(_ huma.API, mux chi.Router, g *guard) {
		mux.With(setPermission(g, op.Permission)).Method(op.Method, op.Path, h)
	}
	return r.add(reg)
}

func (r *Router) add(reg registration) error {
	if err := checkRoute(reg); err != nil {
		return err
	}
	for _, existing := range r.regs {
		if existing.method == reg.method && routeShape(existing.path) == routeShape(reg.path) {
			// chi silently replaces a route whose pattern differs only in
			// parameter names, so these are refused as duplicates too.
			return fmt.Errorf("route %s %s conflicts with %s %s", reg.method, reg.path, existing.method, existing.path)
		}
		if reg.operationID != "" && existing.operationID == reg.operationID {
			return fmt.Errorf("operation id %q already registered for %s %s",
				reg.operationID, existing.method, existing.path)
		}
	}
	r.regs = append(r.regs, reg)
	return nil
}

// RouteInfo describes a registered route.
type RouteInfo struct {
	Method      string
	Path        string
	OperationID string
	Permission  Permission
}

// Routes returns the registered routes, in registration order, including
// the OpenAPI spec route.
func (r *Router) Routes() []RouteInfo {
	out := make([]RouteInfo, 0, len(r.regs))
	for _, reg := range r.regs {
		out = append(out, RouteInfo{Method: reg.method, Path: reg.path, OperationID: reg.operationID, Permission: reg.permission})
	}
	return out
}

// humaConfig is the Huma configuration Build uses. It is a variable only so
// a test can prove Build refuses Huma's automatic routes if they come back.
var humaConfig = func(title, version string) huma.Config {
	cfg := huma.DefaultConfig(title, version)
	// Huma would register these routes itself, outside Register, so they
	// are switched off. The spec is served by specRegistration instead.
	cfg.OpenAPIPath = ""
	cfg.DocsPath = ""
	cfg.SchemasPath = ""
	// The default create hook adds $schema links pointing at SchemasPath,
	// which is no longer served.
	cfg.CreateHooks = nil
	return cfg
}

// Build compiles the registered routes into the handler the server will
// serve. It re-checks every route and refuses to build if any of them
// lacks a declared permission, so a server holding an unprotected route
// cannot start. A route the router would reject (a duplicate, or a path
// that conflicts with another) is returned as an error rather than a panic.
// Finally it refuses any route in the router that no checked registration
// accounts for.
func (r *Router) Build(log *zap.Logger) (http.Handler, error) {
	_, h, err := r.build(log)
	return h, err
}

// build is Build, also returning the huma.API for in-package tests. The
// API never leaves this package.
func (r *Router) build(log *zap.Logger) (huma.API, http.Handler, error) {
	if log == nil {
		log = zap.NewNop()
	}
	for _, reg := range r.regs {
		if err := checkRoute(reg); err != nil {
			return nil, nil, err
		}
		if err := r.resources.Check(reg.permission); err != nil {
			return nil, nil, fmt.Errorf("route %s %s: %w", reg.method, reg.path, err)
		}
		if err := r.checkAuth(reg); err != nil {
			return nil, nil, err
		}
	}
	if r.sessions != nil && r.sessionCookie == "" {
		return nil, nil, ErrNoSessionCookie
	}
	g := &guard{authorizer: r.authorizer, log: log}

	mux := chi.NewRouter()
	// accessLog is outermost so it still logs a request whose handler
	// panicked and was turned into a 500 by recovery. chi's RealIP
	// middleware is deliberately not used: proxy headers are not trusted.
	mux.Use(accessLog(log), recovery(log))
	// The session middleware is inside accessLog so a signed-in user's
	// account ID lands in the access log.
	if r.sessions != nil {
		mux.Use(sessionMiddleware(r.sessions, r.sessionCookie, log))
	}

	var api huma.API
	if err := catch("create API", func() {
		api = humachi.New(mux, humaConfig(r.title, r.version))
	}); err != nil {
		return nil, nil, err
	}

	checked := make(map[string]bool, len(r.regs))
	for _, reg := range r.regs {
		if err := catch(fmt.Sprintf("route %s %s", reg.method, reg.path), func() {
			reg.mount(api, mux, g)
		}); err != nil {
			return nil, nil, err
		}
		checked[reg.method+" "+reg.path] = true
	}

	mounted := make(map[string]bool, len(checked))
	if err := chi.Walk(mux, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !checked[method+" "+route] {
			return fmt.Errorf("%w: %s %s", ErrUncheckedRoute, method, route)
		}
		mounted[method+" "+route] = true
		return nil
	}); err != nil {
		return nil, nil, err
	}
	// chi does not panic on a conflicting pattern: it silently replaces the
	// earlier route. A checked route missing from the router was replaced.
	for _, reg := range r.regs {
		if !mounted[reg.method+" "+reg.path] {
			return nil, nil, fmt.Errorf("route %s %s conflicts with another route and was replaced", reg.method, reg.path)
		}
	}
	return api, handler{mux: mux}, nil
}

// catch runs fn, turning a panic (chi and Huma panic on bad or conflicting
// routes) into an error.
func catch(what string, fn func()) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("%s: %v", what, p)
		}
	}()
	fn()
	return nil
}

// specRegistration serves the generated OpenAPI document. It is rendered on
// first request, after every operation has been added to the API.
func specRegistration() registration {
	return registration{
		method:     http.MethodGet,
		path:       OpenAPIPath,
		permission: Public,
		mount: func(api huma.API, mux chi.Router, g *guard) {
			var (
				once sync.Once
				body []byte
				err  error
			)
			mux.With(setPermission(g, Public)).Get(OpenAPIPath, func(w http.ResponseWriter, _ *http.Request) {
				once.Do(func() { body, err = json.Marshal(api.OpenAPI()) })
				if err != nil {
					http.Error(w, "render OpenAPI spec", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/openapi+json")
				_, _ = w.Write(body)
			})
		},
	}
}

// handler serves a router without exposing it, so callers of Build cannot
// type-assert their way back to the chi router and add unchecked routes.
type handler struct{ mux *chi.Mux }

func (h handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	h.mux.ServeHTTP(w, req)
}

var routeParam = regexp.MustCompile(`\{[^}]*\}`)

// routeShape is path with every {param} reduced to {}, so two patterns that
// differ only in parameter names compare equal.
func routeShape(path string) string {
	return routeParam.ReplaceAllString(path, "{}")
}

// checkAuth refuses a route the router could not authorize.
func (r *Router) checkAuth(reg registration) error {
	p := reg.permission
	if p.IsPublic() {
		return nil
	}
	if r.sessions == nil {
		return fmt.Errorf("route %s %s (%s): %w", reg.method, reg.path, p, ErrNoAuth)
	}
	if !p.IsAuthenticated() && r.authorizer == nil {
		return fmt.Errorf("route %s %s (%s): %w", reg.method, reg.path, p, ErrNoAuth)
	}
	return nil
}

func checkRoute(reg registration) error {
	if err := reg.permission.Validate(); err != nil {
		return fmt.Errorf("route %s %s: %w", reg.method, reg.path, err)
	}
	if reg.method == "" {
		return fmt.Errorf("route %s: missing method", reg.path)
	}
	if !strings.HasPrefix(reg.path, "/") {
		return fmt.Errorf("route %s %q: path must begin with /", reg.method, reg.path)
	}
	if reg.mount == nil {
		return fmt.Errorf("route %s %s: missing handler", reg.method, reg.path)
	}
	return nil
}
