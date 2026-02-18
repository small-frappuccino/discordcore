package moderation

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestBuildMassBanLogDetails(t *testing.T) {
	t.Parallel()

	got := buildMassBanLogDetails(10, 7, []string{"a"}, []string{"b"}, []string{"c", "d"})
	if got == "" {
		t.Fatal("expected details string")
	}
	if !containsAll(got, []string{"Total: 10", "Banned: 7", "Invalid: 1", "Skipped: 1", "Failed: 2"}) {
		t.Fatalf("unexpected details: %q", got)
	}
}

func TestBuildBanCommandMessageUsesUsername(t *testing.T) {
	t.Parallel()

	got := buildBanCommandMessage("alice", "rule violation", false)
	if !containsAll(got, []string{"alice", "rule violation"}) {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestBuildMassBanCommandMessageOnlyCount(t *testing.T) {
	t.Parallel()

	got := buildMassBanCommandMessage(4)
	if got != "Banned 4 user(s)." {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestSendModerationLogNoChannel(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	botID := "bot"

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}
	if cm.Config() == nil {
		t.Fatal("config is nil")
	}
	enabled := true
	cm.Config().RuntimeConfig.ModerationLogging = &enabled

	session := &discordgo.Session{State: discordgo.NewState()}
	session.State.User = &discordgo.User{ID: botID}

	ctx := &core.Context{
		Session: session,
		Config:  cm,
		GuildID: guildID,
		UserID:  "user",
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("sendModerationLog panicked: %v", r)
		}
	}()

	sendModerationLog(ctx, moderationLogPayload{
		Action:      "ban",
		TargetID:    "target",
		TargetLabel: "target",
		Reason:      "reason",
		RequestedBy: "user",
	})
}

func TestResolveModerationActionTypeBanAliases(t *testing.T) {
	t.Parallel()

	inputs := []string{"ban", "massban", "member_ban_add", "AuditLogActionMemberBanAdd", "22"}
	for _, in := range inputs {
		if got := resolveModerationActionType(in); got != "Member Ban Add" {
			t.Fatalf("unexpected action type for %q: %q", in, got)
		}
	}
}

func TestResolveModerationActionTypeNewAliases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "unban", want: "Member Ban Remove"},
		{input: "kick", want: "Member Kick"},
		{input: "timeout", want: "Member Update"},
		{input: "untimeout", want: "Member Update"},
	}

	for _, tt := range tests {
		if got := resolveModerationActionType(tt.input); got != tt.want {
			t.Fatalf("unexpected action type for %q: got %q want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveModerationActionTypeMappedAPIValues(t *testing.T) {
	t.Parallel()

	if got := resolveModerationActionType("191"); got != "Home Settings Update" {
		t.Fatalf("unexpected label for action 191: %q", got)
	}
	if got := resolveModerationActionType("guild_scheduled_event_create"); got != "Guild Scheduled Event Create" {
		t.Fatalf("unexpected label for guild scheduled event create: %q", got)
	}
}

func TestBuildModerationCaseTitle(t *testing.T) {
	t.Parallel()

	if got := buildModerationCaseTitle(12, true, "Ban"); got != "ban | case 12" {
		t.Fatalf("unexpected title: %q", got)
	}
	if got := buildModerationCaseTitle(0, false, "Ban"); got != "ban | case ?" {
		t.Fatalf("unexpected title fallback: %q", got)
	}
}

func TestResolveModerationCaseEmbedMeta(t *testing.T) {
	t.Parallel()

	actionLabel, targetField, detailsField := resolveModerationCaseEmbedMeta("timeout", "Member Update")
	if actionLabel != "timeout" || targetField != "Offender" || detailsField != "Details" {
		t.Fatalf("unexpected timeout embed meta: %q, %q, %q", actionLabel, targetField, detailsField)
	}

	actionLabel, targetField, detailsField = resolveModerationCaseEmbedMeta("unban", "Member Ban Remove")
	if actionLabel != "unban" || targetField != "User" || detailsField != "Details" {
		t.Fatalf("unexpected unban embed meta: %q, %q, %q", actionLabel, targetField, detailsField)
	}
}

func TestFormatTimeoutDuration(t *testing.T) {
	t.Parallel()

	if got := formatTimeoutDuration(30); got != "30 minute(s)" {
		t.Fatalf("unexpected duration for 30 minutes: %q", got)
	}
	if got := formatTimeoutDuration(120); got != "2 hour(s)" {
		t.Fatalf("unexpected duration for 120 minutes: %q", got)
	}
	if got := formatTimeoutDuration(2880); got != "2 day(s)" {
		t.Fatalf("unexpected duration for 2880 minutes: %q", got)
	}
}

func TestResolveMessageDeleteLogChannelFallbackOrder(t *testing.T) {
	t.Parallel()

	gcfg := &files.GuildConfig{
		Channels: files.ChannelsConfig{
			MessageDelete: "c-delete",
			MessageEdit:   "c-edit",
		},
	}
	ctx := &core.Context{GuildConfig: gcfg}

	if got := resolveMessageDeleteLogChannel(ctx); got != "c-delete" {
		t.Fatalf("expected message_delete first, got %q", got)
	}

	gcfg.Channels.MessageDelete = ""
	if got := resolveMessageDeleteLogChannel(ctx); got != "c-edit" {
		t.Fatalf("expected message_edit fallback, got %q", got)
	}

	gcfg.Channels.MessageEdit = ""
	if got := resolveMessageDeleteLogChannel(ctx); got != "" {
		t.Fatalf("expected no channel fallback, got %q", got)
	}
}

func TestShouldSendCleanUsageMessageDeleteEmbed(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}
	ctx := &core.Context{Config: cm, GuildID: guildID}

	if !shouldSendCleanUsageMessageDeleteEmbed(ctx) {
		t.Fatal("expected clean usage delete embed enabled by default")
	}

	cm.Config().RuntimeConfig.DisableMessageLogs = true
	if shouldSendCleanUsageMessageDeleteEmbed(ctx) {
		t.Fatal("expected disabled when runtime disable_message_logs is true")
	}

	cm.Config().RuntimeConfig.DisableMessageLogs = false
	disableMessage := false
	cm.Config().Features.Logging.MessageDelete = &disableMessage
	if shouldSendCleanUsageMessageDeleteEmbed(ctx) {
		t.Fatal("expected disabled when features.logging.message_delete is false")
	}
}

func containsAll(value string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}
