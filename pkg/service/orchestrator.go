package service

import (
	"context"
	"fmt"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// ExecuteOrchestration is a resilient wrapper that executes a service lifecycle step
// using synchronized propagation and explicit preemption.
func ExecuteOrchestration(ctx context.Context, action func(context.Context) error) error {
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("runtime panic caught: %v\n%s", r, debug.Stack())
			}
		}()

		return action(egCtx)
	})

	err := eg.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("shutdown deadline exceeded: pre-empting execution")
	}
	return err
}
