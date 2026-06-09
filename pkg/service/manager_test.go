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
			sm := NewServiceManager()
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

	sm := NewServiceManager()

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

	_ = sm.Register(s1)
	_ = sm.Register(s2)
	_ = sm.Register(s3)

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

	sm := NewServiceManager()
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

	// Give a small buffer to let the manager record the last restart internal state
	time.Sleep(10 * time.Millisecond)

	// Stop everything
	_ = sm.StopAll()

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

	sm := NewServiceManager()
	errFatal := errors.New("fatal simulated")
	sm.Fatal(errFatal)

	err := sm.Wait()
	if !errors.Is(err, errFatal) && err != errFatal {
		t.Errorf("expected Wait to return fatal err, got %v", err)
	}
}
