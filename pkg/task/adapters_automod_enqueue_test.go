package task

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordgo"
)

// TestEnqueueAutomodActionWithKey_DedupOnExplicitKey confirms two enqueues
// with the same explicit idempotency key collide on the router-level
// inflight map. This is the path the AutomodService takes once it has
// computed the per-violation key via AutomodIdempotencyKey: distinct
// per-action gateway events for one violation collapse to one key, so the
// router dedups the second arrival before any handler runs.
func TestEnqueueAutomodActionWithKey_DedupOnExplicitKey(t *testing.T) {
	t.Parallel()

	cfg := RouterConfig{
		DefaultMaxAttempts: 1,
		InitialBackoff:     5 * time.Millisecond,
		MaxBackoff:         5 * time.Millisecond,
		IdempotencyTTL:     500 * time.Millisecond,
		GroupBuffer:        8,
		GroupIdleTTL:       200 * time.Millisecond,
		CleanupInterval:    20 * time.Millisecond,
		GroupMaxParallel:   1,
	}
	router := NewRouter(cfg)
	t.Cleanup(router.Close)

	var calls int32
	router.RegisterHandler(TaskTypeSendAutomodAction, func(_ context.Context, _ any) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	adapters := &NotificationAdapters{Router: router}

	event := &discordgo.AutoModerationActionExecution{
		GuildID:   "g1",
		RuleID:    "r1",
		UserID:    "u1",
		MessageID: "m1",
	}
	key := AutomodIdempotencyKey(event)

	if err := adapters.EnqueueAutomodActionWithKey("c-log", event, key); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if err := adapters.EnqueueAutomodActionWithKey("c-log", event, key); !errors.Is(err, ErrDuplicateTask) {
		t.Fatalf("expected duplicate on second enqueue with same key, got %v", err)
	}
}

// TestEnqueueAutomodAction_DefaultKeyUsesMsgPrecedence ensures the wrapper
// produces the expected per-violation key shape so callers without a
// pre-computed key still hit the same dedup as the explicit path.
func TestEnqueueAutomodAction_DefaultKeyUsesMsgPrecedence(t *testing.T) {
	t.Parallel()

	event := &discordgo.AutoModerationActionExecution{
		GuildID:   "g1",
		RuleID:    "r1",
		UserID:    "u1",
		MessageID: "m1",
	}
	got := AutomodIdempotencyKey(event)
	if got != "automod:g1:r1:u1:msg:m1" {
		t.Fatalf("default key must use msg precedence, got %q", got)
	}
}
