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

	got, ok, err := store.GetLastEvent()
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
		BotInstanceID: "yuzuha",
		Now: func() time.Time {
			return expected
		},
	})

	activity.MarkEvent(context.Background(), "test")

	got, ok, err := store.GetLastEventForBot("yuzuha")
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

func TestRuntimeActivityMarkEventUsesBackgroundWhenContextNil(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-mark-event-nil.db")
	expected := time.Date(2026, time.January, 2, 4, 5, 6, 0, time.UTC)

	var sawNilCtx atomic.Bool
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr: func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
			if ctx == nil {
				sawNilCtx.Store(true)
			}
			return fn(ctx)
		},
		Now: func() time.Time {
			return expected
		},
	})

	activity.MarkEvent(nil, "test")

	if sawNilCtx.Load() {
		t.Fatalf("expected MarkEvent to provide a background context")
	}

	got, ok, err := store.GetLastEvent()
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

func TestRuntimeActivityStartHeartbeatPersistsImmediatelyAndPeriodically(t *testing.T) {
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat.db")
	base := time.Date(2026, time.January, 2, 5, 0, 0, 0, time.UTC)
	var calls atomic.Int32

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           runErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Add(1)) * time.Second)
		},
	})

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	firstDeadline := time.Now().Add(100 * time.Millisecond)
	var first time.Time
	for time.Now().Before(firstDeadline) {
		if ts, ok, err := store.GetHeartbeat(); err == nil && ok {
			first = ts
			break
		}
		time.Sleep(2 * time.Millisecond)
	}
	if first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted")
	}

	updatedDeadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(updatedDeadline) {
		if ts, ok, err := store.GetHeartbeat(); err == nil && ok && ts.After(first) {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("expected periodic heartbeat persistence update after initial write")
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
	})

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ts, ok, err := store.GetHeartbeat(); err == nil && ok && !ts.IsZero() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}

	t.Fatalf("expected heartbeat persistence to recover after initial failure")
}
