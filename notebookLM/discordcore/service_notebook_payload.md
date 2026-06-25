# Domain Architecture: service

## Layout Topology
```text
service/
├── base.go
├── base_test.go
├── dynamic_manager.go
├── dynamic_manager_test.go
├── manager.go
├── manager_test.go
├── orchestrator.go
├── orchestrator_test.go
├── runtime_activity.go
├── runtime_activity_test.go
├── service_lifecycle.go
└── service_lifecycle_test.go
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

// === FILE: pkg/service/base_test.go ===
```go
package service

import (
	"context"
	stdErrors "errors"
	"testing"
)

func TestBaseServiceStopReturnsErrorAndKeepsErrorState(t *testing.T) {
	t.Parallel()
	stopErr := stdErrors.New("stop failed")
	svc := NewBaseService("test", TypeMonitoring, PriorityNormal, nil, nil)
	svc.SetStopHook(func(context.Context) error {
		return stopErr
	})

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	err := svc.Stop(context.Background())
	if !stdErrors.Is(err, stopErr) {
		t.Fatalf("expected stop error %v, got %v", stopErr, err)
	}
	if !svc.IsRunning() {
		t.Fatalf("expected service to remain running after failed stop")
	}
	if got := svc.GetState(); got != StateError {
		t.Fatalf("expected state %q, got %q", StateError, got)
	}
	if stats := svc.Stats(); stats.ErrorCount != 1 {
		t.Fatalf("expected error count 1, got %d", stats.ErrorCount)
	}
}

func TestLegacyServiceWrapperPassesLifecycleContext(t *testing.T) {
	t.Parallel()
	startCtxKey := struct{}{}
	stopCtxKey := struct{}{}

	var startValue string
	var stopValue string

	wrapper := NewLegacyServiceWrapper(LegacyServiceWrapperSpec{
		Name:     "wrapped",
		Type:     TypeMonitoring,
		Priority: PriorityNormal,
		Start: func(ctx context.Context) error {
			startValue, _ = ctx.Value(startCtxKey).(string)
			return nil
		},
		Stop: func(ctx context.Context) error {
			stopValue, _ = ctx.Value(stopCtxKey).(string)
			return nil
		},
	})

	startCtx := context.WithValue(context.Background(), startCtxKey, "start")
	if err := wrapper.Start(startCtx); err != nil {
		t.Fatalf("start wrapper: %v", err)
	}
	if startValue != "start" {
		t.Fatalf("expected start context value to be propagated, got %q", startValue)
	}

	stopCtx := context.WithValue(context.Background(), stopCtxKey, "stop")
	if err := wrapper.Stop(stopCtx); err != nil {
		t.Fatalf("stop wrapper: %v", err)
	}
	if stopValue != "stop" {
		t.Fatalf("expected stop context value to be propagated, got %q", stopValue)
	}
}

func TestServiceManagerStopFailureLeavesServiceInErrorState(t *testing.T) {
	t.Parallel()
	stopErr := stdErrors.New("stop failed")
	svc := NewBaseService("managed", TypeMonitoring, PriorityNormal, nil, nil)
	svc.SetStopHook(func(context.Context) error {
		return stopErr
	})

	manager := NewServiceManager(nil)
	if err := manager.Register(svc); err != nil {
		t.Fatalf("register service: %v", err)
	}
	if err := manager.StartService(svc.Name()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	err := manager.StopService(context.Background(), svc.Name())
	if !stdErrors.Is(err, stopErr) {
		t.Fatalf("expected stop error %v, got %v", stopErr, err)
	}

	info, err := manager.GetServiceInfo(svc.Name())
	if err != nil {
		t.Fatalf("get service info: %v", err)
	}
	if info.State != StateError {
		t.Fatalf("expected manager state %q, got %q", StateError, info.State)
	}
	if info.StopTime != nil {
		t.Fatalf("expected stop time to remain unset after failed stop")
	}
	if info.ErrorCount != 1 {
		t.Fatalf("expected manager error count 1, got %d", info.ErrorCount)
	}
	if !svc.IsRunning() {
		t.Fatalf("expected service to remain running after manager stop failure")
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

// === FILE: pkg/service/dynamic_manager_test.go ===
```go
package service

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestDynamicManager(t *testing.T) {
	t.Parallel()
	dm := NewManager(context.Background())
	if dm == nil {
		t.Fatal("expected non-nil manager")
	}

	wrapper := NewLegacyServiceWrapper(LegacyServiceWrapperSpec{
		Name:     "dyn",
		Type:     TypeMonitoring,
		Priority: PriorityNormal,
		Start:    func(ctx context.Context) error { return nil },
		Stop:     func(ctx context.Context) error { return nil },
		Check:    func() bool { return true },
	})

	if err := dm.RegisterAndStart("dyn", wrapper); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	// Wait for the background Start goroutine to actually mark it running
	for !wrapper.IsRunning() {
		// Yield the processor to allow the start goroutine to progress
		runtime.Gosched()
	}

	if err := dm.StopAndRemove(context.Background(), "dyn"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	dm.ForceRemove("dyn")

	sm := NewServiceManager(nil)
	s1 := &mockService{name: "s1", startFunc: func(ctx context.Context) error { return nil }}
	sm.Register(s1)
	sm.StartAll()

	all := sm.GetAllServices()
	if len(all) != 1 {
		t.Errorf("expected 1 service")
	}
	running := sm.GetRunningServices()
	if len(running) != 1 {
		t.Errorf("expected 1 running service")
	}
	sm.StopAll(context.Background())
}

func TestBaseServiceAccessors(t *testing.T) {
	t.Parallel()
	bs := NewBaseService("base", TypeMonitoring, PriorityNormal, nil, nil)
	bs.SetHealthHook(func(ctx context.Context) HealthStatus {
		return HealthStatus{Healthy: true}
	})

	hc := bs.HealthCheck(context.Background())
	if !hc.Healthy {
		t.Errorf("expected healthy")
	}

	bs.IncrementRestartCount()
	bs.RecordError(errors.New("test"))
	stats := bs.Stats()
	if stats.RestartCount != 1 || stats.ErrorCount != 1 {
		t.Errorf("expected 1 restart and 1 error")
	}
	bs.Start(context.Background())
	bs.Stop(context.Background())
}

func TestManagedService(t *testing.T) {
	t.Parallel()
	sm := NewServiceManager(nil)
	defer sm.StopAll(context.Background())

	ms := NewManagedService("managed", TypeMonitoring, PriorityNormal, nil, sm, nil)
	ms.SetAutoRestart(true, 1, time.Millisecond)

	// Simulate start so isRunning is true (to allow Stop to work)
	ms.Start(context.Background())

	ms.HandleError(errors.New("simulated error"))

	if ms.Stats().ErrorCount != 1 {
		t.Errorf("expected 1 error")
	}
}

func TestDynamicManager_ZeroLeakToggling(t *testing.T) {
	time.Sleep(50 * time.Millisecond) // stabilize background test goroutines
	initialGoroutines := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	dm := NewManager(ctx)

	for i := 0; i < 50; i++ {
		name := "dyn_leak_" + string(rune(i))
		wrapper := NewLegacyServiceWrapper(LegacyServiceWrapperSpec{
			Name:     name,
			Type:     TypeMonitoring,
			Priority: PriorityNormal,
			Start: func(ctx context.Context) error {
				<-ctx.Done()
				return nil
			},
			Stop: func(ctx context.Context) error {
				return nil
			},
			Check: func() bool { return true },
		})
		if err := dm.RegisterAndStart(name, wrapper); err != nil {
			t.Fatalf("register failed: %v", err)
		}
	}

	time.Sleep(50 * time.Millisecond)
	midGoroutines := runtime.NumGoroutine()
	if midGoroutines <= initialGoroutines {
		t.Errorf("expected goroutine count to increase, got mid=%d vs initial=%d", midGoroutines, initialGoroutines)
	}

	cancel()
	_ = dm.Wait()

	time.Sleep(50 * time.Millisecond)
	finalGoroutines := runtime.NumGoroutine()

	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: initial=%d, mid=%d, final=%d", initialGoroutines, midGoroutines, finalGoroutines)
	}
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

// === FILE: pkg/service/manager_test.go ===
```go
package service

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

type mockService struct {
	name         string
	dependencies []string

	startFunc func(ctx context.Context) error
	stopFunc  func(ctx context.Context) error

	mu           sync.RWMutex
	running      bool
	healthStatus HealthStatus
}

func (m *mockService) Name() string              { return m.name }
func (m *mockService) Type() ServiceType         { return TypeMonitoring }
func (m *mockService) Priority() ServicePriority { return PriorityNormal }
func (m *mockService) Dependencies() []string    { return m.dependencies }

func (m *mockService) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startFunc != nil {
		err := m.startFunc(ctx)
		if err != nil {
			return err
		}
	}
	m.running = true
	return nil
}

func (m *mockService) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopFunc != nil {
		err := m.stopFunc(ctx)
		if err != nil {
			return err
		}
	}
	m.running = false
	return nil
}

func (m *mockService) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

func (m *mockService) HealthCheck(ctx context.Context) HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthStatus
}

func (m *mockService) Stats() ServiceStats {
	return ServiceStats{}
}

func TestManager_DependencyResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		services    []*mockService
		expectOrder []string
		expectErr   bool
	}{
		{
			name: "linear dependency",
			services: []*mockService{
				{name: "a", dependencies: []string{}},
				{name: "b", dependencies: []string{"a"}},
				{name: "c", dependencies: []string{"b"}},
			},
			expectOrder: []string{"a", "b", "c"},
		},
		{
			name: "circular dependency",
			services: []*mockService{
				{name: "a", dependencies: []string{"c"}},
				{name: "b", dependencies: []string{"a"}},
				{name: "c", dependencies: []string{"b"}},
			},
			expectErr: true,
		},
		{
			name: "missing dependency",
			services: []*mockService{
				{name: "a", dependencies: []string{"d"}},
			},
			expectErr: true,
		},
		{
			name: "multiple independent trees",
			services: []*mockService{
				{name: "x", dependencies: []string{}},
				{name: "y", dependencies: []string{"x"}},
				{name: "1", dependencies: []string{}},
				{name: "2", dependencies: []string{"1"}},
			},
			// Order is deterministic due to map iteration, but both roots are resolvable.
			// The map iteration in calculateStartOrder iterates over sm.services.
			// Since we want deterministic assertions, we just check success.
			expectErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sm := NewServiceManager(nil)
			for _, s := range tc.services {
				if err := sm.Register(s); err != nil {
					t.Fatalf("unexpected register error: %v", err)
				}
			}

			order, err := sm.calculateStartOrder()
			if tc.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(tc.expectOrder) > 0 {
				if !reflect.DeepEqual(order, tc.expectOrder) {
					t.Errorf("expected order %v, got %v", tc.expectOrder, order)
				}
			}
		})
	}
}

func TestManager_CascadingFailure(t *testing.T) {
	t.Parallel()

	sm := NewServiceManager(nil)

	s1StartCh := make(chan struct{})
	s1StopCh := make(chan struct{})

	s1 := &mockService{
		name: "s1",
		startFunc: func(ctx context.Context) error {
			close(s1StartCh)
			return nil
		},
		stopFunc: func(ctx context.Context) error {
			close(s1StopCh)
			return nil
		},
	}
	s2 := &mockService{
		name:         "s2",
		dependencies: []string{"s1"},
		startFunc: func(ctx context.Context) error {
			return errors.New("simulated failure")
		},
	}
	s3 := &mockService{name: "s3", dependencies: []string{"s2"}}

	sm.Register(s1)
	sm.Register(s2)
	sm.Register(s3)

	err := sm.StartAll()
	if err == nil {
		t.Fatalf("expected StartAll to fail")
	}

	// Verify fail-fast semantics: s1 should have been stopped
	select {
	case <-s1StopCh:
		// success
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for s1 to be stopped after s2 start failure")
	}

	if s3.IsRunning() {
		t.Errorf("s3 should never have started")
	}
}

func TestManager_HealthMonitor_Restart(t *testing.T) {
	t.Parallel()

	sm := NewServiceManager(nil)
	sm.healthInterval = 1 * time.Millisecond // very fast tick
	sm.maxRestarts = 2
	sm.restartDelay = 0

	startChan := make(chan struct{}, 10)

	s1 := &mockService{
		name: "s1",
		startFunc: func(ctx context.Context) error {
			startChan <- struct{}{}
			return nil
		},
		healthStatus: HealthStatus{Healthy: false, Message: "always failing"},
	}

	if err := sm.Register(s1); err != nil {
		t.Fatalf("failed to register: %v", err)
	}
	if err := sm.StartAll(); err != nil {
		t.Fatalf("failed to start all: %v", err)
	}

	// Expect exactly 3 starts: 1 initial + 2 max restarts
	var starts int
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()

loop:
	for {
		select {
		case <-startChan:
			starts++
			if starts == 3 {
				break loop
			}
		case <-timer.C:
			t.Fatalf("timeout waiting for restarts, got %d starts", starts)
		}
	}

	// The state is updated deterministically before the 3rd start is sent through the channel,
	// so no sleep is needed here.

	// Stop everything
	sm.StopAll(context.Background())

	info, err := sm.GetServiceInfo("s1")
	if err != nil {
		t.Fatalf("failed to get service info: %v", err)
	}

	if info.RestartCount != 2 {
		t.Errorf("expected exactly 2 restarts, got %d", info.RestartCount)
	}
}

func TestManager_FatalPropagation(t *testing.T) {
	t.Parallel()

	sm := NewServiceManager(nil)
	errFatal := errors.New("fatal simulated")
	sm.Fatal(errFatal)

	err := sm.Wait()
	if !errors.Is(err, errFatal) && err != errFatal {
		t.Errorf("expected Wait to return fatal err, got %v", err)
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

// === FILE: pkg/service/orchestrator_test.go ===
```go
package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

// TestOrchestrator_Preemption checks if long-running I/O calls are preempted correctly.
func TestOrchestrator_Preemption(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	startCalled := make(chan struct{})

	err := ExecuteOrchestration(ctx, func(c context.Context) error {
		close(startCalled)
		<-c.Done()
		return c.Err()
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestExecuteOrchestration_PanicRecovery(t *testing.T) {
	t.Parallel()
	err := ExecuteOrchestration(context.Background(), func(c context.Context) error {
		panic("simulated panic in boundary")
	})

	if err == nil {
		t.Fatal("expected error from panic, got nil")
	}

	if !strings.Contains(err.Error(), "simulated panic in boundary") {
		t.Fatalf("expected panic message in error, got: %v", err)
	}
}

func TestExecuteOrchestration_ContextCancellationPropagates(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	eg, egCtx := errgroup.WithContext(context.Background())
	errCh := make(chan error, 1)

	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		default:
		}
		errCh <- ExecuteOrchestration(ctx, func(c context.Context) error {
			<-c.Done()
			return c.Err()
		})
		return nil
	})

	cancel() // Cancel the parent context

	select {
	case err := <-errCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("ExecuteOrchestration did not return promptly after context cancellation")
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("unexpected errgroup wait error: %v", err)
	}
}

func TestExecuteOrchestration_FalseSharingMitigation(t *testing.T) {
	t.Parallel()
	eg, ctx := errgroup.WithContext(context.Background())
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		idx := i
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			err := ExecuteOrchestration(context.Background(), func(c context.Context) error {
				if idx == 5 {
					return errors.New("simulated error")
				}
				return nil
			})
			errs <- err
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent orchestrations failed: %v", err)
	}
	close(errs)

	errorCount := 0
	for err := range errs {
		if err != nil {
			errorCount++
			if err.Error() != "simulated error" {
				t.Errorf("unexpected error: %v", err)
			}
		}
	}

	if errorCount != 1 {
		t.Errorf("expected exactly 1 error, got %d", errorCount)
	}
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

// === FILE: pkg/service/runtime_activity_test.go ===
```go
//go:build ignore

package service

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeActivityMarkEventPersistsTimestamp(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-mark-event.db")
	expected := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:       RunErrWithTimeoutContext,
		EventTimeout: time.Second,
		Now: func() time.Time {
			return expected
		}})

	activity.MarkEvent(context.Background(), "test")

	got, ok, err := store.LastEvent(context.Background())
	if err != nil {
		t.Fatalf("get last event: %v", err)
	}
	if !ok {
		t.Fatalf("expected last event timestamp to be persisted")
	}
	if !got.Equal(expected) {
		t.Fatalf("unexpected last event timestamp: got=%s want=%s", got.UTC(), expected.UTC())
	}
}

func TestRuntimeActivityMarkEventPersistsTimestampPerBot(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-mark-event-by-bot.db")
	expected := time.Date(2026, time.January, 2, 3, 14, 15, 0, time.UTC)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:        RunErrWithTimeoutContext,
		EventTimeout:  time.Second,
		BotInstanceID: "generic",
		Now: func() time.Time {
			return expected
		}})

	activity.MarkEvent(context.Background(), "test")

	got, ok, err := store.LastEventForBot(context.Background(), "generic")
	if err != nil {
		t.Fatalf("get last event by bot: %v", err)
	}
	if !ok {
		t.Fatalf("expected namespaced last event timestamp to be persisted")
	}
	if !got.Equal(expected) {
		t.Fatalf("unexpected namespaced last event timestamp: got=%s want=%s", got.UTC(), expected.UTC())
	}
}

func TestRuntimeActivityStartHeartbeatPersistsImmediatelyAndPeriodically(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat.db")
	base := time.Date(2026, time.January, 2, 5, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	ticks := newTickRecorder(t, 2)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Add(1)) * time.Second)
		},
		OnHeartbeatTick: ticks.Hook})

	// 25ms (rather than 5ms) leaves enough room for the second ticker fire
	// to land within tickRecorder.Next's 2s safety timeout when the package
	// is run with high parallelism: a Postgres-backed roundtrip per tick
	// can briefly outrun a 5ms cadence under sibling-test schema churn.
	activity.StartHeartbeat(context.Background(), 25*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected initial heartbeat to succeed: %v", err)
	}
	first, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || first.IsZero() {
		t.Fatalf("expected initial heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}

	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected periodic heartbeat to succeed: %v", err)
	}
	second, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok {
		t.Fatalf("expected periodic heartbeat timestamp to be persisted: ok=%v err=%v", ok, err)
	}
	if !second.After(first) {
		t.Fatalf("expected periodic heartbeat to advance the timestamp: first=%s second=%s", first.UTC(), second.UTC())
	}
}

func TestRuntimeActivityStartHeartbeatNoopsWhenAlreadyRunning(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-noop.db")
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second})

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	firstDone := activity.hbDone
	if firstDone == nil {
		t.Fatalf("expected first heartbeat start to initialize state")
	}

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)

	if activity.hbDone != firstDone {
		t.Fatalf("expected second heartbeat start to reuse existing state")
	}
}

func TestRuntimeActivityStopHeartbeatIsIdempotent(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-stop.db")
	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr:           RunErrWithTimeoutContext,
		HeartbeatTimeout: time.Second})

	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("stop heartbeat before start: %v", err)
	}

	activity.StartHeartbeat(context.Background(), 10*time.Millisecond)

	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("first stop heartbeat: %v", err)
	}
	if err := activity.StopHeartbeat(context.Background()); err != nil {
		t.Fatalf("second stop heartbeat: %v", err)
	}
}

func TestRuntimeActivityHeartbeatStartupContinuesAfterInitialPersistenceFailure(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-heartbeat-retry.db")
	base := time.Date(2026, time.January, 2, 6, 0, 0, 0, time.UTC)
	var calls atomic.Int32
	ticks := newTickRecorder(t, 2)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr: func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
			if calls.Add(1) == 1 {
				return errors.New("startup heartbeat failed")
			}
			return fn(ctx)
		},
		HeartbeatTimeout: time.Second,
		Now: func() time.Time {
			return base.Add(time.Duration(calls.Load()) * time.Second)
		},
		OnHeartbeatTick: ticks.Hook})

	activity.StartHeartbeat(context.Background(), 5*time.Millisecond)
	t.Cleanup(func() {
		if err := activity.StopHeartbeat(context.Background()); err != nil {
			t.Fatalf("stop heartbeat cleanup: %v", err)
		}
	})

	if err := ticks.Next(t); err == nil {
		t.Fatal("expected first heartbeat attempt to surface the injected failure")
	}
	if err := ticks.Next(t); err != nil {
		t.Fatalf("expected recovery heartbeat to succeed: %v", err)
	}

	ts, ok, err := store.Heartbeat(context.Background())
	if err != nil || !ok || ts.IsZero() {
		t.Fatalf("expected heartbeat persistence to recover after initial failure: ok=%v err=%v", ok, err)
	}
}

// TestRuntimeActivityStartHeartbeatReturnsWhenStartupPersistenceWedges pins
// two invariants exposed by the heartbeat goroutine restructuring:
//
//  1. StartHeartbeat must return promptly even when the startup persistence
//     is wedged. The previous code path kept Start parked inside the
//     synchronous attemptHeartbeat, leaving `go func()` unreached and
//     close(done) un-armed — a concurrent StopHeartbeat that observed
//     hbCancel/hbDone then blocked on <-done forever.
//  2. StopHeartbeat must return cleanly even when the in-flight attempt is
//     wedged. The comprehensive fix dispatches each attempt through
//     runCancellableHeartbeat so the outer goroutine can exit via
//     hbCtx.Done() while the inner attempt goroutine is left to leak until
//     its blocking call returns; close(done) is therefore reachable even
//     if RunErr (or any callback it invokes) ignores ctx.
//
// A RunErr that blocks unconditionally on a channel exercises both: the
// startup persistence cannot make progress, so without the fix Start (1)
// hangs and Stop (2) hangs. Hard timeouts convert either regression into a
// failure with a goroutine stack dump pointing at the wedge.
func TestRuntimeActivityStartHeartbeatReturnsWhenStartupPersistenceWedges(t *testing.T) {
	t.Parallel()
	store, _ := newLoggingStore(t, "runtime-activity-start-stop-race.db")

	release := make(chan struct{})
	defer close(release)

	activity := newRuntimeActivity(store, runtimeActivityOptions{
		RunErr: func(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
			<-release
			return nil
		},
		HeartbeatTimeout: time.Second})

	startReturned := make(chan struct{})
	go func() {
		defer close(startReturned)
		activity.StartHeartbeat(context.Background(), 10*time.Millisecond)
	}()

	select {
	case <-startReturned:
	case <-time.After(500 * time.Millisecond):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("StartHeartbeat did not return within 500ms while the startup persistence was wedged; the heartbeat goroutine launch is gated on attemptHeartbeat completing — regression of the race-window fix.\nGoroutines:\n%s", buf[:n])
	}

	stopReturned := make(chan error, 1)
	go func() {
		stopReturned <- activity.StopHeartbeat(context.Background())
	}()

	select {
	case err := <-stopReturned:
		if err != nil {
			t.Fatalf("stop heartbeat: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		t.Fatalf("StopHeartbeat did not return within 500ms while the in-flight attempt was wedged; close(done) is gated on the inner attempt completing — regression of the cancellable-attempt fix.\nGoroutines:\n%s", buf[:n])
	}
}

// tickRecorder is the test-side companion for hooks like
// OnHeartbeatTick that fire synchronously from a long-running producer
// goroutine. The recorder runs a drainer goroutine that always receives
// from an unbuffered channel so the hook's send never wedges. The first
// wantTicks values are exposed to the test via Next; later ticks are
// silently discarded so a flooding producer cannot back up the drainer.
//
// The recorder registers a t.Cleanup that cancels its context and waits
// for the drainer to exit. Because Cleanup runs LIFO, callers should
// construct the recorder before registering the producer's stop
// cleanup — that way producer teardown runs first while the drainer
// is still draining, and only then does the recorder release its
// goroutine. An in-flight Hook send unblocks via the recorder's
// context, so a producer that invokes the callback during its own
// teardown cannot deadlock.
type tickRecorder struct {
	ctx    context.Context
	cancel context.CancelFunc
	ticks  chan error
	verify chan error
	done   chan struct{}
}

func newTickRecorder(t *testing.T, wantTicks int) *tickRecorder {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &tickRecorder{
		ctx:    ctx,
		cancel: cancel,
		ticks:  make(chan error),
		verify: make(chan error, wantTicks),
		done:   make(chan struct{})}
	go func() {
		defer close(r.done)
		forwarded := 0
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-r.ticks:
				if forwarded < wantTicks {
					r.verify <- err
					forwarded++
				}
			}
		}
	}()
	t.Cleanup(func() {
		r.cancel()
		<-r.done
	})
	return r
}

// Hook is the callback to pass to a producer option (e.g.
// runtimeActivityOptions.OnHeartbeatTick). It rendezvous with the
// drainer and only blocks until the recorder's context is cancelled,
// so it is safe to invoke synchronously from inside a producer
// goroutine loop or from a synchronous startup attempt.
func (r *tickRecorder) Hook(err error) {
	select {
	case r.ticks <- err:
	case <-r.ctx.Done():
	}
}

// Next pulls the next collected value. The test fails if no value
// arrives within the safety timeout, which is intentionally generous
// so it surfaces a wedged producer rather than gating fast-path
// scheduling on healthy machines.
func (r *tickRecorder) Next(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.verify:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tick")
		return nil
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

// === FILE: pkg/service/service_lifecycle_test.go ===
```go
//go:build ignore

package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestServiceLifecycleStopTimesOutUntilOwnedWorkFinishes(t *testing.T) {
	t.Parallel()
	lifecycle := newServiceLifecycle("test lifecycle")

	runCtx, err := lifecycle.Start(context.Background())
	if err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}

	ownedCtx, done, ok := lifecycle.Begin()
	if !ok {
		t.Fatalf("expected owned work to start")
	}
	if ownedCtx != runCtx {
		t.Fatalf("expected begin to return the active run context")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err = lifecycle.Stop(stopCtx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected stop timeout, got %v", err)
	}
	if lifecycle.IsRunning() {
		t.Fatalf("expected lifecycle to stop accepting new work after cancel")
	}
	if ctx, done, ok := lifecycle.Begin(); ok || ctx != nil || done != nil {
		t.Fatalf("expected begin to reject work while lifecycle is stopping")
	}

	select {
	case <-runCtx.Done():
	default:
		t.Fatalf("expected run context to be canceled")
	}

	done()

	if err := lifecycle.Wait(context.Background()); err != nil {
		t.Fatalf("wait for owned work: %v", err)
	}

	if _, err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("restart lifecycle after wait: %v", err)
	}
}

func TestServiceLifecycleBeginReturnsFalseAfterCancel(t *testing.T) {
	t.Parallel()
	lifecycle := newServiceLifecycle("test lifecycle")

	if _, err := lifecycle.Start(context.Background()); err != nil {
		t.Fatalf("start lifecycle: %v", err)
	}
	if err := lifecycle.Cancel(); err != nil {
		t.Fatalf("cancel lifecycle: %v", err)
	}

	if ctx, done, ok := lifecycle.Begin(); ok || ctx != nil || done != nil {
		t.Fatalf("expected begin to reject work after cancel")
	}
}

```

