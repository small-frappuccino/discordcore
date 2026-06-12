package logging

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

func TestStartSubServicesRollbackTopology(t *testing.T) {
	var s0Started, s0Stopped int32
	var s1Started, s1Stopped int32
	var s2Started, s2Stopped int32

	entries := []subServiceEntry{
		{
			name:        "s0",
			shouldStart: true,
			start: func() error {
				atomic.AddInt32(&s0Started, 1)
				return nil
			},
			stop: func() error {
				atomic.AddInt32(&s0Stopped, 1)
				return nil
			},
			isRunning: func() bool { return atomic.LoadInt32(&s0Started) > atomic.LoadInt32(&s0Stopped) }},
		{
			name:        "s1",
			shouldStart: true,
			start: func() error {
				atomic.AddInt32(&s1Started, 1)
				return errors.New("synthetic fault")
			},
			stop: func() error {
				atomic.AddInt32(&s1Stopped, 1)
				return nil
			},
			isRunning: func() bool { return atomic.LoadInt32(&s1Started) > atomic.LoadInt32(&s1Stopped) }},
		{
			name:        "s2",
			shouldStart: true,
			start: func() error {
				atomic.AddInt32(&s2Started, 1)
				return nil
			},
			stop: func() error {
				atomic.AddInt32(&s2Stopped, 1)
				return nil
			},
			isRunning: func() bool { return atomic.LoadInt32(&s2Started) > atomic.LoadInt32(&s2Stopped) }}}

	err := executeStartSubServices(context.Background(), entries)
	if err == nil || !strings.Contains(err.Error(), "synthetic fault") {
		t.Fatalf("expected synthetic fault error, got %v", err)
	}

	if atomic.LoadInt32(&s0Started) != 1 {
		t.Errorf("s0 should have started")
	}
	if atomic.LoadInt32(&s0Stopped) != 1 {
		t.Errorf("s0 should have been stopped during rollback")
	}

	if atomic.LoadInt32(&s1Started) != 1 {
		t.Errorf("s1 should have attempted to start")
	}
	if atomic.LoadInt32(&s1Stopped) != 0 {
		t.Errorf("s1 should not have been stopped (start failed)")
	}

	if atomic.LoadInt32(&s2Started) != 0 {
		t.Errorf("s2 should remain strictly uninstantiated")
	}
}

func TestApplySubServiceTogglesStateTransitions(t *testing.T) {
	var s0Started, s0Stopped int32
	var s1Started, s1Stopped int32

	entries := []subServiceEntry{
		{
			name:        "s0",
			shouldStart: false, // Initially running, but now should stop
			start: func() error {
				atomic.AddInt32(&s0Started, 1)
				return nil
			},
			stop: func() error {
				atomic.AddInt32(&s0Stopped, 1)
				return nil
			},
			isRunning: func() bool { return atomic.LoadInt32(&s0Stopped) == 0 }, // Simulates it is running
		},
		{
			name:        "s1",
			shouldStart: true, // Initially stopped, now should start
			start: func() error {
				atomic.AddInt32(&s1Started, 1)
				return nil
			},
			stop: func() error {
				atomic.AddInt32(&s1Stopped, 1)
				return nil
			},
			isRunning: func() bool { return atomic.LoadInt32(&s1Started) > 0 }, // Simulates it is stopped
		}}

	errs := executeApplySubServiceToggles(context.Background(), entries)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if atomic.LoadInt32(&s0Stopped) != 1 {
		t.Errorf("s0 should have been stopped")
	}
	if atomic.LoadInt32(&s0Started) != 0 {
		t.Errorf("s0 should not have been started")
	}

	if atomic.LoadInt32(&s1Started) != 1 {
		t.Errorf("s1 should have been started")
	}
	if atomic.LoadInt32(&s1Stopped) != 0 {
		t.Errorf("s1 should not have been stopped")
	}
}

func TestStopSubServicesTeardownOrder(t *testing.T) {
	var stopOrder []string

	entries := []subServiceEntry{
		{
			name:      "member_event_service",
			isRunning: func() bool { return true },
			stop: func() error {
				stopOrder = append(stopOrder, "member_event_service")
				return nil
			}},
		{
			name:      "message_event_service",
			isRunning: func() bool { return true },
			stop: func() error {
				stopOrder = append(stopOrder, "message_event_service")
				return nil
			}},
		{
			name:      "reaction_event_service",
			isRunning: func() bool { return true },
			stop: func() error {
				stopOrder = append(stopOrder, "reaction_event_service")
				return nil
			}}}

	errs := executeStopSubServices(context.Background(), entries)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if len(stopOrder) != 3 {
		t.Fatalf("expected 3 services to stop, got %d", len(stopOrder))
	}
	if stopOrder[0] != "reaction_event_service" || stopOrder[1] != "message_event_service" || stopOrder[2] != "member_event_service" {
		t.Errorf("incorrect teardown order: %v", stopOrder)
	}
}

func TestBuildSubServiceEntriesClosures(t *testing.T) {
	ctx := context.Background()

	t.Run("nil pointers yield safe bools", func(t *testing.T) {
		ms := &MonitoringService{}

		workload := monitoringWorkloadState{
			memberEventService:   true,
			messageEventService:  true,
			reactionEventService: true}

		entries := ms.buildSubServiceEntries(ctx, workload)
		for _, entry := range entries {
			if entry.isRunning() {
				t.Errorf("expected %s isRunning to be false on nil pointer", entry.name)
			}
			if err := entry.stop(); err != nil {
				t.Errorf("expected %s stop to be safe on nil pointer, got %v", entry.name, err)
			}
		}
	})

	t.Run("instantiated struct branches", func(t *testing.T) {
		ms := &MonitoringService{
			memberEventService:   &MemberEventService{},
			messageEventService:  &MessageEventService{},
			reactionEventService: &ReactionEventService{}}

		entries := ms.buildSubServiceEntries(ctx, monitoringWorkloadState{})
		for _, entry := range entries {
			if entry.isRunning() {
				t.Errorf("expected %s isRunning to default to false on zero struct", entry.name)
			}
			// Safe stop with IsRunning() == false check in closure
			if err := entry.stop(); err != nil {
				t.Errorf("expected stop to be a no-op when not running, got err: %v", err)
			}
		}
	})
}
