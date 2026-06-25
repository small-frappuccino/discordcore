# Domain Architecture: clock

## Layout Topology
```text
clock/
├── clock.go
├── http_clock.go
├── http_clock_test.go
├── mock.go
└── mock_test.go
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

// === FILE: pkg/clock/http_clock_test.go ===
```go
package clock_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/stretchr/testify/require"
)

func TestHTTPClock_Success(t *testing.T) {
	t.Parallel()
	futureTime := time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", futureTime.Format(time.RFC1123))
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := clock.NewHTTPClock(ts.URL)
	now := c.Now()

	// 'now' should be ~5 minutes in the future compared to machine time.
	diff := now.Sub(time.Now())
	require.True(t, diff > 4*time.Minute && diff < 6*time.Minute)
}

func TestHTTPClock_Timeout(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // Block until client times out deterministically
		// w.WriteHeader won't be sent since the client has already timed out
	}))
	defer ts.Close()

	start := time.Now()
	c := clock.NewHTTPClock(ts.URL)
	duration := time.Since(start)

	// Context timeout is 5 seconds. So it should take around 5s.
	require.True(t, duration >= 5*time.Second && duration < 6*time.Second, "Initialization should gracefully fail in ~5s")

	// Should fallback to RealClock (0 offset)
	diff := c.Now().Sub(time.Now())
	require.True(t, diff > -1*time.Second && diff < 1*time.Second)
}

func TestHTTPClock_MalformedHeader(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", "Invalid Date String")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := clock.NewHTTPClock(ts.URL)
	diff := c.Now().Sub(time.Now())
	// Should fallback to RealClock (0 offset)
	require.True(t, diff > -1*time.Second && diff < 1*time.Second)
}

func TestHTTPClock_MissingHeader(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := clock.NewHTTPClock(ts.URL)
	diff := c.Now().Sub(time.Now())
	require.True(t, diff > -1*time.Second && diff < 1*time.Second)
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

// === FILE: pkg/clock/mock_test.go ===
```go
package clock_test

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestMockClock_Concurrency(t *testing.T) {
	t.Parallel()
	c := clock.NewMockClock(time.Now())
	eg, _ := errgroup.WithContext(context.Background())

	numGoroutines := 200

	for i := 0; i < numGoroutines; i++ {
		i := i
		eg.Go(func() error {
			if i%2 == 0 {
				for j := 0; j < 100; j++ {
					_ = c.Now()
				}
			} else {
				for j := 0; j < 100; j++ {
					if j%2 == 0 {
						c.Advance(time.Millisecond)
					} else {
						c.SetTime(time.Now().Add(time.Duration(j) * time.Second))
					}
				}
			}
			return nil
		})
	}

	_ = eg.Wait()
}

func TestMockClock_TimersAndTickers(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := clock.NewMockClock(start)

	timer := c.NewTimer(5 * time.Second)
	ticker := c.NewTicker(2 * time.Second)

	// Advance by 3 seconds. Timer shouldn't fire, ticker should fire once.
	c.Advance(3 * time.Second)

	select {
	case <-timer.C():
		t.Fatal("Timer fired too early")
	default:
	}

	select {
	case tickTime := <-ticker.C():
		require.Equal(t, start.Add(2*time.Second), tickTime)
	default:
		t.Fatal("Ticker should have fired")
	}

	// Advance past timer.
	c.Advance(3 * time.Second)

	select {
	case timerTime := <-timer.C():
		require.Equal(t, start.Add(6*time.Second), timerTime)
	default:
		t.Fatal("Timer should have fired")
	}

	// Ticker should fire again at +4s and +6s (we advanced to +6s).
	select {
	case tickTime := <-ticker.C():
		require.True(t, tickTime.Equal(start.Add(4*time.Second)) || tickTime.Equal(start.Add(6*time.Second)))
	default:
		t.Fatal("Ticker should have fired")
	}

	timer.Stop()
	ticker.Stop()
}

func TestMockClock_NonBlockingDispatch(t *testing.T) {
	t.Parallel()
	// Verify that if a channel isn't read, Advance still proceeds.
	c := clock.NewMockClock(time.Now())
	timer := c.NewTimer(1 * time.Second)
	c.Advance(2 * time.Second)
	c.Advance(2 * time.Second) // Second advance should not block.
	timer.Stop()
}

```

