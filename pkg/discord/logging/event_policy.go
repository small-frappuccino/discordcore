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
	LogEventCleanAction    LogEventType = "clean_action"
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
	EmitReasonFeatureLoggingCleanDisabled      EmitReason = "feature_logging_clean_disabled"
	EmitReasonRuntimeDisableUserLogs           EmitReason = "runtime_disable_user_logs"
	EmitReasonRuntimeDisableEntryExitLogs      EmitReason = "runtime_disable_entry_exit_logs"
	EmitReasonRuntimeDisableMessageLogs        EmitReason = "runtime_disable_message_logs"
	EmitReasonRuntimeDisableReactionLogs       EmitReason = "runtime_disable_reaction_logs"
	EmitReasonRuntimeDisableAutomodLogs        EmitReason = "runtime_disable_automod_logs"
	EmitReasonRuntimeModerationLoggingOff      EmitReason = "runtime_moderation_logging_off"
	EmitReasonRuntimeDisableCleanLog           EmitReason = "runtime_disable_clean_log"
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
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.avatar_logging"},
	},
	LogEventRoleChange: {
		EventType:           LogEventRoleChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.role_update"},
	},
	LogEventMemberJoin: {
		EventType:           LogEventMemberJoin,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_join"},
	},
	LogEventMemberLeave: {
		EventType:           LogEventMemberLeave,
		Category:            LogCategoryUser,
		RequiredIntentsMask: int(discordgo.IntentsGuildMembers),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_leave"},
	},
	LogEventMessageProcess: {
		EventType:           LogEventMessageProcess,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_process"},
	},
	LogEventMessageEdit: {
		EventType:           LogEventMessageEdit,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_edit"},
	},
	LogEventMessageDelete: {
		EventType:           LogEventMessageDelete,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessages),
		RequiredPermsMask:   int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_delete"},
	},
	LogEventReactionMetric: {
		EventType:           LogEventReactionMetric,
		Category:            LogCategoryReaction,
		RequiredIntentsMask: int(discordgo.IntentsGuildMessageReactions),
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_reaction_logs", "features.logging.reaction_metric"},
	},
	LogEventAutomodAction: {
		EventType:            LogEventAutomodAction,
		Category:             LogCategoryAutomod,
		RequiredIntentsMask:  int(discordgo.IntentAutoModerationExecution),
		RequiredPermsMask:    int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:      true,
		Toggles:              []string{"runtime_config.disable_automod_logs", "features.logging.automod_action"},
		ValidateChannelPerms: true,
	},
	LogEventModerationCase: {
		EventType:                  LogEventModerationCase,
		Category:                   LogCategoryModeration,
		RequiredIntentsMask:        0,
		RequiredPermsMask:          int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:            true,
		Toggles:                    []string{"runtime_config.moderation_logging", "features.logging.moderation_case"},
		ValidateChannelPerms:       true,
		RequireExclusiveModeration: true,
	},
	LogEventCleanAction: {
		EventType:            LogEventCleanAction,
		Category:             LogCategoryModeration,
		RequiredIntentsMask:  0,
		RequiredPermsMask:    int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks),
		RequiresChannel:      true,
		Toggles:              []string{"runtime_config.disable_clean_log", "features.logging.clean_action"},
		ValidateChannelPerms: true,
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

// ResolveLogChannel returns the resolved channel ID for an event in a guild.
// Resolution is deterministic and event-specific.
func ResolveLogChannel(eventType LogEventType, guildID string, configManager *files.ConfigManager) string {
	if guildID == "" || configManager == nil {
		return ""
	}
	gcfg := configManager.GuildConfig(guildID)
	return resolveLogChannelForGuild(eventType, gcfg)
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
	case LogEventAvatarChange:
		if rc.DisableUserLogs {
			decision.Reason = EmitReasonRuntimeDisableUserLogs
			return decision
		}
		if !features.Logging.AvatarLogging {
			decision.Reason = EmitReasonFeatureLoggingUserDisabled
			return decision
		}
	case LogEventRoleChange:
		if rc.DisableUserLogs {
			decision.Reason = EmitReasonRuntimeDisableUserLogs
			return decision
		}
		if !features.Logging.RoleUpdate {
			decision.Reason = EmitReasonFeatureLoggingUserDisabled
			return decision
		}
	case LogEventMemberJoin:
		if rc.DisableEntryExitLogs {
			decision.Reason = EmitReasonRuntimeDisableEntryExitLogs
			return decision
		}
		if !features.Logging.MemberJoin {
			decision.Reason = EmitReasonFeatureLoggingEntryExitDisabled
			return decision
		}
	case LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			decision.Reason = EmitReasonRuntimeDisableEntryExitLogs
			return decision
		}
		if !features.Logging.MemberLeave {
			decision.Reason = EmitReasonFeatureLoggingEntryExitDisabled
			return decision
		}
	case LogEventMessageProcess:
		if rc.DisableMessageLogs {
			decision.Reason = EmitReasonRuntimeDisableMessageLogs
			return decision
		}
		if !features.Logging.MessageProcess {
			decision.Reason = EmitReasonFeatureLoggingMessageDisabled
			return decision
		}
	case LogEventMessageEdit:
		if rc.DisableMessageLogs {
			decision.Reason = EmitReasonRuntimeDisableMessageLogs
			return decision
		}
		if !features.Logging.MessageEdit {
			decision.Reason = EmitReasonFeatureLoggingMessageDisabled
			return decision
		}
	case LogEventMessageDelete:
		if rc.DisableMessageLogs {
			decision.Reason = EmitReasonRuntimeDisableMessageLogs
			return decision
		}
		if !features.Logging.MessageDelete {
			decision.Reason = EmitReasonFeatureLoggingMessageDisabled
			return decision
		}
	case LogEventReactionMetric:
		if rc.DisableReactionLogs {
			decision.Reason = EmitReasonRuntimeDisableReactionLogs
			return decision
		}
		if !features.Logging.ReactionMetric {
			decision.Reason = EmitReasonFeatureLoggingReactionDisabled
			return decision
		}
	case LogEventAutomodAction:
		if rc.DisableAutomodLogs {
			decision.Reason = EmitReasonRuntimeDisableAutomodLogs
			return decision
		}
		if !features.Logging.AutomodAction {
			decision.Reason = EmitReasonFeatureLoggingAutomodDisabled
			return decision
		}
	case LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			decision.Reason = EmitReasonRuntimeModerationLoggingOff
			return decision
		}
		if !features.Logging.ModerationCase {
			decision.Reason = EmitReasonFeatureLoggingModerationDisabled
			return decision
		}
	case LogEventCleanAction:
		if rc.DisableCleanLog {
			decision.Reason = EmitReasonRuntimeDisableCleanLog
			return decision
		}
		if !features.Logging.CleanAction {
			decision.Reason = EmitReasonFeatureLoggingCleanDisabled
			return decision
		}
	}

	if capability.RequiresChannel {
		channelID := resolveLogChannelForGuild(eventType, gcfg)
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

func resolveLogChannelForGuild(eventType LogEventType, gcfg *files.GuildConfig) string {
	if gcfg == nil {
		return ""
	}
	channels := gcfg.Channels
	switch eventType {
	case LogEventAvatarChange:
		return firstNonEmptyChannel(channels.AvatarLogging)
	case LogEventRoleChange:
		return firstNonEmptyChannel(channels.RoleUpdate)
	case LogEventMemberJoin:
		return firstNonEmptyChannel(channels.MemberJoin, channels.MemberLeave)
	case LogEventMemberLeave:
		return firstNonEmptyChannel(channels.MemberLeave, channels.MemberJoin)
	case LogEventMessageEdit:
		return firstNonEmptyChannel(channels.MessageEdit, channels.MessageDelete)
	case LogEventMessageDelete:
		return firstNonEmptyChannel(channels.MessageDelete, channels.MessageEdit)
	case LogEventAutomodAction:
		return firstNonEmptyChannel(channels.AutomodAction)
	case LogEventModerationCase:
		return firstNonEmptyChannel(channels.ModerationCase)
	case LogEventCleanAction:
		return firstNonEmptyChannel(channels.CleanAction, channels.ModerationCase)
	default:
		return ""
	}
}

func firstNonEmptyChannel(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
