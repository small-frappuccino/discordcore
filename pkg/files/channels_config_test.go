package files

import (
	"encoding/json"
	"testing"
)

func TestChannelsConfigUnmarshalStrictSchema(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
		"avatar_logging": "c-avatar",
		"role_update": "c-role",
		"member_join": "c-join",
		"member_leave": "c-leave",
		"message_edit": "c-edit",
		"message_delete": "c-delete",
		"automod_action": "c-automod",
		"moderation_case": "c-mod",
		"clean_action": "c-clean",
		"entry_backfill": "c-backfill",
		"verification_cleanup": "c-verify"
	}`)

	var channels ChannelsConfig
	if err := json.Unmarshal(raw, &channels); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}

	if channels.AvatarLogging != "c-avatar" || channels.RoleUpdate != "c-role" {
		t.Fatalf("unexpected user channel mapping: avatar=%q role=%q", channels.AvatarLogging, channels.RoleUpdate)
	}
	if channels.MemberJoin != "c-join" || channels.MemberLeave != "c-leave" {
		t.Fatalf("unexpected member channel mapping: join=%q leave=%q", channels.MemberJoin, channels.MemberLeave)
	}
	if channels.MessageEdit != "c-edit" || channels.MessageDelete != "c-delete" {
		t.Fatalf("unexpected message channel mapping: edit=%q delete=%q", channels.MessageEdit, channels.MessageDelete)
	}
	if channels.AutomodAction != "c-automod" || channels.ModerationCase != "c-mod" || channels.CleanAction != "c-clean" {
		t.Fatalf("unexpected moderation channel mapping: automod=%q moderation=%q clean=%q", channels.AutomodAction, channels.ModerationCase, channels.CleanAction)
	}
	if channels.EntryBackfill != "c-backfill" || channels.VerificationCleanup != "c-verify" {
		t.Fatalf("unexpected utility channels: backfill=%q verify=%q", channels.EntryBackfill, channels.VerificationCleanup)
	}
}

func TestChannelsConfigHelpersStrict(t *testing.T) {
	t.Parallel()

	channels := ChannelsConfig{
		EntryBackfill:       "c-backfill",
		VerificationCleanup: "c-verify",
	}

	if got := channels.BackfillChannelID(); got != "c-backfill" {
		t.Fatalf("expected strict backfill channel, got %q", got)
	}
	if got := channels.VerificationCleanupChannelID(); got != "c-verify" {
		t.Fatalf("expected strict verification channel, got %q", got)
	}
}
