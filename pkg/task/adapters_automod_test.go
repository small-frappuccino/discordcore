package task

import (
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
)

func TestAutomodIdempotencyKey_UsesMessageIDWhenAvailable(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
		GuildID:   "g1",
		RuleID:    "r1",
		UserID:    "u1",
		MessageID: "m1",
	})
	if key != "automod:g1:r1:u1:msg:m1" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestAutomodIdempotencyKey_FallsBackToAlertSystemMessageID(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
		GuildID:              "g1",
		RuleID:               "r1",
		UserID:               "u1",
		AlertSystemMessageID: "a1",
	})
	if key != "automod:g1:r1:u1:alert:a1" {
		t.Fatalf("unexpected key: %q", key)
	}
}

func TestAutomodIdempotencyKey_NoStableIdentifier(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
		GuildID: "g1",
		RuleID:  "r1",
		UserID:  "u1",
	})
	if key != "" {
		t.Fatalf("expected empty key when no stable ID is present, got %q", key)
	}
}

func TestAutomodIdempotencyKey_HashesMatchedContentWhenNoMessageID(t *testing.T) {
	t.Parallel()

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
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

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
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

	base := &discordgo.AutoModerationActionExecution{
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

	base := &discordgo.AutoModerationActionExecution{
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
	k1 := automodIdempotencyKeyAt(&discordgo.AutoModerationActionExecution{
		GuildID:        "g1",
		RuleID:         "r1",
		UserID:         "u1",
		MatchedContent: "BigAss",
	}, now)
	k2 := automodIdempotencyKeyAt(&discordgo.AutoModerationActionExecution{
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

	key := AutomodIdempotencyKey(&discordgo.AutoModerationActionExecution{
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
	key := automodIdempotencyKeyAt(&discordgo.AutoModerationActionExecution{
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
