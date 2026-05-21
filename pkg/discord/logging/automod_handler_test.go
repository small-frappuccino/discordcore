package logging

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

// TestAutomodHandleRawEvent_IgnoresUnrelatedTypes confirms the raw handler
// short-circuits on every non-AutoMod gateway event. The raw handler receives
// every event Discord sends, so the type filter is the first and cheapest
// guard against spending work on the wrong payload.
func TestAutomodHandleRawEvent_IgnoresUnrelatedTypes(t *testing.T) {
	t.Parallel()

	// AutomodService with no session/configManager/adapters: if the filter
	// failed to short-circuit on the wrong type, downstream nil derefs would
	// surface as a panic here.
	as := &AutomodService{}
	as.handleRawEvent(nil, &discordgo.Event{
		Type:     "MESSAGE_CREATE",
		Sequence: 42,
		Struct:   &discordgo.MessageCreate{},
	})
}

// TestAutomodHandleRawEvent_NilEnvelope guards against nil dispatch from any
// future discordgo behavior. The current dispatcher would never produce a nil
// envelope, but the guard is cheap and removes a class of panic.
func TestAutomodHandleRawEvent_NilEnvelope(t *testing.T) {
	t.Parallel()

	as := &AutomodService{}
	as.handleRawEvent(nil, nil)
}

// TestAutomodHandleRawEvent_AbortsOnEmptyGuildID confirms the raw handler
// extracts the typed struct from evt.Struct and then short-circuits inside
// handleAutoModerationAction when the payload has no guild context. With nil
// session and configManager, the only way the test does not panic is if the
// extraction path matched the typed value and the GuildID guard fired before
// any downstream call.
func TestAutomodHandleRawEvent_AbortsOnEmptyGuildID(t *testing.T) {
	t.Parallel()

	as := &AutomodService{}
	as.handleRawEvent(nil, &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 7,
		Struct:   &discordgo.AutoModerationActionExecution{},
	})
}

// TestAutomodHandleRawEvent_FallbackUnmarshalsFromRawData covers the canary
// path: if discordgo ever stops populating evt.Struct with the expected typed
// value (type-registration drift on a version bump), the handler must still
// extract the payload from evt.RawData. The empty-GuildID guard inside
// handleAutoModerationAction stops execution before downstream calls, so the
// test asserts the no-panic path with a wrong-type evt.Struct and a RawData
// payload that unmarshals to an empty-GuildID action.
func TestAutomodHandleRawEvent_FallbackUnmarshalsFromRawData(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(&discordgo.AutoModerationActionExecution{
		// GuildID intentionally empty: handleAutoModerationAction aborts at
		// the GuildID guard so the test exercises the unmarshal path without
		// touching configManager/adapters/session.
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	as := &AutomodService{}
	as.handleRawEvent(nil, &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 1234,
		RawData:  raw,
		// Struct is a wrong-type sentinel to exercise the fallback branch.
		Struct: &discordgo.MessageCreate{},
	})
}

// TestAutomodHandleRawEvent_FallbackOnInvalidRawData confirms the handler
// logs and returns rather than panicking when the fallback unmarshal itself
// fails. Future-proofs against discordgo dispatching with both a wrong-type
// Struct and a malformed RawData (extreme edge that would currently come from
// a hand-rolled mock, not real Discord traffic).
func TestAutomodHandleRawEvent_FallbackOnInvalidRawData(t *testing.T) {
	t.Parallel()

	as := &AutomodService{}
	as.handleRawEvent(nil, &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 1,
		RawData:  []byte("not valid json"),
		Struct:   &discordgo.MessageCreate{},
	})
}

// TestAutomodHandleRawEvent_DedupsSecondEventWithSameSequence is the
// integration-style canary: two raw envelopes with the same Sequence and same
// guild context must land on the same router-level idempotency key, so the
// second one is dropped before any handler runs. This is the load-bearing
// behavior the spec calls out — Discord re-deliveries during a RESUME carry
// the original sequence, and the seq-based key catches them all.
func TestAutomodHandleRawEvent_DedupsSecondEventWithSameSequence(t *testing.T) {
	t.Parallel()

	const (
		guildID   = "g-seq-dedup"
		channelID = "c-auto"
		botID     = "bot"
	)
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID:  guildID,
		Channels: files.ChannelsConfig{AutomodAction: channelID},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, channelID, botID, perms)
	session.Identify.Intents = discordgo.IntentAutoModerationExecution

	cfg := task.RouterConfig{
		DefaultMaxAttempts: 1,
		InitialBackoff:     5 * time.Millisecond,
		MaxBackoff:         5 * time.Millisecond,
		IdempotencyTTL:     500 * time.Millisecond,
		GroupBuffer:        8,
		GroupIdleTTL:       200 * time.Millisecond,
		CleanupInterval:    20 * time.Millisecond,
		GroupMaxParallel:   1,
	}
	router := task.NewRouter(cfg)
	t.Cleanup(router.Close)

	var handlerCalls int32
	router.RegisterHandler(task.TaskTypeSendAutomodAction, func(_ context.Context, _ any) error {
		atomic.AddInt32(&handlerCalls, 1)
		return nil
	})

	svc := NewAutomodService(session, cm)
	svc.SetAdapters(&task.NotificationAdapters{Router: router})

	payload := &discordgo.AutoModerationActionExecution{
		GuildID:   guildID,
		RuleID:    "r1",
		UserID:    "u1",
		ChannelID: channelID,
		MessageID: "m1",
	}
	envelope := &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 4242,
		Struct:   payload,
	}

	// First delivery enqueues; second delivery (same Sequence) must hit the
	// router's inflight map and be dropped before the handler runs.
	svc.handleRawEvent(session, envelope)
	svc.handleRawEvent(session, envelope)

	// Allow the worker goroutine to drain the first task.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&handlerCalls) >= 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&handlerCalls); got != 1 {
		t.Fatalf("expected handler to run exactly once for two same-seq events, got %d", got)
	}
}

// TestAutomodHandleRawEvent_DistinctSequencesBothRun is the inverse canary:
// two envelopes with different Sequences must both flow through. Without this
// test, a regression that hardcodes the seq path or collapses on rule/user
// would silently lose legitimate events.
func TestAutomodHandleRawEvent_DistinctSequencesBothRun(t *testing.T) {
	t.Parallel()

	const (
		guildID   = "g-seq-distinct"
		channelID = "c-auto"
		botID     = "bot"
	)
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID:  guildID,
		Channels: files.ChannelsConfig{AutomodAction: channelID},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, channelID, botID, perms)
	session.Identify.Intents = discordgo.IntentAutoModerationExecution

	cfg := task.RouterConfig{
		DefaultMaxAttempts: 1,
		InitialBackoff:     5 * time.Millisecond,
		MaxBackoff:         5 * time.Millisecond,
		IdempotencyTTL:     500 * time.Millisecond,
		GroupBuffer:        8,
		GroupIdleTTL:       200 * time.Millisecond,
		CleanupInterval:    20 * time.Millisecond,
		GroupMaxParallel:   1,
	}
	router := task.NewRouter(cfg)
	t.Cleanup(router.Close)

	var handlerCalls int32
	router.RegisterHandler(task.TaskTypeSendAutomodAction, func(_ context.Context, _ any) error {
		atomic.AddInt32(&handlerCalls, 1)
		return nil
	})

	svc := NewAutomodService(session, cm)
	svc.SetAdapters(&task.NotificationAdapters{Router: router})

	payload := &discordgo.AutoModerationActionExecution{
		GuildID:   guildID,
		RuleID:    "r1",
		UserID:    "u1",
		ChannelID: channelID,
		MessageID: "m1",
	}
	svc.handleRawEvent(session, &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 100,
		Struct:   payload,
	})
	svc.handleRawEvent(session, &discordgo.Event{
		Type:     automodActionExecutionEventType,
		Sequence: 101,
		Struct:   payload,
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&handlerCalls) >= 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := atomic.LoadInt32(&handlerCalls); got != 2 {
		t.Fatalf("expected handler to run twice for distinct seqs, got %d", got)
	}
}

// TestAutomodEventTypeMatchesDiscordgo is a canary: if a future discordgo bump
// renames AUTO_MODERATION_ACTION_EXECUTION or drops the public Event/Sequence
// fields, this test fails before production traffic does. It does NOT exercise
// the gateway; it just pins the type-name constant we filter on and the shape
// of the *Event handler signature we register with AddHandler.
func TestAutomodEventTypeMatchesDiscordgo(t *testing.T) {
	t.Parallel()

	var evt discordgo.Event
	evt.Sequence = 1
	evt.Type = automodActionExecutionEventType
	if evt.Sequence != 1 {
		t.Fatal("discordgo.Event.Sequence must remain assignable as int64")
	}
	if evt.Type != "AUTO_MODERATION_ACTION_EXECUTION" {
		t.Fatalf("unexpected automod event type constant: %q", evt.Type)
	}

	// Confirm AddHandler accepts a *discordgo.Event handler shape. The
	// returned cancel func is invoked immediately to avoid leaking handlers
	// into other tests.
	s, err := discordgo.New("Bot test")
	if err != nil {
		t.Fatalf("discordgo.New: %v", err)
	}
	cancel := s.AddHandler(func(_ *discordgo.Session, _ *discordgo.Event) {})
	if cancel == nil {
		t.Fatal("discordgo.AddHandler must return a non-nil cancel for *Event handlers")
	}
	cancel()
}
