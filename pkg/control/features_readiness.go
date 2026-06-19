package control

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordgo"
)

// readinessInput carries everything a per-feature readiness checker needs,
// avoiding a per-case ResolveRuntimeConfig recomputation.
type readinessInput struct {
	cfg     *files.BotConfig
	guildID string
	rc      files.RuntimeConfig
	session *discordgo.Session
}

func buildFeatureReadiness(
	cfg *files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	def featureDefinition,
	effectiveEnabled bool,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if !effectiveEnabled {
		return "disabled", nil
	}
	if def.LogEvent != "" {
		return buildLogFeatureReadiness(cfg, configManager, guildID, def.LogEvent, session)
	}

	checker, ok := featureReadinessCheckers[def.ID]
	if !ok {
		return "ready", nil
	}
	return checker(readinessInput{
		cfg:     cfg,
		guildID: guildID,
		rc:      cfg.ResolveRuntimeConfig(guildID),
		session: session,
	})
}

// withGuildSettings runs fn with the guild settings when the request carries a
// registered guild scope; otherwise it returns the default "ready" result (no
// blocking at global scope or for an unregistered guild), preserving the prior
// per-case behavior.
func withGuildSettings(
	cfg *files.BotConfig,
	guildID string,
	fn func(guild files.GuildConfig) (string, []featureBlocker),
) (string, []featureBlocker) {
	if guildID == "" {
		return "ready", nil
	}
	guild, ok := findGuildSettings(*cfg, guildID)
	if !ok {
		return "ready", nil
	}
	return fn(guild)
}

// requireRolesExist returns a blocker when any of the given role IDs is no
// longer present in the guild role index. An unavailable session or index
// yields no blocker, matching the prior "if ...; err == nil" guard.
func requireRolesExist(
	session *discordgo.Session,
	guildID, code, message, field string,
	roleIDs ...string,
) (featureBlocker, bool) {
	roleIndex, err := guildRoleOptionIndex(session, guildID)
	if err != nil {
		return featureBlocker{}, false
	}
	for _, id := range roleIDs {
		if _, ok := roleIndex[strings.TrimSpace(id)]; !ok {
			return featureBlocker{Code: code, Message: message, Field: field}, true
		}
	}
	return featureBlocker{}, false
}

// commandsReadinessGate gates the moderation command features behind the
// Commands service; it is registered for every moderation command ID.
var commandsReadinessGate = func(in readinessInput) (string, []featureBlocker) {
	if !in.cfg.ResolveFeatures(in.guildID).Services.Commands {
		return "blocked", []featureBlocker{{Code: "commands_disabled", Message: "Enable the Commands service before using moderation commands."}}
	}
	return "ready", nil
}

// featureReadinessCheckers maps a feature ID to its readiness checker. Features
// absent from the map (and not log-backed) are always "ready".
var featureReadinessCheckers = map[string]func(readinessInput) (string, []featureBlocker){
	"moderation.mute_role": func(in readinessInput) (string, []featureBlocker) {
		return withGuildSettings(in.cfg, in.guildID, func(guild files.GuildConfig) (string, []featureBlocker) {
			roleID := strings.TrimSpace(guild.Roles.MuteRole)
			if roleID == "" {
				return "blocked", []featureBlocker{{Code: "missing_role", Message: "Choose the role that should be applied by the mute command.", Field: "role_id"}}
			}
			if blocker, blocked := requireRolesExist(in.session, in.guildID, "invalid_role", "The configured mute role is no longer available in this server.", "role_id", roleID); blocked {
				return "blocked", []featureBlocker{blocker}
			}
			return "ready", nil
		})
	},
	"moderation.ban":      commandsReadinessGate,
	"moderation.massban":  commandsReadinessGate,
	"moderation.kick":     commandsReadinessGate,
	"moderation.timeout":  commandsReadinessGate,
	"moderation.warn":     commandsReadinessGate,
	"moderation.warnings": commandsReadinessGate,
	"message_cache.cleanup_on_startup": func(in readinessInput) (string, []featureBlocker) {
		if !in.rc.MessageCacheCleanup {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime message cache cleanup is disabled."}}
		}
		return "ready", nil
	},
	"message_cache.delete_on_log": func(in readinessInput) (string, []featureBlocker) {
		if !in.rc.MessageDeleteOnLog {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime delete-on-log is disabled."}}
		}
		return "ready", nil
	},
	"presence_watch.bot": func(in readinessInput) (string, []featureBlocker) {
		if !in.rc.PresenceWatchBot {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime bot presence watching is disabled.", Field: "watch_bot"}}
		}
		return "ready", nil
	},
	"presence_watch.user": func(in readinessInput) (string, []featureBlocker) {
		if strings.TrimSpace(in.rc.PresenceWatchUserID) == "" {
			return "blocked", []featureBlocker{{Code: "missing_user_id", Message: "Presence watch needs a user ID.", Field: "user_id"}}
		}
		return "ready", nil
	},
	"safety.bot_role_perm_mirror": func(in readinessInput) (string, []featureBlocker) {
		if in.rc.DisableBotRolePermMirror {
			return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "Runtime permission mirroring is disabled."}}
		}
		if in.guildID != "" {
			actorRoleID := strings.TrimSpace(in.rc.BotRolePermMirrorActorRoleID)
			if actorRoleID != "" {
				if blocker, blocked := requireRolesExist(in.session, in.guildID, "invalid_actor_role", "Permission mirror actor role is no longer available in this server.", "actor_role_id", actorRoleID); blocked {
					return "blocked", []featureBlocker{blocker}
				}
			}
		}
		return "ready", nil
	},
	"backfill.enabled": func(in readinessInput) (string, []featureBlocker) {
		channelID := strings.TrimSpace(in.rc.BackfillChannelID)
		if in.guildID != "" {
			if guild, ok := findGuildSettings(*in.cfg, in.guildID); ok {
				channelID = strings.TrimSpace(guild.Channels.BackfillChannelID())
			}
		}
		if channelID == "" {
			return "blocked", []featureBlocker{{Code: "missing_channel", Message: "Backfill needs a configured source channel.", Field: "channel_id"}}
		}
		if strings.TrimSpace(in.rc.BackfillStartDay) == "" && strings.TrimSpace(in.rc.BackfillInitialDate) == "" {
			return "blocked", []featureBlocker{{Code: "missing_schedule_seed", Message: "Backfill needs start_day or initial_date configured.", Field: "start_day"}}
		}
		return "ready", nil
	},
	"stats_channels": func(in readinessInput) (string, []featureBlocker) {
		return withGuildSettings(in.cfg, in.guildID, func(guild files.GuildConfig) (string, []featureBlocker) {
			if len(guild.Stats.Channels) == 0 {
				return "blocked", []featureBlocker{{Code: "missing_channels", Message: "Stats channels need at least one configured target."}}
			}
			return "ready", nil
		})
	},
	"auto_role_assignment": func(in readinessInput) (string, []featureBlocker) {
		return withGuildSettings(in.cfg, in.guildID, func(guild files.GuildConfig) (string, []featureBlocker) {
			auto := guild.Roles.AutoAssignment
			if !auto.Enabled {
				return "blocked", []featureBlocker{{Code: "config_disabled", Message: "Auto assignment config is disabled.", Field: "config_enabled"}}
			}
			if strings.TrimSpace(auto.TargetRoleID) == "" {
				return "blocked", []featureBlocker{{Code: "missing_target_role", Message: "Auto assignment needs a target role.", Field: "target_role_id"}}
			}
			if len(auto.RequiredRoles) != 2 {
				return "blocked", []featureBlocker{{Code: "invalid_required_roles", Message: "Auto assignment needs exactly two required roles in order.", Field: "required_role_ids"}}
			}
			if blocker, blocked := requireRolesExist(in.session, in.guildID, "invalid_target_role", "Auto assignment target role is no longer available in this server.", "target_role_id", auto.TargetRoleID); blocked {
				return "blocked", []featureBlocker{blocker}
			}
			if blocker, blocked := requireRolesExist(in.session, in.guildID, "invalid_required_roles", "Auto assignment required roles are no longer available in this server.", "required_role_ids", auto.RequiredRoles...); blocked {
				return "blocked", []featureBlocker{blocker}
			}
			return "ready", nil
		})
	},
	"user_prune": func(in readinessInput) (string, []featureBlocker) {
		return withGuildSettings(in.cfg, in.guildID, func(guild files.GuildConfig) (string, []featureBlocker) {
			if !guild.UserPrune.Enabled {
				return "blocked", []featureBlocker{{Code: "config_disabled", Message: "User prune config is disabled.", Field: "config_enabled"}}
			}
			return "ready", nil
		})
	},
}

func buildLogFeatureReadiness(
	cfg *files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	eventType logging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if guildID == "" {
		return buildGlobalLogFeatureReadiness(cfg, eventType, session)
	}
	if configManager == nil {
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Config manager is unavailable."}}
	}

	decision := logging.CheckFeatureEnabled(configManager, eventType, guildID)
	if !decision.Enabled {
		return logDecisionToReadiness(decision)
	}

	if decision.Capability.RequiresChannel && decision.ChannelID == "" {
		decision.Reason = logging.EmitReasonNoChannelConfigured
		return logDecisionToReadiness(decision)
	}

	if decision.Capability.RequireExclusiveModeration && decision.ChannelID != "" {
		gcfg := configManager.GuildConfig(guildID)
		if logging.IsSharedModerationChannel(decision.ChannelID, gcfg) {
			decision.Reason = logging.EmitReasonChannelInvalid
			return logDecisionToReadiness(decision)
		}
	}

	if decision.Capability.RequiredIntentsMask != 0 && session != nil {
		currentMask := int(session.Identify.Intents)
		missing := int(decision.Capability.RequiredIntentsMask) &^ currentMask
		if missing != 0 {
			decision.Reason = logging.EmitReasonMissingIntent
			return logDecisionToReadiness(decision)
		}
	}

	return "ready", nil
}

func buildGlobalLogFeatureReadiness(
	cfg *files.BotConfig,
	eventType logging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if blocker, ok := globalLogRuntimeBlocker(cfg.ResolveRuntimeConfig(""), eventType); ok {
		return "blocked", []featureBlocker{blocker}
	}

	capability, ok := logging.LogEventCapabilities()[eventType]
	if ok && capability.RequiredIntentsMask != 0 && session != nil {
		currentMask := int(session.Identify.Intents)
		missing := int(capability.RequiredIntentsMask) &^ currentMask
		if missing != 0 {
			return "blocked", []featureBlocker{{
				Code:    "missing_intent",
				Message: fmt.Sprintf("Gateway intents mask %d is missing required bits %d.", currentMask, missing),
			}}
		}
	}
	return "ready", nil
}

func logDecisionToReadiness(decision logging.EmitDecision) (string, []featureBlocker) {
	switch decision.Reason {
	case logging.EmitReasonRuntimeDisableUserLogs,
		logging.EmitReasonRuntimeDisableEntryExitLogs,
		logging.EmitReasonRuntimeDisableMessageLogs,
		logging.EmitReasonRuntimeDisableReactionLogs,
		logging.EmitReasonRuntimeModerationLoggingOff,
		logging.EmitReasonRuntimeDisableCleanLog:
		return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "A runtime kill switch currently disables this feature."}}
	case logging.EmitReasonNoChannelConfigured:
		return "blocked", []featureBlocker{{Code: "missing_channel", Message: "A channel must be configured for this feature.", Field: "channel_id"}}
	case logging.EmitReasonMissingIntent:
		return "blocked", []featureBlocker{{Code: "missing_intent", Message: fmt.Sprintf("Gateway intents are missing required bits %d.", decision.MissingMask)}}
	case logging.EmitReasonChannelInvalid:
		return "blocked", []featureBlocker{{Code: "invalid_channel", Message: "The configured channel failed validation for this feature.", Field: "channel_id"}}
	case logging.EmitReasonGuildConfigMissing:
		return "blocked", []featureBlocker{{Code: "missing_guild_registration", Message: "This guild is not registered in settings yet."}}
	case logging.EmitReasonConfigManagerUnavailable, logging.EmitReasonConfigUnavailable:
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Feature config is unavailable."}}
	default:
		return "blocked", []featureBlocker{{Code: "blocked", Message: string(decision.Reason)}}
	}
}

func globalLogRuntimeBlocker(rc files.RuntimeConfig, eventType logging.LogEventType) (featureBlocker, bool) {
	switch eventType {
	case logging.LogEventAvatarChange, logging.LogEventRoleChange:
		if rc.DisableUserLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime user logging is disabled."}, true
		}
	case logging.LogEventMemberJoin, logging.LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime entry/exit logging is disabled."}, true
		}
	case logging.LogEventMessageProcess, logging.LogEventMessageEdit, logging.LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime message logging is disabled."}, true
		}
	case logging.LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime reaction logging is disabled."}, true
		}
	case logging.LogEventAutomodAction:
	case logging.LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime moderation logging is disabled."}, true
		}
	case logging.LogEventCleanAction:
		if rc.DisableCleanLog {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime clean logging is disabled."}, true
		}
	}
	return featureBlocker{}, false
}
