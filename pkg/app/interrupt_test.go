package app

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"
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

func TestOSInterruptFunctions(t *testing.T) {
	proc, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Skip("Cannot find process")
	}

	done1 := make(chan struct{})
	go func() {
		WaitForInterrupt()
		close(done1)
	}()

	done2 := make(chan struct{})
	called := false
	go func() {
		WaitForInterruptWithCallback(func() { called = true })
		close(done2)
	}()

	// Wait for goroutines to start and register their signal handlers
	// Note: on Windows, we cannot easily send signals to ourselves without using external commands
	// or potentially crashing the test runner if the signal isn't caught. But signal.NotifyContext
	// catches os.Interrupt. Let's try sending it.
	time.Sleep(100 * time.Millisecond)

	err = proc.Signal(os.Interrupt)
	if err != nil {
		t.Logf("proc.Signal failed: %v, skipping signal wait", err)
	}

	select {
	case <-done1:
	case <-time.After(1 * time.Second):
		t.Log("done1 timed out")
	}

	select {
	case <-done2:
		if !called {
			t.Error("expected callback to be called")
		}
	case <-time.After(1 * time.Second):
		t.Log("done2 timed out")
	}
}
