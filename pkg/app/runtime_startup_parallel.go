package app

import (
	"context"
	stdErrors "errors"
	"fmt"
	"log/slog"
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

	slog.Info("Architectural state transition: Background worker pool initialized",
		slog.Int("parallelism_limit", parallelism),
		slog.Int("queue_capacity", queueSize),
	)

	go worker.dispatch()
	return worker
}

type StartupTaskOrchestrator struct {
	light *RuntimeStartupBackgroundWorker
	heavy *RuntimeStartupBackgroundWorker
}

func NewStartupTaskOrchestrator(runtimeCount int) *StartupTaskOrchestrator {
	slog.Info("Architectural state transition: Startup task orchestrator instantiated",
		slog.Int("runtime_count_heuristic", runtimeCount),
	)

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

	slog.Debug("Tracking complex conditional branch: Injecting closure into orchestrator queue",
		slog.String("task_name", name),
		slog.String("queue_tier", kind),
	)

	worker.Go(func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			if ctx.Err() != nil {
				slog.Debug("Tracking complex conditional branch: Task execution halted via context cancellation",
					slog.String("task_name", name),
				)
				return nil
			}
			slog.Warn("Mitigated service degradation: Background startup task encountered an error and aborted",
				slog.String("task", name),
				slog.String("kind", kind),
				slog.String("error", err.Error()),
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

	slog.Info("Architectural state transition: Halting startup orchestrator and draining worker pools")

	var errs []error
	if o.light != nil {
		if err := o.light.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			errWrap := fmt.Errorf("shutdown light startup tasks: %w", err)
			log.EmitBlockingError("Blocking structural failure: Light worker pool failed to terminate cleanly", errWrap, log.GenerateRequestID())
			errs = append(errs, errWrap)
		}
	}
	if o.heavy != nil {
		if err := o.heavy.Shutdown(ctx); err != nil && !stdErrors.Is(err, context.DeadlineExceeded) {
			errWrap := fmt.Errorf("shutdown heavy startup tasks: %w", err)
			log.EmitBlockingError("Blocking structural failure: Heavy worker pool failed to terminate cleanly", errWrap, log.GenerateRequestID())
			errs = append(errs, errWrap)
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
		slog.Debug("Tracking complex conditional branch: Task rejected, worker pool context already finalized")
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
		slog.Debug("Tracking complex conditional branch: Broadcasting cancellation signal across worker goroutines")
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
		errWrap := ctx.Err()
		slog.Warn("Mitigated service degradation: Context deadline exceeded while awaiting worker pool drain",
			slog.String("error", errWrap.Error()),
		)
		return errWrap
	}
}

func (w *RuntimeStartupBackgroundWorker) dispatch() {
	defer close(w.dispatchDone)

	for {
		select {
		case <-w.ctx.Done():
			slog.Debug("Tracking complex conditional branch: Dispatcher loop terminating via context closure")
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
