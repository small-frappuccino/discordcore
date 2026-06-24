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
