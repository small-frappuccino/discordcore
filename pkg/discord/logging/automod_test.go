package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldEmitLogEventAutomodActionToggles(t *testing.T) {
	t.Parallel()

	guildID := "g-automod-toggles"
	channelID := "c-auto"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			AutomodLog: channelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, channelID, botID, perms)
	session.Identify.Intents = discordgo.IntentAutoModerationExecution

	decision := ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
	if !decision.Enabled {
		t.Fatalf("expected automod logging enabled by default, got reason=%s", decision.Reason)
	}

	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}

	disabled := false
	cfg.Guilds[0].Features.Logging.Automod = &disabled
	decision = ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
	if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingAutomodDisabled {
		t.Fatalf("expected automod disabled by feature toggle, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}

	enabled := true
	cfg.Guilds[0].Features.Logging.Automod = &enabled
	cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = true
	decision = ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
	if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableAutomodLogs {
		t.Fatalf("expected automod disabled by runtime config, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}

	// Moderation logging toggle must not gate automod-native events.
	cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = false
	disabledModeration := false
	cfg.RuntimeConfig.ModerationLogging = &disabledModeration
	decision = ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
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
		session.Identify.Intents = discordgo.IntentAutoModerationExecution
		decision := ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != automodChannelID {
			t.Fatalf("expected channel %q, got %q", automodChannelID, decision.ChannelID)
		}
	})

	t.Run("falls back to moderation channel", func(t *testing.T) {
		guildID := "g-automod-fallback"
		moderationChannelID := "c-mod"

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
		session.Identify.Intents = discordgo.IntentAutoModerationExecution
		decision := ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != moderationChannelID {
			t.Fatalf("expected channel %q, got %q", moderationChannelID, decision.ChannelID)
		}
	})
}
