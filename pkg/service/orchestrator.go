package service

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"golang.org/x/sync/errgroup"
)

// ExecuteOrchestration is a resilient wrapper that executes a service lifecycle step
// using synchronized propagation and explicit preemption.

var shutdownDeadline = 30 * time.Second

func ExecuteOrchestration(rootCtx context.Context, action func(context.Context) error) error {
	eg, ctx := errgroup.WithContext(rootCtx)

	eg.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("runtime panic caught: %v\n%s", r, debug.Stack())
			}
		}()

		if err := action(ctx); err != nil {
			return err
		}
		return nil
	})

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- eg.Wait()
	}()

	select {
	case err := <-waitCh:
		return err
	case <-time.After(shutdownDeadline):
		return fmt.Errorf("shutdown deadline exceeded: pre-empting execution")
	}
}
