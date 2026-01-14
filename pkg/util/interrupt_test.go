package util

import (
	"context"
	"sync"
	"testing"
)

func TestWaitForInterruptContextCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var called bool
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		waitForInterruptContext(ctx, func() { called = true })
	}()

	cancel()
	wg.Wait()

	if !called {
		t.Fatalf("expected callback to be invoked on cancellation")
	}
}
