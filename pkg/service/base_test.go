package service

import (
	"context"
	stdErrors "errors"
	"testing"

	serviceerrors "github.com/small-frappuccino/discordcore/pkg/errors"
)

func TestBaseServiceStopReturnsErrorAndKeepsErrorState(t *testing.T) {
	stopErr := stdErrors.New("stop failed")
	svc := NewBaseService("test", TypeMonitoring, PriorityNormal, nil)
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

func TestServiceWrapperPassesLifecycleContext(t *testing.T) {
	startCtxKey := struct{}{}
	stopCtxKey := struct{}{}

	var startValue string
	var stopValue string

	wrapper := NewServiceWrapper(
		"wrapped",
		TypeMonitoring,
		PriorityNormal,
		nil,
		func(ctx context.Context) error {
			startValue, _ = ctx.Value(startCtxKey).(string)
			return nil
		},
		func(ctx context.Context) error {
			stopValue, _ = ctx.Value(stopCtxKey).(string)
			return nil
		},
		nil,
	)

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
	stopErr := stdErrors.New("stop failed")
	svc := NewBaseService("managed", TypeMonitoring, PriorityNormal, nil)
	svc.SetStopHook(func(context.Context) error {
		return stopErr
	})

	manager := NewServiceManager(serviceerrors.NewErrorHandler())
	if err := manager.Register(svc); err != nil {
		t.Fatalf("register service: %v", err)
	}
	if err := manager.StartService(svc.Name()); err != nil {
		t.Fatalf("start service: %v", err)
	}

	err := manager.StopService(svc.Name())
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
