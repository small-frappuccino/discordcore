package clock

import (
	"sync"
	"time"
)

// MockClock is a Clock implementation that allows manual time manipulation for deterministic testing.
type MockClock struct {
	mu      sync.RWMutex
	now     time.Time
	timers  []*mockTimer
	tickers []*mockTicker
}

// NewMockClock creates a new MockClock initialized to a specific time.
func NewMockClock(t time.Time) *MockClock {
	return &MockClock{now: t}
}

// Now returns the manually controlled current time.
func (m *MockClock) Now() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.now
}

// SetTime manually sets the mocked time.
func (m *MockClock) SetTime(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t
	m.notifyLocked()
}

// Advance moves the mocked time forward by the given duration.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
	m.notifyLocked()
}

func (m *MockClock) notifyLocked() {
	var activeTimers []*mockTimer
	for _, t := range m.timers {
		if t.stopped {
			continue
		}
		if !m.now.Before(t.deadline) {
			select {
			case t.ch <- m.now:
			default:
			}
			t.stopped = true
		} else {
			activeTimers = append(activeTimers, t)
		}
	}
	m.timers = activeTimers

	var activeTickers []*mockTicker
	for _, t := range m.tickers {
		if t.stopped {
			continue
		}
		for !m.now.Before(t.nextTick) {
			select {
			case t.ch <- t.nextTick:
			default:
			}
			t.nextTick = t.nextTick.Add(t.d)
		}
		activeTickers = append(activeTickers, t)
	}
	m.tickers = activeTickers
}

func (m *MockClock) NewTimer(d time.Duration) Timer {
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &mockTimer{
		ch:       make(chan time.Time, 1),
		deadline: m.now.Add(d),
		stopped:  false,
		clock:    m,
	}
	m.timers = append(m.timers, t)
	return t
}

func (m *MockClock) NewTicker(d time.Duration) Ticker {
	if d <= 0 {
		panic("non-positive interval for NewTicker")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	t := &mockTicker{
		ch:       make(chan time.Time, 1),
		d:        d,
		nextTick: m.now.Add(d),
		stopped:  false,
		clock:    m,
	}
	m.tickers = append(m.tickers, t)
	return t
}

type mockTimer struct {
	mu       sync.Mutex
	ch       chan time.Time
	deadline time.Time
	stopped  bool
	clock    *MockClock
}

func (t *mockTimer) C() <-chan time.Time {
	return t.ch
}

func (t *mockTimer) Stop() bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := !t.stopped
	t.stopped = true
	return wasActive
}

func (t *mockTimer) Reset(d time.Duration) bool {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	wasActive := !t.stopped
	t.deadline = t.clock.now.Add(d)
	t.stopped = false
	if !wasActive {
		t.clock.timers = append(t.clock.timers, t)
	}
	return wasActive
}

type mockTicker struct {
	mu       sync.Mutex
	ch       chan time.Time
	d        time.Duration
	nextTick time.Time
	stopped  bool
	clock    *MockClock
}

func (t *mockTicker) C() <-chan time.Time {
	return t.ch
}

func (t *mockTicker) Stop() {
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopped = true
}

func (t *mockTicker) Reset(d time.Duration) {
	if d <= 0 {
		panic("non-positive interval for Reset")
	}
	t.clock.mu.Lock()
	defer t.clock.mu.Unlock()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.d = d
	t.nextTick = t.clock.now.Add(d)
	wasStopped := t.stopped
	t.stopped = false
	if wasStopped {
		t.clock.tickers = append(t.clock.tickers, t)
	}
}
