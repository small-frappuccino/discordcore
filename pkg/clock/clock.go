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
