package logging

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestShouldEmitLogEventRoleChange(t *testing.T) {
	t.Parallel()

	guildID := "g-role"
	channelID := "c-role"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			RoleUpdate: channelID,
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
			RoleUpdate: "c-role",
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
			RoleUpdate: "c-role",
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

	t.Run("prefers member_join", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-join-1",
			Channels: files.ChannelsConfig{
				MemberJoin:  "c-join",
				MemberLeave: "c-leave",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMemberJoin, "g-join-1")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-join" {
			t.Fatalf("expected member_join channel, got %q", decision.ChannelID)
		}
	})

	t.Run("falls back to member_leave", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-join-2",
			Channels: files.ChannelsConfig{
				MemberLeave: "c-leave",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMemberJoin, "g-join-2")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-leave" {
			t.Fatalf("expected member_leave fallback channel, got %q", decision.ChannelID)
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
				ModerationCase: channelID,
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
				ModerationCase: channelID,
			},
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					ModerationCase: &disabled,
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
				ModerationCase: channelID,
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

	t.Run("prefers message_delete", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-1",
			Channels: files.ChannelsConfig{
				MessageDelete: "c-delete",
				MessageEdit:   "c-edit",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-1")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-delete" {
			t.Fatalf("expected message_delete channel, got %q", decision.ChannelID)
		}
	})

	t.Run("falls back to message_edit", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-2",
			Channels: files.ChannelsConfig{
				MessageEdit: "c-edit",
			},
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-2")
		if !decision.Enabled {
			t.Fatalf("expected enabled decision, got reason=%s", decision.Reason)
		}
		if decision.ChannelID != "c-edit" {
			t.Fatalf("expected message_edit fallback channel, got %q", decision.ChannelID)
		}
	})

	t.Run("disabled when no channel configured", func(t *testing.T) {
		cm := files.NewConfigManagerWithPath("test-settings.json")
		if err := cm.AddGuildConfig(files.GuildConfig{
			GuildID: "g-msg-3",
		}); err != nil {
			t.Fatalf("AddGuildConfig: %v", err)
		}

		decision := ShouldEmitLogEvent(session, cm, LogEventMessageDelete, "g-msg-3")
		if decision.Enabled {
			t.Fatal("expected disabled decision")
		}
		if decision.Reason != EmitReasonNoChannelConfigured {
			t.Fatalf("expected no channel reason, got %s", decision.Reason)
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
					MessageProcess: &enabled,
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
		cfg.Guilds[0].Features.Logging.MessageProcess = &disabled
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
					ReactionMetric: &enabled,
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
		cfg.Guilds[0].Features.Logging.ReactionMetric = &disabled
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
				AutomodAction: channelID,
			},
			Features: files.FeatureToggles{
				Logging: files.FeatureLoggingToggles{
					AutomodAction: &enabled,
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
		cfg.Guilds[0].Features.Logging.AutomodAction = &disabled
		decision = ShouldEmitLogEvent(session, cm, LogEventAutomodAction, guildID)
		if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingAutomodDisabled {
			t.Fatalf("expected feature toggle reason, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
		}
	})
}

func TestShouldEmitLogEventCleanAction(t *testing.T) {
	t.Parallel()

	guildID := "g-clean"
	channelID := "c-clean"
	botID := "bot"
	perms := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)

	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			CleanAction: channelID,
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	session := testSessionWithChannel(guildID, channelID, botID, perms)

	decision := ShouldEmitLogEvent(session, cm, LogEventCleanAction, guildID)
	if !decision.Enabled {
		t.Fatalf("expected clean logging enabled by default, got reason=%s", decision.Reason)
	}
	if decision.ChannelID != channelID {
		t.Fatalf("expected clean channel %q, got %q", channelID, decision.ChannelID)
	}

	cfg := cm.Config()
	if cfg == nil {
		t.Fatal("config is nil")
	}

	cfg.Guilds[0].RuntimeConfig.DisableCleanLog = true
	decision = ShouldEmitLogEvent(session, cm, LogEventCleanAction, guildID)
	if decision.Enabled || decision.Reason != EmitReasonRuntimeDisableCleanLog {
		t.Fatalf("expected clean disabled by runtime switch, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}

	cfg.Guilds[0].RuntimeConfig.DisableCleanLog = false
	disabled := false
	cfg.Guilds[0].Features.Logging.CleanAction = &disabled
	decision = ShouldEmitLogEvent(session, cm, LogEventCleanAction, guildID)
	if decision.Enabled || decision.Reason != EmitReasonFeatureLoggingCleanDisabled {
		t.Fatalf("expected clean disabled by feature toggle, got enabled=%v reason=%s", decision.Enabled, decision.Reason)
	}
}

func TestResolveLogChannelCleanFallback(t *testing.T) {
	t.Parallel()

	guildID := "g-clean-fallback"
	cm := files.NewConfigManagerWithPath("test-settings.json")
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID: guildID,
		Channels: files.ChannelsConfig{
			ModerationCase: "c-mod",
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	if got := ResolveLogChannel(LogEventCleanAction, guildID, cm); got != "c-mod" {
		t.Fatalf("expected clean fallback to moderation_case, got %q", got)
	}
}
