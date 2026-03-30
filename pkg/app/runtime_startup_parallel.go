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

func resolveRuntimeStartupParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	case runtimeCount == 2:
		return 2
	default:
		return 3
	}
}

func resolveRuntimeBackgroundParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 1
	default:
		return 2
	}
}

func resolveStartupLightParallelism(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 2
	case runtimeCount == 2:
		return 3
	default:
		return 4
	}
}

func resolveStartupLightQueueSize(runtimeCount int) int {
	switch {
	case runtimeCount <= 1:
		return 4
	case runtimeCount == 2:
		return 6
	default:
		return runtimeCount * 2
	}
}

func openBotRuntimes(botInstances []resolvedBotInstance, runtimeCapabilities map[string]botRuntimeCapabilities) (map[string]*botRuntime, []*botRuntime, error) {
	opened := make([]*botRuntime, len(botInstances))

	var group errgroup.Group
	group.SetLimit(resolveRuntimeStartupParallelism(len(botInstances)))

	for i, instance := range botInstances {
		i := i
		instance := instance
		capabilities := runtimeCapabilities[instance.ID]

		group.Go(func() error {
			runtime, err := openBotRuntimeFn(instance, capabilities)
			if err != nil {
				return err
			}
			opened[i] = runtime
			return nil
		})
	}

	err := group.Wait()

	runtimes := make(map[string]*botRuntime, len(botInstances))
	runtimeOrder := make([]*botRuntime, 0, len(botInstances))
	for _, runtime := range opened {
		if runtime == nil {
			continue
		}
		runtimes[runtime.instanceID] = runtime
		runtimeOrder = append(runtimeOrder, runtime)
	}

	return runtimes, runtimeOrder, err
}

func initializeBotRuntimes(runtimeOrder []*botRuntime, opts botRuntimeOptions) error {
	var group errgroup.Group
	group.SetLimit(resolveRuntimeStartupParallelism(len(runtimeOrder)))

	for _, runtime := range runtimeOrder {
		runtime := runtime
		group.Go(func() error {
			return initializeBotRuntimeFn(runtime, opts)
		})
	}

	return group.Wait()
}

type runtimeStartupBackgroundWorker struct {
	ctx          context.Context
	cancel       context.CancelFunc
	group        *errgroup.Group
	queue        chan func(context.Context) error
	dispatchDone chan struct{}
	shutdownOnce sync.Once
	done         chan struct{}
	waitErr      error
}

func newRuntimeStartupBackgroundWorker(runtimeCount int) *runtimeStartupBackgroundWorker {
	return newRuntimeStartupBackgroundWorkerWithLimits(
		resolveRuntimeBackgroundParallelism(runtimeCount),
		runtimeCount,
	)
}

func newRuntimeStartupBackgroundWorkerWithLimits(parallelism, queueSize int) *runtimeStartupBackgroundWorker {
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)
	if parallelism <= 0 {
		parallelism = 1
	}
	group.SetLimit(parallelism)

	if queueSize <= 0 {
		queueSize = 1
	}

	worker := &runtimeStartupBackgroundWorker{
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

type startupTaskOrchestrator struct {
	light *runtimeStartupBackgroundWorker
	heavy *runtimeStartupBackgroundWorker
}

func newStartupTaskOrchestrator(runtimeCount int) *startupTaskOrchestrator {
	return &startupTaskOrchestrator{
		light: newRuntimeStartupBackgroundWorkerWithLimits(
			resolveStartupLightParallelism(runtimeCount),
			resolveStartupLightQueueSize(runtimeCount),
		),
		heavy: newRuntimeStartupBackgroundWorker(runtimeCount),
	}
}

func (o *startupTaskOrchestrator) GoLight(name string, fn func(context.Context) error) {
	o.goTask(o.light, name, "light", fn)
}

func (o *startupTaskOrchestrator) GoHeavy(name string, fn func(context.Context) error) {
	o.goTask(o.heavy, name, "heavy", fn)
}

func (o *startupTaskOrchestrator) goTask(worker *runtimeStartupBackgroundWorker, name, kind string, fn func(context.Context) error) {
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

func (o *startupTaskOrchestrator) Shutdown(ctx context.Context) error {
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

func (w *runtimeStartupBackgroundWorker) Go(fn func(context.Context) error) {
	if w == nil || fn == nil {
		return
	}
	select {
	case <-w.ctx.Done():
		return
	case w.queue <- fn:
	}
}

func (w *runtimeStartupBackgroundWorker) Shutdown(ctx context.Context) error {
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

func (w *runtimeStartupBackgroundWorker) dispatch() {
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
					return err
				}
				return nil
			})
		}
	}
}
