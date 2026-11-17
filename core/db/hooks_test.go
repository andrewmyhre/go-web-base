package db

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	sqlc "github.com/andrewmyhre/go-web-base/core/db/sqlc"
)

// TestLockOfIsTheLockCoreTakes: the same enforcer gives the same lock,
// another enforcer another, and a writer holding it stops core's reads, so
// app code holding LockOf for its own write is excluded from them.
func TestLockOfIsTheLockCoreTakes(t *testing.T) {
	m, err := model.NewModelFromString(rbacModelString)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatal(err)
	}
	e2, err := casbin.NewEnforcer(m.Copy())
	if err != nil {
		t.Fatal(err)
	}
	if LockOf(e1) != LockOf(e1) {
		t.Fatal("LockOf returned two locks for one enforcer")
	}
	if LockOf(e1) == LockOf(e2) {
		t.Fatal("two enforcers share a lock")
	}

	l := LockOf(e1)
	l.Lock()
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = Authorize(e1, "sub", "global", "Projects", "Read")
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("Authorize read while app code held LockOf for a write")
	case <-time.After(50 * time.Millisecond):
	}
	l.Unlock()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Authorize did not resume after the lock was released")
	}
	wg.Wait()
}

// deadlineDB is a DBTX that records the deadline of the context each call
// got. Begin fails, so a write stops after reporting it.
type deadlineDB struct{ deadlines []time.Time }

var errNoDB = errors.New("no database")

func (d *deadlineDB) note(ctx context.Context) {
	dl, _ := ctx.Deadline() // the zero time when there is none
	d.deadlines = append(d.deadlines, dl)
}
func (d *deadlineDB) Exec(ctx context.Context, _ string, _ ...interface{}) (pgconn.CommandTag, error) {
	d.note(ctx)
	return pgconn.CommandTag{}, errNoDB
}
func (d *deadlineDB) Query(ctx context.Context, _ string, _ ...interface{}) (pgx.Rows, error) {
	d.note(ctx)
	return nil, errNoDB
}
func (d *deadlineDB) QueryRow(ctx context.Context, _ string, _ ...interface{}) pgx.Row {
	d.note(ctx)
	return errRow{}
}
func (d *deadlineDB) Begin(ctx context.Context) (pgx.Tx, error) {
	d.note(ctx)
	return nil, errNoDB
}

type errRow struct{}

func (errRow) Scan(...any) error { return errNoDB }

// TestPolicyWriteTimeoutBoundsEveryAdapterWrite: with a timeout each kind of
// write gets a context that expires about that long from now; without one
// the context has no deadline, as before; loads are never bounded.
func TestPolicyWriteTimeoutBoundsEveryAdapterWrite(t *testing.T) {
	writes := map[string]func(a *CasbinAdapter) error{
		"AddPolicy":            func(a *CasbinAdapter) error { return a.AddPolicy("p", "p", []string{"r", "o", "a"}) },
		"AddPolicies":          func(a *CasbinAdapter) error { return a.AddPolicies("p", "p", [][]string{{"r", "o", "a"}}) },
		"RemovePolicy":         func(a *CasbinAdapter) error { return a.RemovePolicy("p", "p", []string{"r", "o", "a"}) },
		"RemovePolicies":       func(a *CasbinAdapter) error { return a.RemovePolicies("p", "p", [][]string{{"r", "o", "a"}}) },
		"RemoveFilteredPolicy": func(a *CasbinAdapter) error { return a.RemoveFilteredPolicy("p", "p", 0, "r") },
		"SavePolicy": func(a *CasbinAdapter) error {
			m, _ := model.NewModelFromString(rbacModelString)
			return a.SavePolicy(m)
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			bounded := &deadlineDB{}
			before := time.Now()
			_ = write(NewCasbinAdapter(bounded).WithWriteTimeout(time.Hour))
			if len(bounded.deadlines) == 0 {
				t.Fatal("the write reached the database zero times")
			}
			for _, dl := range bounded.deadlines {
				if dl.IsZero() || dl.Before(before.Add(59*time.Minute)) || dl.After(time.Now().Add(time.Hour)) {
					t.Errorf("deadline %v, want about an hour from now", dl)
				}
			}
			free := &deadlineDB{}
			_ = write(NewCasbinAdapter(free))
			for _, dl := range free.deadlines {
				if !dl.IsZero() {
					t.Errorf("an adapter with no timeout gave a write the deadline %v", dl)
				}
			}
		})
	}
	loads := &deadlineDB{}
	m, _ := model.NewModelFromString(rbacModelString)
	_ = NewCasbinAdapter(loads).WithWriteTimeout(time.Hour).LoadPolicy(m)
	if len(loads.deadlines) == 0 || !loads.deadlines[0].IsZero() {
		t.Errorf("a load got deadlines %v, want none: only writes are bounded", loads.deadlines)
	}
}

// TestNegativeWriteTimeoutMeansNone: it cannot make every write expire.
func TestNegativeWriteTimeoutMeansNone(t *testing.T) {
	d := &deadlineDB{}
	_ = NewCasbinAdapter(d).WithWriteTimeout(-time.Second).AddPolicy("p", "p", []string{"r", "o", "a"})
	if len(d.deadlines) == 0 || !d.deadlines[0].IsZero() {
		t.Errorf("deadlines %v, want none", d.deadlines)
	}
}

// TestNewEnforcerAppliesThePolicyWriteTimeout (service test): a 1ns bound
// makes the first policy write fail, and no option writes as before.
func TestNewEnforcerAppliesThePolicyWriteTimeout(t *testing.T) {
	cfg := testSchema(t)
	pool := testPool(t, cfg)
	if err := MigrateWithConfig(context.Background(), cfg.ConnConfig); err != nil {
		t.Fatal(err)
	}
	if _, err := NewEnforcer(context.Background(), pool, "", exampleRoles, WithPolicyWriteTimeout(time.Nanosecond)); err == nil {
		t.Fatal("NewEnforcer with a 1ns write timeout succeeded; the option did not reach the adapter")
	}
	if _, err := NewEnforcer(context.Background(), pool, "", exampleRoles); err != nil {
		t.Fatalf("NewEnforcer with no option: %v", err)
	}
	if _, err := NewEnforcer(context.Background(), pool, "", exampleRoles, WithPolicyWriteTimeout(time.Minute)); err != nil {
		t.Fatalf("NewEnforcer with a generous timeout: %v", err)
	}
}

// failingTx is a pgx.Tx whose Rollback fails.
type failingTx struct{ pgx.Tx }

func (failingTx) Rollback(context.Context) error {
	return errors.New("rollback failed: connection lost")
}

type txDB struct{ deadlineDB }

func (d *txDB) Begin(context.Context) (pgx.Tx, error) { return failingTx{}, nil }

// TestRollbackFailureKeepsTheOriginalError: when the write timed out and the
// rollback then failed too, errors.Is still sees the deadline, so a caller can
// tell "state unknown, retry" from a refusal.
func TestRollbackFailureKeepsTheOriginalError(t *testing.T) {
	a := NewCasbinAdapter(&txDB{})
	err := a.querierWithTx(context.Background(), func(sqlc.Querier) error { return context.DeadlineExceeded })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v; errors.Is(DeadlineExceeded) lost the original error", err)
	}
	if err == nil || !strings.Contains(err.Error(), "rollback failed") {
		t.Errorf("err = %v, want the rollback failure kept too", err)
	}
}
