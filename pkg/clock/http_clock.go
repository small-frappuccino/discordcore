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
