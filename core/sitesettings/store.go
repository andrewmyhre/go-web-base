// Package sitesettings is the site's own settings, kept in Postgres and
// managed by an Admin: whether the site is open for sign-up or invite-only,
// and the invites that let one email address in while it is invite-only.
//
// Core ships the store, a sign-up policy that plugs into core's first
// sign-in (server.SignUpPolicy) and the admin routes (Routes). A setting
// with no row is at its default, so an app that upgrades to a core with this
// package behaves exactly as before until an Admin changes something.
//
// Settings are typed: each one is a definition (key, label, allowed values,
// default) and a write is validated against it. The definitions are a fixed
// list today. The store reads them from one table of definitions, so letting
// an app register its own is an addition, not a redesign.
package sitesettings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/andrewmyhre/go-web-base/core/db/sqlc"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// Resources are the permission resources the admin routes are guarded by.
// core registers them and grants them to the Admin role; an app that wants
// another role to manage them grants them itself.
const (
	SettingsResource server.Resource = "SiteSettings"
	InvitesResource  server.Resource = "SiteInvites"
)

// KeySignUpMode is the key of the sign-up mode setting.
const KeySignUpMode = "sign_up_mode"

// SignUpMode says who may make a first sign-in.
type SignUpMode string

const (
	// SignUpOpen lets anyone with a Google account make a first sign-in
	// (where the app admits new subjects at all). It is the default and
	// what every app did before this setting existed.
	SignUpOpen SignUpMode = "open"
	// SignUpInviteOnly lets only an email address with a pending invite
	// make a first sign-in. Accounts that already exist always sign in.
	SignUpInviteOnly SignUpMode = "invite_only"
)

// InviteOnlyMessage is the page text shown to someone refused because the
// site is invite-only.
const InviteOnlyMessage = "this site is invite-only: ask an administrator to invite your email address, then sign in again"

// DefaultCacheTTL bounds how long a process serves a setting it has read.
// A change made through this process is visible at once; a change made
// through another replica is visible within this long.
const DefaultCacheTTL = 5 * time.Second

// Errors the store returns.
var (
	// ErrUnknownSetting is returned for a key that is not a defined setting.
	ErrUnknownSetting = errors.New("unknown setting")
	// ErrInvalidValue is returned for a value the setting does not allow.
	ErrInvalidValue = errors.New("invalid setting value")
	// ErrInvalidEmail is returned for an invite whose address is not a plain
	// email address.
	ErrInvalidEmail = errors.New("invalid email address")
	// ErrInvitePending is returned when the address already has a pending
	// invite.
	ErrInvitePending = errors.New("an invite for this address is already pending")
	// ErrInviteNotFound is returned when no pending invite has that id: it
	// never existed, or it was already revoked or consumed.
	ErrInviteNotFound = errors.New("no pending invite with that id")
)

// Def defines one setting.
type Def struct {
	Key         string
	Label       string
	Description string
	// Options are the allowed values. Every setting today is a choice of
	// strings.
	Options []string
	Default string
}

// builtinDefs are the settings core ships.
func builtinDefs() []Def {
	return []Def{{
		Key:         KeySignUpMode,
		Label:       "Sign-up",
		Description: "open lets anyone with a Google account make a first sign-in; invite_only lets only an invited email address in. Existing accounts always sign in.",
		Options:     []string{string(SignUpOpen), string(SignUpInviteOnly)},
		Default:     string(SignUpOpen),
	}}
}

// Setting is a definition with its current value.
type Setting struct {
	Def
	Value     string
	UpdatedAt *time.Time
	UpdatedBy string
}

// Invite is one invite. An invite is pending until it is revoked or
// consumed.
type Invite struct {
	ID         int64
	Email      string
	CreatedBy  string
	CreatedAt  time.Time
	RevokedAt  *time.Time
	ConsumedAt *time.Time
	ConsumedBy string
}

// Pending reports whether the invite can still be used.
func (i Invite) Pending() bool { return i.RevokedAt == nil && i.ConsumedAt == nil }

// Store reads and writes the site settings and invites. It is safe for
// concurrent use.
type Store struct {
	q    *sqlc.Queries
	defs []Def
	ttl  time.Duration
	now  func() time.Time

	mu     sync.Mutex
	cache  map[string]sqlc.SiteSetting
	loaded time.Time
}

// Option configures a Store.
type Option func(*Store)

// WithCacheTTL sets how long a read is served from memory. Zero or less
// reads the database every time.
func WithCacheTTL(d time.Duration) Option { return func(s *Store) { s.ttl = d } }

func withClock(now func() time.Time) Option { return func(s *Store) { s.now = now } }

// New returns a Store over db, which is the pool (or a transaction). The
// core migrations must have been applied.
func New(db sqlc.DBTX, opts ...Option) *Store {
	s := &Store{q: sqlc.New(db), defs: builtinDefs(), ttl: DefaultCacheTTL, now: time.Now}
	for _, o := range opts {
		o(s)
	}
	return s
}

func (s *Store) def(key string) (Def, bool) {
	for _, d := range s.defs {
		if d.Key == key {
			return d, true
		}
	}
	return Def{}, false
}

// rows returns every stored setting row, from the cache when it is fresh.
func (s *Store) rows(ctx context.Context) (map[string]sqlc.SiteSetting, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache != nil && s.ttl > 0 && s.now().Sub(s.loaded) < s.ttl {
		return s.cache, nil
	}
	list, err := s.q.ListSiteSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("read site settings: %w", err)
	}
	m := make(map[string]sqlc.SiteSetting, len(list))
	for _, r := range list {
		m[r.Key] = r
	}
	s.cache, s.loaded = m, s.now()
	return m, nil
}

func (s *Store) invalidate() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// value is the stored value of d, or its default when nothing is stored or
// the stored value is no longer one the definition allows.
func value(d Def, row sqlc.SiteSetting, ok bool) string {
	if !ok {
		return d.Default
	}
	var v string
	if err := json.Unmarshal(row.Value, &v); err != nil || !slices.Contains(d.Options, v) {
		return d.Default
	}
	return v
}

// List returns every setting with its current value.
func (s *Store) List(ctx context.Context) ([]Setting, error) {
	rows, err := s.rows(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Setting, 0, len(s.defs))
	for _, d := range s.defs {
		row, ok := rows[d.Key]
		st := Setting{Def: d, Value: value(d, row, ok)}
		if ok {
			t := row.UpdatedAt.Time
			st.UpdatedAt, st.UpdatedBy = &t, row.UpdatedBy
		}
		out = append(out, st)
	}
	return out, nil
}

// Get returns the current value of a setting.
func (s *Store) Get(ctx context.Context, key string) (string, error) {
	d, ok := s.def(key)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownSetting, key)
	}
	rows, err := s.rows(ctx)
	if err != nil {
		return "", err
	}
	row, found := rows[key]
	return value(d, row, found), nil
}

// Set validates v against the setting's definition and stores it, recording
// who changed it. The change is visible to this process at once.
func (s *Store) Set(ctx context.Context, key, v, by string) error {
	d, ok := s.def(key)
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownSetting, key)
	}
	if !slices.Contains(d.Options, v) {
		return fmt.Errorf("%w: %q for %s, want one of %s", ErrInvalidValue, v, key, strings.Join(d.Options, ", "))
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	defer s.invalidate()
	if err := s.q.UpsertSiteSetting(ctx, sqlc.UpsertSiteSettingParams{Key: key, Value: raw, UpdatedBy: by}); err != nil {
		return fmt.Errorf("write site setting %s: %w", key, err)
	}
	return nil
}

// SignUpMode returns the current sign-up mode.
func (s *Store) SignUpMode(ctx context.Context) (SignUpMode, error) {
	v, err := s.Get(ctx, KeySignUpMode)
	return SignUpMode(v), err
}

// NormalizeEmail lower-cases and trims an address and requires a plain
// address with no display name. Invites match on this form, and so does the
// Google-verified address at sign-in.
func NormalizeEmail(email string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(email))
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e || !strings.Contains(e, "@") {
		return "", fmt.Errorf("%w: %q", ErrInvalidEmail, email)
	}
	return e, nil
}

func toInvite(r sqlc.SiteInvite) Invite {
	i := Invite{ID: r.ID, Email: r.Email, CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt.Time, ConsumedBy: r.ConsumedBy.String}
	if r.RevokedAt.Valid {
		t := r.RevokedAt.Time
		i.RevokedAt = &t
	}
	if r.ConsumedAt.Valid {
		t := r.ConsumedAt.Time
		i.ConsumedAt = &t
	}
	return i
}

// CreateInvite invites an email address, recording which admin did. An
// address with an invite already pending gets ErrInvitePending.
func (s *Store) CreateInvite(ctx context.Context, email, by string) (Invite, error) {
	e, err := NormalizeEmail(email)
	if err != nil {
		return Invite{}, err
	}
	row, err := s.q.CreateSiteInvite(ctx, sqlc.CreateSiteInviteParams{Email: e, CreatedBy: by})
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return Invite{}, ErrInvitePending
	}
	if err != nil {
		return Invite{}, fmt.Errorf("create invite: %w", err)
	}
	return toInvite(row), nil
}

// ListInvites returns every invite, newest first, whatever its state.
func (s *Store) ListInvites(ctx context.Context) ([]Invite, error) {
	rows, err := s.q.ListSiteInvites(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invites: %w", err)
	}
	out := make([]Invite, 0, len(rows))
	for _, r := range rows {
		out = append(out, toInvite(r))
	}
	return out, nil
}

// RevokeInvite withdraws a pending invite. A revoked invite never admits
// anyone. An invite that is not pending (unknown, revoked, consumed) gets
// ErrInviteNotFound.
func (s *Store) RevokeInvite(ctx context.Context, id int64) (Invite, error) {
	row, err := s.q.RevokeSiteInvite(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return Invite{}, ErrInviteNotFound
	}
	if err != nil {
		return Invite{}, fmt.Errorf("revoke invite: %w", err)
	}
	return toInvite(row), nil
}

// Admit is server.SignUpPolicy.Admit. In open mode it admits everyone. In
// invite-only mode it admits a subject whose Google-verified email has a
// pending invite and refuses every other with a server.RefusalError that
// says the site is invite-only: no email (Google did not verify one) is
// refused too. A failure to read the mode or the invites is returned as is,
// which core answers with 503: it never admits on a failed read.
func (s *Store) Admit(ctx context.Context, id server.Identity) error {
	mode, err := s.SignUpMode(ctx)
	if err != nil {
		return err
	}
	if mode != SignUpInviteOnly {
		return nil
	}
	refused := &server.RefusalError{Message: InviteOnlyMessage}
	email, err := NormalizeEmail(id.Email)
	if err != nil {
		return refused
	}
	ok, err := s.q.HasPendingSiteInvite(ctx, email)
	if err != nil {
		return fmt.Errorf("read invites: %w", err)
	}
	if !ok {
		return refused
	}
	return nil
}

// Admitted is server.SignUpPolicy.Admitted: it consumes the subject's
// pending invite, if there is one, so an invite admits one first sign-in and
// no more. It does so in either mode, so an invite sent while the site was
// open is spent by that person's first sign-in too.
func (s *Store) Admitted(ctx context.Context, id server.Identity) error {
	email, err := NormalizeEmail(id.Email)
	if err != nil {
		return nil // no verified email, so no invite could have admitted it
	}
	if _, err := s.q.ConsumeSiteInvite(ctx, sqlc.ConsumeSiteInviteParams{ConsumedBy: pgtype.Text{String: id.Sub, Valid: true}, Email: email}); err != nil {
		return fmt.Errorf("consume invite: %w", err)
	}
	return nil
}

var _ server.SignUpPolicy = (*Store)(nil)
