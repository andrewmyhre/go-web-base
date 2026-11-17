package server

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// hookRig is a sign-in rig whose gate is the app's SignIn, over bindings
// that bind nobody: core's own gate would refuse everyone.
func hookRig(t *testing.T, signIn SignIn, onSignedIn OnSignedIn) *signInRig {
	t.Helper()
	b := &bindings{bound: map[string]bool{}}
	return newSignInRigWith(t, b, func(a *Auth) { a.WithSignIn(signIn, onSignedIn) })
}

// TestSignInHookReplacesTheGate: a subject with no binding is admitted when
// the hook says nil, and core's binding check is never consulted.
func TestSignInHookReplacesTheGate(t *testing.T) {
	var got Identity
	rig := hookRig(t, func(_ context.Context, id Identity) error { got = id; return nil }, nil)
	rec := signIn(t, rig)
	wantSignedIn(t, rec)
	if got.Sub != fakeSub || got.Email == "" {
		t.Errorf("SignIn got %+v, want the identity Google vouched for", got)
	}
	if len(rig.bindings.calls) != 0 {
		t.Errorf("core's gate read the bindings %v although SignIn replaces it", rig.bindings.calls)
	}
	lines := signInLines(t, rig.logs)
	if len(lines) != 1 || lines[0]["decision"] != "allow" || lines[0]["subject"] != fakeSub || lines[0]["domain"] != signInDomain {
		t.Errorf("rbac lines = %v, want one allow for %s", lines, fakeSub)
	}
}

// TestSignInHookRefusalStatuses: ErrSignInRefused (even wrapped) is 403,
// any other error 503, both with the generic body, no session, the earlier
// session ended and one deny line.
func TestSignInHookRefusalStatuses(t *testing.T) {
	for name, c := range map[string]struct {
		err    error
		status int
	}{
		"refused":         {ErrSignInRefused, http.StatusForbidden},
		"wrapped refused": {errors.Join(errors.New("blocked"), ErrSignInRefused), http.StatusForbidden},
		"other error":     {errors.New("directory down"), http.StatusServiceUnavailable},
	} {
		t.Run(name, func(t *testing.T) {
			rig := hookRig(t, func(context.Context, Identity) error { return c.err }, nil)
			callback, state := rig.start(t)
			rec := rig.do(http.MethodGet, callback, state, &http.Cookie{Name: testNames.SessionCookie, Value: testSessionID})
			assertRefused(t, rig, rec, c.status, fakeSub)
		})
	}
}

// TestSignInHookKeepsTheNonPolicyParts: state, PKCE and nonce are still
// enforced, so a forged callback never reaches the hook.
func TestSignInHookKeepsTheNonPolicyParts(t *testing.T) {
	called := false
	rig := hookRig(t, func(context.Context, Identity) error { called = true; return nil }, nil)
	rec := rig.do(http.MethodGet, AuthCallbackPath+"?state=bad&code=x")
	if rec.Code != http.StatusBadRequest || called {
		t.Errorf("forged callback = %d, hook called %v; want 400 and not called", rec.Code, called)
	}
}

// TestOnSignedInRunsAfterTheGateBeforeTheSession: it sees the admitted
// identity, no session exists yet, and a refused subject never reaches it.
func TestOnSignedInRunsAfterTheGateBeforeTheSession(t *testing.T) {
	var (
		order     []string
		sessionsN = -1
		got       Identity
		rig       *signInRig
	)
	rig = hookRig(t,
		func(context.Context, Identity) error { order = append(order, "gate"); return nil },
		func(_ context.Context, id Identity) {
			order = append(order, "signed-in")
			got = id
			sessionsN = len(rig.sessions.data)
		})
	wantSignedIn(t, signIn(t, rig))
	if len(order) != 2 || order[0] != "gate" || order[1] != "signed-in" {
		t.Errorf("order = %v, want gate then signed-in", order)
	}
	if got.Sub != fakeSub {
		t.Errorf("OnSignedIn got %+v", got)
	}
	if sessionsN != 1 { // only the fixture session: the new one is minted after
		t.Errorf("sessions when OnSignedIn ran = %d, want 1 (the new one is minted after)", sessionsN)
	}

	order = nil
	refusing := hookRig(t,
		func(context.Context, Identity) error { order = append(order, "gate"); return ErrSignInRefused },
		func(context.Context, Identity) { order = append(order, "signed-in") })
	if rec := signIn(t, refusing); rec.Code != http.StatusForbidden {
		t.Fatalf("callback = %d, want 403", rec.Code)
	}
	if len(order) != 1 {
		t.Errorf("order = %v, want OnSignedIn skipped for a refused subject", order)
	}
}

// TestOnSignedInAloneKeepsCoresGate: OnSignedIn without SignIn leaves
// core's gate in charge (an unbound subject is still refused) and runs only
// for an admitted one.
func TestOnSignedInAloneKeepsCoresGate(t *testing.T) {
	calls := 0
	hook := func(context.Context, Identity) { calls++ }
	unbound := newSignInRigWith(t, &bindings{bound: map[string]bool{}}, func(a *Auth) { a.WithSignIn(nil, hook) })
	if rec := signIn(t, unbound); rec.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("unbound = %d with %d hook calls; want 403 and none", rec.Code, calls)
	}
	bound := newSignInRigWith(t, &bindings{bound: map[string]bool{fakeSub: true}}, func(a *Auth) { a.WithSignIn(nil, hook) })
	wantSignedIn(t, signIn(t, bound))
	if calls != 1 {
		t.Errorf("hook calls for a bound subject = %d, want 1", calls)
	}
}
