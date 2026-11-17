package sitesettings

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/andrewmyhre/go-web-base/core/db"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// testStore returns a Store over a migrated scratch schema, with no cache so
// each assertion sees the database.
func testStore(t *testing.T, opts ...Option) (*Store, *pgxpool.Pool) {
	t.Helper()
	if os.Getenv("CORE_SKIP_SERVICE_TESTS") == "1" {
		t.Skip("CORE_SKIP_SERVICE_TESTS=1: skipping a test that needs the test Postgres")
	}
	dsn := os.Getenv("CORE_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("CORE_TEST_DATABASE_URL is not set; sitesettings tests need the test Postgres (see core/README.md)")
	}
	ctx := context.Background()
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	schema := "t_" + hex.EncodeToString(buf)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE")
		_ = admin.Close(context.Background())
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	if err := db.MigrateWithConfig(ctx, cfg.ConnConfig); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := db.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return New(pool, append([]Option{WithCacheTTL(0)}, opts...)...), pool
}

var ctx = context.Background()

func mustMode(t *testing.T, s *Store, m SignUpMode) {
	t.Helper()
	if err := s.Set(ctx, KeySignUpMode, string(m), "admin"); err != nil {
		t.Fatalf("set mode %s: %v", m, err)
	}
}

func visitor(email string) server.Identity {
	return server.Identity{Sub: "sub-" + email, Email: email, Name: "V"}
}

func wantRefusal(t *testing.T, err error) {
	t.Helper()
	var r *server.RefusalError
	if !errors.Is(err, server.ErrSignInRefused) || !errors.As(err, &r) || r.Message != InviteOnlyMessage {
		t.Fatalf("err = %v, want a server.ErrSignInRefused carrying the invite-only message", err)
	}
}

// A fresh database is open: every app that upgrades keeps today's
// behaviour until an Admin changes it.
func TestDefaultIsOpenAndAdmitsEveryone(t *testing.T) {
	s, _ := testStore(t)
	if m, err := s.SignUpMode(ctx); err != nil || m != SignUpOpen {
		t.Fatalf("mode = %q, %v; want open", m, err)
	}
	for _, id := range []server.Identity{visitor("a@example.com"), {Sub: "no-email"}} {
		if err := s.Admit(ctx, id); err != nil {
			t.Errorf("open mode refused %+v: %v", id, err)
		}
	}
}

func TestInviteOnlyRefusesAnEmailWithNoInvite(t *testing.T) {
	s, _ := testStore(t)
	mustMode(t, s, SignUpInviteOnly)
	wantRefusal(t, s.Admit(ctx, visitor("stranger@example.com")))
	// Google gave no verified email: nothing to match an invite on.
	wantRefusal(t, s.Admit(ctx, server.Identity{Sub: "x"}))
	// An invite for somebody else does not admit this address.
	if _, err := s.CreateInvite(ctx, "friend@example.com", "admin"); err != nil {
		t.Fatal(err)
	}
	wantRefusal(t, s.Admit(ctx, visitor("stranger@example.com")))
}

func TestInviteOnlyAdmitsAPendingInviteCaseInsensitively(t *testing.T) {
	s, _ := testStore(t)
	mustMode(t, s, SignUpInviteOnly)
	inv, err := s.CreateInvite(ctx, "  Friend@Example.COM ", "admin-sub")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Email != "friend@example.com" || inv.CreatedBy != "admin-sub" || !inv.Pending() {
		t.Fatalf("invite = %+v", inv)
	}
	if err := s.Admit(ctx, visitor("FRIEND@example.com")); err != nil {
		t.Fatalf("an invited address was refused: %v", err)
	}
}

func TestAnInviteIsConsumedOnce(t *testing.T) {
	s, _ := testStore(t)
	mustMode(t, s, SignUpInviteOnly)
	if _, err := s.CreateInvite(ctx, "friend@example.com", "admin"); err != nil {
		t.Fatal(err)
	}
	id := visitor("friend@example.com")
	if err := s.Admit(ctx, id); err != nil {
		t.Fatal(err)
	}
	// Admit alone does not use the invite up: the sign-in may still fail
	// after it (the hook, the bind) and the person must be able to retry.
	if err := s.Admit(ctx, id); err != nil {
		t.Fatalf("a retry before the sign-in finished was refused: %v", err)
	}
	if err := s.Admitted(ctx, id); err != nil {
		t.Fatal(err)
	}
	// Now it is spent: another account with the same address is refused.
	wantRefusal(t, s.Admit(ctx, server.Identity{Sub: "another-sub", Email: "friend@example.com"}))
	list, err := s.ListInvites(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("invites = %+v, %v", list, err)
	}
	if list[0].ConsumedAt == nil || list[0].ConsumedBy != id.Sub || list[0].Pending() {
		t.Fatalf("invite after consumption = %+v, want consumed by %s", list[0], id.Sub)
	}
	// Consuming again changes nothing and does not fail.
	if err := s.Admitted(ctx, id); err != nil {
		t.Fatal(err)
	}
}

func TestARevokedInviteAdmitsNobody(t *testing.T) {
	s, _ := testStore(t)
	mustMode(t, s, SignUpInviteOnly)
	inv, err := s.CreateInvite(ctx, "friend@example.com", "admin")
	if err != nil {
		t.Fatal(err)
	}
	revoked, err := s.RevokeInvite(ctx, inv.ID)
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoke = %+v, %v", revoked, err)
	}
	wantRefusal(t, s.Admit(ctx, visitor("friend@example.com")))
	// It cannot be revoked twice, and a revoked invite is not consumed by a
	// later sign-in.
	if _, err := s.RevokeInvite(ctx, inv.ID); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("second revoke err = %v, want ErrInviteNotFound", err)
	}
	if err := s.Admitted(ctx, visitor("friend@example.com")); err != nil {
		t.Fatal(err)
	}
	list, _ := s.ListInvites(ctx)
	if list[0].ConsumedAt != nil {
		t.Fatalf("a revoked invite was consumed: %+v", list[0])
	}
	// The address can be invited again after a revocation.
	if _, err := s.CreateInvite(ctx, "friend@example.com", "admin"); err != nil {
		t.Fatalf("re-invite after revoke: %v", err)
	}
	if err := s.Admit(ctx, visitor("friend@example.com")); err != nil {
		t.Fatalf("re-invited address refused: %v", err)
	}
}

func TestAConsumedInviteCannotBeRevokedAndAPendingOneCannotBeStacked(t *testing.T) {
	s, _ := testStore(t)
	inv, err := s.CreateInvite(ctx, "friend@example.com", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateInvite(ctx, "Friend@example.com", "admin"); !errors.Is(err, ErrInvitePending) {
		t.Fatalf("duplicate invite err = %v, want ErrInvitePending", err)
	}
	if err := s.Admitted(ctx, visitor("friend@example.com")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RevokeInvite(ctx, inv.ID); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("revoke of a consumed invite err = %v, want ErrInviteNotFound", err)
	}
	if _, err := s.RevokeInvite(ctx, 9999); !errors.Is(err, ErrInviteNotFound) {
		t.Fatalf("revoke of an unknown invite err = %v, want ErrInviteNotFound", err)
	}
}

func TestInviteRejectsAnythingButAPlainAddress(t *testing.T) {
	s, _ := testStore(t)
	for _, bad := range []string{"", "nobody", "Name <a@example.com>", "a@example.com, b@example.com"} {
		if _, err := s.CreateInvite(ctx, bad, "admin"); !errors.Is(err, ErrInvalidEmail) {
			t.Errorf("CreateInvite(%q) err = %v, want ErrInvalidEmail", bad, err)
		}
	}
}

func TestSetValidatesAgainstTheDefinition(t *testing.T) {
	s, _ := testStore(t)
	if err := s.Set(ctx, KeySignUpMode, "closed", "admin"); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("bad value err = %v, want ErrInvalidValue", err)
	}
	if err := s.Set(ctx, "nope", "x", "admin"); !errors.Is(err, ErrUnknownSetting) {
		t.Fatalf("unknown key err = %v, want ErrUnknownSetting", err)
	}
	if m, _ := s.SignUpMode(ctx); m != SignUpOpen {
		t.Fatalf("a refused write moved the mode to %q", m)
	}
	mustMode(t, s, SignUpInviteOnly)
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || list[0].Value != "invite_only" || list[0].UpdatedBy != "admin" || list[0].UpdatedAt == nil || list[0].Default != "open" {
		t.Fatalf("List = %+v, %v", list, err)
	}
}

// A stored value the definition no longer allows reads as the default: a
// bad row can never lock everybody out or let everybody in by accident.
func TestAnUnreadableStoredValueFallsBackToTheDefault(t *testing.T) {
	s, pool := testStore(t)
	if _, err := pool.Exec(ctx, `INSERT INTO site_setting (key, value) VALUES ('sign_up_mode', '"bogus"')`); err != nil {
		t.Fatal(err)
	}
	if m, err := s.SignUpMode(ctx); err != nil || m != SignUpOpen {
		t.Fatalf("mode = %q, %v; want the default open", m, err)
	}
}

// The cache serves a read for its TTL, a write through the same store shows
// at once, and a change made behind its back shows once the TTL passes.
func TestCachedReads(t *testing.T) {
	now := time.Unix(1_000_000, 0)
	s, pool := testStore(t, WithCacheTTL(5*time.Second), withClock(func() time.Time { return now }))
	if m, _ := s.SignUpMode(ctx); m != SignUpOpen {
		t.Fatal("not open at start")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO site_setting (key, value) VALUES ('sign_up_mode', '"invite_only"')`); err != nil {
		t.Fatal(err)
	}
	if m, _ := s.SignUpMode(ctx); m != SignUpOpen {
		t.Fatalf("a change behind the cache showed at once: %q", m)
	}
	now = now.Add(6 * time.Second)
	if m, _ := s.SignUpMode(ctx); m != SignUpInviteOnly {
		t.Fatalf("after the TTL the mode is %q, want invite_only", m)
	}
	mustMode(t, s, SignUpOpen)
	if m, _ := s.SignUpMode(ctx); m != SignUpOpen {
		t.Fatalf("a write through the store did not show at once: %q", m)
	}
}

// A database that cannot be read is never an admission.
func TestAFailedReadRefusesWithAnErrorNotAnAdmission(t *testing.T) {
	s, pool := testStore(t)
	mustMode(t, s, SignUpInviteOnly)
	pool.Close()
	err := s.Admit(ctx, visitor("friend@example.com"))
	if err == nil || errors.Is(err, server.ErrSignInRefused) {
		t.Fatalf("err = %v, want a plain failure (core answers 503) that is neither nil nor a refusal", err)
	}
}
