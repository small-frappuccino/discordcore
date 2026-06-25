# Domain Architecture: service

## Layout Topology
```text
service/
├── base.go
├── dynamic_manager.go
├── manager.go
├── orchestrator.go
├── runtime_activity.go
└── service_lifecycle.go
```

## Source Stream Aggregation

// === FILE: pkg/service/base.go ===
```go
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
	stateMutex   sync.Mutex
	isRunning    bool
	startTime    *time.Time
	stopTime     *time.Time
	restartCount int
	errorCount   int
	lastError    error

	// Health monitoring
	lastHealthCheck time.Time
	healthStatus    HealthStatus
	healthMutex     sync.Mutex

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
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()
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
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()

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
	bs.stateMutex.Lock()
	defer bs.stateMutex.Unlock()
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

```

// === FILE: pkg/service/dynamic_manager.go ===
```go
package service

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sync/errgroup"
)

type ServiceWrapper interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Done() <-chan struct{}
}

type serviceState struct {
	wrapper    ServiceWrapper
	cancelFunc context.CancelFunc
	runDone    chan struct{}
}

type Manager struct {
	mu       sync.Mutex
	services map[string]*serviceState
	eg       *errgroup.Group
	egCtx    context.Context
}

func NewManager(ctx context.Context) *Manager {
	eg, egCtx := errgroup.WithContext(ctx)
	return &Manager{
		services: make(map[string]*serviceState),
		eg:       eg,
		egCtx:    egCtx,
	}
}

func (m *Manager) RegisterAndStart(name string, svc ServiceWrapper) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; exists {
		return errors.New("service already registered: " + name)
	}

	ctx, cancel := context.WithCancel(m.egCtx)
	state := &serviceState{
		wrapper:    svc,
		cancelFunc: cancel,
		runDone:    make(chan struct{}),
	}
	m.services[name] = state

	m.eg.Go(func() error {
		defer close(state.runDone)
		if err := svc.Start(ctx); err != nil {
			fmt.Printf("fatal: service %s stopped: %v\n", name, err)
			return fmt.Errorf("service %s failed: %w", name, err)
		}
		return nil
	})

	return nil
}

func (m *Manager) StopAndRemove(ctx context.Context, name string) error {
	m.mu.Lock()
	state, exists := m.services[name]
	if !exists {
		m.mu.Unlock()
		return errors.New("service not found: " + name)
	}
	delete(m.services, name)
	m.mu.Unlock()

	state.cancelFunc()

	if err := state.wrapper.Stop(ctx); err != nil {
		return fmt.Errorf("stop signal failed for %s: %w", name, err)
	}

	select {
	case <-state.runDone:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("drain timeout exceeded for %s: %w", name, ctx.Err())
	}
}

func (m *Manager) ForceRemove(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if state, exists := m.services[name]; exists {
		delete(m.services, name)
		state.cancelFunc()
	}
}

func (m *Manager) StopAll(ctx context.Context) error {
	m.mu.Lock()
	var names []string
	for name := range m.services {
		names = append(names, name)
	}
	m.mu.Unlock()

	var errs []error
	for _, name := range names {
		if err := m.StopAndRemove(ctx, name); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to stop some services: %v", errs)
	}
	return nil
}

// Wait blocks until the underlying errgroup completes.
func (m *Manager) Wait() error {
	return m.eg.Wait()
}

```

// === FILE: pkg/service/manager.go ===
```go
package service

import (
	"context"
	stdErrors "errors"
	"fmt"
	"runtime/debug"
	"slices"
	"sync"
	"time"

	"log/slog"

	"golang.org/x/sync/errgroup"
)

// ServiceState represents the current state of a service
type ServiceState string

// StateInitializing defines state initializing.
// StateRunning defines state running.
// StateUninitialized defines state uninitialized.
// StateStopped defines state stopped.
// StateError defines state error.
// StateRecovering defines state recovering.
// StateStopping defines state stopping.
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

// TypeMonitoring defines type monitoring.
// TypeCommands defines type commands.
// TypeCache defines type cache.
// TypeNotifier defines type notifier.
// TypeAutomod defines type automod.
const (
	TypeMonitoring ServiceType = "monitoring"
	TypeAutomod    ServiceType = "automod"
	TypeCommands   ServiceType = "commands"
	TypeCache      ServiceType = "cache"
	TypeNotifier   ServiceType = "notifier"
)

// ServicePriority determines startup/shutdown order (higher number = higher priority)
type ServicePriority int

// PriorityLow defines priority low.
// PriorityNormal defines priority normal.
// PriorityHigh defines priority high.
const (
	PriorityLow    ServicePriority = 1
	PriorityNormal ServicePriority = 5
	PriorityHigh   ServicePriority = 10
)

// HealthStatus represents the health of a service
type HealthStatus struct {
	Healthy   bool              `json:"healthy"`
	Message   string            `json:"message"`
	LastCheck time.Time         `json:"last_check"`
	Details   map[string]string `json:"details,omitempty"`
}

// ServiceMetric is a single pre-formatted display row a service exposes via
// Stats() for surfaces like /admin status. Pre-formatting on the producer side
// keeps the rendering responsibility next to the data semantics — boolean
// fields, counters, and durations choose their own human-readable form
// instead of being %v-rendered by every consumer.
//
// Stable-ordering contract: producers append rows in display order; consumers
// MUST iterate in slice order rather than re-sorting, so operators see the
// same layout across calls.
type ServiceMetric struct {
	// Label is the human-facing identifier shown on the left side of the
	// row. Use sentence case ("Roles cache size"), not snake_case.
	Label string `json:"label"`
	// Value is the already-formatted display string. Numbers should be
	// thousand-separated where appropriate, durations should be readable,
	// booleans should be "Yes"/"No" or similar.
	Value string `json:"value"`
}

// ServiceStats provides runtime statistics for a service. The Metrics slice
// replaces the older CustomMetrics map[string]any bag: typing the rows as
// pre-formatted Label/Value pairs lets each service own its rendering and
// keeps the consumer (/admin status today) free of %v-formatting drift.
type ServiceStats struct {
	StartTime    time.Time       `json:"start_time"`
	Uptime       time.Duration   `json:"uptime"`
	RestartCount int             `json:"restart_count"`
	ErrorCount   int             `json:"error_count"`
	LastError    *time.Time      `json:"last_error,omitempty"`
	Metrics      []ServiceMetric `json:"metrics,omitempty"`
}

// ServiceIdentity defines the static metadata of a service.
type ServiceIdentity interface {
	// Name returns the unique name of the service
	Name() string

	// Type returns the service type
	Type() ServiceType

	// Priority returns the startup/shutdown priority
	Priority() ServicePriority

	// Dependencies returns a list of service names this service depends on
	Dependencies() []string
}

// ServiceLifecycle defines the state transitions of a service.
type ServiceLifecycle interface {
	// Start initializes and starts the service
	Start(ctx context.Context) error

	// Stop gracefully stops the service
	Stop(ctx context.Context) error

	// IsRunning returns true if the service is currently running
	IsRunning() bool
}

// ServiceObservability defines the health and metrics inspection surface.
type ServiceObservability interface {
	// HealthCheck returns the current health status
	HealthCheck(ctx context.Context) HealthStatus

	// Stats returns runtime statistics
	Stats() ServiceStats
}

// Service defines the interface that all services must implement by composing
// identity, lifecycle, and observability.
type Service interface {
	ServiceIdentity
	ServiceLifecycle
	ServiceObservability
}

// ServiceInfo holds metadata about a registered service
type ServiceInfo struct {
	Service       Service      `json:"-"`
	State         ServiceState `json:"state"`
	LastStateTime time.Time    `json:"last_state_time"`
	StartTime     *time.Time   `json:"start_time,omitempty"`
	StopTime      *time.Time   `json:"stop_time,omitempty"`
	RestartCount  int          `json:"restart_count"`
	ErrorCount    int          `json:"error_count"`
	LastError     error        `json:"last_error,omitempty"`
}

// ServiceManager coordinates the lifecycle of all services
type ServiceManager struct {
	services   map[string]*ServiceInfo
	dependsOn  map[string][]string // service -> dependencies
	dependents map[string][]string // service -> dependents
	mu         sync.Mutex
	ctx        context.Context
	cancel     context.CancelFunc
	// Stop channel for health monitor
	healthStop     chan struct{}
	healthStopOnce sync.Once

	// Configuration
	shutdownTimeout time.Duration
	healthInterval  time.Duration
	maxRestarts     int
	restartDelay    time.Duration
	logger          *slog.Logger

	eg *errgroup.Group
}

// NewServiceManager creates a new service manager
func NewServiceManager(logger *slog.Logger) *ServiceManager {
	ctx, cancel := context.WithCancel(context.Background())
	eg, egCtx := errgroup.WithContext(ctx)

	return &ServiceManager{
		services:        make(map[string]*ServiceInfo),
		dependsOn:       make(map[string][]string),
		dependents:      make(map[string][]string),
		ctx:             egCtx, // Contexto global cancelado pelo errgroup em caso de erro
		cancel:          cancel,
		eg:              eg,
		healthStop:      make(chan struct{}),
		shutdownTimeout: 30 * time.Second,
		healthInterval:  5 * time.Minute,
		maxRestarts:     3,
		restartDelay:    5 * time.Second,
		logger:          logger,
	}
}

// log returns the configured logger or a default logger.
func (sm *ServiceManager) log() *slog.Logger {
	if sm == nil || sm.logger == nil {
		return slog.Default()
	}
	return sm.logger
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

	sm.log().Info("Service registered", "service", name, "type", service.Type(), "priority", service.Priority(), "dependencies", service.Dependencies())

	return nil
}

// StartAll starts all services in dependency order
func (sm *ServiceManager) StartAll() error {
	sm.log().Info("Starting all services...")

	startOrder, err := sm.calculateStartOrder()
	if err != nil {
		return fmt.Errorf("failed to calculate start order: %w", err)
	}

	for _, name := range startOrder {
		if err := sm.StartService(name); err != nil {
			// Fail-fast and trigger immediate cancellation of all adjacent processes
			sm.cancel()
			sm.StopAll(context.Background())
			return fmt.Errorf("failed to start service '%s': %w", name, err)
		}
	}

	// Start health monitoring
	sm.healthStopOnce = sync.Once{}
	sm.healthStop = make(chan struct{})

	sm.eg.Go(func() error {
		// Tie health monitor to the manager's context lifecycle via errgroup
		sm.healthMonitor()
		return nil
	})

	sm.log().Info("All services started successfully", "services_count", len(sm.services))
	return nil
}

// Fatal allows a service to signal a terminal failure, triggering global cancellation.
func (sm *ServiceManager) Fatal(err error) {
	sm.eg.Go(func() error {
		return err // Retornar erro no errgroup aciona o cancelamento do egCtx
	})
}

// RunBackground runs a function in the background tracked by the manager's errgroup.
func (sm *ServiceManager) RunBackground(fn func(context.Context)) {
	sm.eg.Go(func() error {
		defer func() {
			if r := recover(); r != nil {
				sm.log().Error("panic in background task", "panic", r)
			}
		}()
		fn(sm.ctx)
		return nil
	})
}

// Wait blocks until the service manager's error group finishes.
func (sm *ServiceManager) Wait() error {
	return sm.eg.Wait()
}

// StopAll stops all services in reverse dependency order
func (sm *ServiceManager) StopAll(ctx context.Context) error {
	sm.log().Info("Stopping all services...")

	// Cancel context to signal shutdown
	sm.cancel()
	// Signal health monitor to stop immediately
	sm.healthStopOnce.Do(func() { close(sm.healthStop) })

	startOrder, err := sm.calculateStartOrder()
	if err != nil {
		return fmt.Errorf("failed to calculate stop order: %w", err)
	}

	// Reverse the start order for shutdown
	stopOrder := slices.Clone(startOrder)
	slices.Reverse(stopOrder)

	var stopErrors []error
	for _, name := range stopOrder {
		if err := sm.StopService(ctx, name); err != nil {
			stopErrors = append(stopErrors, fmt.Errorf("failed to stop service '%s': %w", name, err))
		}
	}

	if len(stopErrors) > 0 {
		sm.log().Error("Some services failed to stop cleanly", "errors", stopErrors)
		return fmt.Errorf("failed to stop some services: %w", stdErrors.Join(stopErrors...))
	}

	sm.log().Info("All services stopped successfully")
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

	sm.log().Info("Starting service...", "service", name)

	err := info.Service.Start(ctx)

	sm.mu.Lock()
	if err != nil {
		serviceErr := fmt.Errorf("service failed to start: %w", err)
		info.LastError = serviceErr
		info.ErrorCount++
		sm.updateServiceState(info, StateError)
		sm.mu.Unlock()
		return fmt.Errorf("ServiceManager.StartService: %w", err)
	}

	now := time.Now()
	info.StartTime = &now
	sm.updateServiceState(info, StateRunning)
	sm.mu.Unlock()

	sm.log().Info("Service started successfully", "service", name)
	return nil
}

// StopService stops a specific service and its dependents
func (sm *ServiceManager) StopService(ctx context.Context, name string) error {
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
		if err := sm.StopService(ctx, dependent); err != nil {
			sm.log().Error("Failed to stop dependent service", "service", name, "dependent", dependent, "err", err)
		}
	}

	// Stop the service
	stopCtx, cancel := context.WithTimeoutCause(ctx, sm.shutdownTimeout, fmt.Errorf("shutdown timeout for service %q", name))
	defer cancel()

	sm.log().Info("Stopping service...", "service", name)

	err := info.Service.Stop(stopCtx)

	sm.mu.Lock()
	if err != nil {
		serviceErr := fmt.Errorf("service failed to stop cleanly: %w", err)
		info.LastError = serviceErr
		info.ErrorCount++
		sm.updateServiceState(info, StateError)
		sm.mu.Unlock()
		sm.log().Error("Service stop failed", "service", name, "err", err)
		return fmt.Errorf("ServiceManager.StopService: %w", err)
	}

	now := time.Now()
	info.StopTime = &now
	sm.updateServiceState(info, StateStopped)
	sm.mu.Unlock()

	sm.log().Info("Service stopped successfully", "service", name)
	return nil
}

// RestartService restarts a specific service
func (sm *ServiceManager) RestartService(ctx context.Context, name string) error {
	sm.log().Info("Restarting service...", "service", name)

	if err := sm.StopService(ctx, name); err != nil {
		sm.log().Error("Failed to stop service for restart", "service", name, "err", err)
	}

	// Wait a bit before restarting
	timer := time.NewTimer(sm.restartDelay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
	}

	return sm.StartService(name)
}

// GetServiceInfo returns information about a specific service
func (sm *ServiceManager) GetServiceInfo(name string) (*ServiceInfo, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

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
	sm.mu.Lock()
	defer sm.mu.Unlock()

	result := make(map[string]ServiceInfo, len(sm.services))
	for name, info := range sm.services {
		result[name] = *info
	}
	return result
}

// GetRunningServices returns a list of currently running services
func (sm *ServiceManager) GetRunningServices() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	running := make([]string, 0, len(sm.services))
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
				return fmt.Errorf("ServiceManager.calculateStartOrder: %w", err)
			}
		}
		temp[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for name := range sm.services {
		if err := visit(name); err != nil {
			return nil, fmt.Errorf("ServiceManager.calculateStartOrder: %w", err)
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
	defer func() {
		if r := recover(); r != nil {
			sm.logger.Error("ServiceManager healthMonitor panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ticker := time.NewTicker(sm.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sm.ctx.Done():
			return
		case <-sm.healthStop:
			return
		case <-ticker.C:
			sm.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all running services
func (sm *ServiceManager) performHealthChecks() {
	sm.mu.Lock()
	var runningServices []*ServiceInfo
	for _, info := range sm.services {
		if info.State == StateRunning {
			runningServices = append(runningServices, info)
		}
	}
	sm.mu.Unlock()

	for _, info := range runningServices {
		i := info
		sm.RunBackground(func(ctx context.Context) {
			sm.checkServiceHealth(i)
		})
	}
}

// checkServiceHealth performs a health check on a single service
func (sm *ServiceManager) checkServiceHealth(info *ServiceInfo) {
	ctx, cancel := context.WithTimeout(sm.ctx, 10*time.Second)
	defer cancel()

	health := info.Service.HealthCheck(ctx)

	if !health.Healthy {
		sm.log().Error("Service health check failed", "service", info.Service.Name(), "message", health.Message, "details", health.Details)

		// Consider restarting the service if it's been unhealthy
		sm.mu.Lock()
		info.ErrorCount++
		if info.RestartCount < sm.maxRestarts {
			info.RestartCount++ // Increment before spawning to prevent concurrent overlapping restarts
			sm.mu.Unlock()
			sm.RunBackground(func(ctx context.Context) {
				sm.log().Warn("Attempting to restart unhealthy service", "service", info.Service.Name())
				if err := sm.RestartService(ctx, info.Service.Name()); err != nil {
					sm.log().Error("Failed to restart unhealthy service", "service", info.Service.Name(), "err", err)
				}
			})
		} else {
			sm.mu.Unlock()
			sm.log().Error("Service exceeded maximum restart attempts", "service", info.Service.Name())
		}
	}
}

```

// === FILE: pkg/service/orchestrator.go ===
```go
package service

import (
	"context"
	"fmt"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// ExecuteOrchestration is a resilient wrapper that executes a service lifecycle step
// using synchronized propagation and explicit preemption.
func ExecuteOrchestration(ctx context.Context, action func(context.Context) error) error {
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("runtime panic caught: %v\n%s", r, debug.Stack())
			}
		}()

		return action(egCtx)
	})

	err := eg.Wait()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("shutdown deadline exceeded: pre-empting execution")
	}
	return err
}

```

// === FILE: pkg/service/runtime_activity.go ===
```go
package service

import (
	"context"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"log/slog"

	"github.com/small-frappuccino/discordcore/pkg/system"
)

type RuntimeActivityRunner func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error

type RuntimeActivityOptions struct {
	RunErr           RuntimeActivityRunner
	EventTimeout     time.Duration
	HeartbeatTimeout time.Duration
	BotInstanceID    string
	Logger           *slog.Logger
	Now              func() time.Time
	// OnHeartbeatTick fires after every heartbeat persistence attempt
	// (the startup attempt and each ticker firing), with the error
	// returned by RunErr. Test-only seam — production callers leave
	// it nil so the heartbeat loop adds zero work per tick.
	//
	// The callback runs synchronously inside the inner attempt
	// goroutine spawned by runCancellableHeartbeat. A callback that
	// blocks indefinitely no longer deadlocks StopHeartbeat (the
	// outer goroutine still exits via hbCtx.Done()), but it does
	// leak the inner attempt goroutine until the callback returns.
	// Tests that observe ticks should pass tickRecorder.Hook so the
	// dedicated drainer absorbs ticks the test is not asserting on
	// and releases in-flight sends via context cancel during cleanup.
	OnHeartbeatTick func(err error)
}

type RuntimeActivity struct {
	store            system.Repository
	runErr           RuntimeActivityRunner
	eventTimeout     time.Duration
	heartbeatTimeout time.Duration
	botInstanceID    string
	logger           *slog.Logger
	now              func() time.Time
	onHeartbeatTick  func(err error)

	mu       sync.Mutex
	hbCancel context.CancelFunc
	hbDone   chan struct{}
	hbWg     sync.WaitGroup
}

func NewRuntimeActivity(store system.Repository, opts RuntimeActivityOptions) *RuntimeActivity {
	runErr := opts.RunErr
	if runErr == nil {
		runErr = RunErrWithTimeoutContext
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}

	return &RuntimeActivity{
		store:            store,
		runErr:           runErr,
		eventTimeout:     opts.EventTimeout,
		heartbeatTimeout: opts.HeartbeatTimeout,
		botInstanceID:    strings.TrimSpace(opts.BotInstanceID),
		logger:           opts.Logger,
		now:              now,
		onHeartbeatTick:  opts.OnHeartbeatTick,
	}
}

func NewMonitoringRuntimeActivity(store system.Repository, logger *slog.Logger, botInstanceID ...string) *RuntimeActivity {
	scopedBotInstanceID := ""
	if len(botInstanceID) > 0 {
		scopedBotInstanceID = botInstanceID[0]
	}
	return NewRuntimeActivity(store, RuntimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		EventTimeout:     15 * time.Second,
		HeartbeatTimeout: 15 * time.Second,
		BotInstanceID:    scopedBotInstanceID,
		Logger:           logger,
	})
}

// MarkEvent marks event.
func (ra *RuntimeActivity) MarkEvent(ctx context.Context, source string) {
	if ra == nil || ra.store == nil {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if err := ra.runErr(ctx, ra.eventTimeout, func(runCtx context.Context) error {
		return ra.store.SetLastEventForBot(runCtx, ra.botInstanceID, ra.now())
	}); err != nil && ra.logger != nil {
		ra.logger.LogAttrs(ctx, slog.LevelWarn, "Failed to persist last event timestamp", slog.String("source", source), slog.Any("error", err))
	}
}

// StartHeartbeat starts heartbeat.
func (ra *RuntimeActivity) StartHeartbeat(ctx context.Context, interval time.Duration) {
	if ra == nil || ra.store == nil || interval <= 0 {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ra.mu.Lock()
	if ra.hbCancel != nil {
		ra.mu.Unlock()
		return
	}

	hbCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	ra.hbCancel = cancel
	ra.hbDone = done
	ra.mu.Unlock()

	// Both the startup persistence and each ticker firing are dispatched
	// through runCancellableHeartbeat so the outer goroutine can always
	// exit via hbCtx.Done(): a RunErr or OnHeartbeatTick that ignores ctx
	// only wedges the inner attempt goroutine (which is left to leak until
	// its blocking call returns), never close(done) or StopHeartbeat.
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if ra.logger != nil {
					ra.logger.Error("RuntimeActivity heartbeat loop panic caught", "panic", r, "stack", string(debug.Stack()))
				}
			}
			close(done)
		}()

		if !ra.runCancellableHeartbeat(hbCtx, "Failed to persist startup heartbeat") {
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if !ra.runCancellableHeartbeat(hbCtx, "Failed to persist heartbeat") {
					return
				}
			case <-hbCtx.Done():
				return
			}
		}
	}()
}

func (ra *RuntimeActivity) attemptHeartbeat(ctx context.Context, failureMessage string) {
	err := ra.runErr(ctx, ra.heartbeatTimeout, func(runCtx context.Context) error {
		return ra.store.SetHeartbeatForBot(runCtx, ra.botInstanceID, ra.now())
	})
	if err != nil && ra.logger != nil {
		ra.logger.LogAttrs(ctx, slog.LevelWarn, failureMessage, slog.Any("error", err))
	}
	if ra.onHeartbeatTick != nil {
		ra.onHeartbeatTick(err)
	}
}

// runCancellableHeartbeat runs a single attemptHeartbeat in a child
// goroutine and returns true when the attempt completes, false when ctx is
// canceled first. On cancellation the child goroutine is left running and
// exits when its underlying call eventually returns. The leak is the price
// for keeping close(done) and StopHeartbeat reachable when RunErr (or any
// callback it invokes) ignores ctx; in production the call respects ctx
// and the child returns promptly, so the leak is transient.
func (ra *RuntimeActivity) runCancellableHeartbeat(ctx context.Context, failureMessage string) bool {
	attemptDone := make(chan struct{})
	go func() {
		defer close(attemptDone)
		ra.attemptHeartbeat(ctx, failureMessage)
	}()
	select {
	case <-attemptDone:
		return true
	case <-ctx.Done():
		return false
	}
}

// StopHeartbeat stops heartbeat.
func (ra *RuntimeActivity) StopHeartbeat(ctx context.Context) error {
	if ra == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	ra.mu.Lock()
	cancel := ra.hbCancel
	done := ra.hbDone
	ra.hbCancel = nil
	ra.hbDone = nil
	ra.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done == nil {
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

```

// === FILE: pkg/service/service_lifecycle.go ===
```go
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
	mu     sync.Mutex
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
	sl.mu.Lock()
	if sl.state != lifecycleStateRunning || sl.runCtx == nil {
		sl.mu.Unlock()
		return nil, nil, false
	}
	sl.wg.Add(1)
	runCtx := sl.runCtx
	sl.mu.Unlock()

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
	sl.mu.Lock()
	defer sl.mu.Unlock()
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

```

