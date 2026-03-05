package logging

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

type serviceLifecycle struct {
	name   string
	mu     sync.RWMutex
	state  lifecycleState
	runCtx context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newServiceLifecycle(name string) serviceLifecycle {
	return serviceLifecycle{name: name}
}

func (sl *serviceLifecycle) Start(parent context.Context) (context.Context, error) {
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

func (sl *serviceLifecycle) Begin() (context.Context, func(), bool) {
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

func (sl *serviceLifecycle) Cancel() error {
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

func (sl *serviceLifecycle) Wait(ctx context.Context) error {
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

func (sl *serviceLifecycle) Stop(ctx context.Context) error {
	if err := sl.Cancel(); err != nil {
		return err
	}
	return sl.Wait(ctx)
}

func (sl *serviceLifecycle) IsRunning() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.state == lifecycleStateRunning
}

func runWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
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

func runErrWithTimeout(ctx context.Context, timeout time.Duration, fn func() error) error {
	_, err := runWithTimeout(ctx, timeout, func() (struct{}, error) {
		return struct{}{}, fn()
	})
	return err
}

func runWithTimeoutContext[T any](ctx context.Context, timeout time.Duration, fn func(context.Context) (T, error)) (T, error) {
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

func runErrWithTimeoutContext(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	_, err := runWithTimeoutContext(ctx, timeout, func(runCtx context.Context) (struct{}, error) {
		if fn == nil {
			return struct{}{}, nil
		}
		return struct{}{}, fn(runCtx)
	})
	return err
}
