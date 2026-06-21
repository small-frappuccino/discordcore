package qotd

import (
	"context"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestRuntimeService_GracefulShutdown(t *testing.T) {
	// Verify that no goroutines leak
	defer goleak.VerifyNone(t,
		goleak.IgnoreTopFunction("net/http.(*persistConn).readLoop"),
		goleak.IgnoreTopFunction("net/http.(*persistConn).writeLoop"),
	)

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

	// Let it spin
	time.Sleep(50 * time.Millisecond)

	// Stop
	if err := daemon.Stop(ctx); err != nil {
		t.Fatalf("failed to stop: %v", err)
	}

	// The goleak.VerifyNone at the end of the function ensures the loop() goroutine
	// and its timers successfully exited.
}
