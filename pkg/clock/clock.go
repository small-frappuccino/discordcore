package clock

import "time"

// Clock provides a mockable interface for retrieving the current time.
type Clock interface {
	Now() time.Time
}

// RealClock is the standard implementation that uses the system time.
type RealClock struct{}

// Now returns the current local time.
func (RealClock) Now() time.Time {
	return time.Now()
}
