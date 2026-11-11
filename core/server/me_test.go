package server

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/andrewmyhre/go-web-base/core/db"
)

// roleReader is a RoleReader over a fixed list of bindings per subject. It
// returns err when that is set.
type roleReader struct {
	bound map[string][]db.Binding
	err   error
	calls []string
}

func (r *roleReader) RoleBindingsOf(sub string) ([]db.Binding, error) {
	r.calls = append(r.calls, sub)
	if r.err != nil {
		return nil, r.err
	}
	return r.bound[sub], nil
}

func meHandler(t *testing.T, roles RoleReader) http.Handler {
	t.Helper()
	r := testRouter("t", "0").WithAuth(newMemStore(), nil, testNames.SessionCookie)
	if err := Register(r, op(http.MethodGet, "/me", Authenticated), NewMe(roles, nil)); err != nil {
		t.Fatal(err)
	}
	return mustBuild(t, r, nil)
}

// TestMeReportsRolesFromTheReader: /me lists every binding the reader
// returns for the signed-in subject, in its order, and asks for no one
// else's.
func TestMeReportsRolesFromTheReader(t *testing.T) {
	rr := &roleReader{bound: map[string][]db.Binding{
		testSub: {{Role: "Admin", Domain: db.Global}, {Role: "Contributor", Domain: "proj-a"}},
	}}
	rec := request(meHandler(t, rr), http.MethodGet, "/me", testSessionID)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /me = %d: %s", rec.Code, rec.Body)
	}
	want := `{"id":"` + testSub + `","name":"Test User","email":"test@example.com","avatar":"https://example.com/a.png",` +
		`"roles":[{"role":"Admin","domain":"global"},{"role":"Contributor","domain":"proj-a"}]}`
	if got := string(bytes.TrimSpace(rec.Body.Bytes())); got != want {
		t.Fatalf("GET /me = %s, want %s", got, want)
	}
	if len(rr.calls) != 1 || rr.calls[0] != testSub {
		t.Fatalf("reader asked for %v, want [%s]", rr.calls, testSub)
	}
}

// TestMeWithNoBindingsIsAnEmptyList: a subject the reader knows nothing
// about gets "roles": [], never null.
func TestMeWithNoBindingsIsAnEmptyList(t *testing.T) {
	rec := request(meHandler(t, &roleReader{}), http.MethodGet, "/me", testSessionID)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"roles":[]`)) {
		t.Fatalf("GET /me = %d %s, want 200 with \"roles\":[]", rec.Code, rec.Body)
	}
}

// TestMeRoleReadFailureIs503: a failure to read the bindings is a 503, so
// an outage never shows the user as holding no roles.
func TestMeRoleReadFailureIs503(t *testing.T) {
	rec := request(meHandler(t, &roleReader{err: errors.New("db down")}), http.MethodGet, "/me", testSessionID)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET /me with the reader failing = %d %s, want 503", rec.Code, rec.Body)
	}
}
