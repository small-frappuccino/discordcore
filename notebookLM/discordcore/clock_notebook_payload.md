# Domain Architecture: clock

## Layout Topology
```text
clock/
├── clock.go
├── http_clock.go
└── mock.go
```

## Source Stream Aggregation

// === FILE: pkg/clock/clock.go ===
```go
package clock

import "time"

// Timer provides a mockable interface for time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Ticker provides a mockable interface for time.Ticker.
type Ticker interface {
	C() <-chan time.Time
	Stop()
	Reset(d time.Duration)
}

// Clock provides a mockable interface for retrieving the current time and timers.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
	NewTicker(d time.Duration) Ticker
}

// RealClock is the standard implementation that uses the system time.
type RealClock struct{}

// Now returns the current local time.
func (RealClock) Now() time.Time {
	return time.Now()
}

// RealTimer wraps the standard time.Timer.
type RealTimer struct {
	t *time.Timer
}

func (r *RealTimer) C() <-chan time.Time        { return r.t.C }
func (r *RealTimer) Stop() bool                 { return r.t.Stop() }
func (r *RealTimer) Reset(d time.Duration) bool { return r.t.Reset(d) }

// NewTimer creates a new Timer using the system time.
func (RealClock) NewTimer(d time.Duration) Timer {
	return &RealTimer{t: time.NewTimer(d)}
}

// RealTicker wraps the standard time.Ticker.
type RealTicker struct {
	t *time.Ticker
}

func (r *RealTicker) C() <-chan time.Time   { return r.t.C }
func (r *RealTicker) Stop()                 { r.t.Stop() }
func (r *RealTicker) Reset(d time.Duration) { r.t.Reset(d) }

// NewTicker creates a new Ticker using the system time.
func (RealClock) NewTicker(d time.Duration) Ticker {
	return &RealTicker{t: time.NewTicker(d)}
}

```

// === FILE: pkg/clock/http_clock.go ===
```go
package clock

import (
	"context"
	"net/http"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// HTTPClock applies an offset to the system time based on the Date header
// returned from an external HTTP server.
type HTTPClock struct {
	offset time.Duration
}

// Now returns the current time adjusted by the HTTP offset.
func (c *HTTPClock) Now() time.Time {
	return time.Now().Add(c.offset)
}

// NewTimer creates a new Timer using the system time (duration doesn't need offset).
func (c *HTTPClock) NewTimer(d time.Duration) Timer {
	return &RealTimer{t: time.NewTimer(d)}
}

// NewTicker creates a new Ticker using the system time.
func (c *HTTPClock) NewTicker(d time.Duration) Ticker {
	return &RealTicker{t: time.NewTicker(d)}
}

// NewHTTPClock performs a single HEAD request to the target URL to capture
// the server's HTTP Date header and calculate the delta between the OS clock
// and the server clock. If the request fails or times out, it falls back to
// a 0 offset (equivalent to the OS clock) and logs a warning.
func NewHTTPClock(url string) Clock {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		log.ApplicationLogger().Warn("Failed to build HTTPClock request; falling back to OS time", "url", url, "err", err)
		return &HTTPClock{}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.ApplicationLogger().Warn("HTTPClock sync failed; falling back to OS time", "url", url, "err", err)
		return &HTTPClock{}
	}
	defer resp.Body.Close()

	dateStr := resp.Header.Get("Date")
	if dateStr == "" {
		log.ApplicationLogger().Warn("HTTPClock sync failed: no Date header; falling back to OS time", "url", url)
		return &HTTPClock{}
	}

	serverTime, err := time.Parse(time.RFC1123, dateStr)
	if err != nil {
		log.ApplicationLogger().Warn("HTTPClock sync failed: unparseable Date header; falling back to OS time", "url", url, "header", dateStr, "err", err)
		return &HTTPClock{}
	}

	offset := serverTime.Sub(time.Now())
	log.ApplicationLogger().Info("HTTPClock sync completed", "url", url, "offset", offset)
	return &HTTPClock{offset: offset}
}

```

// === FILE: pkg/clock/mock.go ===
```go
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

```

