package task

import (
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestAutomodIdempotencyKey_UsesMessageIDWhenAvailable(t *testing.T) {
	t.Parallel()

	key := automodIdempotencyKey(&discordgo.AutoModerationActionExecution{
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

	key := automodIdempotencyKey(&discordgo.AutoModerationActionExecution{
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

	key := automodIdempotencyKey(&discordgo.AutoModerationActionExecution{
		GuildID: "g1",
		RuleID:  "r1",
		UserID:  "u1",
	})
	if key != "" {
		t.Fatalf("expected empty key when no stable ID is present, got %q", key)
	}
}
