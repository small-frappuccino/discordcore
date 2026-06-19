package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"log/slog"
)

// BaseService provides common functionality for all services. Services that
// want to surface display metrics via Stats() override Stats() and append
// ServiceMetric rows themselves; BaseService no longer carries an internal
// metric bag because the previous map[string]any path was never populated by
// any caller and only blurred the contract.
type BaseService struct {
	name         string
	serviceType  ServiceType
	priority     ServicePriority
	dependencies []string
	logger       *slog.Logger

	// State management
	state        ServiceState
	stateMutex   sync.RWMutex
	isRunning    bool
	startTime    *time.Time
	stopTime     *time.Time
	restartCount int
	errorCount   int
	lastError    error

	// Health monitoring
	lastHealthCheck time.Time
	healthStatus    HealthStatus
	healthMutex     sync.RWMutex

	// Hooks for subclasses to implement
	startHook  func(ctx context.Context) error
	stopHook   func(ctx context.Context) error
	healthHook func(ctx context.Context) HealthStatus
}

// NewBaseService creates a new base service
func NewBaseService(name string, serviceType ServiceType, priority ServicePriority, dependencies []string, logger *slog.Logger) *BaseService {
	return &BaseService{
		name:         name,
		serviceType:  serviceType,
		priority:     priority,
		dependencies: dependencies,
		logger:       logger,
		state:        StateUninitialized,
		healthStatus: HealthStatus{
			Healthy:   true,
			Message:   "Service initialized",
			LastCheck: time.Now(),
		},
	}
}

// log returns the configured logger or a default logger.
func (bs *BaseService) log() *slog.Logger {
	if bs == nil || bs.logger == nil {
		return slog.Default()
	}
	return bs.logger
}

// Name returns the service name
func (bs *BaseService) Name() string {
	return bs.name
}

// Type returns the service type
func (bs *BaseService) Type() ServiceType {
	return bs.serviceType
}

// Priority returns the service priority
func (bs *BaseService) Priority() ServicePriority {
	return bs.priority
}

// Dependencies returns the service dependencies
func (bs *BaseService) Dependencies() []string {
	return bs.dependencies
}

// Start starts the service
func (bs *BaseService) Start(ctx context.Context) error {
	bs.stateMutex.Lock()
	if bs.isRunning {
		bs.stateMutex.Unlock()
		return nil // Already running
	}

	bs.log().Info("Starting service...", "service", bs.name)
	bs.state = StateInitializing
	bs.stateMutex.Unlock()

	var startErr error
	// Call the start hook if provided
	if bs.startHook != nil {
		startErr = ExecuteOrchestration(ctx, bs.startHook)
	}

	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()

	if startErr != nil {
		bs.state = StateError
		bs.errorCount++
		serviceErr := fmt.Errorf("service start hook failed: %w", startErr)
		bs.lastError = serviceErr
		bs.log().Error("Service start failed", "service", bs.name, "err", startErr)
		return serviceErr
	}

	// Update state
	bs.isRunning = true
	bs.state = StateRunning
	now := time.Now()
	bs.startTime = &now
	bs.stopTime = nil

	bs.log().Info("Service started successfully", "service", bs.name)
	return nil
}

// Stop stops the service
func (bs *BaseService) Stop(ctx context.Context) error {
	bs.stateMutex.Lock()
	if !bs.isRunning {
		bs.stateMutex.Unlock()
		return nil // Already stopped
	}

	bs.log().Info("Stopping service...", "service", bs.name)
	bs.state = StateStopping
	bs.stateMutex.Unlock()

	var stopErr error
	// Call the stop hook if provided
	if bs.stopHook != nil {
		stopErr = ExecuteOrchestration(ctx, bs.stopHook)
	}

	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()

	if stopErr != nil {
		bs.errorCount++
		serviceErr := fmt.Errorf("service stop hook failed: %w", stopErr)
		bs.lastError = serviceErr
		bs.state = StateError
		bs.log().Error("Service stop failed", "service", bs.name, "err", stopErr)
		return serviceErr
	}

	// Update state
	bs.isRunning = false
	bs.state = StateStopped
	now := time.Now()
	bs.stopTime = &now

	bs.log().Info("Service stopped", "service", bs.name)
	return nil
}

// IsRunning returns true if the service is running
func (bs *BaseService) IsRunning() bool {
	bs.stateMutex.RLock()
	defer bs.stateMutex.RUnlock()
	return bs.isRunning
}

// HealthCheck performs a health check
func (bs *BaseService) HealthCheck(ctx context.Context) HealthStatus {
	bs.healthMutex.Lock()
	defer bs.healthMutex.Unlock()

	// Update last check time
	bs.lastHealthCheck = time.Now()

	// If we have a custom health hook, use it
	if bs.healthHook != nil {
		bs.healthStatus = bs.healthHook(ctx)
	} else {
		// Default health check - just verify we're running
		bs.healthStatus = HealthStatus{
			Healthy:   bs.IsRunning(),
			Message:   bs.getDefaultHealthMessage(),
			LastCheck: bs.lastHealthCheck,
			Details: map[string]string{
				"state":         string(bs.GetState()),
				"uptime":        bs.getUptime().String(),
				"restart_count": strconv.Itoa(bs.restartCount),
				"error_count":   strconv.Itoa(bs.errorCount),
			},
		}
	}

	return bs.healthStatus
}

// Stats returns service statistics. The Metrics slice stays nil at the base
// level; concrete services that want to expose display rows override Stats()
// and populate them from their own typed sources.
func (bs *BaseService) Stats() ServiceStats {
	bs.stateMutex.RLock()
	defer bs.stateMutex.RUnlock()

	stats := ServiceStats{
		RestartCount: bs.restartCount,
		ErrorCount:   bs.errorCount,
	}

	if bs.startTime != nil {
		stats.StartTime = *bs.startTime
		stats.Uptime = time.Since(*bs.startTime)
	}

	if bs.lastError != nil {
		now := time.Now()
		stats.LastError = &now
	}

	return stats
}

// GetState returns the current service state
func (bs *BaseService) GetState() ServiceState {
	bs.stateMutex.RLock()
	defer bs.stateMutex.RUnlock()
	return bs.state
}

// SetStartHook sets the function to call when starting the service
func (bs *BaseService) SetStartHook(hook func(ctx context.Context) error) {
	bs.startHook = hook
}

// SetStopHook sets the function to call when stopping the service
func (bs *BaseService) SetStopHook(hook func(ctx context.Context) error) {
	bs.stopHook = hook
}

// SetHealthHook sets the function to call for health checks
func (bs *BaseService) SetHealthHook(hook func(ctx context.Context) HealthStatus) {
	bs.healthHook = hook
}

// IncrementRestartCount increments the restart counter
func (bs *BaseService) IncrementRestartCount() {
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()
	bs.restartCount++
}

// RecordError records an error
func (bs *BaseService) RecordError(err error) {
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()
	bs.errorCount++
	bs.lastError = err
}

// getDefaultHealthMessage returns a default health message based on current state
func (bs *BaseService) getDefaultHealthMessage() string {
	switch bs.state {
	case StateRunning:
		return "Service is running normally"
	case StateStopped:
		return "Service is stopped"
	case StateError:
		if bs.lastError != nil {
			return fmt.Sprintf("Service error: %v", bs.lastError)
		}
		return "Service is in error state"
	case StateInitializing:
		return "Service is starting up"
	case StateStopping:
		return "Service is shutting down"
	default:
		return "Service state unknown"
	}
}

// getUptime calculates the service uptime
func (bs *BaseService) getUptime() time.Duration {
	if bs.startTime == nil {
		return 0
	}
	if bs.stopTime != nil {
		return bs.stopTime.Sub(*bs.startTime)
	}
	return time.Since(*bs.startTime)
}

// LegacyServiceWrapper can be used to wrap existing services that don't implement the Service interface
type LegacyServiceWrapper struct {
	*BaseService
	wrappedStart func(context.Context) error
	wrappedStop  func(context.Context) error
	wrappedCheck func() bool
	doneChan     chan struct{}
	closeDone    sync.Once
}

// Stop stops the wrapper and safely signals done
func (s *LegacyServiceWrapper) Stop(ctx context.Context) error {
	err := s.BaseService.Stop(ctx)
	s.closeDone.Do(func() { close(s.doneChan) })
	return err
}

// LegacyServiceWrapperSpec configures a LegacyServiceWrapper: identity/metadata plus the
// start/stop/health callbacks invoked through the wrapper's hooks.
type LegacyServiceWrapperSpec struct {
	Name         string
	Type         ServiceType
	Priority     ServicePriority
	Dependencies []string
	Start        func(context.Context) error
	Stop         func(context.Context) error
	Check        func() bool
	Logger       *slog.Logger
}

// NewLegacyServiceWrapper creates a wrapper for existing services
func NewLegacyServiceWrapper(spec LegacyServiceWrapperSpec) *LegacyServiceWrapper {
	wrapper := &LegacyServiceWrapper{
		BaseService:  NewBaseService(spec.Name, spec.Type, spec.Priority, spec.Dependencies, spec.Logger),
		wrappedStart: spec.Start,
		wrappedStop:  spec.Stop,
		wrappedCheck: spec.Check,
		doneChan:     make(chan struct{}),
	}

	// Set up hooks to call the wrapped functions
	wrapper.SetStartHook(func(ctx context.Context) error {
		if wrapper.wrappedStart != nil {
			return wrapper.wrappedStart(ctx)
		}
		return nil
	})

	wrapper.SetStopHook(func(ctx context.Context) error {
		defer wrapper.closeDone.Do(func() { close(wrapper.doneChan) })
		if wrapper.wrappedStop != nil {
			return wrapper.wrappedStop(ctx)
		}
		return nil
	})

	wrapper.SetHealthHook(func(ctx context.Context) HealthStatus {
		healthy := true
		message := "Service is healthy"

		if wrapper.wrappedCheck != nil {
			healthy = wrapper.wrappedCheck()
			if !healthy {
				message = "Wrapped service health check failed"
			}
		}

		return HealthStatus{
			Healthy:   healthy,
			Message:   message,
			LastCheck: time.Now(),
			Details: map[string]string{
				"wrapped_service": spec.Name,
			},
		}
	})

	return wrapper
}

// Done returns a channel that is closed when the service stops.
func (s *LegacyServiceWrapper) Done() <-chan struct{} {
	return s.doneChan
}

// ManagedService provides a higher-level service implementation with automatic lifecycle management
type ManagedService struct {
	*BaseService
	manager      *ServiceManager
	autoRestart  bool
	maxRestarts  int
	restartDelay time.Duration
}

// NewManagedService creates a managed service that can handle its own lifecycle
func NewManagedService(
	name string,
	serviceType ServiceType,
	priority ServicePriority,
	dependencies []string,
	manager *ServiceManager,
	logger *slog.Logger,
) *ManagedService {
	return &ManagedService{
		BaseService:  NewBaseService(name, serviceType, priority, dependencies, logger),
		manager:      manager,
		autoRestart:  true,
		maxRestarts:  3,
		restartDelay: 5 * time.Second,
	}
}

// SetAutoRestart configures automatic restart behavior
func (ms *ManagedService) SetAutoRestart(enabled bool, maxRestarts int, delay time.Duration) {
	ms.autoRestart = enabled
	ms.maxRestarts = maxRestarts
	ms.restartDelay = delay
}

// HandleError processes service errors and potentially triggers restarts
func (ms *ManagedService) HandleError(err error) {
	serviceErr := fmt.Errorf("service encountered an error during operation: %w", err)

	ms.RecordError(serviceErr)

	if ms.autoRestart && ms.restartCount < ms.maxRestarts {
		// Use the package logger (simple categories are available)
		ms.log().Warn("Service error detected, attempting restart", "service", ms.name, "err", err)
		ms.manager.RunBackground(func(ctx context.Context) {
			select {
			case <-time.After(ms.restartDelay):
			case <-ctx.Done():
				return
			}
			if restartErr := ms.manager.RestartService(ctx, ms.name); restartErr != nil {
				ms.log().Error("Failed to restart service after error", "service", ms.name, "err", restartErr)
			}
		})
	}
}
