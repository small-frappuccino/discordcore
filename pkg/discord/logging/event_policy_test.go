package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldEmitLogEventRoleChange(t *testing.T) {
	t.Parallel()

	guildID := "g-role"
	channelID := "c-user"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			UserActivityLog: channelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuildMembers,
		},
	}

	decision := ShouldEmitLogEvent(session, cm, LogEventRoleChange, guildID)
	if !decision.Enabled {
		t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
	}
	if decision.ChannelID != channelID {
		t.Fatalf("expected channel %q, got %q", channelID, decision.ChannelID)
	}
}

func TestShouldEmitLogEventRoleChangeMissingIntent(t *testing.T) {
	t.Parallel()

	guildID := "g-role-intent"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			UserActivityLog: "c-user",
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuilds,
		},
	}

	decision := ShouldEmitLogEvent(session, cm, LogEventRoleChange, guildID)
	if decision.Enabled {
		t.Fatal("expected disabled decision for missing intent")
	}
	if decision.Reason != EmitReasonMissingIntent {
		t.Fatalf("expected reason %s, got %s", EmitReasonMissingIntent, decision.Reason)
	}
}

func TestShouldEmitLogEventRoleChangeDisabledByRuntime(t *testing.T) {
	t.Parallel()

	guildID := "g-role-runtime"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			UserActivityLog: "c-user",
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}
	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}
	cfg.RuntimeConfig.DisableUserLogs = true

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuildMembers,
		},
	}
	decision := ShouldEmitLogEvent(session, cm, LogEventRoleChange, guildID)
	if decision.Enabled {
		t.Fatal("expected disabled decision")
	}
	if decision.Reason != EmitReasonRuntimeDisableUserLogs {
		t.Fatalf("expected reason %s, got %s", EmitReasonRuntimeDisableUserLogs, decision.Reason)
	}
}

func TestShouldEmitLogEventMemberJoinChannelFallback(t *testing.T) {
	t.Parallel()

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuildMembers,
		},
	}

	t.Run("prefers entry_leave_log", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-join-1",
			Channels: files.ChannelsConfig{
				EntryLeaveLog:   "c-entry",
				UserActivityLog: "c-user",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMemberJoin, "g-join-1")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-entry" {
			t.Fatalf("expected entry/leave channel, got %q", decision.ChannelID)
		}
	})

	t.Run("falls back to user_activity_log", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-join-2",
			Channels: files.ChannelsConfig{
				UserActivityLog: "c-user",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMemberJoin, "g-join-2")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-user" {
			t.Fatalf("expected user activity fallback channel, got %q", decision.ChannelID)
		}
	})
}

func TestShouldEmitLogEventModerationCase(t *testing.T) {
	t.Parallel()

	guildID := "g-mod"
	channelID := "c-mod"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	t.Run("disabled by runtime moderation_logging", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			Channels: files.ChannelsConfig{
				ModerationLog: channelID,
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}
		cfg := cm.Config()
		if cfg == nil {
			t.Fatal("config is nil")
		}
		disabled := false
		cfg.RuntimeConfig.ModerationLogging = &disabled

		decision := ShouldEmitLogEvent(testSessionWithChannel(guildID, channelID, botID, perms), cm, LogEventModerationCase, guildID)
		if decision.Enabled {
			t.Fatal("expected disabled decision")
		}
		if decision.Reason != EmitReasonRuntimeModerationLoggingOff {
			t.Fatalf("expected reason %s, got %s", EmitReasonRuntimeModerationLoggingOff, decision.Reason)
		}
	})

	t.Run("disabled by feature toggle", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		disabled := false
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID + "-feature",
			Channels: files.ChannelsConfig{
				ModerationLog: channelID,
			},
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					Moderation: &disabled,
				},
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(testSessionWithChannel(guildID+"-feature", channelID, botID, perms), cm, LogEventModerationCase, guildID+"-feature")
		if decision.Enabled {
			t.Fatal("expected disabled decision")
		}
		if decision.Reason != EmitReasonFeatureLoggingModerationDisabled {
			t.Fatalf("expected reason %s, got %s", EmitReasonFeatureLoggingModerationDisabled, decision.Reason)
		}
	})

	t.Run("enabled with valid channel", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID + "-enabled",
			Channels: files.ChannelsConfig{
				ModerationLog: channelID,
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(testSessionWithChannel(guildID+"-enabled", channelID, botID, perms), cm, LogEventModerationCase, guildID+"-enabled")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != channelID {
			t.Fatalf("expected channel %q, got %q", channelID, decision.ChannelID)
		}
	})
}

func TestShouldEmitLogEventMessageDeleteChannelFallback(t *testing.T) {
	t.Parallel()

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuildMessages,
		},
	}

	t.Run("prefers message_audit_log", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-1",
			Channels: files.ChannelsConfig{
				MessageAuditLog: "c-message",
				UserActivityLog: "c-user",
				Commands:        "c-commands",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-1")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-message" {
			t.Fatalf("expected message audit channel, got %q", decision.ChannelID)
		}
	})

	t.Run("falls back to user_activity_log then commands", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-2",
			Channels: files.ChannelsConfig{
				UserActivityLog: "c-user",
				Commands:        "c-commands",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-2")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-user" {
			t.Fatalf("expected user activity fallback channel, got %q", decision.ChannelID)
		}
	})

	t.Run("uses commands when others missing", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-3",
			Channels: files.ChannelsConfig{
				Commands: "c-commands",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-3")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-commands" {
			t.Fatalf("expected commands fallback channel, got %q", decision.ChannelID)
		}
	})
}

func TestShouldEmitLogEventReactionMetric(t *testing.T) {
	t.Parallel()

	guildID := "g-react"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := &discordgo.Session{
		Identify: discordgo.Identify{
			Intents: discordgo.IntentsGuildMessageReactions,
		},
	}

	decision := ShouldEmitLogEvent(session, cm, LogEventReactionMetric, guildID)
	if !decision.Enabled {
		t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
	}
	if decision.ChannelID != "" {
		t.Fatalf("expected no channel for metric event, got %q", decision.ChannelID)
	}

	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}
	cfg.Guilds[0].RuntimeConfig.DisableReactionLogs = true
	decision = ShouldEmitLogEvent(session, cm, LogEventReactionMetric, guildID)
	if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableReactionLogs {
		t.Fatalf("expected reaction disabled by runtime config, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}
}

func TestShouldEmitLogEventTogglePrecedence(t *testing.T) {
	t.Parallel()

	t.Run("message_process runtime kill switch wins over feature toggle", func(t *testing.T) {
		guildID := "g-precedence-message"
		cm := files.NewConfigManagerWithPath("test-settings.json")
		enabled := true
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					Message: &enabled,
				},
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		session := &discordgo.Session{
			Identify: discordgo.Identify{
				Intents: discordgo.IntentsGuildMessages,
			},
		}

		cfg := cm.Config()
		if cfg == nil {
			t.Fatal("config is nil")
		}
		cfg.Guilds[0].RuntimeConfig.DisableMessageLogs = true
		decision := ShouldEmitLogEvent(session, cm, LogEventMessageProcess, guildID)
		if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableMessageLogs {
			t.Fatalf("expected runtime kill switch reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}

		cfg.Guilds[0].RuntimeConfig.DisableMessageLogs = false
		disabled := false
		cfg.Guilds[0].Features.Logging.Message = &disabled
		decision = ShouldEmitLogEvent(session, cm, LogEventMessageProcess, guildID)
		if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingMessageDisabled {
			t.Fatalf("expected feature toggle reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}
	})

	t.Run("reaction_metric runtime kill switch wins over feature toggle", func(t *testing.T) {
		guildID := "g-precedence-reaction"
		cm := files.NewConfigManagerWithPath("test-settings.json")
		enabled := true
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					Reaction: &enabled,
				},
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		session := &discordgo.Session{
			Identify: discordgo.Identify{
				Intents: discordgo.IntentsGuildMessageReactions,
			},
		}

		cfg := cm.Config()
		if cfg == nil {
			t.Fatal("config is nil")
		}
		cfg.Guilds[0].RuntimeConfig.DisableReactionLogs = true
		decision := ShouldEmitLogEvent(session, cm, LogEventReactionMetric, guildID)
		if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableReactionLogs {
			t.Fatalf("expected runtime kill switch reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}

		cfg.Guilds[0].RuntimeConfig.DisableReactionLogs = false
		disabled := false
		cfg.Guilds[0].Features.Logging.Reaction = &disabled
		decision = ShouldEmitLogEvent(session, cm, LogEventReactionMetric, guildID)
		if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingReactionDisabled {
			t.Fatalf("expected feature toggle reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}
	})

	t.Run("automod_action runtime kill switch wins over feature toggle", func(t *testing.T) {
		guildID := "g-precedence-automod"
		channelID := "c-auto"
		botID := "bot"
		perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

		enabled := true
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: guildID,
			Channels: files.ChannelsConfig{
				AutomodLog: channelID,
			},
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					Automod: &enabled,
				},
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		session := testSessionWithChannel(guildID, channelID, botID, perms)
		session.Identify.Intents = discordgo.IntentAutoModerationExecution

		cfg := cm.Config()
		if cfg == nil {
			t.Fatal("config is nil")
		}
		cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = true
		decision := ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
		if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableAutomodLogs {
			t.Fatalf("expected runtime kill switch reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}

		cfg.Guilds[0].RuntimeConfig.DisableAutomodLogs = false
		disabled := false
		cfg.Guilds[0].Features.Logging.Automod = &disabled
		decision = ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
		if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingAutomodDisabled {
			t.Fatalf("expected feature toggle reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}
	})
}
