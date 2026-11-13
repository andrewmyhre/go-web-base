package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/andrewmyhre/go-web-base/core/config"
	"github.com/andrewmyhre/go-web-base/core/server"
)

// blockUntilDone is a listener that serves until ctx is cancelled.
func blockUntilDone(ctx context.Context, _ *server.Router) error {
	<-ctx.Done()
	return nil
}

func serveTest(t *testing.T, ctx context.Context, o Options, listen func(context.Context, *server.Router) error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- serveWith(ctx, mustInstall(t, o), zap.NewNop(), Deps{}, listen) }()
	return waitFor(t, done, "serve to return")
}

// waitFor fails the test if ch does not close or deliver within a second.
func waitFor[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", what)
		panic("unreachable")
	}
}

// TestRunHookStartsBeforeListenerAndGetsACancellableContext: the app's
// worker is registered through Options.Run, is running by the time the
// listener starts, and its context is cancelled on shutdown.
func TestRunHookStartsBeforeListenerAndGetsACancellableContext(t *testing.T) {
	started := make(chan RunDeps, 1)
	stopped := make(chan struct{})
	o := testOpts
	o.Run = func(ctx context.Context, d RunDeps) error {
		started <- d
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- serveTest(t, ctx, o, func(lctx context.Context, r *server.Router) error {
			select {
			case <-started:
			case <-time.After(time.Second):
				t.Error("the listener started but Run had not")
			}
			return blockUntilDone(lctx, r)
		})
	}()
	// Run started (listener consumed it) and is still running.
	select {
	case <-stopped:
		t.Fatal("Run stopped before shutdown")
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := waitFor(t, done, "serve to return"); err != nil {
		t.Fatalf("serve = %v, want nil on a clean shutdown", err)
	}
	waitFor(t, stopped, "Run to see its context cancelled")
}

func TestRunHookGetsTheLoggerAndDeps(t *testing.T) {
	got := make(chan RunDeps, 1)
	o := testOpts
	o.Run = func(ctx context.Context, d RunDeps) error { got <- d; return nil }
	authz := specOnlyAuthorizer{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- serveWith(ctx, mustInstall(t, o), zap.NewNop(), Deps{Authorizer: authz}, blockUntilDone)
	}()
	d := waitFor(t, got, "Run")
	if d.Log == nil || d.Authorizer != server.Authorizer(authz) {
		t.Errorf("RunDeps = %+v, want the logger and the authorizer", d)
	}
	cancel()
	waitFor(t, done, "serve")
}

// TestShutdownWaitsForRunBeforeTheCallerClosesThePool: serveWith returns
// only after Run has returned, and serve closes the pool after serveWith.
func TestShutdownWaitsForRunBeforeTheCallerClosesThePool(t *testing.T) {
	var runReturned atomic.Bool
	o := testOpts
	o.Run = func(ctx context.Context, _ RunDeps) error {
		<-ctx.Done()
		time.Sleep(100 * time.Millisecond) // a slow drain
		runReturned.Store(true)
		return nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- serveTest(t, ctx, o, blockUntilDone) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := waitFor(t, done, "serve"); err != nil {
		t.Fatalf("serve = %v", err)
	}
	if !runReturned.Load() {
		t.Fatal("serve returned (and so would close the pool) while Run was still running")
	}
}

// TestRunErrorIsFatal: a non-nil error before shutdown stops the listener
// and is serve's error.
func TestRunErrorIsFatal(t *testing.T) {
	boom := errors.New("consumer lost its broker")
	o := testOpts
	o.Run = func(context.Context, RunDeps) error { return boom }
	listenerStopped := make(chan struct{})
	err := serveTest(t, t.Context(), o, func(ctx context.Context, r *server.Router) error {
		defer close(listenerStopped)
		return blockUntilDone(ctx, r)
	})
	if !errors.Is(err, boom) {
		t.Fatalf("serve = %v, want %v", err, boom)
	}
	waitFor(t, listenerStopped, "the listener to be stopped")
}

func TestRunPanicIsFatal(t *testing.T) {
	o := testOpts
	o.Run = func(context.Context, RunDeps) error { panic("kaboom") }
	err := serveTest(t, t.Context(), o, blockUntilDone)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte("kaboom")) {
		t.Fatalf("serve = %v, want an error naming the panic", err)
	}
}

// TestRunReturningNilEarlyIsNotFatal: finished work does not take the API
// down.
func TestRunReturningNilEarlyIsNotFatal(t *testing.T) {
	ran := make(chan struct{})
	o := testOpts
	o.Run = func(context.Context, RunDeps) error { close(ran); return nil }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- serveTest(t, ctx, o, blockUntilDone) }()
	waitFor(t, ran, "Run")
	select {
	case err := <-done:
		t.Fatalf("serve returned (%v) after Run returned nil", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	if err := waitFor(t, done, "serve"); err != nil {
		t.Fatalf("serve = %v, want nil", err)
	}
}

// TestRunErrorAfterCancelIsStillReported: a failed drain exits non-zero,
// but context.Canceled does not.
func TestRunErrorAfterCancelIsStillReported(t *testing.T) {
	bad := errors.New("drain failed")
	o := testOpts
	o.Run = func(ctx context.Context, _ RunDeps) error { <-ctx.Done(); return bad }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- serveTest(t, ctx, o, blockUntilDone) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := waitFor(t, done, "serve"); !errors.Is(err, bad) {
		t.Fatalf("serve = %v, want %v", err, bad)
	}
}

// TestListenerErrorStopsRun: a listener that fails cancels Run's context
// and serve still waits for it.
func TestListenerErrorStopsRun(t *testing.T) {
	var runReturned atomic.Bool
	o := testOpts
	o.Run = func(ctx context.Context, _ RunDeps) error {
		<-ctx.Done()
		runReturned.Store(true)
		return nil
	}
	bad := errors.New("address in use")
	err := serveTest(t, t.Context(), o, func(context.Context, *server.Router) error {
		time.Sleep(20 * time.Millisecond)
		return bad
	})
	if !errors.Is(err, bad) || !runReturned.Load() {
		t.Fatalf("serve = %v, Run returned = %v; want the listener's error after Run returned", err, runReturned.Load())
	}
}

// TestOpenAPICommandDoesNotStartRun: the openapi command connects to
// nothing and starts no worker.
func TestOpenAPICommandDoesNotStartRun(t *testing.T) {
	var called atomic.Bool
	o := testOpts
	o.Run = func(context.Context, RunDeps) error { called.Store(true); return nil }
	cmd := NewRootCmd(o)
	cmd.SetOut(io.Discard)
	cmd.SetArgs([]string{"openapi"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("openapi: %v", err)
	}
	if _, err := OpenAPISpec(o); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if called.Load() {
		t.Fatal("Options.Run was started by the openapi command")
	}
}

// TestRoutesSeeAPoolField pins that Deps carries Pool, nil under openapi.
func TestRoutesSeeANilPoolUnderOpenAPI(t *testing.T) {
	o := testOpts
	saw := false
	o.Routes = func(_ *server.Router, d Deps) error { saw = true; return errors.Join(poolMustBeNil(d)) }
	if _, err := OpenAPISpec(o); err != nil {
		t.Fatal(err)
	}
	if !saw {
		t.Fatal("Routes did not run")
	}
}

func poolMustBeNil(d Deps) error {
	if d.Pool != nil {
		return errors.New("Deps.Pool must be nil under openapi")
	}
	return nil
}

// TestConnectGivesRoutesAndRunThePool: with a database configured, connect
// opens the pool and puts it in Deps, which Routes and Run both see.
func TestConnectGivesRoutesAndRunThePool(t *testing.T) {
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
	defer func() { _ = admin.Close(ctx) }()
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

	var routesPool, runPool atomic.Pointer[pgxpool.Pool]
	o := testOpts
	o.Routes = func(_ *server.Router, d Deps) error { routesPool.Store(d.Pool); return nil }
	ran := make(chan struct{})
	o.Run = func(ctx context.Context, d RunDeps) error {
		runPool.Store(d.Pool)
		close(ran)
		<-ctx.Done()
		return nil
	}
	inst := mustInstall(t, o)
	d, cleanup, err := connect(ctx, inst, cfg, zap.NewNop())
	defer cleanup()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if d.Pool == nil {
		t.Fatal("connect left Deps.Pool nil although a database is configured")
	}
	if err := d.Pool.Ping(ctx); err != nil {
		t.Fatalf("pool does not work: %v", err)
	}
	sctx, cancel := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- serveWith(sctx, inst, zap.NewNop(), d, blockUntilDone) }()
	waitFor(t, ran, "Run")
	cancel()
	waitFor(t, done, "serve")
	if routesPool.Load() != d.Pool || runPool.Load() != d.Pool {
		t.Errorf("Routes got %p and Run got %p, want the pool connect opened (%p)", routesPool.Load(), runPool.Load(), d.Pool)
	}
}
