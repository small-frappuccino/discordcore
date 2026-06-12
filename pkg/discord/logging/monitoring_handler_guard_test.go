package logging

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordgo"
)

func TestGuardedHandlerIsolation(t *testing.T) {
	t.Run("yields false state", func(t *testing.T) {
		sl := newServiceLifecycle("test-false")

		var executions int32
		handlerFunc := func(ctx context.Context, s *discordgo.Session, e *discordgo.MessageCreate) {
			atomic.AddInt32(&executions, 1)
		}

		wrapped := guardedHandler(&sl, handlerFunc)

		wrapped(&discordgo.Session{}, &discordgo.MessageCreate{})

		if got := atomic.LoadInt32(&executions); got != 0 {
			t.Fatalf("expected 0 executions when Begin yields false, got %d", got)
		}
	})

	t.Run("yields true state", func(t *testing.T) {
		sl := newServiceLifecycle("test-true")
		_, err := sl.Start(context.Background())
		if err != nil {
			t.Fatalf("start failed: %v", err)
		}

		var executions int32
		handlerFunc := func(ctx context.Context, s *discordgo.Session, e *discordgo.MessageCreate) {
			atomic.AddInt32(&executions, 1)
		}

		wrapped := guardedHandler(&sl, handlerFunc)

		wrapped(&discordgo.Session{}, &discordgo.MessageCreate{})

		if got := atomic.LoadInt32(&executions); got != 1 {
			t.Fatalf("expected 1 execution when Begin yields true, got %d", got)
		}

		sl.Cancel()

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- sl.Wait(context.Background())
		}()

		select {
		case err := <-waitCh:
			if err != nil {
				t.Fatalf("unexpected Wait error: %v", err)
			}
		case <-time.After(1 * time.Second):
			t.Fatalf("Wait timed out, expected wg token to be decremented")
		}
	})
}
