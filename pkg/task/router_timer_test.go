package task_test

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

type mockTicker struct {
	c chan time.Time
}

func (m *mockTicker) C() <-chan time.Time   { return m.c }
func (m *mockTicker) Stop()                 {}
func (m *mockTicker) Reset(d time.Duration) {}

type mockClock struct {
	ticker *mockTicker
}

func (m *mockClock) Now() time.Time { return time.Now() }
func (m *mockClock) NewTimer(d time.Duration) clock.Timer {
	return nil
}
func (m *mockClock) NewTicker(d time.Duration) clock.Ticker {
	return m.ticker
}

func TestTaskRouter_MockTickerInjection(t *testing.T) {
	mTicker := &mockTicker{c: make(chan time.Time)}
	mClock := &mockClock{ticker: mTicker}

	cfg := task.RouterConfig{
		Clock:           mClock,
		CleanupInterval: 100 * time.Hour, // Unreachable normally
	}
	router := task.NewRouter(cfg)

	// Router starts automatically in NewRouter
	time.Sleep(50 * time.Millisecond)

	// Send tick simulating time passage
	select {
	case mTicker.c <- time.Now():
		// Success: background loop processed the mock ticker tick
	case <-time.After(1 * time.Second):
		t.Fatal("Failed to inject tick into mock ticker; router blocked or ignoring clock interface")
	}

	time.Sleep(50 * time.Millisecond)

	router.Close()
}
