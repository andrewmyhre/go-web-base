package server

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// signInLines returns the rbac lines the sign-in gate logged into buf.
func signInLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, e := range rbacLines(t, buf) {
		if e["object"] == signInObject {
			out = append(out, e)
		}
	}
	return out
}

// assertRefused checks a refused callback: the status, a body that names
// nobody and gives no reason, the state and session cookies cleared, the
// browser's earlier session ended, no new session in the store, and one
// rbac deny line for sub.
func assertRefused(t *testing.T, rig *signInRig, rec *httptest.ResponseRecorder, status int, sub string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("callback = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	body := rec.Body.String()
	for _, leak := range []string{sub, "andrew@example.com", "Andrew", "binding", "role"} {
		if strings.Contains(body, leak) {
			t.Fatalf("refusal body leaks %q: %s", leak, body)
		}
	}
	if c := cookieNamed(rec, testNames.SessionCookie); c == nil || c.MaxAge >= 0 || c.Value != "" {
		t.Fatalf("refused callback session cookie = %v, want it cleared", c)
	}
	if c := cookieNamed(rec, testNames.StateCookie); c == nil || c.MaxAge >= 0 {
		t.Fatal("refused callback did not clear the state cookie")
	}
	if rig.sessions.has(testSessionID) {
		t.Fatal("the browser's earlier session survived a refused sign-in")
	}
	if n := len(rig.sessions.data); n != 0 {
		t.Fatalf("store holds %d sessions after a refused sign-in, want 0", n)
	}
	lines := signInLines(t, rig.logs)
	if len(lines) != 1 || lines[0]["decision"] != "deny" || lines[0]["subject"] != sub {
		t.Fatalf("sign-in rbac lines = %v, want one deny for %s", lines, sub)
	}
	if lines[0]["request_id"] != rec.Header().Get(RequestIDHeader) {
		t.Fatalf("rbac request_id = %v, want %q", lines[0]["request_id"], rec.Header().Get(RequestIDHeader))
	}
}

// TestUnboundUserIsRefused: a user Google vouches for but who holds no
// role binding gets a generic 403 and no session, and the session the
// browser held before is ended rather than left valid.
func TestUnboundUserIsRefused(t *testing.T) {
	b := &bindings{bound: map[string]bool{"someone-else": true}}
	rig := newSignInRigWith(t, b)
	callback, stateCookie := rig.start(t)
	old := &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID}
	rec := rig.do(http.MethodGet, callback, stateCookie, old)
	assertRefused(t, rig, rec, http.StatusForbidden, fakeSub)
	if len(b.calls) != 1 || b.calls[0] != fakeSub {
		t.Fatalf("bindings asked about %v, want [%s]", b.calls, fakeSub)
	}
	if me := rig.do(http.MethodGet, "/me", old); me.Code != http.StatusUnauthorized {
		t.Fatalf("GET /me with the old cookie after a refusal = %d, want 401", me.Code)
	}
}

// TestBindingLookupFailureFailsClosed: when the bindings cannot be read the
// user is refused with 503 and no session, never let in.
func TestBindingLookupFailureFailsClosed(t *testing.T) {
	rig := newSignInRigWith(t, &bindings{bound: map[string]bool{fakeSub: true}, err: errors.New("postgres is down")})
	callback, stateCookie := rig.start(t)
	rec := rig.do(http.MethodGet, callback, stateCookie, &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
	assertRefused(t, rig, rec, http.StatusServiceUnavailable, fakeSub)
	if strings.Contains(rec.Body.String(), "postgres") {
		t.Fatalf("refusal body leaks the error: %s", rec.Body)
	}
}

// TestNoBindingsFailsClosed: an Auth built without bindings refuses
// everyone rather than letting everyone in.
func TestNoBindingsFailsClosed(t *testing.T) {
	rig := newSignInRigWith(t, nil)
	callback, stateCookie := rig.start(t)
	rec := rig.do(http.MethodGet, callback, stateCookie, &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
	assertRefused(t, rig, rec, http.StatusServiceUnavailable, fakeSub)
}

// TestBoundUserIsLetIn: the gate logs an allow for a bound user, who then
// gets a session.
func TestBoundUserIsLetIn(t *testing.T) {
	rig := newSignInRig(t)
	callback, stateCookie := rig.start(t)
	rec := rig.do(http.MethodGet, callback, stateCookie)
	if rec.Code != http.StatusFound || cookieNamed(rec, testNames.SessionCookie) == nil {
		t.Fatalf("callback = %d, want 302 with a session: %s", rec.Code, rec.Body)
	}
	lines := signInLines(t, rig.logs)
	if len(lines) != 1 || lines[0]["decision"] != "allow" || lines[0]["subject"] != fakeSub || lines[0]["domain"] != signInDomain {
		t.Fatalf("sign-in rbac lines = %v, want one allow for %s in %s", lines, fakeSub, signInDomain)
	}
}
