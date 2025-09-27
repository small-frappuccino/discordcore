package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/pkg/errors"
	"github.com/alice-bnuy/discordcore/pkg/log"
)

// ServiceState represents the current state of a service
type ServiceState string

const (
	StateUninitialized ServiceState = "uninitialized"
	StateInitializing  ServiceState = "initializing"
	StateRunning       ServiceState = "running"
	StateStopping      ServiceState = "stopping"
	StateStopped       ServiceState = "stopped"
	StateError         ServiceState = "error"
	StateRecovering    ServiceState = "recovering"
)

// ServiceType represents different types of services
type ServiceType string

const (
	TypeMonitoring ServiceType = "monitoring"
	TypeAutomod    ServiceType = "automod"
	TypeCommands   ServiceType = "commands"
	TypeCache      ServiceType = "cache"
	TypeNotifier   ServiceType = "notifier"
)

// ServicePriority determines startup/shutdown order (higher number = higher priority)
type ServicePriority int

const (
	PriorityLow    ServicePriority = 1
	PriorityNormal ServicePriority = 5
	PriorityHigh   ServicePriority = 10
)

// HealthStatus represents the health of a service
type HealthStatus struct {
	Healthy   bool                   `json:"healthy"`
	Message   string                 `json:"message"`
	LastCheck time.Time              `json:"last_check"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ServiceStats provides runtime statistics for a service
type ServiceStats struct {
	StartTime     time.Time              `json:"start_time"`
	Uptime        time.Duration          `json:"uptime"`
	RestartCount  int                    `json:"restart_count"`
	ErrorCount    int                    `json:"error_count"`
	LastError     *time.Time             `json:"last_error,omitempty"`
	CustomMetrics map[string]interface{} `json:"custom_metrics,omitempty"`
}

// Service defines the interface that all services must implement
type Service interface {
	// Name returns the unique name of the service
	Name() string

	// Type returns the service type
	Type() ServiceType

	// Priority returns the startup/shutdown priority
	Priority() ServicePriority

	// Dependencies returns a list of service names this service depends on
	Dependencies() []string

	// Start initializes and starts the service
	Start(ctx context.Context) error

	// Stop gracefully stops the service
	Stop(ctx context.Context) error

	// IsRunning returns true if the service is currently running
	IsRunning() bool

	// HealthCheck returns the current health status
	HealthCheck(ctx context.Context) HealthStatus

	// Stats returns runtime statistics
	Stats() ServiceStats
}

// ServiceInfo holds metadata about a registered service
type ServiceInfo struct {
	Service       Service              `json:"-"`
	State         ServiceState         `json:"state"`
	LastStateTime time.Time            `json:"last_state_time"`
	StartTime     *time.Time           `json:"start_time,omitempty"`
	StopTime      *time.Time           `json:"stop_time,omitempty"`
	RestartCount  int                  `json:"restart_count"`
	ErrorCount    int                  `json:"error_count"`
	LastError     *errors.ServiceError `json:"last_error,omitempty"`
}

// ServiceManager coordinates the lifecycle of all services
type ServiceManager struct {
	services     map[string]*ServiceInfo
	dependsOn    map[string][]string // service -> dependencies
	dependents   map[string][]string // service -> dependents
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	errorHandler *errors.ErrorHandler

	// Configuration
	shutdownTimeout time.Duration
	healthInterval  time.Duration
	maxRestarts     int
	restartDelay    time.Duration
}

// NewServiceManager creates a new service manager
func NewServiceManager(errorHandler *errors.ErrorHandler) *ServiceManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &ServiceManager{
		services:        make(map[string]*ServiceInfo),
		dependsOn:       make(map[string][]string),
		dependents:      make(map[string][]string),
		ctx:             ctx,
		cancel:          cancel,
		errorHandler:    errorHandler,
		shutdownTimeout: 30 * time.Second,
		healthInterval:  1 * time.Minute,
		maxRestarts:     3,
		restartDelay:    5 * time.Second,
	}
}

// Register adds a service to the manager
func (sm *ServiceManager) Register(service Service) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	name := service.Name()
	if _, exists := sm.services[name]; exists {
		return fmt.Errorf("service '%s' is already registered", name)
	}

	info := &ServiceInfo{
		Service:       service,
		State:         StateUninitialized,
		LastStateTime: time.Now(),
	}

	sm.services[name] = info
	sm.dependsOn[name] = service.Dependencies()

	// Build reverse dependency map
	for _, dep := range service.Dependencies() {
		sm.dependents[dep] = append(sm.dependents[dep], name)
	}

	log.Info().Applicationf("Service registered: service=%s type=%s priority=%d dependencies=%v",
		name, service.Type(), service.Priority(), service.Dependencies())

	return nil
}

// StartAll starts all services in dependency order
func (sm *ServiceManager) StartAll() error {
	log.Info().Applicationf("Starting all services...")

	startOrder, err := sm.calculateStartOrder()
	if err != nil {
		return fmt.Errorf("failed to calculate start order: %w", err)
	}

	var startErrors []error
	for _, name := range startOrder {
		if err := sm.StartService(name); err != nil {
			startErrors = append(startErrors, fmt.Errorf("failed to start service '%s': %w", name, err))
		}
	}

	if len(startErrors) > 0 {
		// Try to stop services that were started successfully
		sm.StopAll()
		return fmt.Errorf("failed to start services: %v", startErrors)
	}

	// Start health monitoring
	go sm.healthMonitor()

	log.Info().Applicationf("All services started successfully; services_count=%d", len(sm.services))
	return nil
}

// StopAll stops all services in reverse dependency order
func (sm *ServiceManager) StopAll() error {
	log.Info().Applicationf("Stopping all services...")

	// Cancel context to signal shutdown
	sm.cancel()

	startOrder, err := sm.calculateStartOrder()
	if err != nil {
		return fmt.Errorf("failed to calculate stop order: %w", err)
	}

	// Reverse the start order for shutdown
	stopOrder := make([]string, len(startOrder))
	for i, j := 0, len(startOrder)-1; i < j; i, j = i+1, j-1 {
		stopOrder[i], stopOrder[j] = startOrder[j], startOrder[i]
	}

	var stopErrors []error
	for _, name := range stopOrder {
		if err := sm.StopService(name); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("failed to stop service '%s': %w", name, err))
		}
	}

	if len(stopErrors) > 0 {
		log.Error().Errorf("Some services failed to stop cleanly: %v", stopErrors)
		return fmt.Errorf("failed to stop some services: %v", stopErrors)
	}

	log.Info().Applicationf("All services stopped successfully")
	return nil
}

// StartService starts a specific service and its dependencies
func (sm *ServiceManager) StartService(name string) error {
	sm.mu.Lock()
	info, exists := sm.services[name]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("service '%s' not found", name)
	}

	if info.State == StateRunning {
		sm.mu.Unlock()
		return nil // Already running
	}

	if info.State == StateInitializing {
		sm.mu.Unlock()
		return fmt.Errorf("service '%s' is already initializing", name)
	}

	sm.updateServiceState(info, StateInitializing)
	sm.mu.Unlock()

	// Start dependencies first
	for _, dep := range sm.dependsOn[name] {
		if err := sm.StartService(dep); err != nil {
			sm.mu.Lock()
			sm.updateServiceState(info, StateError)
			sm.mu.Unlock()
			return fmt.Errorf("failed to start dependency '%s': %w", dep, err)
		}
	}

	// Start the service
	ctx, cancel := context.WithTimeout(sm.ctx, 30*time.Second)
	defer cancel()

	log.Info().Applicationf("service %s: Starting service...", name)

	err := sm.errorHandler.HandleWithRetry(ctx, "start_service", name, func() error {
		return info.Service.Start(ctx)
	})

	sm.mu.Lock()
	if err != nil {
		serviceErr := errors.NewServiceError(
			errors.CategoryService,
			errors.SeverityHigh,
			name,
			"start",
			"Service failed to start",
			err,
		)
		info.LastError = serviceErr
		info.ErrorCount++
		sm.updateServiceState(info, StateError)
		sm.mu.Unlock()
		return err
	}

	now := time.Now()
	info.StartTime = &now
	sm.updateServiceState(info, StateRunning)
	sm.mu.Unlock()

	log.Info().Applicationf("service %s: Service started successfully", name)
	return nil
}

// StopService stops a specific service and its dependents
func (sm *ServiceManager) StopService(name string) error {
	sm.mu.Lock()
	info, exists := sm.services[name]
	if !exists {
		sm.mu.Unlock()
		return fmt.Errorf("service '%s' not found", name)
	}

	if info.State != StateRunning {
		sm.mu.Unlock()
		return nil // Not running
	}

	sm.updateServiceState(info, StateStopping)
	sm.mu.Unlock()

	// Stop dependents first
	for _, dependent := range sm.dependents[name] {
		if err := sm.StopService(dependent); err != nil {
			log.Error().Errorf("Failed to stop dependent service: service=%s dependent=%s error=%v", name, dependent, err)
		}
	}

	// Stop the service
	ctx, cancel := context.WithTimeout(context.Background(), sm.shutdownTimeout)
	defer cancel()

	log.Info().Applicationf("service %s: Stopping service...", name)

	err := info.Service.Stop(ctx)

	sm.mu.Lock()
	if err != nil {
		serviceErr := errors.NewServiceError(
			errors.CategoryService,
			errors.SeverityMedium,
			name,
			"stop",
			"Service failed to stop cleanly",
			err,
		)
		info.LastError = serviceErr
		info.ErrorCount++
	}

	now := time.Now()
	info.StopTime = &now
	sm.updateServiceState(info, StateStopped)
	sm.mu.Unlock()

	if err != nil {
		log.Error().Errorf("Service %s stopped with errors: %v", name, err)
		return err
	}

	log.Info().Applicationf("service %s: Service stopped successfully", name)
	return nil
}

// RestartService restarts a specific service
func (sm *ServiceManager) RestartService(name string) error {
	log.Info().Applicationf("service %s: Restarting service...", name)

	if err := sm.StopService(name); err != nil {
		log.Error().Errorf("Failed to stop service for restart: service=%s error=%v", name, err)
	}

	// Wait a bit before restarting
	time.Sleep(sm.restartDelay)

	sm.mu.Lock()
	info := sm.services[name]
	info.RestartCount++
	sm.mu.Unlock()

	return sm.StartService(name)
}

// GetServiceInfo returns information about a specific service
func (sm *ServiceManager) GetServiceInfo(name string) (*ServiceInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	info, exists := sm.services[name]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found", name)
	}

	// Create a copy to avoid race conditions
	infoCopy := *info
	return &infoCopy, nil
}

// GetAllServices returns information about all registered services
func (sm *ServiceManager) GetAllServices() map[string]ServiceInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]ServiceInfo)
	for name, info := range sm.services {
		result[name] = *info
	}
	return result
}

// GetRunningServices returns a list of currently running services
func (sm *ServiceManager) GetRunningServices() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var running []string
	for name, info := range sm.services {
		if info.State == StateRunning {
			running = append(running, name)
		}
	}
	return running
}

// calculateStartOrder determines the order in which services should be started
func (sm *ServiceManager) calculateStartOrder() ([]string, error) {
	// Topological sort to handle dependencies
	visited := make(map[string]bool)
	temp := make(map[string]bool)
	var order []string

	var visit func(string) error
	visit = func(name string) error {
		if temp[name] {
			return fmt.Errorf("circular dependency detected involving service '%s'", name)
		}
		if visited[name] {
			return nil
		}

		temp[name] = true
		for _, dep := range sm.dependsOn[name] {
			if _, exists := sm.services[dep]; !exists {
				return fmt.Errorf("service '%s' depends on unknown service '%s'", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}
		temp[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for name := range sm.services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// updateServiceState updates the state of a service (assumes lock is held)
func (sm *ServiceManager) updateServiceState(info *ServiceInfo, state ServiceState) {
	info.State = state
	info.LastStateTime = time.Now()
}

// healthMonitor runs periodic health checks on all services
func (sm *ServiceManager) healthMonitor() {
	ticker := time.NewTicker(sm.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-ticker.C:
			sm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all running services
func (sm *ServiceManager) performHealthChecks() {
	sm.mu.RLock()
	var runningServices []*ServiceInfo
	for _, info := range sm.services {
		if info.State == StateRunning {
			runningServices = append(runningServices, info)
		}
	}
	sm.mu.RUnlock()

	for _, info := range runningServices {
		go sm.checkServiceHealth(info)
	}
}

// checkServiceHealth performs a health check on a single service
func (sm *ServiceManager) checkServiceHealth(info *ServiceInfo) {
	ctx, cancel := context.WithTimeout(sm.ctx, 10*time.Second)
	defer cancel()

	health := info.Service.HealthCheck(ctx)

	if !health.Healthy {
		log.Error().Errorf("Service health check failed: service=%s message=%s details=%v", info.Service.Name(), health.Message, health.Details)

		// Consider restarting the service if it's been unhealthy
		sm.mu.Lock()
		info.ErrorCount++
		if info.RestartCount < sm.maxRestarts {
			sm.mu.Unlock()
			go func() {
				log.Info().Applicationf("service %s: Attempting to restart unhealthy service", info.Service.Name())
				if err := sm.RestartService(info.Service.Name()); err != nil {
				log.Error().Errorf("Failed to restart unhealthy service: service=%s error=%v", info.Service.Name(), err)
				}
			}()
		} else {
			sm.mu.Unlock()
			log.Error().Errorf("Service %s exceeded maximum restart attempts", info.Service.Name())
		}
	}
}
