package task_test

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

type mockTimer struct {
	c chan time.Time
}

func (m *mockTimer) C() <-chan time.Time        { return m.c }
func (m *mockTimer) Stop() bool                 { return true }
func (m *mockTimer) Reset(d time.Duration) bool { return true }

type mockClock struct {
	timer *mockTimer
}

func (m *mockClock) Now() time.Time { return time.Now() }
func (m *mockClock) NewTimer(d time.Duration) clock.Timer {
	return m.timer
}

func TestTaskRouter_MockTimerInjection(t *testing.T) {
	mTimer := &mockTimer{c: make(chan time.Time)}
	mClock := &mockClock{timer: mTimer}

	cfg := task.RouterConfig{
		Clock:           mClock,
		CleanupInterval: 100 * time.Hour, // Unreachable normally
	}
	router := task.NewRouter(cfg)

	// Router starts automatically in NewRouter
	time.Sleep(50 * time.Millisecond)

	// Send tick simulating time passage
	select {
	case mTimer.c <- time.Now():
		// Success: background loop processed the mock timer tick
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to inject tick into mock timer; router blocked or ignoring clock interface")
	}

	time.Sleep(50 * time.Millisecond)

	router.Close()
}
