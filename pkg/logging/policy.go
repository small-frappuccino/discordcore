package logging

import (
	"fmt"
	"strings"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// LogEventType identifies an internal logging event kind.
type LogEventType string

// LogEventRoleChange defines log event role change.
// LogEventMemberJoin defines log event member join.
// LogEventMessageProcess defines log event message process.
// LogEventMessageEdit defines log event message edit.
// LogEventMessageDelete defines log event message delete.
// LogEventReactionMetric defines log event reaction metric.
// LogEventAutomodAction defines log event automod action.
// LogEventModerationCase defines log event moderation case.
// LogEventMemberLeave defines log event member leave.
// LogEventAvatarChange defines log event avatar change.
// LogEventCleanAction defines log event clean action.
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

// LogCategoryAutomod defines log category automod.
// LogCategoryModeration defines log category moderation.
// LogCategoryUser defines log category user.
// LogCategoryReaction defines log category reaction.
// LogCategoryMessage defines log category message.
const (
	LogCategoryUser       LogEventCategory = "user"
	LogCategoryMessage    LogEventCategory = "message"
	LogCategoryReaction   LogEventCategory = "reaction"
	LogCategoryAutomod    LogEventCategory = "automod"
	LogCategoryModeration LogEventCategory = "moderation"
)

// EmitReason is a deterministic reason for a should-emit decision.
type EmitReason string

// EmitReasonFeatureLoggingMessageDisabled defines emit reason feature logging message disabled.
// EmitReasonFeatureLoggingAutomodDisabled defines emit reason feature logging automod disabled.
// EmitReasonFeatureLoggingUserDisabled defines emit reason feature logging user disabled.
// EmitReasonFeatureLoggingModerationDisabled defines emit reason feature logging moderation disabled.
// EmitReasonConfigUnavailable defines emit reason config unavailable.
// EmitReasonConfigManagerUnavailable defines emit reason config manager unavailable.
// EmitReasonUnknownEvent defines emit reason unknown event.
// EmitReasonEnabled defines emit reason enabled.
// EmitReasonFeatureLoggingCleanDisabled defines emit reason feature logging clean disabled.
// EmitReasonRuntimeDisableUserLogs defines emit reason runtime disable user logs.
// EmitReasonFeatureLoggingReactionDisabled defines emit reason feature logging reaction disabled.
// EmitReasonGuildConfigMissing defines emit reason guild config missing.
// EmitReasonFeatureLoggingEntryExitDisabled defines emit reason feature logging entry exit disabled.
// EmitReasonRuntimeDisableReactionLogs defines emit reason runtime disable reaction logs.
// EmitReasonRuntimeModerationLoggingOff defines emit reason runtime moderation logging off.
// EmitReasonRuntimeDisableCleanLog defines emit reason runtime disable clean log.
// EmitReasonNoChannelConfigured defines emit reason no channel configured.
// EmitReasonMissingIntent defines emit reason missing intent.
// EmitReasonChannelInvalid defines emit reason channel invalid.
// EmitReasonRuntimeDisableEntryExitLogs defines emit reason runtime disable entry exit logs.
// EmitReasonRuntimeDisableMessageLogs defines emit reason runtime disable message logs.
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
	RequiredIntentsMask gateway.Intents
	RequiredPermsMask   discord.Permissions
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
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.avatar_logging"},
	},
	LogEventRoleChange: {
		EventType:           LogEventRoleChange,
		Category:            LogCategoryUser,
		RequiredIntentsMask: gateway.IntentGuildMembers,
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_user_logs", "features.logging.role_update"},
	},
	LogEventMemberJoin: {
		EventType:           LogEventMemberJoin,
		Category:            LogCategoryUser,
		RequiredIntentsMask: gateway.IntentGuildMembers,
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_join"},
	},
	LogEventMemberLeave: {
		EventType:           LogEventMemberLeave,
		Category:            LogCategoryUser,
		RequiredIntentsMask: gateway.IntentGuildMembers,
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_entry_exit_logs", "features.logging.member_leave"},
	},
	LogEventMessageProcess: {
		EventType:           LogEventMessageProcess,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: gateway.IntentGuildMessages,
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_process"},
	},
	LogEventMessageEdit: {
		EventType:           LogEventMessageEdit,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: gateway.IntentGuildMessages,
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_edit"},
	},
	LogEventMessageDelete: {
		EventType:           LogEventMessageDelete,
		Category:            LogCategoryMessage,
		RequiredIntentsMask: gateway.IntentGuildMessages,
		RequiredPermsMask:   discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:     true,
		Toggles:             []string{"runtime_config.disable_message_logs", "features.logging.message_delete"},
	},
	LogEventReactionMetric: {
		EventType:           LogEventReactionMetric,
		Category:            LogCategoryReaction,
		RequiredIntentsMask: gateway.IntentGuildMessageReactions,
		RequiredPermsMask:   0,
		RequiresChannel:     false,
		Toggles:             []string{"runtime_config.disable_reaction_logs", "features.logging.reaction_metric"},
	},
	LogEventAutomodAction: {
		EventType:            LogEventAutomodAction,
		Category:             LogCategoryAutomod,
		RequiredIntentsMask:  gateway.Intents(1 << 20), // IntentAutoModerationExecution
		RequiredPermsMask:    discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:      true,
		Toggles:              []string{"runtime_config.disable_automod_logs", "features.logging.automod_action"},
		ValidateChannelPerms: true,
	},
	LogEventModerationCase: {
		EventType:                  LogEventModerationCase,
		Category:                   LogCategoryModeration,
		RequiredIntentsMask:        0,
		RequiredPermsMask:          discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
		RequiresChannel:            true,
		Toggles:                    []string{"runtime_config.moderation_logging", "features.logging.moderation_case"},
		ValidateChannelPerms:       true,
		RequireExclusiveModeration: true,
	},
	LogEventCleanAction: {
		EventType:            LogEventCleanAction,
		Category:             LogCategoryModeration,
		RequiredIntentsMask:  0,
		RequiredPermsMask:    discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks,
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

// CheckFeatureEnabled only verifies if the configuration allows the event to be emitted.
// It DOES NOT check intents or permissions. It should be used by the domain layer to determine if
// an event should be processed.
func CheckFeatureEnabled(configManager *files.ConfigManager, eventType LogEventType, guildID string) EmitDecision {
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
		channelID := resolveLogChannelForGuild(eventType, gcfg)
		if channelID == "" {
			decision.Reason = EmitReasonNoChannelConfigured
			return decision
		}
		decision.ChannelID = channelID
	}

	decision.Enabled = true
	decision.Reason = EmitReasonEnabled
	return decision
}

// ValidateLogCapability checks gateway intents and channel permissions.
// It expects CheckFeatureEnabled to have returned Enabled = true.
func ValidateLogCapability(state *state.State, currentIntents gateway.Intents, decision EmitDecision, guildID string, configManager *files.ConfigManager) (EmitReason, gateway.Intents, bool) {
	if !decision.Enabled {
		return decision.Reason, 0, false
	}

	if decision.Capability.RequiresChannel && decision.ChannelID != "" {
		gcfg := configManager.GuildConfig(guildID)
		if reason, ok := validateResolvedLogChannel(state, decision.Capability, decision.ChannelID, guildID, gcfg); !ok {
			return reason, 0, false
		}
	}

	if reason, mask, gated := evaluateIntentRequirements(decision.Capability, currentIntents); gated {
		return reason, mask, false
	}

	return EmitReasonEnabled, 0, true
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
	case LogEventRoleChange:
		if rc.DisableUserLogs {
			return EmitReasonRuntimeDisableUserLogs, true
		}
	case LogEventMemberJoin:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
	case LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return EmitReasonRuntimeDisableEntryExitLogs, true
		}
	case LogEventMessageProcess:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventMessageEdit:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return EmitReasonRuntimeDisableMessageLogs, true
		}
	case LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return EmitReasonRuntimeDisableReactionLogs, true
		}
	case LogEventAutomodAction:
		// No runtime config disable override for automod logs.
	case LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return EmitReasonRuntimeModerationLoggingOff, true
		}
	case LogEventCleanAction:
		if rc.DisableCleanLog {
			return EmitReasonRuntimeDisableCleanLog, true
		}
	}
	return "", false
}

// validateResolvedLogChannel validates exclusivity and bot permissions for a resolved channel.
func validateResolvedLogChannel(st *state.State, capability LogEventCapability, channelID string, guildID string, gcfg *files.GuildConfig) (EmitReason, bool) {
	if !capability.ValidateChannelPerms {
		return EmitReasonEnabled, true
	}
	if capability.RequireExclusiveModeration && IsSharedModerationChannel(channelID, gcfg) {
		return EmitReasonChannelInvalid, false
	}

	if err := ValidateModerationLogChannel(st, guildID, channelID); err != nil {
		return EmitReasonChannelInvalid, false
	}
	return EmitReasonEnabled, true
}

// evaluateIntentRequirements reports whether the capability's required gateway intents are
// missing from the active session, returning the missing-bit mask when they are.
func evaluateIntentRequirements(capability LogEventCapability, currentIntents gateway.Intents) (EmitReason, gateway.Intents, bool) {
	if capability.RequiredIntentsMask == 0 {
		return "", 0, false
	}
	missing := capability.RequiredIntentsMask &^ currentIntents
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

// IsSharedModerationChannel is shared moderation channel.
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

// ValidateModerationLogChannel validates moderation log channel.
func ValidateModerationLogChannel(st *state.State, guildID, channelIDStr string) error {
	if st == nil {
		return fmt.Errorf("state is nil")
	}
	if guildID == "" || channelIDStr == "" {
		return fmt.Errorf("missing guildID or channelID")
	}

	channelID, err := discord.ParseSnowflake(channelIDStr)
	if err != nil {
		return fmt.Errorf("invalid channel ID: %w", err)
	}
	guildIDParsed, err := discord.ParseSnowflake(guildID)
	if err != nil {
		return fmt.Errorf("invalid guild ID: %w", err)
	}

	ch, err := st.Channel(discord.ChannelID(channelID))
	if err != nil {
		return fmt.Errorf("channel lookup failed: %w", err)
	}

	if ch.GuildID != discord.GuildID(guildIDParsed) {
		return fmt.Errorf("channel guild mismatch")
	}
	if ch.Type != discord.GuildText && ch.Type != discord.GuildNews {
		return fmt.Errorf("channel is not a guild text channel")
	}

	me, err := st.Me()
	if err != nil || me == nil {
		return fmt.Errorf("bot identity not available")
	}

	perms, err := st.Permissions(discord.ChannelID(channelID), me.ID)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	required := discord.PermissionViewChannel | discord.PermissionSendMessages | discord.PermissionEmbedLinks
	if perms&required != required {
		return fmt.Errorf("missing permissions (need view/send/embed)")
	}
	return nil
}
