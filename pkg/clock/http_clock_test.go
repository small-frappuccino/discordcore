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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(6 * time.Second)
		w.WriteHeader(http.StatusOK)
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
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	c := clock.NewHTTPClock(ts.URL)
	diff := c.Now().Sub(time.Now())
	require.True(t, diff > -1*time.Second && diff < 1*time.Second)
}
