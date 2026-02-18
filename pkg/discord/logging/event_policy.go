package logging

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// LogEventType identifies an internal logging event kind.
type LogEventType string

const (
	LogEventAvatarChange   LogEventType = "avatar_change"
	LogEventRoleChange     LogEventType = "role_change"
	LogEventMemberJoin     LogEventType = "member_join"
	LogEventMemberLeave    LogEventType = "member_leave"
	LogEventMessageProcess LogEventType = "message_process"
	LogEventMessageEdit    LogEventType = "message_edit"
	LogEventMessageDelete  LogEventType = "message_delete"
	LogEventReactionMetric LogEventType = "reaction_metric"
	LogEventAutomodAction  LogEventType = "automod_action"
	LogEventModerationCase LogEventType = "moderation_case"
)

// LogEventCategory groups events by subsystem.
type LogEventCategory string

const (
	LogCategoryUser       LogEventCategory = "user"
	LogCategoryMessage    LogEventCategory = "message"
	LogCategoryReaction   LogEventCategory = "reaction"
	LogCategoryAutomod    LogEventCategory = "automod"
	LogCategoryModeration LogEventCategory = "moderation"
)

type logChannelKey string

const (
	logChannelUserActivity logChannelKey = "user_activity_log"
	logChannelEntryLeave   logChannelKey = "entry_leave_log"
	logChannelMessageAudit logChannelKey = "message_audit_log"
	logChannelCommands     logChannelKey = "commands"
	logChannelAutomod      logChannelKey = "automod_log"
	logChannelModeration   logChannelKey = "moderation_log"
)

// EmitReason is a deterministic reason for a should-emit decision.
type EmitReason string

const (
	EmitReasonEnabled                          EmitReason = "enabled"
	EmitReasonUnknownEvent                     EmitReason = "unknown_event"
	EmitReasonConfigManagerUnavailable         EmitReason = "config_manager_unavailable"
	EmitReasonConfigUnavailable                EmitReason = "config_unavailable"
	EmitReasonGuildConfigMissing               EmitReason = "guild_config_missing"
	EmitReasonFeatureLoggingUserDisabled       EmitReason = "feature_logging_user_disabled"
	EmitReasonFeatureLoggingEntryExitDisabled  EmitReason = "feature_logging_entry_exit_disabled"
	EmitReasonFeatureLoggingMessageDisabled    EmitReason = "feature_logging_message_disabled"
	EmitReasonFeatureLoggingReactionDisabled   EmitReason = "feature_logging_reaction_disabled"
	EmitReasonFeatureLoggingAutomodDisabled    EmitReason = "feature_logging_automod_disabled"
	EmitReasonFeatureLoggingModerationDisabled EmitReason = "feature_logging_moderation_disabled"
	EmitReasonRuntimeDisableUserLogs           EmitReason = "runtime_disable_user_logs"
	EmitReasonRuntimeDisableEntryExitLogs      EmitReason = "runtime_disable_entry_exit_logs"
	EmitReasonRuntimeDisableMessageLogs        EmitReason = "runtime_disable_message_logs"
	EmitReasonRuntimeDisableReactionLogs       EmitReason = "runtime_disable_reaction_logs"
	EmitReasonRuntimeDisableAutomodLogs        EmitReason = "runtime_disable_automod_logs"
	EmitReasonRuntimeModerationLoggingOff      EmitReason = "runtime_moderation_logging_off"
	EmitReasonNoChannelConfigured              EmitReason = "no_channel_configured"
	EmitReasonMissingIntent                    EmitReason = "missing_intent"
	EmitReasonChannelInvalid                   EmitReason = "channel_invalid"
)

// LogEventCapability describes governance and requirements for a log event.
type LogEventCapability struct {
	EventType           LogEventType
	Category            LogEventCategory
	RequiredIntentsMask int
	RequiredPermsMask   int64
	RequiresChannel     bool
	PreferredChannel    logChannelKey
	FallbackChannels    []logChannelKey
	// Toggles documents the two policy layers:
	// 1) `features.*` => product behavior (fine-grain enablement)
	// 2) `runtime_config.disable_*` => operational kill switch
	// Precedence is enforced in ShouldEmitLogEvent: kill switch wins.
	Toggles                    []string
	ValidateChannelPerms       bool
	RequireExclusiveModeration bool
}

// EmitDecision is the result of ShouldEmitLogEvent.
type EmitDecision struct {
	EventType   LogEventType
	Category    LogEventCategory
	Enabled     bool
	Reason      EmitReason
	ChannelID   string
	Capability  LogEventCapability
	MissingMask int
}

var logEventCapabilities = map[LogEventType]LogEventCapability{
	LogEventAvatarChange: {
		EventType:           LogEventAvatarChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: 0,
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelUserActivity,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.user"},
	},
	LogEventRoleChange: {
		EventType:           LogEventRoleChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelUserActivity,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.user"},
	},
	LogEventMemberJoin: {
		EventType:           LogEventMemberJoin,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelEntryLeave,
		FallbackChannels:    []logChannelKey{logChannelUserActivity},
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.entry_exit"},
	},
	LogEventMemberLeave: {
		EventType:           LogEventMemberLeave,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelEntryLeave,
		FallbackChannels:    []logChannelKey{logChannelUserActivity},
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.entry_exit"},
	},
	LogEventMessageProcess: {
		EventType:           LogEventMessageProcess,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message"},
	},
	LogEventMessageEdit: {
		EventType:           LogEventMessageEdit,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelMessageAudit,
		FallbackChannels:    []logChannelKey{logChannelUserActivity, logChannelCommands},
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message"},
	},
	LogEventMessageDelete: {
		EventType:           LogEventMessageDelete,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		PreferredChannel:    logChannelMessageAudit,
		FallbackChannels:    []logChannelKey{logChannelUserActivity, logChannelCommands},
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message"},
	},
	LogEventReactionMetric: {
		EventType:           LogEventReactionMetric,
		Category:            LogCategoryReaction,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessageReactions),
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_reaction_logs", "features.logging.reaction"},
	},
	LogEventAutomodAction: {
		EventType:            LogEventAutomodAction,
		Category:             LogCategoryAutomod,
		RequiredIntentsMask:  int(discordgo.IntentAutoModerationExecution),
		RequiredPermsMask:    int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:      true,
		PreferredChannel:     logChannelAutomod,
		FallbackChannels:     []logChannelKey{logChannelModeration},
		Toggles:              []string{"runtime_config.disable_automod_logs", "features.logging.automod"},
		ValidateChannelPerms: true,
	},
	LogEventModerationCase: {
		EventType:                  LogEventModerationCase,
		Category:                   LogCategoryModeration,
		RequiredIntentsMask:        0,
		RequiredPermsMask:          int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:            true,
		PreferredChannel:           logChannelModeration,
		Toggles:                    []string{"runtime_config.moderation_logging", "features.logging.moderation"},
		ValidateChannelPerms:       true,
		RequireExclusiveModeration: true,
	},
}

// LogEventCapabilities returns a copy of the event capability map.
func LogEventCapabilities() map[LogEventType]LogEventCapability {
	out := make(map[LogEventType]LogEventCapability, len(logEventCapabilities))
	for k, v := range logEventCapabilities {
		out[k] = v
	}
	return out
}

// ShouldEmitLogEvent centralizes event gating and channel resolution.
//
// Toggle precedence (explicit and stable):
// 1) Runtime kill switch (`runtime_config.disable_*` / moderation_logging=false)
// 2) Feature toggle (`features.logging.*`)
// 3) Channel resolution and validation
// 4) Intent requirements
//
// This means emergency runtime disables always override product-level feature flags.
func ShouldEmitLogEvent(session *discordgo.Session, configManager *files.ConfigManager, eventType LogEventType, guildID string) EmitDecision {
	capability, ok := logEventCapabilities[eventType]
	if !ok {
		return EmitDecision{EventType: eventType, Enabled: false, Reason: EmitReasonUnknownEvent}
	}

	decision := EmitDecision{
		EventType:  eventType,
		Category:   capability.Category,
		Enabled:    false,
		Reason:     EmitReasonConfigUnavailable,
		Capability: capability,
	}

	if configManager == nil {
		decision.Reason = EmitReasonConfigManagerUnavailable
		return decision
	}
	cfg := configManager.Config()
	if cfg == nil {
		decision.Reason = EmitReasonConfigUnavailable
		return decision
	}
	gcfg := configManager.GuildConfig(guildID)
	if gcfg == nil {
		decision.Reason = EmitReasonGuildConfigMissing
		return decision
	}

	rc := cfg.ResolveRuntimeConfig(guildID)
	features := cfg.ResolveFeatures(guildID)

	switch eventType {
	case LogEventAvatarChange, LogEventRoleChange:
		if rc.DisableUserLogs {
			decision.Reason = EmitReasonRuntimeDisableUserLogs
			return decision
		}
		if !features.Logging.User {
			decision.Reason = EmitReasonFeatureLoggingUserDisabled
			return decision
		}
	case LogEventMemberJoin, LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			decision.Reason = EmitReasonRuntimeDisableEntryExitLogs
			return decision
		}
		if !features.Logging.EntryExit {
			decision.Reason = EmitReasonFeatureLoggingEntryExitDisabled
			return decision
		}
	case LogEventMessageProcess, LogEventMessageEdit, LogEventMessageDelete:
		if rc.DisableMessageLogs {
			decision.Reason = EmitReasonRuntimeDisableMessageLogs
			return decision
		}
		if !features.Logging.Message {
			decision.Reason = EmitReasonFeatureLoggingMessageDisabled
			return decision
		}
	case LogEventReactionMetric:
		if rc.DisableReactionLogs {
			decision.Reason = EmitReasonRuntimeDisableReactionLogs
			return decision
		}
		if !features.Logging.Reaction {
			decision.Reason = EmitReasonFeatureLoggingReactionDisabled
			return decision
		}
	case LogEventAutomodAction:
		if rc.DisableAutomodLogs {
			decision.Reason = EmitReasonRuntimeDisableAutomodLogs
			return decision
		}
		if !features.Logging.Automod {
			decision.Reason = EmitReasonFeatureLoggingAutomodDisabled
			return decision
		}
	case LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			decision.Reason = EmitReasonRuntimeModerationLoggingOff
			return decision
		}
		if !features.Logging.Moderation {
			decision.Reason = EmitReasonFeatureLoggingModerationDisabled
			return decision
		}
	}

	if capability.RequiresChannel {
		channelID := resolveLogChannelFromCapability(gcfg, capability)
		if channelID == "" {
			decision.Reason = EmitReasonNoChannelConfigured
			return decision
		}
		decision.ChannelID = channelID

		if capability.ValidateChannelPerms {
			if capability.RequireExclusiveModeration && isSharedModerationChannel(channelID, gcfg) {
				decision.Reason = EmitReasonChannelInvalid
				return decision
			}
			botID := ""
			if session != nil && session.State != nil && session.State.User != nil {
				botID = session.State.User.ID
			}
			if err := validateModerationLogChannel(session, guildID, channelID, botID); err != nil {
				decision.Reason = EmitReasonChannelInvalid
				return decision
			}
		}
	}

	if capability.RequiredIntentsMask != 0 && session != nil {
		currentMask := int(session.Identify.Intents)
		missing := capability.RequiredIntentsMask &^ currentMask
		if missing != 0 {
			decision.Reason = EmitReasonMissingIntent
			decision.MissingMask = missing
			return decision
		}
	}

	decision.Enabled = true
	decision.Reason = EmitReasonEnabled
	return decision
}

func resolveLogChannelFromCapability(gcfg *files.GuildConfig, capability LogEventCapability) string {
	if gcfg == nil {
		return ""
	}
	channelID := resolveLogChannelKey(gcfg, capability.PreferredChannel)
	if channelID != "" {
		return channelID
	}
	for _, fallback := range capability.FallbackChannels {
		channelID = resolveLogChannelKey(gcfg, fallback)
		if channelID != "" {
			return channelID
		}
	}
	return ""
}

func resolveLogChannelKey(gcfg *files.GuildConfig, key logChannelKey) string {
	if gcfg == nil {
		return ""
	}
	switch key {
	case logChannelUserActivity:
		return strings.TrimSpace(gcfg.Channels.UserActivityLog)
	case logChannelEntryLeave:
		return strings.TrimSpace(gcfg.Channels.EntryLeaveLog)
	case logChannelMessageAudit:
		return strings.TrimSpace(gcfg.Channels.MessageAuditLog)
	case logChannelCommands:
		return strings.TrimSpace(gcfg.Channels.Commands)
	case logChannelAutomod:
		return strings.TrimSpace(gcfg.Channels.AutomodLog)
	case logChannelModeration:
		return strings.TrimSpace(gcfg.Channels.ModerationLog)
	default:
		return ""
	}
}
