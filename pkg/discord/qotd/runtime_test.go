package qotd

import (
	"context"
	"runtime"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m,
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	)
}

func TestRuntimeService_GracefulShutdown(t *testing.T) {
	t.Parallel()

	cfg := Config{
		PublishInterval: 10 * time.Millisecond,
		ReconcileEvery:  20 * time.Millisecond,
	}

	// Instantiate
	daemon := NewRuntimeService(cfg, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start
	if err := daemon.Start(ctx); err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// Let it spin deterministically
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}

	// Stop
	if err := daemon.Stop(ctx); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	// The goleak.VerifyNone at the end of the function ensures the loop() goroutine
	// and its timers successfully exited.
}
