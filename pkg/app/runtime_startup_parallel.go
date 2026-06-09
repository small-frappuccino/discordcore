package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"sync"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"golang.org/x/sync/errgroup"
)

var (
	openBotRuntimeFn       = openBotRuntime
	initializeBotRuntimeFn = initializeBotRuntime
	shutdownBotRuntimeFn   = shutdownBotRuntime
)

func ResolveRuntimeStartupParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	case runtimeCount == 2:
		return 2
	default:
		return 3
	}
}

func ResolveRuntimeBackgroundParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	default:
		return 2
	}
}

func ResolveStartupLightParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 2
	case runtimeCount == 2:
		return 3
	default:
		return 4
	}
}

func ResolveStartupLightQueueSize(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 4
	case runtimeCount == 2:
		return 6
	default:
		return runtimeCount * 2
	}
}

type RuntimeStartupBackgroundWorker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	group        *errgroup.Group
	queue        chan func(context.Context) error
	dispatchDone chan struct{}
	shutdownOnce sync.Once
	done         chan struct{}
	waitErr      error
}

func NewRuntimeStartupBackgroundWorker(runtimeCount int) *RuntimeStartupBackgroundWorker {
	return NewRuntimeStartupBackgroundWorkerWithLimits(
		ResolveRuntimeBackgroundParallelism(runtimeCount),
		runtimeCount,
	)
}

func NewRuntimeStartupBackgroundWorkerWithLimits(parallelism, queueSize int) *RuntimeStartupBackgroundWorker {
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)
	if parallelism <= 0 {
		parallelism = 1
	}
	group.SetLimit(parallelism)

	if queueSize <= 0 {
		queueSize = 1
	}

	worker := &RuntimeStartupBackgroundWorker{
		ctx:          groupCtx,
		cancel:       cancel,
		group:        group,
		queue:        make(chan func(context.Context) error, queueSize),
		dispatchDone: make(chan struct{}),
		done:         make(chan struct{}),
	}
	go worker.dispatch()
	return worker
}

type StartupTaskOrchestrator struct {
	light *RuntimeStartupBackgroundWorker
	heavy *RuntimeStartupBackgroundWorker
}

func NewStartupTaskOrchestrator(runtimeCount int) *StartupTaskOrchestrator {
	return &StartupTaskOrchestrator{
		light: NewRuntimeStartupBackgroundWorkerWithLimits(
			ResolveStartupLightParallelism(runtimeCount),
			ResolveStartupLightQueueSize(runtimeCount),
		),
		heavy: NewRuntimeStartupBackgroundWorker(runtimeCount),
	}
}

// GoLight gos light.
func (o *StartupTaskOrchestrator) GoLight(name string, fn func(context.Context) error) {
	if o == nil {
		return
	}
	o.goTask(o.light, name, "light", fn)
}

// GoHeavy gos heavy.
func (o *StartupTaskOrchestrator) GoHeavy(name string, fn func(context.Context) error) {
	if o == nil {
		return
	}
	o.goTask(o.heavy, name, "heavy", fn)
}

func (o *StartupTaskOrchestrator) goTask(worker *RuntimeStartupBackgroundWorker, name, kind string, fn func(context.Context) error) {
	if o == nil || worker == nil || fn == nil {
		return
	}

	worker.Go(func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.ApplicationLogger().Warn(
				"Startup background task failed",
				"task", name,
				"kind", kind,
				"err", err,
			)
		}
		return nil
	})
}

// Shutdown shutdowns.
func (o *StartupTaskOrchestrator) Shutdown(ctx context.Context) error {
	if o == nil {
		return nil
	}

	var errs []error
	if o.light != nil {
		if err := o.light.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			errs = append(errs, fmt.Errorf("shutdown light startup tasks: %w", err))
		}
	}
	if o.heavy != nil {
		if err := o.heavy.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			errs = append(errs, fmt.Errorf("shutdown heavy startup tasks: %w", err))
		}
	}
	return stdErrors.Join(errs...)
}

// Go gos.
func (w *RuntimeStartupBackgroundWorker) Go(fn func(context.Context) error) {
	if w == nil || fn == nil {
		return
	}
	select {
	case <-w.ctx.Done():
		return
	case w.queue <- fn:
	}
}

// Shutdown shutdowns.
func (w *RuntimeStartupBackgroundWorker) Shutdown(ctx context.Context) error {
	if w == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	w.shutdownOnce.Do(func() {
		if w.cancel != nil {
			w.cancel()
		}
		go func() {
			<-w.dispatchDone
			w.waitErr = w.group.Wait()
			close(w.done)
		}()
	})

	select {
	case <-w.done:
		return w.waitErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *RuntimeStartupBackgroundWorker) dispatch() {
	defer close(w.dispatchDone)

	for {
		select {
		case <-w.ctx.Done():
			return
		case fn := <-w.queue:
			if fn == nil {
				continue
			}
			w.group.Go(func() error {
				if err := w.ctx.Err(); err != nil {
					return nil
				}
				if err := fn(w.ctx); err != nil {
					if w.ctx.Err() != nil {
						return nil
					}
					return fmt.Errorf("RuntimeStartupBackgroundWorker.dispatch: %w", err)
				}
				return nil
			})
		}
	}
}
