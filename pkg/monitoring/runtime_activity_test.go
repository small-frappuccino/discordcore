//go:build ignore

package monitoring

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeActivityMarkEventPersistsTimestamp(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-mark-event.db")
	expected := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:       RunErrWithTimeoutContext,
		EventTimeout: time.Second,
		Now: func() time.Time {
			return expected
		}})

	activity.MarkEvent(context.Background(), "test")

	got, ok, err := store.LastEvent(context.Background())
	if err != nil {
		t.Fatalf("get last event: %v", err)
	}
	if !ok {
		t.Fatalf("expected last event timestamp to be persisted")
	}
	if !got.Equal(expected) {
		t.Fatalf("unexpected last event timestamp: got=%s want=%s", got.UTC(), expected.UTC())
	}
}

func TestRuntimeActivityMarkEventPersistsTimestampPerBot(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-mark-event-by-bot.db")
	expected := time.Date(2026, time.January, 2, 3, 14, 15, 0, time.UTC)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:        RunErrWithTimeoutContext,
		EventTimeout:  time.Second,
		BotInstanceID: "companion",
		Now: func() time.Time {
			return expected
		}})

	activity.MarkEvent(context.Background(), "test")

	got, ok, err := store.LastEventForBot(context.Background(), "companion")
	if err != nil {
		t.Fatalf("get last event by bot: %v", err)
	}
	if !ok {
		t.Fatalf("expected namespaced last event timestamp to be persisted")
	}
	if !got.Equal(expected) {
		t.Fatalf("unexpected namespaced last event timestamp: got=%s want=%s", got.UTC(), expected.UTC())
	}
}

func TestRuntimeActivityStartHeartbeatPersistsImmediatelyAndPeriodically(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat.db")
	base := time.Date(2026, time.January, 2, 5, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	ticks := newTickRecorder(t, 2)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Add(1)) * time.Second)
		},
		OnHeartbeatTick: ticks.Hook})

	// 25ms (rather than 5ms) leaves enough room for the second ticker fire
	// to land within tickRecorder.Next's 2s safety timeout when the package
	// is run with high parallelism: a Postgres-backed roundtrip per tick
	// can briefly outrun a 5ms cadence under sibling-test schema churn.
	activity.StartHeartbeat(context.Background(), 25*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected initial heartbeat to succeed: %v", err)
	}
	first, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected periodic heartbeat to succeed: %v", err)
	}
	second, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected periodic heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}
	if !second.After(first) {
		t.Fatalf("expected periodic heartbeat to advance the timestamp: first=%s second=%s", first.UTC(), second.UTC())
	}
}

func TestRuntimeActivityStartHeartbeatNoopsWhenAlreadyRunning(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-noop.db")
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second})

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	firstDone := activity.hbDone
	if firstDone == nil {
		t.Fatalf("expected first heartbeat start to initialize state")
	}

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)

	if activity.hbDone != firstDone {
		t.Fatalf("expected second heartbeat start to reuse existing state")
	}
}

func TestRuntimeActivityStopHeartbeatIsIdempotent(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-stop.db")
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second})

	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("stop heartbeat before start: %v", err)
	}

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)

	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("first stop heartbeat: %v", err)
	}
	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("second stop heartbeat: %v", err)
	}
}

func TestRuntimeActivityHeartbeatStartupContinuesAfterInitialPersistenceFailure(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-retry.db")
	base := time.Date(2026, time.January, 2, 6, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	ticks := newTickRecorder(t, 2)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr: func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
			if calls.Add(1) == 1 {
				return errors.New("startup heartbeat failed")
			}
			return fn(ctx)
		},
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Load()) * time.Second)
		},
		OnHeartbeatTick: ticks.Hook})

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	if err := ticks.Next(t); err == nil {
		t.Fatal("expected first heartbeat attempt to surface the injected failure")
	}
	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected recovery heartbeat to succeed: %v", err)
	}

	ts, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || ts.IsZero() {
		t.Fatalf("expected heartbeat persistence to recover after initial failure: ok=%v err=%v", ok, err)
	}
}

// TestRuntimeActivityStartHeartbeatReturnsWhenStartupPersistenceWedges pins
// two invariants exposed by the heartbeat goroutine restructuring:
//
//  1. StartHeartbeat must return promptly even when the startup persistence
//     is wedged. The previous code path kept Start parked inside the
//     synchronous attemptHeartbeat, leaving `go func()` unreached and
//     close(done) un-armed — a concurrent StopHeartbeat that observed
//     hbCancel/hbDone then blocked on <-done forever.
//  2. StopHeartbeat must return cleanly even when the in-flight attempt is
//     wedged. The comprehensive fix dispatches each attempt through
//     runCancellableHeartbeat so the outer goroutine can exit via
//     hbCtx.Done() while the inner attempt goroutine is left to leak until
//     its blocking call returns; close(done) is therefore reachable even
//     if RunErr (or any callback it invokes) ignores ctx.
//
// A RunErr that blocks unconditionally on a channel exercises both: the
// startup persistence cannot make progress, so without the fix Start (1)
// hangs and Stop (2) hangs. Hard timeouts convert either regression into a
// failure with a goroutine stack dump pointing at the wedge.
func TestRuntimeActivityStartHeartbeatReturnsWhenStartupPersistenceWedges(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-start-stop-race.db")

	release := make(chan struct{})
	defer close(release)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr: func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
			<-release
			return nil
		},
		HeartbeatTimeout: time.Second})

	startReturned := make(chan struct{})
	go func() {
		defer close(startReturned)
		activity.StartHeartbeat(context.Background(), 10*time.Millisecond)
	}()

	select {
	case <-startReturned:
	case <-time.After(500 * time.Millisecond):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("StartHeartbeat did not return within 500ms while the startup persistence was wedged; the heartbeat goroutine launch is gated on attemptHeartbeat completing — regression of the race-window fix.\nGoroutines:\n%s", buf[:n])
	}

	stopReturned := make(chan error, 1)
	go func() {
		stopReturned <- activity.StopHeartbeat(context.Background())
	}()

	select {
	case err := <-stopReturned:
		if err != nil {
			t.Fatalf("stop heartbeat: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("StopHeartbeat did not return within 500ms while the in-flight attempt was wedged; close(done) is gated on the inner attempt completing — regression of the cancellable-attempt fix.\nGoroutines:\n%s", buf[:n])
	}
}

// tickRecorder is the test-side companion for hooks like
// OnHeartbeatTick that fire synchronously from a long-running producer
// goroutine. The recorder runs a drainer goroutine that always receives
// from an unbuffered channel so the hook's send never wedges. The first
// wantTicks values are exposed to the test via Next; later ticks are
// silently discarded so a flooding producer cannot back up the drainer.
//
// The recorder registers a t.Cleanup that cancels its context and waits
// for the drainer to exit. Because Cleanup runs LIFO, callers should
// construct the recorder before registering the producer's stop
// cleanup — that way producer teardown runs first while the drainer
// is still draining, and only then does the recorder release its
// goroutine. An in-flight Hook send unblocks via the recorder's
// context, so a producer that invokes the callback during its own
// teardown cannot deadlock.
type tickRecorder struct {
	ctx    context.Context
	cancel context.CancelFunc
	ticks  chan error
	verify chan error
	done   chan struct{}
}

func newTickRecorder(t *testing.T, wantTicks int) *tickRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &tickRecorder{
		ctx:    ctx,
		cancel: cancel,
		ticks:  make(chan error),
		verify: make(chan error, wantTicks),
		done:   make(chan struct{})}
	go func() {
		defer close(r.done)
		forwarded := 0
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-r.ticks:
				if forwarded < wantTicks {
					r.verify <- err
					forwarded++
				}
			}
		}
	}()
	t.Cleanup(func() {
		r.cancel()
		<-r.done
	})
	return r
}

// Hook is the callback to pass to a producer option (e.g.
// runtimeActivityOptions.OnHeartbeatTick). It rendezvous with the
// drainer and only blocks until the recorder's context is cancelled,
// so it is safe to invoke synchronously from inside a producer
// goroutine loop or from a synchronous startup attempt.
func (r *tickRecorder) Hook(err error) {
	select {
	case r.ticks <- err:
	case <-r.ctx.Done():
	}
}

// Next pulls the next collected value. The test fails if no value
// arrives within the safety timeout, which is intentionally generous
// so it surfaces a wedged producer rather than gating fast-path
// scheduling on healthy machines.
func (r *tickRecorder) Next(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.verify:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick")
		return nil
	}
}
