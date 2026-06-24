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
