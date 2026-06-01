package logpolicy

import (
	"fmt"
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

	if reason, disabled := evaluateEventToggle(eventType, rc, features); disabled {
		decision.Reason = reason
		return decision
	}

	if capability.RequiresChannel {
		channelID, reason, ok := resolveValidatedLogChannel(session, capability, eventType, guildID, gcfg)
		if channelID != "" {
			decision.ChannelID = channelID
		}
		if !ok {
			decision.Reason = reason
			return decision
		}
	}

	if reason, mask, gated := evaluateIntentRequirements(capability, session); gated {
		decision.Reason = reason
		decision.MissingMask = mask
		return decision
	}

	decision.Enabled = true
	decision.Reason = EmitReasonEnabled
	return decision
}

// evaluateEventToggle applies the per-event runtime kill switch and feature toggle in
// that precedence order. It returns the disabling reason and true when the event is
// gated off, or ("", false) when neither toggle blocks emission.
func evaluateEventToggle(eventType LogEventType, rc files.RuntimeConfig, features files.ResolvedFeatureToggles) (EmitReason, bool) {
	switch eventType {
	case LogEventAvatarChange:
		if rc.DisableUserLogs {
			return EmitReasonRuntimeDisableUserLogs, true
		}
		if !features.Logging.AvatarLogging {
			return EmitReasonFeatureLoggingUserDisabled, true
		}
	case LogEventRoleChange:
		if rc.DisableUserLogs {
			return EmitReasonRuntimeDisableUserLogs, true
		}
		if !features.Logging.RoleUpdate {
			return EmitReasonFeatureLoggingUserDisabled, true
		}
	case LogEventMemberJoin:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
		if !features.Logging.MemberJoin {
			return EmitReasonFeatureLoggingEntryExitDisabled, true
		}
	case LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
		if !features.Logging.MemberLeave {
			return EmitReasonFeatureLoggingEntryExitDisabled, true
		}
	case LogEventMessageProcess:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
		if !features.Logging.MessageProcess {
			return EmitReasonFeatureLoggingMessageDisabled, true
		}
	case LogEventMessageEdit:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
		if !features.Logging.MessageEdit {
			return EmitReasonFeatureLoggingMessageDisabled, true
		}
	case LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
		if !features.Logging.MessageDelete {
			return EmitReasonFeatureLoggingMessageDisabled, true
		}
	case LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return EmitReasonRuntimeDisableReactionLogs, true
		}
		if !features.Logging.ReactionMetric {
			return EmitReasonFeatureLoggingReactionDisabled, true
		}
	case LogEventAutomodAction:
		if rc.DisableAutomodLogs {
			return EmitReasonRuntimeDisableAutomodLogs, true
		}
		if !features.Logging.AutomodAction {
			return EmitReasonFeatureLoggingAutomodDisabled, true
		}
	case LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return EmitReasonRuntimeModerationLoggingOff, true
		}
		if !features.Logging.ModerationCase {
			return EmitReasonFeatureLoggingModerationDisabled, true
		}
	case LogEventCleanAction:
		if rc.DisableCleanLog {
			return EmitReasonRuntimeDisableCleanLog, true
		}
		if !features.Logging.CleanAction {
			return EmitReasonFeatureLoggingCleanDisabled, true
		}
	}
	return "", false
}

// resolveValidatedLogChannel resolves the destination channel for eventType and, when the
// capability requires it, validates exclusivity and bot permissions. The resolved channel
// is returned even on a validation failure so the caller can still surface it; ok reports
// whether the channel is usable.
func resolveValidatedLogChannel(session *discordgo.Session, capability LogEventCapability, eventType LogEventType, guildID string, gcfg *files.GuildConfig) (string, EmitReason, bool) {
	channelID := resolveLogChannelForGuild(eventType, gcfg)
	if channelID == "" {
		return "", EmitReasonNoChannelConfigured, false
	}
	if capability.ValidateChannelPerms {
		if capability.RequireExclusiveModeration && IsSharedModerationChannel(channelID, gcfg) {
			return channelID, EmitReasonChannelInvalid, false
		}
		botID := ""
		if session != nil && session.State != nil && session.State.User != nil {
			botID = session.State.User.ID
		}
		if err := ValidateModerationLogChannel(session, guildID, channelID, botID); err != nil {
			return channelID, EmitReasonChannelInvalid, false
		}
	}
	return channelID, EmitReasonEnabled, true
}

// evaluateIntentRequirements reports whether the capability's required gateway intents are
// missing from the active session, returning the missing-bit mask when they are.
func evaluateIntentRequirements(capability LogEventCapability, session *discordgo.Session) (EmitReason, int, bool) {
	if capability.RequiredIntentsMask == 0 || session == nil {
		return "", 0, false
	}
	currentMask := int(session.Identify.Intents)
	missing := capability.RequiredIntentsMask &^ currentMask
	if missing != 0 {
		return EmitReasonMissingIntent, missing, true
	}
	return "", 0, false
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

func IsSharedModerationChannel(channelID string, gcfg *files.GuildConfig) bool {
	channelID = strings.TrimSpace(channelID)
	if gcfg == nil || channelID == "" {
		return false
	}
	sharedCandidates := []string{
		gcfg.Channels.Commands,
		gcfg.Channels.AvatarLogging,
		gcfg.Channels.RoleUpdate,
		gcfg.Channels.MemberJoin,
		gcfg.Channels.MemberLeave,
		gcfg.Channels.MessageEdit,
		gcfg.Channels.MessageDelete,
		gcfg.Channels.AutomodAction,
		gcfg.Channels.CleanAction,
	}
	for _, candidate := range sharedCandidates {
		if strings.TrimSpace(candidate) == channelID {
			return true
		}
	}
	return false
}

func ValidateModerationLogChannel(session *discordgo.Session, guildID, channelID, botID string) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	if guildID == "" || channelID == "" {
		return fmt.Errorf("missing guildID or channelID")
	}

	var ch *discordgo.Channel
	if session.State != nil {
		if cached, _ := session.State.Channel(channelID); cached != nil {
			ch = cached
		}
	}
	if ch == nil {
		c, err := session.Channel(channelID)
		if err != nil {
			return fmt.Errorf("channel lookup failed: %w", err)
		}
		ch = c
	}

	if ch == nil {
		return fmt.Errorf("channel not found")
	}
	if ch.GuildID != "" && ch.GuildID != guildID {
		return fmt.Errorf("channel guild mismatch")
	}
	if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
		return fmt.Errorf("channel is not a guild text channel")
	}

	if botID == "" {
		return fmt.Errorf("bot identity not available")
	}

	perms, err := session.UserChannelPermissions(botID, channelID)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	required := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
	if perms&required != required {
		return fmt.Errorf("missing permissions (need view/send/embed)")
	}
	return nil
}
