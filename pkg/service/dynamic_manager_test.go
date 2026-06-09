package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDynamicManager(t *testing.T) {
	t.Parallel()
	dm := NewManager()
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
		time.Sleep(5 * time.Millisecond)
	}

	if err := dm.StopAndRemove(context.Background(), "dyn"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}

	dm.ForceRemove("dyn")

	sm := NewServiceManager()
	s1 := &mockService{name: "s1", startFunc: func(ctx context.Context) error { return nil }}
	_ = sm.Register(s1)
	_ = sm.StartAll()

	all := sm.GetAllServices()
	if len(all) != 1 {
		t.Errorf("expected 1 service")
	}
	running := sm.GetRunningServices()
	if len(running) != 1 {
		t.Errorf("expected 1 running service")
	}
	_ = sm.StopAll()
}

func TestBaseServiceAccessors(t *testing.T) {
	t.Parallel()
	bs := NewBaseService("base", TypeMonitoring, PriorityNormal, nil)
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
	_ = bs.Start(context.Background())
	_ = bs.Stop(context.Background())
}

func TestManagedService(t *testing.T) {
	t.Parallel()
	sm := NewServiceManager()
	ms := NewManagedService("managed", TypeMonitoring, PriorityNormal, nil, sm)
	ms.SetAutoRestart(true, 1, time.Millisecond)

	// Simulate start so isRunning is true (to allow Stop to work)
	_ = ms.Start(context.Background())

	ms.HandleError(errors.New("simulated error"))

	// wait for async restart attempt
	time.Sleep(10 * time.Millisecond)

	if ms.Stats().ErrorCount != 1 {
		t.Errorf("expected 1 error")
	}
}
