package control

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	discordlogging "github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

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

	rc := cfg.ResolveRuntimeConfig(guildID)
	switch def.ID {
	case "moderation.mute_role":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				roleID := strings.TrimSpace(guild.Roles.MuteRole)
				if roleID == "" {
					return "blocked", []featureBlocker{{Code: "missing_role", Message: "Choose the role that should be applied by the mute command.", Field: "role_id"}}
				}
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[roleID]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_role", Message: "The configured mute role is no longer available in this server.", Field: "role_id"}}
					}
				}
			}
		}
	case "moderation.ban", "moderation.massban", "moderation.kick", "moderation.timeout", "moderation.warn", "moderation.warnings":
		if !cfg.ResolveFeatures(guildID).Services.Commands {
			return "blocked", []featureBlocker{{Code: "commands_disabled", Message: "Enable the Commands service before using moderation commands."}}
		}
	case "message_cache.cleanup_on_startup":
		if !rc.MessageCacheCleanup {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime message cache cleanup is disabled."}}
		}
	case "message_cache.delete_on_log":
		if !rc.MessageDeleteOnLog {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime delete-on-log is disabled."}}
		}
	case "presence_watch.bot":
		if !rc.PresenceWatchBot {
			return "blocked", []featureBlocker{{Code: "runtime_disabled", Message: "Runtime bot presence watching is disabled.", Field: "watch_bot"}}
		}
	case "presence_watch.user":
		if strings.TrimSpace(rc.PresenceWatchUserID) == "" {
			return "blocked", []featureBlocker{{Code: "missing_user_id", Message: "Presence watch needs a user ID.", Field: "user_id"}}
		}
	case "safety.bot_role_perm_mirror":
		if rc.DisableBotRolePermMirror {
			return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "Runtime permission mirroring is disabled."}}
		}
		if guildID != "" {
			actorRoleID := strings.TrimSpace(rc.BotRolePermMirrorActorRoleID)
			if actorRoleID != "" {
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[actorRoleID]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_actor_role", Message: "Permission mirror actor role is no longer available in this server.", Field: "actor_role_id"}}
					}
				}
			}
		}
	case "backfill.enabled":
		channelID := strings.TrimSpace(rc.BackfillChannelID)
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				channelID = strings.TrimSpace(guild.Channels.BackfillChannelID())
			}
		}
		if channelID == "" {
			return "blocked", []featureBlocker{{Code: "missing_channel", Message: "Backfill needs a configured source channel.", Field: "channel_id"}}
		}
		if strings.TrimSpace(rc.BackfillStartDay) == "" && strings.TrimSpace(rc.BackfillInitialDate) == "" {
			return "blocked", []featureBlocker{{Code: "missing_schedule_seed", Message: "Backfill needs start_day or initial_date configured.", Field: "start_day"}}
		}
	case "stats_channels":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
				if !guild.Stats.Enabled {
					return "blocked", []featureBlocker{{Code: "config_disabled", Message: "Stats channel config is disabled.", Field: "config_enabled"}}
				}
				if len(guild.Stats.Channels) == 0 {
					return "blocked", []featureBlocker{{Code: "missing_channels", Message: "Stats channels need at least one configured target."}}
				}
			}
		}
	case "auto_role_assignment":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok {
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
				if roleIndex, err := guildRoleOptionIndex(session, guildID); err == nil {
					if _, ok := roleIndex[strings.TrimSpace(auto.TargetRoleID)]; !ok {
						return "blocked", []featureBlocker{{Code: "invalid_target_role", Message: "Auto assignment target role is no longer available in this server.", Field: "target_role_id"}}
					}
					for _, roleID := range auto.RequiredRoles {
						if _, ok := roleIndex[strings.TrimSpace(roleID)]; !ok {
							return "blocked", []featureBlocker{{Code: "invalid_required_roles", Message: "Auto assignment required roles are no longer available in this server.", Field: "required_role_ids"}}
						}
					}
				}
			}
		}
	case "user_prune":
		if guildID != "" {
			if guild, ok := findGuildSettings(*cfg, guildID); ok && !guild.UserPrune.Enabled {
				return "blocked", []featureBlocker{{Code: "config_disabled", Message: "User prune config is disabled.", Field: "config_enabled"}}
			}
		}
	}
	return "ready", nil
}

func buildLogFeatureReadiness(
	cfg *files.BotConfig,
	configManager *files.ConfigManager,
	guildID string,
	eventType discordlogging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if guildID == "" {
		return buildGlobalLogFeatureReadiness(cfg, eventType, session)
	}
	if configManager == nil {
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Config manager is unavailable."}}
	}

	decision := discordlogging.ShouldEmitLogEvent(session, configManager, eventType, guildID)
	if decision.Enabled {
		return "ready", nil
	}
	return logDecisionToReadiness(decision)
}

func buildGlobalLogFeatureReadiness(
	cfg *files.BotConfig,
	eventType discordlogging.LogEventType,
	session *discordgo.Session,
) (string, []featureBlocker) {
	if blocker, ok := globalLogRuntimeBlocker(cfg.ResolveRuntimeConfig(""), eventType); ok {
		return "blocked", []featureBlocker{blocker}
	}

	capability, ok := discordlogging.LogEventCapabilities()[eventType]
	if ok && capability.RequiredIntentsMask != 0 && session != nil {
		currentMask := int(session.Identify.Intents)
		missing := capability.RequiredIntentsMask &^ currentMask
		if missing != 0 {
			return "blocked", []featureBlocker{{
				Code:    "missing_intent",
				Message: fmt.Sprintf("Gateway intents mask %d is missing required bits %d.", currentMask, missing),
			}}
		}
	}
	return "ready", nil
}

func logDecisionToReadiness(decision discordlogging.EmitDecision) (string, []featureBlocker) {
	switch decision.Reason {
	case discordlogging.EmitReasonRuntimeDisableUserLogs,
		discordlogging.EmitReasonRuntimeDisableEntryExitLogs,
		discordlogging.EmitReasonRuntimeDisableMessageLogs,
		discordlogging.EmitReasonRuntimeDisableReactionLogs,
		discordlogging.EmitReasonRuntimeDisableAutomodLogs,
		discordlogging.EmitReasonRuntimeModerationLoggingOff,
		discordlogging.EmitReasonRuntimeDisableCleanLog:
		return "blocked", []featureBlocker{{Code: "runtime_kill_switch", Message: "A runtime kill switch currently disables this feature."}}
	case discordlogging.EmitReasonNoChannelConfigured:
		return "blocked", []featureBlocker{{Code: "missing_channel", Message: "A channel must be configured for this feature.", Field: "channel_id"}}
	case discordlogging.EmitReasonMissingIntent:
		return "blocked", []featureBlocker{{Code: "missing_intent", Message: fmt.Sprintf("Gateway intents are missing required bits %d.", decision.MissingMask)}}
	case discordlogging.EmitReasonChannelInvalid:
		return "blocked", []featureBlocker{{Code: "invalid_channel", Message: "The configured channel failed validation for this feature.", Field: "channel_id"}}
	case discordlogging.EmitReasonGuildConfigMissing:
		return "blocked", []featureBlocker{{Code: "missing_guild_registration", Message: "This guild is not registered in settings yet."}}
	case discordlogging.EmitReasonConfigManagerUnavailable, discordlogging.EmitReasonConfigUnavailable:
		return "blocked", []featureBlocker{{Code: "config_unavailable", Message: "Feature config is unavailable."}}
	default:
		return "blocked", []featureBlocker{{Code: "blocked", Message: string(decision.Reason)}}
	}
}

func globalLogRuntimeBlocker(rc files.RuntimeConfig, eventType discordlogging.LogEventType) (featureBlocker, bool) {
	switch eventType {
	case discordlogging.LogEventAvatarChange, discordlogging.LogEventRoleChange:
		if rc.DisableUserLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime user logging is disabled."}, true
		}
	case discordlogging.LogEventMemberJoin, discordlogging.LogEventMemberLeave:
		if rc.DisableEntryExitLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime entry/exit logging is disabled."}, true
		}
	case discordlogging.LogEventMessageProcess, discordlogging.LogEventMessageEdit, discordlogging.LogEventMessageDelete:
		if rc.DisableMessageLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime message logging is disabled."}, true
		}
	case discordlogging.LogEventReactionMetric:
		if rc.DisableReactionLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime reaction logging is disabled."}, true
		}
	case discordlogging.LogEventAutomodAction:
		if rc.DisableAutomodLogs {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime AutoMod logging is disabled."}, true
		}
	case discordlogging.LogEventModerationCase:
		if !rc.ModerationLoggingEnabled() {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime moderation logging is disabled."}, true
		}
	case discordlogging.LogEventCleanAction:
		if rc.DisableCleanLog {
			return featureBlocker{Code: "runtime_kill_switch", Message: "Runtime clean logging is disabled."}, true
		}
	}
	return featureBlocker{}, false
}
