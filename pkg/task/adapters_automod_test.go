package task

import (
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/automod"
)

func TestAutomodIdempotencyKey_UsesMessageIDWhenAvailable(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&automod.ActionExecution{
		GuildID:   "g1",
		RuleID:    "r1",
		UserID:    "u1",
		MessageID: "m1",
	})
	if key != "automod:g1:r1:u1:msg:m1" {
		t.Fatalf("unexpected key: %q", key)
	}
}

// TestAutomodIdempotencyKey_AlertOnlyEventFallsToTBucket pins the
// removal of the per-action AlertSystemMessageID fallback. Earlier
// versions keyed such events on alert:<id>, which split a single
// violation across its block + alert action stream. The key now falls
// through to the (guild, rule, user) tbucket so the alert event coalesces
// with its sibling under the same tbucket window.
func TestAutomodIdempotencyKey_AlertOnlyEventFallsToTBucket(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := automodIdempotencyKeyAt(&automod.ActionExecution{
		GuildID:              "g1",
		RuleID:               "r1",
		UserID:               "u1",
		AlertSystemMessageID: "a1",
	}, now)
	if !strings.HasPrefix(key, "automod:g1:r1:u1:tbucket:") {
		t.Fatalf("expected tbucket fallback for alert-only event, got %q", key)
	}
}

// TestAutomodIdempotencyKey_FallsBackToTBucketWhenNoStableID confirms
// the defensive fallback always returns a non-empty key so router-level
// dedup stays active even when an event carries no per-violation payload
// identifier (a hypothetical future trigger shape).
func TestAutomodIdempotencyKey_FallsBackToTBucketWhenNoStableID(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := automodIdempotencyKeyAt(&automod.ActionExecution{
		GuildID: "g1",
		RuleID:  "r1",
		UserID:  "u1",
	}, now)
	if !strings.HasPrefix(key, "automod:g1:r1:u1:tbucket:") {
		t.Fatalf("expected tbucket fallback key, got %q", key)
	}
}

func TestAutomodIdempotencyKey_HashesMatchedContentWhenNoMessageID(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "BigAss",
	})
	if !strings.HasPrefix(key, "automod:g1:r1:u1:content:") {
		t.Fatalf("unexpected key prefix: %q", key)
	}
}

func TestAutomodIdempotencyKey_FallsBackToMatchedKeyword(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedKeyword: "ass",
	})
	if !strings.HasPrefix(key, "automod:g1:r1:u1:keyword:") {
		t.Fatalf("expected keyword-based key, got %q", key)
	}
}

func TestAutomodIdempotencyKey_MatchedContentDoesNotBucket(t *testing.T) {
	t.Parallel()

	base := &automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "BigAss",
	}
	t1 := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 19, 9, 13, 30, 0, time.UTC)
	k1 := automodIdempotencyKeyAt(base, t1)
	k2 := automodIdempotencyKeyAt(base, t2)
	if k1 != k2 || k1 == "" {
		t.Fatalf("MatchedContent must be time-independent, got %q vs %q", k1, k2)
	}
}

func TestAutomodIdempotencyKey_MatchedKeywordBucketsBySecond(t *testing.T) {
	t.Parallel()

	base := &automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedKeyword: "ass",
	}
	tA := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	tAEnd := time.Date(2026, 5, 19, 9, 13, 0, int(time.Millisecond*900), time.UTC)
	tB := time.Date(2026, 5, 19, 9, 13, 1, 0, time.UTC)

	kSameA := automodIdempotencyKeyAt(base, tA)
	kSameB := automodIdempotencyKeyAt(base, tAEnd)
	if kSameA != kSameB || kSameA == "" {
		t.Fatalf("same-second calls must collide, got %q vs %q", kSameA, kSameB)
	}

	kDiff := automodIdempotencyKeyAt(base, tB)
	if kDiff == kSameA {
		t.Fatalf("next-second call must produce a different key, both got %q", kDiff)
	}
}

func TestAutomodIdempotencyKey_DifferentContentProducesDifferentKey(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	k1 := automodIdempotencyKeyAt(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "BigAss",
	}, now)
	k2 := automodIdempotencyKeyAt(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "DonkeyKong",
	}, now)
	if k1 == k2 {
		t.Fatalf("different content must produce different keys, both got %q", k1)
	}
}

func TestAutomodIdempotencyKey_MessageIDPrecedenceOverFragment(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MessageID:      "m1",
		MatchedContent: "BigAss",
	})
	if key != "automod:g1:r1:u1:msg:m1" {
		t.Fatalf("MessageID must win over MatchedContent fallback, got %q", key)
	}
}

func TestAutomodIdempotencyKey_ContentPrecedenceOverKeyword(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 19, 9, 13, 0, 0, time.UTC)
	key := automodIdempotencyKeyAt(&automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "BigAss",
		MatchedKeyword: "ass",
	}, now)
	if !strings.HasPrefix(key, "automod:g1:r1:u1:content:") {
		t.Fatalf("expected content-based key when both fields present, got %q", key)
	}
}

// TestAutomodIdempotencyKey_TBucketCoalescesWithinWindow pins the
// defensive coalescing behavior of the tbucket fallback: two events with
// no per-violation identifier that arrive within the same
// automodCoalesceBucketSec window collapse to the same key.
func TestAutomodIdempotencyKey_TBucketCoalescesWithinWindow(t *testing.T) {
	t.Parallel()

	event := &automod.ActionExecution{
		GuildID: "g1",
		RuleID:  "r1",
		UserID:  "u1",
	}
	// Pick a base time aligned to the bucket so both samples fall inside it.
	base := time.Unix((time.Now().Unix()/automodCoalesceBucketSec)*automodCoalesceBucketSec, 0).UTC()
	k1 := automodIdempotencyKeyAt(event, base)
	k2 := automodIdempotencyKeyAt(event, base.Add(time.Duration(automodCoalesceBucketSec-1)*time.Second))
	if k1 != k2 || k1 == "" {
		t.Fatalf("tbucket must coalesce within window, got %q vs %q", k1, k2)
	}
}

// TestAutomodIdempotencyKey_TBucketDistinctAcrossWindows pins the inverse:
// the same event in two non-adjacent bucket windows produces distinct
// keys, so genuinely-separate violations on the same (guild, rule, user)
// tuple still emit independently.
func TestAutomodIdempotencyKey_TBucketDistinctAcrossWindows(t *testing.T) {
	t.Parallel()

	event := &automod.ActionExecution{
		GuildID: "g1",
		RuleID:  "r1",
		UserID:  "u1",
	}
	base := time.Unix((time.Now().Unix()/automodCoalesceBucketSec)*automodCoalesceBucketSec, 0).UTC()
	k1 := automodIdempotencyKeyAt(event, base)
	// Jump two full buckets ahead so we are unambiguously in a later window.
	k2 := automodIdempotencyKeyAt(event, base.Add(time.Duration(2*automodCoalesceBucketSec)*time.Second))
	if k1 == k2 {
		t.Fatalf("tbucket must differ across separate windows, both got %q", k1)
	}
}

// TestAutomodIdempotencyKey_CoalescesActionsForMessageViolation pins the
// 1:1-parity contract for message-triggered rules: Discord fires one
// AUTO_MODERATION_ACTION_EXECUTION per configured action (e.g.
// BLOCK_MESSAGE + SEND_ALERT_MESSAGE) on a single violation. Both events
// share MessageID; the key must be the same so the second hits dedup.
func TestAutomodIdempotencyKey_CoalescesActionsForMessageViolation(t *testing.T) {
	t.Parallel()

	block := &automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MessageID:      "m1",
		MatchedKeyword: "spam",
		ActionType:     1, // BLOCK_MESSAGE
	}
	alert := &automod.ActionExecution{
		GuildID:              "g1",
		RuleID:               "r1",
		UserID:               "u1",
		MessageID:            "m1",
		MatchedKeyword:       "spam",
		AlertSystemMessageID: "a1",
		ActionType:           2, // SEND_ALERT_MESSAGE
	}
	if k1, k2 := AutomodIdempotencyKey(block), AutomodIdempotencyKey(alert); k1 != k2 || k1 == "" {
		t.Fatalf("per-action events for the same message violation must share a key, got %q vs %q", k1, k2)
	}
}

// TestAutomodIdempotencyKey_CoalescesActionsForProfileViolation pins the
// same contract for member-profile rules: BLOCK_MEMBER_INTERACTION +
// SEND_ALERT_MESSAGE for a single profile update share MatchedContent
// and must collapse to one key, even though only the alert event carries
// AlertSystemMessageID.
func TestAutomodIdempotencyKey_CoalescesActionsForProfileViolation(t *testing.T) {
	t.Parallel()

	block := &automod.ActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedKeyword: "tits",
		MatchedContent: "Itsuki's Tits",
		ActionType:     4, // BLOCK_MEMBER_INTERACTION
	}
	alert := &automod.ActionExecution{
		GuildID:              "g1",
		RuleID:               "r1",
		UserID:               "u1",
		MatchedKeyword:       "tits",
		MatchedContent:       "Itsuki's Tits",
		AlertSystemMessageID: "a1",
		ActionType:           2, // SEND_ALERT_MESSAGE
	}
	if k1, k2 := AutomodIdempotencyKey(block), AutomodIdempotencyKey(alert); k1 != k2 || k1 == "" {
		t.Fatalf("per-action events for the same profile violation must share a key, got %q vs %q", k1, k2)
	}
}
