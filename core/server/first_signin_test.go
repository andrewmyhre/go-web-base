package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func signIn(t *testing.T, rig *signInRig) *httptest.ResponseRecorder {
	t.Helper()
	callback, state := rig.start(t)
	return rig.do(http.MethodGet, callback, state)
}

func wantSignedIn(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != http.StatusFound || cookieNamed(rec, testNames.SessionCookie) == nil {
		t.Fatalf("callback = %d, session cookie %v; want 302 and a session: %s", rec.Code, cookieNamed(rec, testNames.SessionCookie), rec.Body)
	}
}

func wantRefused(t *testing.T, rig *signInRig, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("callback = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if s := cookieNamed(rec, testNames.SessionCookie); s != nil && s.MaxAge >= 0 {
		t.Fatalf("a refused sign-in set a session cookie: %v", s)
	}
	if n := len(rig.sessions.data); n != 1 { // only the fixture session
		t.Fatalf("store holds %d sessions after a refusal, want 1", n)
	}
}

func withBinder(rig **signInRig, hook FirstSignIn, def string) func(*Auth) {
	return func(a *Auth) {
		a.WithFirstSignIn(hook, def, binderOf(rig))
	}
}

// binderOf resolves the rig's bindings lazily: the rig is built after the
// configure callback runs.
type lazyBinder struct{ rig **signInRig }

func (l lazyBinder) BindRole(sub, role string) error { return (*l.rig).bindings.BindRole(sub, role) }
func binderOf(rig **signInRig) RoleBinder            { return lazyBinder{rig} }

func newFirstRig(t *testing.T, hook FirstSignIn, def string) *signInRig {
	t.Helper()
	var rig *signInRig
	b := &bindings{bound: map[string]bool{}}
	rig = newSignInRigWith(t, b, withBinder(&rig, hook, def))
	return rig
}

func TestFirstSignIn(t *testing.T) {
	t.Run("default role binds and signs in", func(t *testing.T) {
		rig := newFirstRig(t, nil, "Member")
		wantSignedIn(t, signIn(t, rig))
		if want := []string{fakeSub + "|Member"}; !slices.Equal(rig.bindings.binds, want) {
			t.Fatalf("BindRole calls = %v, want %v", rig.bindings.binds, want)
		}
		// The second sign-in finds the binding and binds nothing more.
		wantSignedIn(t, signIn(t, rig))
		if len(rig.bindings.binds) != 1 {
			t.Fatalf("BindRole calls after a second sign-in = %v, want 1", rig.bindings.binds)
		}
	})

	t.Run("hook gets the identity and picks the role", func(t *testing.T) {
		var got Identity
		rig := newFirstRig(t, func(_ context.Context, id Identity) (string, error) {
			got = id
			return "Editor", nil
		}, "Member")
		wantSignedIn(t, signIn(t, rig))
		want := Identity{Sub: fakeSub, Name: "Andrew", Email: "andrew@example.com", Picture: "https://example.com/andrew.png"}
		if got != want {
			t.Fatalf("hook identity = %+v, want %+v", got, want)
		}
		if want := []string{fakeSub + "|Editor"}; !slices.Equal(rig.bindings.binds, want) {
			t.Fatalf("BindRole calls = %v, want %v", rig.bindings.binds, want)
		}
	})

	t.Run("hook returning no role falls back to the default", func(t *testing.T) {
		rig := newFirstRig(t, func(context.Context, Identity) (string, error) { return "", nil }, "Member")
		wantSignedIn(t, signIn(t, rig))
		if want := []string{fakeSub + "|Member"}; !slices.Equal(rig.bindings.binds, want) {
			t.Fatalf("BindRole calls = %v, want %v", rig.bindings.binds, want)
		}
	})

	t.Run("hook with no role at all refuses", func(t *testing.T) {
		rig := newFirstRig(t, func(context.Context, Identity) (string, error) { return "", nil }, "")
		wantRefused(t, rig, signIn(t, rig), http.StatusForbidden)
		if len(rig.bindings.binds) != 0 {
			t.Fatalf("BindRole calls = %v, want none", rig.bindings.binds)
		}
	})

	t.Run("hook error refuses with 503 and binds nothing", func(t *testing.T) {
		rig := newFirstRig(t, func(context.Context, Identity) (string, error) { return "Member", errors.New("db down") }, "Member")
		wantRefused(t, rig, signIn(t, rig), http.StatusServiceUnavailable)
		if len(rig.bindings.binds) != 0 {
			t.Fatalf("BindRole calls = %v, want none", rig.bindings.binds)
		}
	})

	t.Run("hook ErrSignInRefused is a 403", func(t *testing.T) {
		rig := newFirstRig(t, func(context.Context, Identity) (string, error) { return "", ErrSignInRefused }, "Member")
		wantRefused(t, rig, signIn(t, rig), http.StatusForbidden)
		if len(rig.bindings.binds) != 0 {
			t.Fatalf("BindRole calls = %v, want none", rig.bindings.binds)
		}
	})

	t.Run("a failed bind refuses with 503", func(t *testing.T) {
		rig := newFirstRig(t, nil, "Member")
		rig.bindings.bindErr = errors.New("no such role")
		wantRefused(t, rig, signIn(t, rig), http.StatusServiceUnavailable)
	})

	t.Run("a bound subject never reaches the hook", func(t *testing.T) {
		var calls atomic.Int32
		var rig *signInRig
		rig = newSignInRigWith(t, &bindings{bound: map[string]bool{fakeSub: true}}, withBinder(&rig, func(context.Context, Identity) (string, error) {
			calls.Add(1)
			return "Member", nil
		}, "Member"))
		wantSignedIn(t, signIn(t, rig))
		if calls.Load() != 0 || len(rig.bindings.binds) != 0 {
			t.Fatalf("hook ran %d times and bound %v for a subject that already had a binding", calls.Load(), rig.bindings.binds)
		}
	})
}

// TestNothingRegisteredStillRefusesAnUnboundSubject: an app that registers
// neither a hook nor a default role keeps core's refuse-by-default.
func TestNothingRegisteredStillRefusesAnUnboundSubject(t *testing.T) {
	var rig *signInRig
	for name, configure := range map[string]func(*Auth){
		"never configured": func(*Auth) {},
		"empty options":    withBinder(&rig, nil, ""),
		"no binder":        func(a *Auth) { a.WithFirstSignIn(nil, "Member", nil) },
	} {
		t.Run(name, func(t *testing.T) {
			rig = newSignInRigWith(t, &bindings{bound: map[string]bool{}}, configure)
			wantRefused(t, rig, signIn(t, rig), http.StatusForbidden)
			if len(rig.bindings.binds) != 0 {
				t.Fatalf("BindRole calls = %v, want none", rig.bindings.binds)
			}
		})
	}
}

// TestConcurrentFirstSignInsRunTheHookOnce: two first sign-ins of one
// subject at the same moment both get in, the hook runs once, and the
// subject ends with one binding.
func TestConcurrentFirstSignInsRunTheHookOnce(t *testing.T) {
	var calls atomic.Int32
	release := make(chan struct{})
	rig := newFirstRig(t, func(context.Context, Identity) (string, error) {
		calls.Add(1)
		<-release
		return "Member", nil
	}, "")
	const n = 8
	var wg sync.WaitGroup
	results := make([]bool, n)
	started := make(chan struct{}, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started <- struct{}{}
			r := httptest.NewRequest(http.MethodGet, AuthCallbackPath, nil)
			bound, status := rig.auth.firstSignIn(r, Identity{Sub: fakeSub})
			results[i] = bound && status == 0
		}()
	}
	for range n {
		<-started
	}
	close(release)
	wg.Wait()
	for i, ok := range results {
		if !ok {
			t.Errorf("sign-in %d was refused", i)
		}
	}
	if calls.Load() != 1 {
		t.Errorf("hook ran %d times for %d concurrent first sign-ins, want 1", calls.Load(), n)
	}
	if len(rig.bindings.binds) != 1 {
		t.Errorf("BindRole calls = %v, want exactly 1", rig.bindings.binds)
	}
	if len(rig.auth.firstSub) != 0 {
		t.Errorf("per-subject locks left behind: %d", len(rig.auth.firstSub))
	}
}
