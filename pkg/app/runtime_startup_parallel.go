package app

import (
	"context"
	"sync"

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
	ctx, cancel := context.WithCancel(context.Background())
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(resolveRuntimeBackgroundParallelism(runtimeCount))

	queueSize := runtimeCount
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
