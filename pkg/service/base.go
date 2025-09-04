package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/v2/pkg/errors"
	"github.com/alice-bnuy/logutil"
)

// BaseService provides common functionality for all services
type BaseService struct {
	name         string
	serviceType  ServiceType
	priority     ServicePriority
	dependencies []string

	// State management
	state        ServiceState
	stateMutex   sync.RWMutex
	isRunning    bool
	startTime    *time.Time
	stopTime     *time.Time
	restartCount int
	errorCount   int
	lastError    *errors.ServiceError

	// Health monitoring
	lastHealthCheck time.Time
	healthStatus    HealthStatus
	healthMutex     sync.RWMutex

	// Custom metrics
	customMetrics    map[string]interface{}
	customMetricsMux sync.RWMutex

	// Hooks for subclasses to implement
	startHook  func(ctx context.Context) error
	stopHook   func(ctx context.Context) error
	healthHook func(ctx context.Context) HealthStatus

	logger *logutil.Logger
}

// NewBaseService creates a new base service
func NewBaseService(name string, serviceType ServiceType, priority ServicePriority, dependencies []string) *BaseService {
	return &BaseService{
		name:          name,
		serviceType:   serviceType,
		priority:      priority,
		dependencies:  dependencies,
		state:         StateUninitialized,
		customMetrics: make(map[string]interface{}),
		healthStatus: HealthStatus{
			Healthy:   true,
			Message:   "Service initialized",
			LastCheck: time.Now(),
		},
		logger: logutil.WithField("service", name),
	}
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
	defer bs.stateMutex.Unlock()

	if bs.isRunning {
		return nil // Already running
	}

	bs.logger.Info("Starting service...")
	bs.state = StateInitializing

	// Call the start hook if provided
	if bs.startHook != nil {
		if err := bs.startHook(ctx); err != nil {
			bs.state = StateError
			bs.errorCount++
			serviceErr := errors.NewServiceError(
				errors.CategoryService,
				errors.SeverityHigh,
				bs.name,
				"start",
				"Service start hook failed",
				err,
			)
			bs.lastError = serviceErr
			bs.logger.WithField("error", err).Error("Service start failed")
			return serviceErr
		}
	}

	// Update state
	bs.isRunning = true
	bs.state = StateRunning
	now := time.Now()
	bs.startTime = &now
	bs.stopTime = nil

	bs.logger.Info("Service started successfully")
	return nil
}

// Stop stops the service
func (bs *BaseService) Stop(ctx context.Context) error {
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()

	if !bs.isRunning {
		return nil // Already stopped
	}

	bs.logger.Info("Stopping service...")
	bs.state = StateStopping

	// Call the stop hook if provided
	if bs.stopHook != nil {
		if err := bs.stopHook(ctx); err != nil {
			bs.errorCount++
			serviceErr := errors.NewServiceError(
				errors.CategoryService,
				errors.SeverityMedium,
				bs.name,
				"stop",
				"Service stop hook failed",
				err,
			)
			bs.lastError = serviceErr
			bs.logger.WithField("error", err).Warn("Service stop failed")
			// Continue with shutdown even if hook fails
		}
	}

	// Update state
	bs.isRunning = false
	bs.state = StateStopped
	now := time.Now()
	bs.stopTime = &now

	bs.logger.Info("Service stopped")
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
			Details: map[string]interface{}{
				"state":         bs.GetState(),
				"uptime":        bs.getUptime(),
				"restart_count": bs.restartCount,
				"error_count":   bs.errorCount,
			},
		}
	}

	return bs.healthStatus
}

// Stats returns service statistics
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
		stats.LastError = &bs.lastError.Timestamp
	}

	// Add custom metrics
	bs.customMetricsMux.RLock()
	if len(bs.customMetrics) > 0 {
		stats.CustomMetrics = make(map[string]interface{})
		for k, v := range bs.customMetrics {
			stats.CustomMetrics[k] = v
		}
	}
	bs.customMetricsMux.RUnlock()

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

// SetCustomMetric sets a custom metric value
func (bs *BaseService) SetCustomMetric(key string, value interface{}) {
	bs.customMetricsMux.Lock()
	defer bs.customMetricsMux.Unlock()
	bs.customMetrics[key] = value
}

// GetCustomMetric gets a custom metric value
func (bs *BaseService) GetCustomMetric(key string) (interface{}, bool) {
	bs.customMetricsMux.RLock()
	defer bs.customMetricsMux.RUnlock()
	value, exists := bs.customMetrics[key]
	return value, exists
}

// IncrementRestartCount increments the restart counter
func (bs *BaseService) IncrementRestartCount() {
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()
	bs.restartCount++
}

// RecordError records an error
func (bs *BaseService) RecordError(err *errors.ServiceError) {
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
			return fmt.Sprintf("Service error: %s", bs.lastError.Message)
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

// ServiceWrapper can be used to wrap existing services that don't implement the Service interface
type ServiceWrapper struct {
	*BaseService
	wrappedStart func() error
	wrappedStop  func() error
	wrappedCheck func() bool
}

// NewServiceWrapper creates a wrapper for existing services
func NewServiceWrapper(
	name string,
	serviceType ServiceType,
	priority ServicePriority,
	dependencies []string,
	startFunc func() error,
	stopFunc func() error,
	checkFunc func() bool,
) *ServiceWrapper {
	wrapper := &ServiceWrapper{
		BaseService:  NewBaseService(name, serviceType, priority, dependencies),
		wrappedStart: startFunc,
		wrappedStop:  stopFunc,
		wrappedCheck: checkFunc,
	}

	// Set up hooks to call the wrapped functions
	wrapper.SetStartHook(func(ctx context.Context) error {
		if wrapper.wrappedStart != nil {
			return wrapper.wrappedStart()
		}
		return nil
	})

	wrapper.SetStopHook(func(ctx context.Context) error {
		if wrapper.wrappedStop != nil {
			return wrapper.wrappedStop()
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
			Details: map[string]interface{}{
				"wrapped_service": name,
			},
		}
	})

	return wrapper
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
) *ManagedService {
	return &ManagedService{
		BaseService:  NewBaseService(name, serviceType, priority, dependencies),
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
	serviceErr := errors.NewServiceError(
		errors.CategoryService,
		errors.SeverityMedium,
		ms.name,
		"runtime",
		"Service encountered an error during operation",
		err,
	)

	ms.RecordError(serviceErr)

	if ms.autoRestart && ms.restartCount < ms.maxRestarts {
		ms.logger.WithField("error", err).Info("Service error detected, attempting restart")
		go func() {
			time.Sleep(ms.restartDelay)
			if restartErr := ms.manager.RestartService(ms.name); restartErr != nil {
				ms.logger.WithField("restart_error", restartErr).Error("Failed to restart service after error")
			}
		}()
	}
}
