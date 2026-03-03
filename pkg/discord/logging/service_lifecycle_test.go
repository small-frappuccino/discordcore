package logging

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceLifecycleStopTimesOutUntilOwnedWorkFinishes(t *testing.T) {
	lifecycle := newServiceLifecycle("test lifecycle")

	runCtx, err := lifecycle.Start(context.Background())
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}

	ownedCtx, done, ok := lifecycle.Begin()
	if !ok {
		t.Fatalf("expected owned work to start")
	}
	if ownedCtx != runCtx {
		t.Fatalf("expected begin to return the active run context")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = lifecycle.Stop(stopCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected stop timeout, got %v", err)
	}
	if lifecycle.IsRunning() {
		t.Fatalf("expected lifecycle to stop accepting new work after cancel")
	}
	if ctx, done, ok := lifecycle.Begin(); ok || ctx != nil || done != nil {
		t.Fatalf("expected begin to reject work while lifecycle is stopping")
	}

	select {
	case <-runCtx.Done():
	default:
		t.Fatalf("expected run context to be canceled")
	}

	done()

	if err := lifecycle.Wait(context.Background()); err != nil {
		t.Fatalf("wait for owned work: %v", err)
	}

	if _, err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("restart lifecycle after wait: %v", err)
	}
}

func TestServiceLifecycleBeginReturnsFalseAfterCancel(t *testing.T) {
	lifecycle := newServiceLifecycle("test lifecycle")

	if _, err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	if err := lifecycle.Cancel(); err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}

	if ctx, done, ok := lifecycle.Begin(); ok || ctx != nil || done != nil {
		t.Fatalf("expected begin to reject work after cancel")
	}
}
