package monitoring

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type lifecycleState uint8

const loggingDependencyTimeout = 15 * time.Second

const (
	lifecycleStateStopped lifecycleState = iota
	lifecycleStateRunning
	lifecycleStateStopping
)

type ServiceLifecycle struct {
	name   string
	mu     sync.RWMutex
	state  lifecycleState
	runCtx context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewServiceLifecycle(name string) ServiceLifecycle {
	return ServiceLifecycle{name: name}
}

// Start starts.
func (sl *ServiceLifecycle) Start(parent context.Context) (context.Context, error) {
	if parent == nil {
		parent = context.Background()
	}

	base := context.WithoutCancel(parent)
	runCtx, cancel := context.WithCancel(base)

	sl.mu.Lock()
	defer sl.mu.Unlock()

	switch sl.state {
	case lifecycleStateRunning:
		cancel()
		return nil, fmt.Errorf("%s is already running", sl.name)
	case lifecycleStateStopping:
		cancel()
		return nil, fmt.Errorf("%s is stopping", sl.name)
	}

	sl.state = lifecycleStateRunning
	sl.runCtx = runCtx
	sl.cancel = cancel
	return runCtx, nil
}

// Begin begins.
func (sl *ServiceLifecycle) Begin() (context.Context, func(), bool) {
	sl.mu.RLock()
	if sl.state != lifecycleStateRunning || sl.runCtx == nil {
		sl.mu.RUnlock()
		return nil, nil, false
	}
	sl.wg.Add(1)
	runCtx := sl.runCtx
	sl.mu.RUnlock()

	return runCtx, sl.wg.Done, true
}

// Cancel cancels.
func (sl *ServiceLifecycle) Cancel() error {
	sl.mu.Lock()
	if sl.state != lifecycleStateRunning {
		sl.mu.Unlock()
		return fmt.Errorf("%s is not running", sl.name)
	}

	cancel := sl.cancel
	sl.state = lifecycleStateStopping
	sl.runCtx = nil
	sl.cancel = nil
	sl.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return nil
}

// Wait waits.
func (sl *ServiceLifecycle) Wait(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		sl.wg.Wait()
	}()

	select {
	case <-done:
		sl.mu.Lock()
		if sl.state == lifecycleStateStopping {
			sl.state = lifecycleStateStopped
		}
		sl.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop stops.
func (sl *ServiceLifecycle) Stop(ctx context.Context) error {
	if err := sl.Cancel(); err != nil {
		return fmt.Errorf("ServiceLifecycle.Stop: %w", err)
	}
	return sl.Wait(ctx)
}

// IsRunning is running.
func (sl *ServiceLifecycle) IsRunning() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.state == lifecycleStateRunning
}

func RunWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	type result struct {
		value T
		err   error
	}

	resultCh := make(chan result, 1)
	go func() {
		value, err := fn()
		resultCh <- result{value: value, err: err}
	}()

	select {
	case res := <-resultCh:
		return res.value, res.err
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}

func RunErrWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	_, err := RunWithTimeout(ctx, timeout, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func RunWithTimeoutContext[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	if ctx == nil {
		ctx = context.Background()
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if fn == nil {
		return zero, nil
	}
	return fn(ctx)
}

func RunErrWithTimeoutContext(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	_, err := RunWithTimeoutContext(ctx, timeout, func(runCtx context.Context) (struct{}, error) {
		if fn == nil {
			return struct{}{}, nil
		}
		return struct{}{}, fn(runCtx)
	})
	return err
}
