package moderation

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
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
	if got != "4 users were banned." {
		t.Fatalf("unexpected message: %q", got)
	}
}

func TestRegisterModerationCommandsLimitsScope(t *testing.T) {
	t.Parallel()

	session := &discordgo.Session{State: discordgo.NewState()}
	router := core.NewCommandRouter(session, files.NewMemoryConfigManager())

	RegisterModerationCommands(router)

	if _, ok := router.GetRegistry().GetCommand("clean"); ok {
		t.Fatal("did not expect /clean to remain registered")
	}

	cmd, ok := router.GetRegistry().GetCommand("moderation")
	if !ok {
		t.Fatal("expected moderation group command to be registered")
	}

	options := cmd.Options()
	got := make([]string, 0, len(options))
	for _, option := range options {
		got = append(got, option.Name)
	}
	expected := []string{"ban", "kick", "massban", "mute", "timeout", "warn", "warnings"}
	if !sameStringSet(got, expected) {
		t.Fatalf("unexpected moderation subcommands: got=%v want=%v", got, expected)
	}
}

func TestEnsureModerationCommandEnabledRejectsDisabledCommands(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			Moderation: files.FeatureModerationToggles{
				Ban:      boolPtr(false),
				MassBan:  boolPtr(false),
				Kick:     boolPtr(false),
				Timeout:  boolPtr(false),
				Warn:     boolPtr(false),
				Warnings: boolPtr(false),
			},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	ctx := &core.Context{Config: cm, GuildID: guildID}

	tests := []struct {
		name      string
		featureID string
		message   string
	}{
		{name: "ban", featureID: "moderation.ban", message: "Ban command is disabled for this server."},
		{name: "massban", featureID: "moderation.massban", message: "Mass ban command is disabled for this server."},
		{name: "kick", featureID: "moderation.kick", message: "Kick command is disabled for this server."},
		{name: "timeout", featureID: "moderation.timeout", message: "Timeout command is disabled for this server."},
		{name: "warn", featureID: "moderation.warn", message: "Warn command is disabled for this server."},
		{name: "warnings", featureID: "moderation.warnings", message: "Warnings command is disabled for this server."},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := ensureModerationCommandEnabled(ctx, tt.featureID, tt.message)
			if err == nil {
				t.Fatalf("expected %s to be rejected", tt.name)
			}
			cmdErr, ok := err.(*core.CommandError)
			if !ok {
				t.Fatalf("expected *core.CommandError, got %T", err)
			}
			if cmdErr.Message != tt.message {
				t.Fatalf("unexpected message: got %q want %q", cmdErr.Message, tt.message)
			}
			if !cmdErr.Ephemeral {
				t.Fatal("expected command error to be ephemeral")
			}
		})
	}
}

func TestBanCommandHandleRejectsWhenFeatureDisabled(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			Moderation: files.FeatureModerationToggles{
				Ban: boolPtr(false),
			},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	err := newBanCommand().Handle(&core.Context{
		Config:  cm,
		GuildID: guildID,
	})
	if err == nil {
		t.Fatal("expected ban command to be rejected")
	}
	cmdErr, ok := err.(*core.CommandError)
	if !ok {
		t.Fatalf("expected *core.CommandError, got %T", err)
	}
	if cmdErr.Message != "Ban command is disabled for this server." {
		t.Fatalf("unexpected error message: %q", cmdErr.Message)
	}
}

func TestSendModerationLogNoChannel(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	botID := "bot"

	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}
	if cm.Config() == nil {
		t.Fatal("config is nil")
	}
	enabled := true
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		cfg.RuntimeConfig.ModerationLogging = &enabled
	})

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
		{input: "mute", want: "Member Role Update"},
		{input: "timeout", want: "Member Update"},
		{input: "untimeout", want: "Member Update"},
		{input: "warn", want: "Warning Issued"},
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

	actionLabel, targetField, detailsField = resolveModerationCaseEmbedMeta("mute", "Member Role Update")
	if actionLabel != "mute" || targetField != "Offender" || detailsField != "Details" {
		t.Fatalf("unexpected mute embed meta: %q, %q, %q", actionLabel, targetField, detailsField)
	}

	actionLabel, targetField, detailsField = resolveModerationCaseEmbedMeta("warn", "Warning Issued")
	if actionLabel != "warn" || targetField != "Offender" || detailsField != "Details" {
		t.Fatalf("unexpected warn embed meta: %q, %q, %q", actionLabel, targetField, detailsField)
	}
}

func TestResolveConfiguredMuteRole(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Features: files.FeatureToggles{
			MuteRole: boolPtr(true),
		},
		Roles: files.RolesConfig{
			MuteRole: "mute-role",
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	ctx := &core.Context{
		Config:  cm,
		GuildID: guildID,
		UserID:  "moderator",
		GuildConfig: &files.GuildConfig{
			GuildID: guildID,
			Roles: files.RolesConfig{
				MuteRole: "mute-role",
			},
		},
	}
	actionCtx := &banContext{
		rolesByID: map[string]*discordgo.Role{
			"mute-role": {ID: "mute-role", Name: "Muted", Position: 2},
		},
		actorRolePos: 5,
		botRolePos:   6,
	}

	role, roleID, err := resolveConfiguredMuteRole(ctx, actionCtx)
	if err != nil {
		t.Fatalf("resolveConfiguredMuteRole returned error: %v", err)
	}
	if roleID != "mute-role" {
		t.Fatalf("unexpected role id: %q", roleID)
	}
	if role == nil || role.Name != "Muted" {
		t.Fatalf("unexpected role: %+v", role)
	}
}

func TestBuildWarningsCommandMessage(t *testing.T) {
	t.Parallel()

	message := buildWarningsCommandMessage("alice", []storage.ModerationWarning{
		{
			CaseNumber:  3,
			ModeratorID: "mod-1",
			Reason:      "Spam",
		},
		{
			CaseNumber:  2,
			ModeratorID: "mod-2",
			Reason:      "Off-topic flood",
		},
	})

	if !containsAll(message, []string{"alice", "#3", "Spam", "#2", "Off-topic flood"}) {
		t.Fatalf("unexpected warnings message: %q", message)
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

func containsAll(value string, parts []string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]int, len(left))
	for _, item := range left {
		seen[item]++
	}
	for _, item := range right {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}

func boolPtr(v bool) *bool {
	return &v
}
