package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/redis"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// mapSessions is a SessionStore in memory, so the test needs no Redis.
type mapSessions struct {
	mu   sync.Mutex
	data map[string]map[string]string
}

func (m *mapSessions) Create(_ context.Context, _ time.Duration, d map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.data == nil {
		m.data = map[string]map[string]string{}
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	m.data[id] = d
	return id, nil
}
func (m *mapSessions) Get(_ context.Context, id string) (map[string]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.data[id]; ok {
		return d, nil
	}
	return nil, redis.ErrNotFound
}
func (m *mapSessions) Take(ctx context.Context, id string) (map[string]string, error) {
	return m.Get(ctx, id)
}
func (m *mapSessions) Delete(context.Context, string) error { return nil }

// siteSettingsRig is a real router over a real database in a scratch schema:
// the admin routes, the role graph and Casbin, with two users, an Admin and
// a Member.
type siteSettingsRig struct {
	h        http.Handler
	sessions *mapSessions
	cookie   string
	d        Deps
}

func newSiteSettingsRig(t *testing.T) *siteSettingsRig {
	t.Helper()
	if os.Getenv("CORE_SKIP_SERVICE_TESTS") == "1" {
		t.Skip("CORE_SKIP_SERVICE_TESTS=1: skipping a test that needs the test Postgres")
	}
	dsn := os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("CORE_TEST_DATABASE_URL is not set; this test needs the test Postgres (see core/README.md)")
	}
	ctx := t.Context()
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	schema := "t_" + hex.EncodeToString(buf)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE") })
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("options", "-csearch_path="+schema)
	u.RawQuery = q.Encode()
	cfg := config.Default()
	cfg.DatabaseDSN = u.String()
	cfg.AdminSub = "admin-sub"

	o := testOpts
	o.Roles = []db.RoleDef{
		{Name: db.AdminRole, Permissions: []string{"Projects: Read"}},
		{Name: "Member", Permissions: []string{"Projects: Read"}},
	}
	inst := mustInstall(t, o)
	d, cleanup, err := connect(ctx, inst, cfg, zap.NewNop())
	t.Cleanup(cleanup)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := d.Authorizer.(server.CasbinAuthorizer).BindRole("member-sub", "Member"); err != nil {
		t.Fatal(err)
	}
	rig := &siteSettingsRig{sessions: &mapSessions{}, d: d}
	d.Sessions = rig.sessions
	router, err := newRouter(inst, d)
	if err != nil {
		t.Fatal(err)
	}
	rig.h, err = router.Build(zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}
	return rig
}

// as signs in as sub and returns the cookie header value.
func (r *siteSettingsRig) as(t *testing.T, sub string) string {
	t.Helper()
	id, err := r.sessions.Create(context.Background(), time.Hour, map[string]string{"sub": sub, "name": sub})
	if err != nil {
		t.Fatal(err)
	}
	return server.NewNames(testOpts.Name, testOpts.CookiePrefix).SessionCookie + "=" + id
}

func (r *siteSettingsRig) do(method, path, cookie string, body any) *httptest.ResponseRecorder {
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec
}

// Every admin route is for Admin only: the Admin gets in, a Member is
// forbidden and a visitor with no session is unauthorized. Each of the five
// routes is tried by each of the three, so a route that declared the wrong
// permission, or none an Admin alone holds, fails here.
func TestSiteSettingsRoutesAreAdminOnly(t *testing.T) {
	rig := newSiteSettingsRig(t)
	adminCookie, memberCookie := rig.as(t, "admin-sub"), rig.as(t, "member-sub")

	// The Admin creates an invite first so the revoke route has a target.
	rec := rig.do(http.MethodPost, "/admin/invites", adminCookie, map[string]string{"email": "first@example.com"})
	if rec.Code != http.StatusOK {
		t.Fatalf("admin POST /admin/invites = %d: %s", rec.Code, rec.Body)
	}
	var inv struct{ ID int64 }
	if err := json.Unmarshal(rec.Body.Bytes(), &inv); err != nil || inv.ID == 0 {
		t.Fatalf("invite response %s (%v)", rec.Body, err)
	}

	type call struct {
		method, path string
		body         any
	}
	calls := []call{
		{http.MethodGet, "/admin/settings", nil},
		{http.MethodPut, "/admin/settings/sign_up_mode", map[string]string{"value": "open"}},
		{http.MethodGet, "/admin/invites", nil},
		{http.MethodPost, "/admin/invites", map[string]string{"email": "second@example.com"}},
		{http.MethodDelete, "/admin/invites/" + strconv.FormatInt(inv.ID, 10), nil},
	}
	for _, c := range calls {
		if rec := rig.do(c.method, c.path, "", c.body); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s with no session = %d, want 401", c.method, c.path, rec.Code)
		}
		if rec := rig.do(c.method, c.path, memberCookie, c.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s as a Member = %d, want 403", c.method, c.path, rec.Code)
		}
	}
	// The refused calls changed nothing: the invite is still pending and the
	// Member's POST made no second invite.
	rec = rig.do(http.MethodGet, "/admin/invites", adminCookie, nil)
	var list struct {
		Invites []struct {
			Email, Status string
		}
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Invites) != 1 || list.Invites[0].Status != "pending" {
		t.Fatalf("invites after refused calls = %+v, want exactly the one pending invite", list.Invites)
	}
	for _, c := range calls {
		if rec := rig.do(c.method, c.path, adminCookie, c.body); rec.Code >= 300 {
			t.Errorf("%s %s as Admin = %d: %s", c.method, c.path, rec.Code, rec.Body)
		}
	}
}

// The Admin changes the mode through the API and the store, which the
// sign-in policy reads, sees it at once.
func TestSiteSettingsAdminChangesTheMode(t *testing.T) {
	rig := newSiteSettingsRig(t)
	admin := rig.as(t, "admin-sub")
	mode := func() string {
		m, err := rig.d.SiteSettings.SignUpMode(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return string(m)
	}
	if mode() != "open" {
		t.Fatalf("mode before any change = %q, want open", mode())
	}
	if rec := rig.do(http.MethodPut, "/admin/settings/sign_up_mode", admin, map[string]string{"value": "invite_only"}); rec.Code != http.StatusOK {
		t.Fatalf("PUT = %d: %s", rec.Code, rec.Body)
	}
	if mode() != "invite_only" {
		t.Fatalf("mode after the PUT = %q, want invite_only", mode())
	}
	for _, c := range []struct {
		path, body string
		want       int
	}{
		{"/admin/settings/sign_up_mode", `{"value":"closed"}`, http.StatusUnprocessableEntity},
		{"/admin/settings/nope", `{"value":"open"}`, http.StatusNotFound},
	} {
		req := httptest.NewRequest(http.MethodPut, c.path, bytes.NewBufferString(c.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", admin)
		rec := httptest.NewRecorder()
		rig.h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("PUT %s %s = %d, want %d: %s", c.path, c.body, rec.Code, c.want, rec.Body)
		}
	}
	if mode() != "invite_only" {
		t.Fatalf("a refused change moved the mode to %q", mode())
	}
}
