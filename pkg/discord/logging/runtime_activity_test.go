package logging

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeActivityMarkEventPersistsTimestamp(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-mark-event.db")
	expected := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:       runErrWithTimeoutContext,
		EventTimeout: time.Second,
		Now: func() time.Time {
			return expected
		},
	})

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
		RunErr:        runErrWithTimeoutContext,
		EventTimeout:  time.Second,
		BotInstanceID: "companion",
		Now: func() time.Time {
			return expected
		},
	})

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
	ticks := make(chan error, 8)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           runErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Add(1)) * time.Second)
		},
		OnHeartbeatTick: func(err error) { ticks <- err },
	})

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	if err := waitForHeartbeatTick(t, ticks); err != nil {
		t.Fatalf("expected initial heartbeat to succeed: %v", err)
	}
	first, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}

	if err := waitForHeartbeatTick(t, ticks); err != nil {
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
		RunErr:           runErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
	})

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
		RunErr:           runErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
	})

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
	ticks := make(chan error)
	const expectedTicks = 2
	verifyTicks := make(chan error, expectedTicks)
	drainCtx, drainCancel := context.WithCancel(context.Background())
	drainDone := make(chan struct{})

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
		// Send via select so the heartbeat goroutine cannot wedge on an unread
		// tick during teardown — StopHeartbeat waits on the goroutine's done
		// channel and the send happens synchronously inside attemptHeartbeat.
		OnHeartbeatTick: func(err error) {
			select {
			case ticks <- err:
			case <-drainCtx.Done():
			}
		},
	})

	// Drain the unbuffered ticks channel from a dedicated goroutine so the
	// heartbeat send never blocks indefinitely (the startup heartbeat runs
	// synchronously inside StartHeartbeat, so a receiver must already be
	// running before that call). The first expectedTicks values are forwarded
	// into verifyTicks for the test's assertions; later ticks are discarded so
	// a flood from a contended scheduler cannot wedge the heartbeat. drainCtx
	// signals the drainer to exit during cleanup.
	go func() {
		defer close(drainDone)
		forwarded := 0
		for {
			select {
			case <-drainCtx.Done():
				return
			case err := <-ticks:
				if forwarded < expectedTicks {
					verifyTicks <- err
					forwarded++
				}
			}
		}
	}()

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		drainCancel()
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
		<-drainDone
	})

	if err := waitForHeartbeatTick(t, verifyTicks); err == nil {
		t.Fatal("expected first heartbeat attempt to surface the injected failure")
	}
	if err := waitForHeartbeatTick(t, verifyTicks); err != nil {
		t.Fatalf("expected recovery heartbeat to succeed: %v", err)
	}

	ts, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || ts.IsZero() {
		t.Fatalf("expected heartbeat persistence to recover after initial failure: ok=%v err=%v", ok, err)
	}
}

// waitForHeartbeatTick blocks until the next OnHeartbeatTick fires or
// the safety timeout expires. The timeout is intentionally generous —
// it exists to fail loudly if the heartbeat goroutine is wedged, not
// to gate fast-path scheduling. Each test still completes in a few
// ticker intervals on a healthy machine.
func waitForHeartbeatTick(t *testing.T, ticks <-chan error) error {
	t.Helper()
	select {
	case err := <-ticks:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for heartbeat tick")
		return nil
	}
}
