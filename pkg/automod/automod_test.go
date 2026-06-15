package automod

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordgo"
)

func TestShouldEmitLogEventAutomodActionToggles(t *testing.T) {
	t.Parallel()

	guildID := "g-automod-toggles"
	channelID := "c-auto"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := newTestConfigManager(t)
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			AutomodAction: channelID}}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(t, guildID, channelID, botID, perms)
	session.Identify.Intents = discordgo.IntentAutoModerationExecution

	decision := logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
	if !decision.Enabled {
		t.Fatalf("expected automod logging enabled by default, got reason=%s", decision.Reason)
	}

	if cm.Config() == nil {
		t.Fatal("config is nil")
	}

	disabled := false
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].Features.Logging.AutomodAction = &disabled
	})
	decision = logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
	if decision.Enabled || decision.Reason != logpolicy.EmitReasonFeatureLoggingAutomodDisabled {
		t.Fatalf("expected automod disabled by feature toggle, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}

	enabled := true
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].Features.Logging.AutomodAction = &enabled
		cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = true
	})
	decision = logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
	if decision.Enabled || decision.Reason != logpolicy.EmitReasonRuntimeDisableAutomodLogs {
		t.Fatalf("expected automod disabled by runtime config, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}

	// Moderation logging toggle must not gate automod-native events.
	disabledModeration := false
	mustUpdateConfig(t, cm, func(cfg *files.BotConfig) {
		cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = false
		cfg.RuntimeConfig.ModerationLogging = &disabledModeration
	})
	decision = logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
	if !decision.Enabled {
		t.Fatalf("expected automod logging independent from moderation_logging, got reason=%s", decision.Reason)
	}
}

func TestShouldEmitLogEventAutomodActionChannelResolution(t *testing.T) {
	t.Parallel()

	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	t.Run("prefers dedicated automod channel", func(t *testing.T) {
		guildID := "g-automod-pref"
		automodChannelID := "c-auto"

		cm := newTestConfigManager(t)
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			Channels: files.ChannelsConfig{
				AutomodAction: automodChannelID}}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		session := testSessionWithChannel(t, guildID, automodChannelID, botID, perms)
		session.Identify.Intents = discordgo.IntentAutoModerationExecution
		decision := logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != automodChannelID {
			t.Fatalf("expected channel %q, got %q", automodChannelID, decision.ChannelID)
		}
	})

	t.Run("requires dedicated automod channel", func(t *testing.T) {
		guildID := "g-automod-fallback"

		cm := newTestConfigManager(t)
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		session := testSessionWithChannel(t, guildID, "c-other", botID, perms)
		session.Identify.Intents = discordgo.IntentAutoModerationExecution
		decision := logpolicy.ShouldEmitLogEvent(session, cm, logpolicy.LogEventAutomodAction, guildID)
		if decision.Enabled {
			t.Fatal("expected disabled decision")
		}
		if decision.Reason != logpolicy.EmitReasonNoChannelConfigured {
			t.Fatalf("expected reason %s, got %s", logpolicy.EmitReasonNoChannelConfigured, decision.Reason)
		}
	})
}
