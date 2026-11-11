// Package app is the whole API program, minus what an app supplies: the
// cobra root with the serve and openapi commands, configuration, the Redis
// and Postgres connections, migrations, the Casbin enforcer and admin
// bootstrap, the built-in routes (health, /me, Google sign-in, logout) and
// serving. An app's main calls Main with its Options: its role graph, its
// own routes and migrations, and the optional Features it installs.
//
//	myapp serve [--addr :8080] [--log-level info]
//	myapp openapi > ui/openapi.json
//
// Configuration is also read from <EnvPrefix>_ADDR, <EnvPrefix>_LOG_LEVEL
// and the other keys in package config; a flag set on the command line wins
// over the environment.
package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/logger"
	"github.com/andrewmyhre/go-web-base/core/redis"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// Options are the values core cannot know: everything specific to one app.
type Options struct {
	// Name is the app's slug. It is the binary name, the log "service"
	// field, the OpenAPI title and the app named in the sign-in refusal
	// ("you cannot sign in to <Name>"). Required.
	Name string
	// Version is the API version in the OpenAPI document. Defaults to
	// "0.0.0".
	Version string
	// EnvPrefix prefixes every environment variable, e.g. "MY_APP" reads
	// MY_APP_ADDR. Required.
	EnvPrefix string
	// CookiePrefix prefixes the cookie names and the Redis keys, e.g.
	// "my_app" gives the cookies my_app_session and my_app_oauth_state and
	// the keys my_app:session: and my_app:signin:. It must be a
	// cookie-safe token (letters, digits, underscore). Required.
	CookiePrefix string
	// Roles is the app's role graph: the roles, what each inherits and the
	// permissions it grants, as "Resource: Verb" strings (see db.RoleDef).
	// Core ships none. It must define db.AdminRole, which the admin
	// bootstrap binds, and every permission must name a resource the app or
	// a feature registered, or startup fails. Features add grants to these
	// roles with Installer.Grant. Required.
	Roles []db.RoleDef
	// Resources are the resources the app's routes and roles name, with any
	// verbs beyond Read, Create, Patch and Delete. Features register their
	// own through Installer.Resource.
	Resources []server.ResourceDef
	// Migrations holds the app's own goose migrations (*.sql at the root of
	// the FS; use fs.Sub on an embed.FS). They run last, after core's and
	// the features', in the version table goose_app_version. Optional.
	Migrations fs.FS
	// Features are the optional modules this app installs, in order. The
	// order is the order their migrations run and their routes register.
	Features []Feature
	// Routes registers the app's own routes. It is optional, and runs after
	// the built-ins, with the router already holding the session store and
	// authorizer when they are configured. Every route must go through
	// server.Register (or RegisterHandler) with a declared permission, as
	// the built-ins do; Build refuses one that does not. The same
	// function runs for the openapi command, with stand-in Deps, so the
	// routes must not do work at registration other than registering.
	Routes func(r *server.Router, d Deps) error
}

// validate fills defaults and refuses an Options core cannot run with.
func (o Options) validate() (Options, error) {
	if o.Name == "" {
		return o, errors.New("app.Options.Name is required")
	}
	if o.EnvPrefix == "" {
		return o, errors.New("app.Options.EnvPrefix is required")
	}
	if o.CookiePrefix == "" {
		return o, errors.New("app.Options.CookiePrefix is required")
	}
	if o.Version == "" {
		o.Version = "0.0.0"
	}
	if len(o.Roles) == 0 {
		return o, errors.New("app.Options.Roles is required: core ships no roles, the app supplies its role graph")
	}
	return o, nil
}

func (o Options) names() server.Names { return server.NewNames(o.Name, o.CookiePrefix) }

// Main runs the app's CLI and exits non-zero on error.
func Main(o Options) {
	if err := NewRootCmd(o).Execute(); err != nil {
		// Cobra has already printed the error.
		os.Exit(1)
	}
}

// NewRootCmd is the app's cobra root with the serve and openapi
// subcommands. An invalid Options makes every subcommand fail with the
// reason.
func NewRootCmd(o Options) *cobra.Command {
	root := &cobra.Command{
		Use:           o.Name,
		Short:         "The " + o.Name + " API",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	// Features are installed now, before cobra parses a flag, because their
	// settings become flags of the serve command. A failure is kept and
	// returned by every subcommand.
	inst, err := newInstallation(o)
	root.AddCommand(newServeCmd(o, inst, err), newOpenAPICmd(inst, err))
	return root
}

func newServeCmd(o Options, inst *installation, instErr error) *cobra.Command {
	v := config.NewViper(o.EnvPrefix)
	if inst != nil {
		v = inst.v
	}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if instErr != nil {
				return instErr
			}
			cfg, err := config.Load(v)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return serve(ctx, inst, cfg)
		},
	}
	if err := config.BindFlags(cmd.Flags(), v, o.EnvPrefix); err != nil {
		panic(err) // a programming error: the flags are defined just above
	}
	if inst != nil {
		if err := inst.addFlags(cmd.Flags()); err != nil {
			panic(err) // Install declared the flags; a clash was refused there
		}
	}
	return cmd
}

// newOpenAPICmd prints the OpenAPI document of the full API, every route
// registered, without connecting to anything. An app commits this output
// (ui/openapi.json) and generates its UI's typed client from it.
func newOpenAPICmd(inst *installation, instErr error) *cobra.Command {
	return &cobra.Command{
		Use:   "openapi",
		Short: "Print the API's OpenAPI document as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if instErr != nil {
				return instErr
			}
			spec, err := openAPISpec(inst)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(spec)
			return err
		},
	}
}

// OpenAPISpec renders GET /openapi.json for a router with every route
// registered, indented and ending in a newline so the committed copy diffs
// cleanly. The stores are stand-ins that are never called: rendering the
// spec runs no handler but the spec's own. The Google sign-in routes are
// plain handlers, not Huma operations, so they are not in the document;
// logout is an operation, so the router gets a stand-in Auth that is
// never called either. An app tests that its committed spec equals this.
func OpenAPISpec(o Options) ([]byte, error) {
	inst, err := newInstallation(o)
	if err != nil {
		return nil, err
	}
	return openAPISpec(inst)
}

func openAPISpec(inst *installation) ([]byte, error) {
	o := inst.o
	google, err := server.NewGoogle(server.GoogleConfig{
		ClientID: "spec-only", ClientSecret: "spec-only", RedirectURL: "https://spec.invalid" + server.AuthCallbackPath,
	})
	if err != nil {
		return nil, err
	}
	store := specOnlyStore{}
	auth := server.NewAuth(google, store, store, specOnlyAuthorizer{}, nil, "spec-only", o.names())
	router, err := newRouter(inst, Deps{Sessions: store, Authorizer: specOnlyAuthorizer{}, Auth: auth})
	if err != nil {
		return nil, err
	}
	h, err := router.Build(nil)
	if err != nil {
		return nil, err
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, server.OpenAPIPath, nil))
	if rec.Code != http.StatusOK {
		return nil, fmt.Errorf("GET %s = %d: %s", server.OpenAPIPath, rec.Code, rec.Body)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, rec.Body.Bytes(), "", "  "); err != nil {
		return nil, fmt.Errorf("indent spec: %w", err)
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// errSpecOnly is what the stand-in stores answer if anything ever calls them.
var errSpecOnly = errors.New("spec-only store: not for serving")

type specOnlyStore struct{}

func (specOnlyStore) Create(context.Context, time.Duration, map[string]string) (string, error) {
	return "", errSpecOnly
}
func (specOnlyStore) Get(context.Context, string) (map[string]string, error) { return nil, errSpecOnly }
func (specOnlyStore) Take(context.Context, string) (map[string]string, error) {
	return nil, errSpecOnly
}
func (specOnlyStore) Delete(context.Context, string) error { return errSpecOnly }

type specOnlyAuthorizer struct{}

func (specOnlyAuthorizer) Authorize(string, string, string, string) (bool, error) {
	return false, errSpecOnly
}
func (specOnlyAuthorizer) HasRoleBinding(string) (bool, error) { return false, errSpecOnly }

// HealthOutput is the response of GET /health.
type HealthOutput struct {
	Body struct {
		Status string `json:"status" example:"ok" doc:"Always ok while the server is serving."`
	}
}

func health(context.Context, *struct{}) (*HealthOutput, error) {
	out := &HealthOutput{}
	out.Body.Status = "ok"
	return out, nil
}

// Deps are the stores and services the routes need. Each may be nil, and
// a built-in route whose dependency is missing is not registered. An app's
// Routes function receives the same value, so it can use the session
// store, the authorizer and the logger.
type Deps struct {
	// Sessions holds signed-in sessions; without it only /health is served.
	Sessions server.SessionStore
	// Authorizer decides role permissions (Casbin over Postgres).
	Authorizer server.Authorizer
	// Roles lists a user's role bindings for /me (Casbin over Postgres).
	Roles server.RoleReader
	// Auth is the Google sign-in flow; without it there is no way to sign
	// in, though /me still answers (401).
	Auth *server.Auth
	// Log is where handlers log; nil logs nothing.
	Log *zap.Logger
}

// newRouter registers every operation the API serves: the built-ins, then
// the features' in their order, then the app's own through o.Routes. Each must declare the permission a caller
// needs to invoke it, or server.Public; one without is refused, and the
// server does not start. /health is Public so probes need no credentials.
// /me is Authenticated: any signed-in user may read their own account.
func newRouter(inst *installation, d Deps) (*server.Router, error) {
	o := inst.o
	router := server.NewRouter(o.Name, o.Version).WithResources(inst.newRouterResources())
	if err := registerBuiltins(router, o, d); err != nil {
		return nil, err
	}
	for _, h := range inst.routes {
		if err := h.fn(router, d); err != nil {
			return nil, fmt.Errorf("register %s routes: %w", h.feature, err)
		}
	}
	if o.Routes != nil {
		if err := o.Routes(router, d); err != nil {
			return nil, fmt.Errorf("register app routes: %w", err)
		}
	}
	return router, nil
}

func registerBuiltins(router *server.Router, o Options, d Deps) error {
	if err := server.Register(router, server.Operation{
		OperationID: "get-health",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "Report that the server is serving",
		Tags:        []string{"server"},
		Permission:  server.Public,
	}, health); err != nil {
		return err
	}
	if d.Sessions == nil {
		return nil
	}
	router.WithAuth(d.Sessions, d.Authorizer, o.names().SessionCookie)
	if err := server.Register(router, server.Operation{
		OperationID: "get-me",
		Method:      http.MethodGet,
		Path:        "/me",
		Summary:     "Return the signed-in user",
		Tags:        []string{"auth"},
		Permission:  server.Authenticated,
	}, server.NewMe(d.Roles, d.Log)); err != nil {
		return err
	}
	if d.Auth == nil {
		return nil
	}
	for _, r := range []struct {
		id, method, path, summary string
		h                         http.Handler
	}{
		{"auth-google-start", http.MethodGet, server.AuthStartPath, "Begin Google sign-in", d.Auth.Start()},
		{"auth-google-callback", http.MethodGet, server.AuthCallbackPath, "Complete Google sign-in", d.Auth.Callback()},
	} {
		if err := server.RegisterHandler(router, server.Operation{
			OperationID: r.id,
			Method:      r.method,
			Path:        r.path,
			Summary:     r.summary,
			Tags:        []string{"auth"},
			Permission:  server.Public,
		}, r.h); err != nil {
			return err
		}
	}
	// Logout is a Huma operation, not a plain handler, so it is in the
	// OpenAPI spec and the UI calls it through the generated client.
	return server.Register(router, server.Operation{
		OperationID: "auth-logout",
		Method:      http.MethodPost,
		Path:        server.LogoutPath,
		Summary:     "End the session",
		Tags:        []string{"auth"},
		Permission:  server.Public,
	}, d.Auth.Logout)
}

func serve(ctx context.Context, inst *installation, cfg config.Config) error {
	o := inst.o
	log, err := logger.New(o.Name, cfg.LogLevel, os.Stderr)
	if err != nil {
		return err
	}
	defer func() { _ = log.Sync() }()

	d, cleanup, err := connect(ctx, inst, cfg, log)
	defer cleanup()
	if err != nil {
		log.Error("start", zap.Error(err))
		return err
	}
	d.Log = log
	router, err := newRouter(inst, d)
	if err != nil {
		return fmt.Errorf("register routes: %w", err)
	}
	if err := server.New(cfg, log, router).Start(ctx); err != nil {
		log.Error("server failed", zap.Error(err))
		return err
	}
	return nil
}

// connect opens what cfg configures: Redis for sessions, Postgres (after
// applying the migrations) for the RBAC enforcer and the admin bootstrap,
// and Google sign-in. Sign-in needs Redis for its sessions and Postgres for
// the role bindings it checks, so a client id without either is an error.
// cleanup closes whatever was opened, even on error.
func connect(ctx context.Context, inst *installation, cfg config.Config, log *zap.Logger) (Deps, func(), error) {
	o := inst.o
	var (
		d       Deps
		closers []func()
	)
	cleanup := func() {
		for i := len(closers) - 1; i >= 0; i-- {
			closers[i]()
		}
	}

	if cfg.GoogleClientID != "" && cfg.RedisAddr == "" {
		return d, cleanup, fmt.Errorf("google sign-in is configured (%s_GOOGLE_CLIENT_ID) but %s_REDIS_ADDR is not set", o.EnvPrefix, o.EnvPrefix)
	}
	if cfg.GoogleClientID != "" && cfg.DatabaseDSN == "" {
		// Sign-in refuses anyone without a role binding, and the bindings
		// live in the database: without it nobody could ever sign in.
		return d, cleanup, fmt.Errorf("google sign-in is configured (%s_GOOGLE_CLIENT_ID) but %s_DATABASE_DSN is not set", o.EnvPrefix, o.EnvPrefix)
	}

	var (
		sessions, attempts *redis.Store
		// bindings is the sign-in gate; it needs the database.
		bindings server.RoleBindings
	)
	if cfg.RedisAddr != "" {
		client, err := redis.NewClient(ctx, redis.Config{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
		if err != nil {
			return d, cleanup, fmt.Errorf("connect to redis: %w", err)
		}
		closers = append(closers, func() { _ = client.Close() })
		sessions = redis.NewStore(client, o.CookiePrefix+":session:")
		attempts = redis.NewStore(client, o.CookiePrefix+":signin:")
		d.Sessions = sessions
	}

	if cfg.DatabaseDSN != "" {
		// Migrations run at startup, before anything reads the schema: core's,
		// then each feature's in order, then the app's, each in its own
		// version table. goose records what it applied, so a restart
		// applies nothing.
		if err := db.Migrate(ctx, cfg.DatabaseDSN, inst.migrations...); err != nil {
			return d, cleanup, err
		}
		pool, err := db.New(ctx, cfg.DatabaseDSN)
		if err != nil {
			return d, cleanup, err
		}
		closers = append(closers, pool.Close)
		e, err := db.NewEnforcer(ctx, pool, cfg.AdminSub, inst.roles)
		if err != nil {
			return d, cleanup, err
		}
		authz := server.CasbinAuthorizer{Enforcer: e}
		d.Authorizer = authz
		d.Roles = authz
		bindings = authz
	}

	if cfg.GoogleClientID == "" {
		log.Warn(fmt.Sprintf("google sign-in is disabled: %s_GOOGLE_CLIENT_ID is not set", o.EnvPrefix))
		return d, cleanup, nil
	}
	if sessions == nil || bindings == nil {
		return d, cleanup, errors.New("google sign-in is configured but its session store or role bindings are missing")
	}
	google, err := server.NewGoogle(server.GoogleConfig{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURI,
	})
	if err != nil {
		return d, cleanup, err
	}
	d.Auth = server.NewAuth(google, sessions, attempts, bindings, log, cfg.GoogleClientSecret, o.names())
	return d, cleanup, nil
}
