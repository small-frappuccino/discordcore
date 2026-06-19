package maintenance

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestUserPruneService_ShutdownPreemptionAndLeaks(t *testing.T) {
	runtime.Gosched()
	baseline := runtime.NumGoroutine()

	s := NewUserPruneService(nil, nil, nil, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	current := runtime.NumGoroutine()
	// Allow minor fluctuation in baseline goroutines from test framework
	if current > baseline+2 {
		t.Errorf("Goroutine leak detected: baseline=%d, current=%d", baseline, current)
	}
}
