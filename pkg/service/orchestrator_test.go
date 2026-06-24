package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// TestOrchestrator_Preemption checks if long-running I/O calls are preempted correctly.
func TestOrchestrator_Preemption(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	startCalled := make(chan struct{})

	err := ExecuteOrchestration(ctx, func(c context.Context) error {
		close(startCalled)
		<-c.Done()
		return c.Err()
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecuteOrchestration_PanicRecovery(t *testing.T) {
	t.Parallel()
	err := ExecuteOrchestration(context.Background(), func(c context.Context) error {
		panic("simulated panic in boundary")
	})

	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}

	if !strings.Contains(err.Error(), "simulated panic in boundary") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}
}

func TestExecuteOrchestration_ContextCancellationPropagates(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	eg, egCtx := errgroup.WithContext(context.Background())
	errCh := make(chan error, 1)

	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		default:
		}
		errCh <- ExecuteOrchestration(ctx, func(c context.Context) error {
			<-c.Done()
			return c.Err()
		})
		return nil
	})

	cancel() // Cancel the parent context

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ExecuteOrchestration did not return promptly after context cancellation")
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("unexpected errgroup wait error: %v", err)
	}
}

func TestExecuteOrchestration_FalseSharingMitigation(t *testing.T) {
	t.Parallel()
	eg, ctx := errgroup.WithContext(context.Background())
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		idx := i
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			err := ExecuteOrchestration(context.Background(), func(c context.Context) error {
				if idx == 5 {
					return errors.New("simulated error")
				}
				return nil
			})
			errs <- err
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent orchestrations failed: %v", err)
	}
	close(errs)

	errorCount := 0
	for err := range errs {
		if err != nil {
			errorCount++
			if err.Error() != "simulated error" {
				t.Errorf("unexpected error: %v", err)
			}
		}
	}

	if errorCount != 1 {
		t.Errorf("expected exactly 1 error, got %d", errorCount)
	}
}
