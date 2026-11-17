package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
)

// fakePolicy is a SignUpPolicy that records its calls.
type fakePolicy struct {
	admit    error
	admitted int
	order    *[]string
}

func (p *fakePolicy) Admit(context.Context, Identity) error {
	if p.order != nil {
		*p.order = append(*p.order, "policy")
	}
	return p.admit
}

func (p *fakePolicy) Admitted(context.Context, Identity) error {
	p.admitted++
	return nil
}

func policyRig(t *testing.T, p *fakePolicy, hook FirstSignIn, bound bool) *signInRig {
	t.Helper()
	var rig *signInRig
	b := &bindings{bound: map[string]bool{fakeSub: bound}}
	rig = newSignInRigWith(t, b, withBinder(&rig, hook, "Member"), func(a *Auth) { a.WithSignUpPolicy(p) })
	return rig
}

func TestSignUpPolicy(t *testing.T) {
	t.Run("admitting runs before the hook and is recorded after the bind", func(t *testing.T) {
		var order []string
		p := &fakePolicy{order: &order}
		rig := policyRig(t, p, func(context.Context, Identity) (string, error) {
			order = append(order, "hook")
			return "Member", nil
		}, false)
		wantSignedIn(t, signIn(t, rig))
		if got := strings.Join(order, ","); got != "policy,hook" {
			t.Fatalf("order = %s, want policy before hook", got)
		}
		if p.admitted != 1 {
			t.Fatalf("Admitted ran %d times, want 1", p.admitted)
		}
	})

	t.Run("a refusal never reaches the hook, binds nothing and shows its message", func(t *testing.T) {
		hookRan := false
		p := &fakePolicy{admit: &RefusalError{Message: "this site is invite-only"}}
		rig := policyRig(t, p, func(context.Context, Identity) (string, error) {
			hookRan = true
			return "Member", nil
		}, false)
		rec := signIn(t, rig)
		wantRefused(t, rig, rec, http.StatusForbidden)
		if hookRan || len(rig.bindings.binds) != 0 {
			t.Fatalf("hook ran %v, binds %v after a policy refusal", hookRan, rig.bindings.binds)
		}
		if !strings.Contains(rec.Body.String(), "this site is invite-only") {
			t.Fatalf("body = %s, want the refusal message", rec.Body)
		}
		if p.admitted != 0 {
			t.Fatalf("Admitted ran %d times for a refusal", p.admitted)
		}
	})

	t.Run("a plain ErrSignInRefused is the generic 403", func(t *testing.T) {
		rig := policyRig(t, &fakePolicy{admit: ErrSignInRefused}, nil, false)
		rec := signIn(t, rig)
		wantRefused(t, rig, rec, http.StatusForbidden)
		if !strings.Contains(rec.Body.String(), "you cannot sign in to") {
			t.Fatalf("body = %s, want the default refusal text", rec.Body)
		}
	})

	t.Run("a policy failure is a 503, never an admission", func(t *testing.T) {
		rig := policyRig(t, &fakePolicy{admit: errors.New("db down")}, nil, false)
		wantRefused(t, rig, signIn(t, rig), http.StatusServiceUnavailable)
		if len(rig.bindings.binds) != 0 {
			t.Fatalf("binds = %v after a policy failure", rig.bindings.binds)
		}
	})

	t.Run("an existing account never meets the policy", func(t *testing.T) {
		p := &fakePolicy{admit: ErrSignInRefused}
		rig := policyRig(t, p, nil, true)
		wantSignedIn(t, signIn(t, rig))
		if p.admitted != 0 {
			t.Fatalf("Admitted ran for an account that already existed")
		}
	})

	t.Run("a hook's own refusal message is shown too", func(t *testing.T) {
		rig := policyRig(t, &fakePolicy{}, func(context.Context, Identity) (string, error) {
			return "", &RefusalError{Message: "only staff"}
		}, false)
		rec := signIn(t, rig)
		wantRefused(t, rig, rec, http.StatusForbidden)
		if !strings.Contains(rec.Body.String(), "only staff") {
			t.Fatalf("body = %s, want the hook's message", rec.Body)
		}
	})
}
