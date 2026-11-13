package server

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

const (
	fakeClientID     = "test-client.apps.googleusercontent.com"
	fakeClientSecret = "fake-client-secret-0123456789"
	fakeRedirect     = "https://testapp.test/auth/google/callback"
)

// fakeGoogle is an OpenID provider standing in for Google. Its consent
// step is skipped: authorize records what /auth/google sent and returns a
// code; its token endpoint checks the client secret, the redirect URI and
// the PKCE verifier against the recorded S256 challenge, then returns an ID
// token built by claims.
type fakeGoogle struct {
	t      *testing.T
	srv    *httptest.Server
	key    *rsa.PrivateKey
	signer *rsa.PrivateKey // signs ID tokens; key unless a test swaps it

	mu     sync.Mutex
	codes  map[string]url.Values
	claims func(auth url.Values) map[string]any
	// tokenBody, when set, is returned by the token endpoint with a 400.
	tokenBody string
}

func newFakeGoogle(t *testing.T) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeGoogle{t: t, key: key, signer: key, codes: map[string]url.Values{}}
	f.claims = func(auth url.Values) map[string]any {
		return map[string]any{
			"iss": f.srv.URL, "aud": fakeClientID, "azp": fakeClientID,
			"sub": "112233445566778899000", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"nonce": auth.Get("nonce"), "email": "andrew@example.com", "email_verified": true,
			"name": "Andrew", "picture": "https://example.com/andrew.png",
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{
			Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig",
		}}})
	})
	mux.HandleFunc("/token", f.token)
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGoogle) config() GoogleConfig {
	return GoogleConfig{
		ClientID: fakeClientID, ClientSecret: fakeClientSecret, RedirectURL: fakeRedirect,
		Issuer: f.srv.URL, AuthURL: f.srv.URL + "/auth", TokenURL: f.srv.URL + "/token", JWKSURL: f.srv.URL + "/jwks",
		HTTPClient: f.srv.Client(),
	}
}

// authorize plays the consent screen for the redirect /auth/google sent.
func (f *fakeGoogle) authorize(location string) (state, code string) {
	f.t.Helper()
	u, err := url.Parse(location)
	if err != nil {
		f.t.Fatal(err)
	}
	q := u.Query()
	code, _ = randomHex(8)
	f.mu.Lock()
	f.codes[code] = q
	f.mu.Unlock()
	return q.Get("state"), code
}

func (f *fakeGoogle) token(w http.ResponseWriter, r *http.Request) {
	if f.tokenBody != "" {
		http.Error(w, f.tokenBody, http.StatusBadRequest)
		return
	}
	id, secret, _ := r.BasicAuth()
	if id != url.QueryEscape(fakeClientID) || secret != url.QueryEscape(fakeClientSecret) {
		http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
		return
	}
	_ = r.ParseForm()
	f.mu.Lock()
	auth, ok := f.codes[r.PostForm.Get("code")]
	delete(f.codes, r.PostForm.Get("code"))
	f.mu.Unlock()
	if !ok || r.PostForm.Get("redirect_uri") != fakeRedirect {
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	if auth.Get("code_challenge_method") != "S256" ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != auth.Get("code_challenge") {
		http.Error(w, `{"error":"invalid_grant","error_description":"pkce"}`, http.StatusBadRequest)
		return
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: f.signer},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		f.t.Error(err)
		return
	}
	payload, _ := json.Marshal(f.claims(auth))
	obj, err := signer.Sign(payload)
	if err != nil {
		f.t.Error(err)
		return
	}
	raw, _ := obj.CompactSerialize()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": raw,
	})
}

// signInRig is the router with sign-in, /me and logout, over memStores.
type signInRig struct {
	google             *fakeGoogle
	sessions, attempts *memStore
	bindings           *bindings
	auth               *Auth
	h                  http.Handler
	logs               *bytes.Buffer
}

func newSignInRig(t *testing.T) *signInRig {
	t.Helper()
	return newSignInRigWith(t, &bindings{bound: map[string]bool{fakeSub: true}})
}

// fakeSub is the subject the fake Google's ID tokens carry by default.
const fakeSub = "112233445566778899000"

// newSignInRigWith is newSignInRig with b as the sign-in gate's bindings;
// a nil b leaves the gate with none.
func newSignInRigWith(t *testing.T, b *bindings, configure ...func(*Auth)) *signInRig {
	t.Helper()
	rig := &signInRig{google: newFakeGoogle(t), sessions: newMemStore(), attempts: newMemStore(), bindings: b, logs: &bytes.Buffer{}}
	g, err := NewGoogle(rig.google.config())
	if err != nil {
		t.Fatal(err)
	}
	log := bufferLogger(rig.logs)
	var rb RoleBindings
	if b != nil {
		rb = b
	}
	auth := NewAuth(g, rig.sessions, rig.attempts, rb, log, fakeClientSecret, testNames)
	for _, c := range configure {
		c(auth)
	}
	rig.auth = auth
	r := testRouter("t", "0").WithAuth(rig.sessions, nil, testNames.SessionCookie)
	if err := Register(r, op(http.MethodGet, "/me", Authenticated), NewMe(nil, nil)); err != nil {
		t.Fatal(err)
	}
	for _, x := range []struct {
		method, path string
		h            http.Handler
	}{
		{http.MethodGet, AuthStartPath, auth.Start()},
		{http.MethodGet, AuthCallbackPath, auth.Callback()},
	} {
		if err := RegisterHandler(r, op(x.method, x.path, Public), x.h); err != nil {
			t.Fatal(err)
		}
	}
	if err := Register(r, op(http.MethodPost, LogoutPath, Public), auth.Logout); err != nil {
		t.Fatal(err)
	}
	rig.h = mustBuild(t, r, log)
	return rig
}

func (rig *signInRig) do(method, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rig.h.ServeHTTP(rec, req)
	return rec
}

func cookieNamed(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func assertCookieFlags(t *testing.T, c *http.Cookie, path string) {
	t.Helper()
	if c == nil {
		t.Fatal("cookie not set")
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode || c.Path != path {
		t.Fatalf("cookie %s = HttpOnly %v Secure %v SameSite %v Path %q, want true true Lax %q",
			c.Name, c.HttpOnly, c.Secure, c.SameSite, c.Path, path)
	}
}

// start runs GET /auth/google and the fake consent step, returning the
// callback URL and the state cookie.
func (rig *signInRig) start(t *testing.T) (callback string, stateCookie *http.Cookie) {
	t.Helper()
	rec := rig.do(http.MethodGet, AuthStartPath)
	if rec.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302", AuthStartPath, rec.Code)
	}
	state, code := rig.google.authorize(rec.Header().Get("Location"))
	stateCookie = cookieNamed(rec, testNames.StateCookie)
	assertCookieFlags(t, stateCookie, AuthStartPath)
	if stateCookie.Value != state || stateCookie.MaxAge <= 0 {
		t.Fatalf("state cookie = %q (MaxAge %d), want the state %q with a lifetime", stateCookie.Value, stateCookie.MaxAge, state)
	}
	return AuthCallbackPath + "?" + url.Values{"state": {state}, "code": {code}}.Encode(), stateCookie
}

func TestStartRedirectsWithStatePKCEAndNonce(t *testing.T) {
	rig := newSignInRig(t)
	rec := rig.do(http.MethodGet, AuthStartPath)
	u, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Path != "/auth" || q.Get("client_id") != fakeClientID || q.Get("redirect_uri") != fakeRedirect ||
		q.Get("response_type") != "code" || q.Get("scope") != "openid email profile" {
		t.Fatalf("consent URL = %s", u)
	}
	if q.Get("code_challenge_method") != "S256" || len(q.Get("code_challenge")) != 43 {
		t.Fatalf("consent URL lacks an S256 PKCE challenge: %s", u)
	}
	if len(q.Get("nonce")) != 64 || !validID(q.Get("state")) {
		t.Fatalf("consent URL nonce %q / state %q, want 32 random bytes each", q.Get("nonce"), q.Get("state"))
	}
	attempt, ok := rig.attempts.data[q.Get("state")]
	if !ok || attempt[attemptNonce] != q.Get("nonce") || attempt[attemptVerifier] == "" {
		t.Fatalf("attempt stored under the state = %v (found %v), want the nonce and a verifier", attempt, ok)
	}
	if rig.attempts.ttl[q.Get("state")] != stateTTL {
		t.Fatalf("attempt TTL = %v, want %v", rig.attempts.ttl[q.Get("state")], stateTTL)
	}
	// Two starts never share a state, nonce or verifier.
	q2, _ := url.Parse(rig.do(http.MethodGet, AuthStartPath).Header().Get("Location"))
	if q2.Query().Get("state") == q.Get("state") || q2.Query().Get("nonce") == q.Get("nonce") ||
		q2.Query().Get("code_challenge") == q.Get("code_challenge") {
		t.Fatal("two sign-in attempts share a state, nonce or PKCE challenge")
	}
}

func TestSignInThenMe(t *testing.T) {
	rig := newSignInRig(t)
	callback, stateCookie := rig.start(t)
	old := &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID}

	// return_to is ignored: the landing page is fixed.
	rec := rig.do(http.MethodGet, callback+"&return_to=https://evil.example/", stateCookie, old)
	// The literal, not the constant: the landing page is the UI's home, and
	// the constant changing silently would move every sign-in.
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/" {
		t.Fatalf("callback = %d to %q, want 302 to /: %s", rec.Code, rec.Header().Get("Location"), rec.Body)
	}
	session := cookieNamed(rec, testNames.SessionCookie)
	assertCookieFlags(t, session, "/")
	if session.MaxAge != int(SessionTTL.Seconds()) {
		t.Fatalf("session cookie MaxAge = %d, want %d", session.MaxAge, int(SessionTTL.Seconds()))
	}
	if !validID(session.Value) || session.Value == testSessionID {
		t.Fatalf("session id %q, want a fresh random id", session.Value)
	}
	if rig.sessions.has(testSessionID) {
		t.Fatal("the session the browser had before signing in still exists (fixation)")
	}
	if rig.sessions.ttl[session.Value] != SessionTTL {
		t.Fatalf("session TTL = %v, want %v", rig.sessions.ttl[session.Value], SessionTTL)
	}
	if c := cookieNamed(rec, testNames.StateCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("the callback did not clear the state cookie")
	}
	if rig.attempts.has(stateCookie.Value) {
		t.Fatal("the sign-in attempt is still in the store after the callback used it")
	}

	me := rig.do(http.MethodGet, "/me", session)
	if me.Code != http.StatusOK {
		t.Fatalf("GET /me = %d: %s", me.Code, me.Body)
	}
	want := `{"id":"112233445566778899000","name":"Andrew","email":"andrew@example.com","avatar":"https://example.com/andrew.png","roles":[]}`
	if got := strings.TrimSpace(me.Body.String()); got != want {
		t.Fatalf("GET /me = %s, want %s", got, want)
	}

	// The state is single-use: replaying the callback fails.
	if rec := rig.do(http.MethodGet, callback, stateCookie); rec.Code != http.StatusBadRequest {
		t.Fatalf("replayed callback = %d, want 400", rec.Code)
	}

	// Logout ends the session server-side and clears the cookie.
	out := rig.do(http.MethodPost, LogoutPath, session)
	if out.Code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204", out.Code)
	}
	if c := cookieNamed(out, testNames.SessionCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("logout did not clear the session cookie")
	}
	if rig.sessions.has(session.Value) {
		t.Fatal("logout left the session in the store")
	}
	if me := rig.do(http.MethodGet, "/me", session); me.Code != http.StatusUnauthorized {
		t.Fatalf("GET /me after logout = %d, want 401", me.Code)
	}
}

func TestCallbackRefusals(t *testing.T) {
	cases := map[string]struct {
		setup func(rig *signInRig)
		// mangle changes the callback request; it gets the URL and the
		// state cookie and returns what to send.
		mangle func(callback string, c *http.Cookie) (string, []*http.Cookie)
		status int
	}{
		"no state cookie (another browser)": {
			mangle: func(cb string, _ *http.Cookie) (string, []*http.Cookie) { return cb, nil },
			status: http.StatusBadRequest,
		},
		"state cookie from another attempt": {
			mangle: func(cb string, c *http.Cookie) (string, []*http.Cookie) {
				other := *c
				other.Value = strings.Repeat("e", 64)
				return cb, []*http.Cookie{&other}
			},
			status: http.StatusBadRequest,
		},
		"expired attempt": {
			setup: func(rig *signInRig) {
				for id := range rig.attempts.data {
					delete(rig.attempts.data, id)
				}
			},
			status: http.StatusBadRequest,
		},
		"google error": {
			mangle: func(_ string, c *http.Cookie) (string, []*http.Cookie) {
				return AuthCallbackPath + "?error=access_denied&state=" + c.Value, []*http.Cookie{c}
			},
			status: http.StatusBadRequest,
		},
		"wrong nonce": {
			setup:  func(rig *signInRig) { rig.override("nonce", strings.Repeat("0", 64)) },
			status: http.StatusBadGateway,
		},
		"wrong audience": {
			setup:  func(rig *signInRig) { rig.override("aud", "someone-else.apps.googleusercontent.com") },
			status: http.StatusBadGateway,
		},
		"wrong azp": {
			setup:  func(rig *signInRig) { rig.override("azp", "someone-else.apps.googleusercontent.com") },
			status: http.StatusBadGateway,
		},
		"wrong issuer": {
			setup:  func(rig *signInRig) { rig.override("iss", "https://evil.example") },
			status: http.StatusBadGateway,
		},
		"expired token": {
			setup:  func(rig *signInRig) { rig.override("exp", time.Now().Add(-time.Minute).Unix()) },
			status: http.StatusBadGateway,
		},
		"no sub": {
			setup:  func(rig *signInRig) { rig.override("sub", "") },
			status: http.StatusBadGateway,
		},
		"signed by another key": {
			setup: func(rig *signInRig) {
				k, err := rsa.GenerateKey(rand.Reader, 2048)
				if err != nil {
					panic(err)
				}
				rig.google.signer = k
			},
			status: http.StatusBadGateway,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rig := newSignInRig(t)
			callback, stateCookie := rig.start(t)
			if c.setup != nil {
				c.setup(rig)
			}
			cookies := []*http.Cookie{stateCookie}
			if c.mangle != nil {
				callback, cookies = c.mangle(callback, stateCookie)
			}
			rec := rig.do(http.MethodGet, callback, cookies...)
			if rec.Code != c.status {
				t.Fatalf("callback = %d, want %d: %s", rec.Code, c.status, rec.Body)
			}
			if s := cookieNamed(rec, testNames.SessionCookie); s != nil {
				t.Fatalf("a refused callback set a session cookie: %v", s)
			}
			if n := len(rig.sessions.data); n != 1 { // only the fixture session
				t.Fatalf("store holds %d sessions after a refused callback, want 1", n)
			}
		})
	}
}

// override makes the fake's ID token carry value for claim.
func (rig *signInRig) override(claim string, value any) {
	base := rig.google.claims
	rig.google.claims = func(auth url.Values) map[string]any {
		c := base(auth)
		c[claim] = value
		return c
	}
}

func TestUnverifiedEmailIsNotShown(t *testing.T) {
	rig := newSignInRig(t)
	rig.override("email_verified", false)
	callback, stateCookie := rig.start(t)
	rec := rig.do(http.MethodGet, callback, stateCookie)
	session := cookieNamed(rec, testNames.SessionCookie)
	if session == nil {
		t.Fatalf("callback = %d, want a session: %s", rec.Code, rec.Body)
	}
	if got := rig.sessions.data[session.Value][sessionEmail]; got != "" {
		t.Fatalf("session email = %q, want empty for an unverified address", got)
	}
}

func TestPKCEVerifierIsSentToTheTokenEndpoint(t *testing.T) {
	rig := newSignInRig(t)
	callback, stateCookie := rig.start(t)
	// Corrupt the stored verifier: the fake's PKCE check must then refuse.
	for id := range rig.attempts.data {
		rig.attempts.data[id][attemptVerifier] = strings.Repeat("x", 43)
	}
	if rec := rig.do(http.MethodGet, callback, stateCookie); rec.Code != http.StatusBadGateway {
		t.Fatalf("callback with the wrong verifier = %d, want 502", rec.Code)
	}
}

func TestSecretsAndCodesAreNotLogged(t *testing.T) {
	rig := newSignInRig(t)
	callback, stateCookie := rig.start(t)
	// A token endpoint that echoes the secret back in its error.
	rig.google.tokenBody = `{"error":"invalid_client","error_description":"bad secret ` + fakeClientSecret + `"}`
	if rec := rig.do(http.MethodGet, callback, stateCookie); rec.Code != http.StatusBadGateway {
		t.Fatalf("callback = %d, want 502", rec.Code)
	}
	logs := rig.logs.String()
	if !strings.Contains(logs, "sign-in failed") {
		t.Fatalf("no sign-in failure line was logged: %s", logs)
	}
	u, _ := url.Parse(callback)
	for name, secret := range map[string]string{
		"client secret": fakeClientSecret, "code": u.Query().Get("code"), "state": u.Query().Get("state"),
	} {
		if strings.Contains(logs, secret) {
			t.Fatalf("the %s appears in the logs: %s", name, logs)
		}
	}
}

func TestNewGoogleRefusesBadConfig(t *testing.T) {
	ok := GoogleConfig{ClientID: "id", ClientSecret: "s", RedirectURL: "https://h/auth/google/callback"}
	if _, err := NewGoogle(ok); err != nil {
		t.Fatalf("NewGoogle(valid) = %v", err)
	}
	for name, mutate := range map[string]func(*GoogleConfig){
		"no id":            func(c *GoogleConfig) { c.ClientID = "" },
		"no secret":        func(c *GoogleConfig) { c.ClientSecret = "" },
		"no redirect":      func(c *GoogleConfig) { c.RedirectURL = "" },
		"relative":         func(c *GoogleConfig) { c.RedirectURL = "/auth/google/callback" },
		"plain http":       func(c *GoogleConfig) { c.RedirectURL = "http://h/auth/google/callback" },
		"loopback is fine": nil,
	} {
		c := ok
		if mutate == nil {
			c.RedirectURL = "http://localhost:8080/auth/google/callback"
			if _, err := NewGoogle(c); err != nil {
				t.Errorf("%s: %v", name, err)
			}
			continue
		}
		mutate(&c)
		if _, err := NewGoogle(c); err == nil {
			t.Errorf("%s: NewGoogle accepted it", name)
		}
	}
}

// TestLogoutStoreFailureKeepsTheCookie: when the session cannot be deleted
// logout is a 503 and does not clear the cookie, so the browser is never
// told it signed out while the session lives on.
func TestLogoutStoreFailureKeepsTheCookie(t *testing.T) {
	rig := newSignInRig(t)
	rig.sessions.fail = true
	rec := rig.do(http.MethodPost, LogoutPath, &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("logout with the store down = %d, want 503", rec.Code)
	}
	if c := cookieNamed(rec, testNames.SessionCookie); c != nil {
		t.Fatalf("logout with the store down set %v, want no session cookie", c)
	}
}

// TestLogoutWithoutASessionIs204: logging out with no cookie, or a
// malformed one, still answers 204 and clears the cookie.
func TestLogoutWithoutASessionIs204(t *testing.T) {
	rig := newSignInRig(t)
	for name, cookies := range map[string][]*http.Cookie{
		"none":      nil,
		"malformed": {{Name: testNames.SessionCookie, Value: "not-a-session-id"}},
	} {
		rec := rig.do(http.MethodPost, LogoutPath, cookies...)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("%s: logout = %d, want 204", name, rec.Code)
		}
		if c := cookieNamed(rec, testNames.SessionCookie); c == nil || c.MaxAge >= 0 {
			t.Fatalf("%s: logout did not clear the session cookie", name)
		}
	}
}

// testNames are arbitrary per-app names: core fixes none.
var testNames = NewNames("testapp", "testapp")
