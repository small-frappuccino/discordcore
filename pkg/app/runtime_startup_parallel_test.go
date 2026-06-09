package app_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/app"
)

func TestStartupTaskOrchestrator_GoLight(t *testing.T) {
	orchestrator := app.NewStartupTaskOrchestrator(1)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(1)

	orchestrator.GoLight("test_light", func(ctx context.Context) error {
		atomic.AddInt32(&executed, 1)
		wg.Done()
		return nil
	})

	// Wait for the task to complete
	wg.Wait()

	if atomic.LoadInt32(&executed) != 1 {
		t.Errorf("Expected GoLight task to execute exactly once")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_GoHeavy(t *testing.T) {
	orchestrator := app.NewStartupTaskOrchestrator(2)

	var executed int32
	var wg sync.WaitGroup
	wg.Add(2)

	for i := 0; i < 2; i++ {
		orchestrator.GoHeavy("test_heavy", func(ctx context.Context) error {
			atomic.AddInt32(&executed, 1)
			wg.Done()
			return nil
		})
	}

	wg.Wait()

	if atomic.LoadInt32(&executed) != 2 {
		t.Errorf("Expected GoHeavy task to execute exactly twice")
	}

	if err := orchestrator.Shutdown(context.Background()); err != nil {
		t.Fatalf("Unexpected error during shutdown: %v", err)
	}
}

func TestStartupTaskOrchestrator_ShutdownWithContextCancellation(t *testing.T) {
	orchestrator := app.NewStartupTaskOrchestrator(1)

	// Block the worker
	taskStarted := make(chan struct{})
	unblockTask := make(chan struct{})

	orchestrator.GoLight("blocking_task", func(ctx context.Context) error {
		close(taskStarted)
		select {
		case <-unblockTask:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})

	<-taskStarted

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := orchestrator.Shutdown(ctx)
	if err == nil {
		t.Errorf("Expected context cancellation error during shutdown")
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled, got %v", err)
	}

	close(unblockTask)
}

func TestStartupTaskOrchestrator_ShutdownTaskErrorSwallowed(t *testing.T) {
	orchestrator := app.NewStartupTaskOrchestrator(1)

	var wg sync.WaitGroup
	wg.Add(1)

	expectedErr := errors.New("simulated task error")

	orchestrator.GoHeavy("error_task", func(ctx context.Context) error {
		defer wg.Done()
		return expectedErr
	})

	wg.Wait()

	// Task errors are swallowed and logged by goTask, so Shutdown should return nil.
	err := orchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error because task errors are swallowed, got '%v'", err)
	}
}

func TestStartupTaskOrchestrator_GoNil(t *testing.T) {
	orchestrator := app.NewStartupTaskOrchestrator(1)

	// Should not panic
	orchestrator.GoLight("nil_task", nil)
	orchestrator.GoHeavy("nil_task", nil)

	var nilOrchestrator *app.StartupTaskOrchestrator
	nilOrchestrator.GoLight("nil_task", func(ctx context.Context) error { return nil })
	err := nilOrchestrator.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Expected nil error for nil orchestrator shutdown")
	}
}

func TestResolveRuntimeStartupParallelism(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 3},
		{10, 3},
	}
	for _, tc := range tests {
		if got := app.ResolveRuntimeStartupParallelism(tc.count); got != tc.expected {
			t.Errorf("ResolveRuntimeStartupParallelism(%d) = %d; want %d", tc.count, got, tc.expected)
		}
	}
}

func TestResolveRuntimeBackgroundParallelism(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{10, 2},
	}
	for _, tc := range tests {
		if got := app.ResolveRuntimeBackgroundParallelism(tc.count); got != tc.expected {
			t.Errorf("ResolveRuntimeBackgroundParallelism(%d) = %d; want %d", tc.count, got, tc.expected)
		}
	}
}

func TestResolveStartupLightParallelism(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, 2},
		{1, 2},
		{2, 3},
		{3, 4},
		{10, 4},
	}
	for _, tc := range tests {
		if got := app.ResolveStartupLightParallelism(tc.count); got != tc.expected {
			t.Errorf("ResolveStartupLightParallelism(%d) = %d; want %d", tc.count, got, tc.expected)
		}
	}
}

func TestResolveStartupLightQueueSize(t *testing.T) {
	tests := []struct {
		count    int
		expected int
	}{
		{0, 4},
		{1, 4},
		{2, 6},
		{3, 6},
		{10, 20},
	}
	for _, tc := range tests {
		if got := app.ResolveStartupLightQueueSize(tc.count); got != tc.expected {
			t.Errorf("ResolveStartupLightQueueSize(%d) = %d; want %d", tc.count, got, tc.expected)
		}
	}
}
