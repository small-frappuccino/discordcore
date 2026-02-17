package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldLogAutomodEvent(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}

	// Defaults: automod logging enabled.
	if !shouldLogAutomodEvent(cm, guildID) {
		t.Fatal("expected automod logging enabled by default")
	}

	// Must obey logging.automod toggle.
	disabled := false
	cfg.Guilds[0].Features.Logging.Automod = &disabled
	if shouldLogAutomodEvent(cm, guildID) {
		t.Fatal("expected automod logging disabled by feature toggle")
	}

	enabled := true
	cfg.Guilds[0].Features.Logging.Automod = &enabled
	cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = true
	if shouldLogAutomodEvent(cm, guildID) {
		t.Fatal("expected automod logging disabled by runtime config")
	}

	// Moderation logging toggle must not gate automod-native events.
	cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = false
	disabledModeration := false
	cfg.RuntimeConfig.ModerationLogging = &disabledModeration
	if !shouldLogAutomodEvent(cm, guildID) {
		t.Fatal("expected automod logging independent from moderation_logging")
	}
}

func TestResolveAutomodLogChannelPrefersDedicatedChannel(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	automodChannelID := "c-auto"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			AutomodLog:    automodChannelID,
			ModerationLog: "c-mod",
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, automodChannelID, botID, perms)
	got, ok := resolveAutomodLogChannel(session, cm, guildID)
	if !ok {
		t.Fatal("expected automod log channel to validate")
	}
	if got != automodChannelID {
		t.Fatalf("expected channel %q, got %q", automodChannelID, got)
	}
}

func TestResolveAutomodLogChannelFallsBackToModerationChannel(t *testing.T) {
	t.Parallel()

	guildID := "g1"
	moderationChannelID := "c-mod"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			ModerationLog: moderationChannelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, moderationChannelID, botID, perms)
	got, ok := resolveAutomodLogChannel(session, cm, guildID)
	if !ok {
		t.Fatal("expected moderation fallback channel to validate")
	}
	if got != moderationChannelID {
		t.Fatalf("expected channel %q, got %q", moderationChannelID, got)
	}
}
