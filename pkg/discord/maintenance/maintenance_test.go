package maintenance

import (
	"context"
	"runtime"
	"testing"
	"time"
)

func TestCalculateJitter_Variance(t *testing.T) {
	base := 10 * time.Minute
	var maxVal, minVal time.Duration = 0, 100 * time.Hour

	for i := 0; i < 10000; i++ {
		j := calculateJitter(base)
		if j > maxVal {
			maxVal = j
		}
		if j < minVal {
			minVal = j
		}
	}

	expectedMin := time.Duration(float64(base) * 1.1)
	expectedMax := time.Duration(float64(base) * 1.2)

	if minVal < expectedMin || minVal > time.Duration(float64(base)*1.11) {
		t.Errorf("min variance is out of bounds: %v (expected ~%v)", minVal, expectedMin)
	}
	if maxVal > expectedMax || maxVal < time.Duration(float64(base)*1.19) {
		t.Errorf("max variance is out of bounds: %v (expected ~%v)", maxVal, expectedMax)
	}
}

func TestUserPruneService_ShutdownPreemptionAndLeaks(t *testing.T) {
	runtime.Gosched()
	baseline := runtime.NumGoroutine()

	s := NewUserPruneService(nil, nil, nil, "")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if err := s.Stop(ctx); err != nil {
		t.Fatalf("Failed to stop service: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	current := runtime.NumGoroutine()
	// Allow minor fluctuation in baseline goroutines from test framework
	if current > baseline+2 {
		t.Errorf("Goroutine leak detected: baseline=%d, current=%d", baseline, current)
	}
}
