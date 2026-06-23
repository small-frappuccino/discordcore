package qotd

import (
	"context"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/small-frappuccino/discordcore/pkg/log"
	domain "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// Config holds runtime configuration.
type Config struct {
	PublishInterval time.Duration
	ReconcileEvery  time.Duration
}

// RuntimeService is the background daemon that orchestrates domain
// loops for QOTD.
type RuntimeService struct {
	cfg Config
	svc *domain.Service

	running   atomic.Bool
	startTime time.Time

	cancel context.CancelFunc
	eg     *errgroup.Group
	mu     sync.Mutex
}

// NewRuntimeService creates a new runtime daemon.
func NewRuntimeService(cfg Config, svc *domain.Service) *RuntimeService {
	return &RuntimeService{
		cfg: cfg,
		svc: svc,
	}
}

// Start begins the daemon.
func (s *RuntimeService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running.Load() {
		return nil
	}
	s.running.Store(true)
	s.startTime = time.Now()

	runCtx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.eg, _ = errgroup.WithContext(runCtx)

	s.eg.Go(func() error {
		defer func() {
			if r := recover(); r != nil {
				log.ApplicationLogger().Error("QOTD runtime panic", "panic", r, "stack", string(debug.Stack()))
			}
		}()
		s.loop(runCtx)
		return nil
	})

	return nil
}

// Stop shuts down the daemon gracefully.
func (s *RuntimeService) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running.Load() || s.cancel == nil {
		s.mu.Unlock()
		return nil
	}
	s.cancel()
	eg := s.eg
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		_ = eg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.running.Store(false)
	return nil
}

func (s *RuntimeService) loop(ctx context.Context) {
	// The loop will sleep and occasionally wake up to process guilds.
	// We use a mocked interval loop here for the rewrite.
	publishTimer := time.NewTimer(s.cfg.PublishInterval)
	defer publishTimer.Stop()

	for {
		select {
		case <-publishTimer.C:
			// In a real system, this iterates through guilds and calls
			// s.svc.PublishScheduledIfDue and s.svc.ReconcileGuild
			publishTimer.Reset(s.cfg.PublishInterval)
		case <-ctx.Done():
			return
		}
	}
}

// HealthCheck returns health status.
func (s *RuntimeService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   s.running.Load(),
		Message:   "QOTD daemon",
		LastCheck: time.Now(),
	}
}

// Name returns service name.
func (s *RuntimeService) Name() string { return "qotd" }

// Type returns service type.
func (s *RuntimeService) Type() service.ServiceType { return service.TypeMonitoring }

// Dependencies returns service dependencies.
func (s *RuntimeService) Dependencies() []string { return nil }

// IsRunning returns whether the service is currently running.
func (s *RuntimeService) IsRunning() bool {
	return s.running.Load()
}

// Priority returns the service startup priority.
func (s *RuntimeService) Priority() service.ServicePriority {
	return service.PriorityNormal
}

// Stats returns runtime statistics.
func (s *RuntimeService) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
