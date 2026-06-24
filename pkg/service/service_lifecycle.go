package service

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type lifecycleState uint8

const DependencyTimeout = 15 * time.Second

const (
	lifecycleStateStopped lifecycleState = iota
	lifecycleStateRunning
	lifecycleStateStopping
)

type BaseLifecycle struct {
	name   string
	mu     sync.RWMutex
	state  lifecycleState
	runCtx context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewBaseLifecycle(name string) BaseLifecycle {
	return BaseLifecycle{name: name}
}

// Start starts.
func (sl *BaseLifecycle) Start(parent context.Context) (context.Context, error) {
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
func (sl *BaseLifecycle) Begin() (context.Context, func(), bool) {
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
func (sl *BaseLifecycle) Cancel() error {
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
func (sl *BaseLifecycle) Wait(ctx context.Context) error {
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
func (sl *BaseLifecycle) Stop(ctx context.Context) error {
	if err := sl.Cancel(); err != nil {
		return fmt.Errorf("BaseLifecycle.Stop: %w", err)
	}
	return sl.Wait(ctx)
}

// IsRunning is running.
func (sl *BaseLifecycle) IsRunning() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()
	return sl.state == lifecycleStateRunning
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
