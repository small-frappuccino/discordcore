package clock

import (
	"sync"
	"time"
)

// MockClock is a Clock implementation that allows manual time manipulation for deterministic testing.
type MockClock struct {
	mu  sync.RWMutex
	now time.Time
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
}

// Advance moves the mocked time forward by the given duration.
func (m *MockClock) Advance(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
}

// NewTimer creates a new Timer. For MockClock, we just return a RealTimer for now
// to preserve previous behavior where components used OS timers directly.
func (m *MockClock) NewTimer(d time.Duration) Timer {
	return &RealTimer{t: time.NewTimer(d)}
}
