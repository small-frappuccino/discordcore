package clock

import "time"

// Timer provides a mockable interface for time.Timer.
type Timer interface {
	C() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

// Clock provides a mockable interface for retrieving the current time and timers.
type Clock interface {
	Now() time.Time
	NewTimer(d time.Duration) Timer
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
