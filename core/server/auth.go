package server

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/andrewmyhre/go-web-base/core/logging"
	"github.com/andrewmyhre/go-web-base/core/redis"
)

const (
	// AuthStartPath is where a browser goes to begin Google sign-in.
	AuthStartPath = "/auth/google"
	// AuthCallbackPath is where Google sends the browser back. Its absolute
	// form must be a redirect URI registered with the Google OAuth client.
	AuthCallbackPath = "/auth/google/callback"
	// LogoutPath ends the session.
	LogoutPath = "/auth/logout"

	// stateTTL bounds how long a sign-in attempt can be completed.
	stateTTL = 10 * time.Minute

	// AfterSignInPath is where the browser lands once signed in: the UI's
	// home page, which reads /me itself. It is fixed: there is no return-to
	// parameter, so there is no open redirect.
	AfterSignInPath = "/"

	// signInObject is the object an rbac line names for the sign-in gate,
	// and signInDomain its domain: the gate asks for a binding in any
	// domain, not in one.
	signInObject = "sign-in"
	signInDomain = "*"

	attemptVerifier = "verifier"
	attemptNonce    = "nonce"
)

// Names are the per-app values the sign-in flow cannot fix: core names no
// app.
type Names struct {
	// App appears in the refusal shown to a user with no role binding
	// ("you cannot sign in to <App>").
	App string
	// SessionCookie carries the session id; the router must be given the
	// same name in WithAuth.
	SessionCookie string
	// StateCookie binds a sign-in attempt to the browser that started it:
	// the callback refuses a state that is not also in this cookie.
	StateCookie string
}

// NewNames derives the cookie names from a prefix: <prefix>_session and
// <prefix>_oauth_state. prefix should be a cookie-safe token such as
// "my_app".
func NewNames(app, cookiePrefix string) Names {
	return Names{App: app, SessionCookie: cookiePrefix + "_session", StateCookie: cookiePrefix + "_oauth_state"}
}

// Auth serves the Google sign-in flow and logout.
type Auth struct {
	names    Names
	google   *Google
	sessions SessionStore
	attempts SessionStore
	bindings RoleBindings
	log      *zap.Logger
	redact   *logging.Logger
}

// NewAuth builds the sign-in handlers. sessions holds the signed-in
// sessions and attempts the one-shot sign-in attempts; they may share a
// server but should use different key prefixes. bindings decides who may
// sign in at all: a user with no role binding is refused before a session
// is created. A nil bindings refuses everyone (fail closed). clientSecret is
// redacted from any error the handlers log. names supplies the cookie names
// and the app name in the refusal.
func NewAuth(google *Google, sessions, attempts SessionStore, bindings RoleBindings, log *zap.Logger, clientSecret string, names Names) *Auth {
	if log == nil {
		log = zap.NewNop()
	}
	return &Auth{names: names, google: google, sessions: sessions, attempts: attempts, bindings: bindings, log: log, redact: logging.New(log, clientSecret)}
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Start handles GET /auth/google. It records a sign-in attempt (a PKCE
// verifier and a nonce) in the store under a random id, which is the OAuth
// state, sets the same state in a cookie, and redirects to Google.
func (a *Auth) Start() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce, err := randomHex(32)
		if err != nil {
			a.fail(w, r, http.StatusInternalServerError, "generate nonce", err)
			return
		}
		verifier := oauth2.GenerateVerifier()
		state, err := a.attempts.Create(r.Context(), stateTTL, map[string]string{
			attemptVerifier: verifier,
			attemptNonce:    nonce,
		})
		if err != nil {
			a.fail(w, r, http.StatusServiceUnavailable, "store sign-in attempt", err)
			return
		}
		setCookie(w, a.names.StateCookie, state, AuthStartPath, int(stateTTL.Seconds()))
		w.Header().Set("Cache-Control", "no-store")
		http.Redirect(w, r, a.google.AuthCodeURL(state, nonce, verifier), http.StatusFound)
	})
}

// Callback handles GET /auth/google/callback. The state must match the
// browser's state cookie and name a live attempt, which is consumed
// whatever happens next. The code is exchanged with the attempt's PKCE
// verifier, the ID token is verified and must carry the attempt's nonce,
// and any session the browser already had is ended. Then the user must hold
// a role binding in some domain: one with none, or whose bindings cannot be
// read, is refused with a generic page and no session. Otherwise a new
// session is created under a fresh random id.
func (a *Auth) Callback() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// The state cookie is single-use too: clear it on every outcome.
		setCookie(w, a.names.StateCookie, "", AuthStartPath, -1)

		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			a.fail(w, r, http.StatusBadRequest, "google refused sign-in", errors.New(e))
			return
		}
		state, code := q.Get("state"), q.Get("code")
		c, err := r.Cookie(a.names.StateCookie)
		if err != nil || state == "" || code == "" ||
			subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
			a.fail(w, r, http.StatusBadRequest, "state does not match this browser's sign-in", nil)
			return
		}
		if !validID(state) {
			a.fail(w, r, http.StatusBadRequest, "malformed state", nil)
			return
		}
		attempt, err := a.attempts.Take(r.Context(), state)
		if errors.Is(err, redis.ErrNotFound) {
			a.fail(w, r, http.StatusBadRequest, "sign-in attempt is unknown, used or expired", nil)
			return
		}
		if err != nil {
			a.fail(w, r, http.StatusServiceUnavailable, "take sign-in attempt", err)
			return
		}

		id, err := a.google.Exchange(r.Context(), code, attempt[attemptVerifier], attempt[attemptNonce])
		if err != nil {
			a.fail(w, r, http.StatusBadGateway, "complete sign-in with google", err)
			return
		}

		// No fixation: whatever session the browser presented is ended, and
		// the new one is always a freshly minted id. This happens before
		// the binding check, so a refused browser keeps no older session.
		if old, err := r.Cookie(a.names.SessionCookie); err == nil && validID(old.Value) {
			if err := a.sessions.Delete(r.Context(), old.Value); err != nil {
				a.log.Warn("end previous session", zap.Error(err))
			}
		}

		if status := a.gate(r, id.Sub); status != 0 {
			setCookie(w, a.names.SessionCookie, "", "/", -1)
			msg := "you cannot sign in to " + a.names.App
			if status == http.StatusServiceUnavailable {
				msg = "sign-in is unavailable; try again"
			}
			writeProblem(w, status, msg)
			return
		}
		s := &Session{UserID: id.Sub, Name: id.Name, Email: id.Email, Avatar: id.Picture}
		sid, err := a.sessions.Create(r.Context(), SessionTTL, s.data())
		if err != nil {
			a.fail(w, r, http.StatusServiceUnavailable, "create session", err)
			return
		}
		setCookie(w, a.names.SessionCookie, sid, "/", int(SessionTTL.Seconds()))
		a.log.Info("signed in", zap.String("user_sub", id.Sub), zap.String("request_id", RequestIDFromContext(r.Context())))
		http.Redirect(w, r, AfterSignInPath, http.StatusFound)
	})
}

// gate decides whether sub may sign in at all: it must hold a role binding
// in some domain. It returns 0 to let the user in, 403 for a user with no
// binding and 503 when the bindings cannot be read (fail closed), and logs
// one rbac line either way. The caller's response carries no reason.
func (a *Auth) gate(r *http.Request, sub string) int {
	decision := logging.RBACDecision{
		Subject:   sub,
		Domain:    signInDomain,
		Object:    signInObject,
		Decision:  logging.Deny,
		RequestID: RequestIDFromContext(r.Context()),
	}
	rbac := logging.New(a.log)
	if a.bindings == nil {
		a.log.Error("sign-in has no role bindings to check; refusing everyone", zap.String("request_id", decision.RequestID))
		rbac.Check(decision)
		return http.StatusServiceUnavailable
	}
	bound, err := a.bindings.HasRoleBinding(sub)
	if err != nil {
		a.log.Warn("read role bindings", zap.String("request_id", decision.RequestID), zap.Error(err))
		rbac.Check(decision)
		return http.StatusServiceUnavailable
	}
	if !bound {
		rbac.Check(decision)
		return http.StatusForbidden
	}
	decision.Decision = logging.Allow
	rbac.Check(decision)
	return 0
}

// LogoutInput is the request of POST /auth/logout. It has no fields: the
// session cookie's name is per app, and a Huma cookie parameter fixes its
// name in a struct tag, so the session middleware hands the presented id
// to Logout through the request context instead. The cookie was never in
// the OpenAPI document (it was hidden, as a script cannot set an HttpOnly
// cookie), so the document is the same.
type LogoutInput struct{}

// LogoutOutput is the response of POST /auth/logout: 204 and a cookie that
// clears the session cookie.
type LogoutOutput struct {
	SetCookie http.Cookie `header:"Set-Cookie" doc:"Clears the session cookie."`
}

// Logout handles POST /auth/logout: it deletes the session from the store
// and clears the cookie. It answers 204 whether or not a session existed.
// A store failure is a 503 and leaves the cookie alone, so the browser is
// not told it is signed out while the session lives on.
//
// It needs no CSRF token. The session cookie is SameSite=Lax, so a
// cross-site POST does not carry it and cannot end anyone's session
// server-side.
func (a *Auth) Logout(ctx context.Context, _ *LogoutInput) (*LogoutOutput, error) {
	if id := sessionIDFromContext(ctx); id != "" {
		if err := a.sessions.Delete(ctx, id); err != nil {
			a.log.Warn("sign-out failed",
				zap.String("reason", "end session"),
				zap.String("request_id", RequestIDFromContext(ctx)),
				zap.String("error", a.redact.Redact(err.Error())))
			return nil, huma.Error503ServiceUnavailable("could not end the session; try again")
		}
	}
	return &LogoutOutput{SetCookie: cookie(a.names.SessionCookie, "", "/", -1)}, nil
}

// fail logs why a sign-in step failed and answers with a generic message:
// the reason is for the operator, not the browser. err is redacted of the
// client secret before it is logged. Neither the code nor the state is
// logged.
func (a *Auth) fail(w http.ResponseWriter, r *http.Request, status int, reason string, err error) {
	fields := []zap.Field{
		zap.String("reason", reason),
		zap.String("request_id", RequestIDFromContext(r.Context())),
	}
	if err != nil {
		fields = append(fields, zap.String("error", a.redact.Redact(err.Error())))
	}
	a.log.Warn("sign-in failed", fields...)
	msg := "sign-in failed; start again at " + AuthStartPath
	if status == http.StatusServiceUnavailable {
		msg = "sign-in is unavailable; try again"
	}
	writeProblem(w, status, msg)
}
