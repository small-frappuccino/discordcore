package control

import (
	"slices"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func featureEditableFields(def featureDefinition, guildID string) []string {
	if guildID == "" {
		return slices.Clone(def.GlobalEditableFields)
	}
	return slices.Clone(def.GuildEditableFields)
}

func featureOverrideState(cfg *files.BotConfig, guildID, featureID string) string {
	if guildID == "" {
		ptr := getGlobalFeatureToggle(cfg.Features, featureID)
		if ptr == nil {
			return "default"
		}
		if *ptr {
			return "enabled"
		}
		return "disabled"
	}

	guild, ok := findGuildSettings(*cfg, guildID)
	if !ok {
		return "inherit"
	}
	ptr := getGuildFeatureToggle(&guild, featureID)
	if ptr == nil {
		return "inherit"
	}
	if *ptr {
		return "enabled"
	}
	return "disabled"
}

func featureEffectiveSource(cfg *files.BotConfig, guildID, featureID string) string {
	if guildID != "" {
		if guild, ok := findGuildSettings(*cfg, guildID); ok && getGuildFeatureToggle(&guild, featureID) != nil {
			return "guild"
		}
	}
	if getGlobalFeatureToggle(cfg.Features, featureID) != nil {
		return "global"
	}
	return "built_in"
}

func resolvedFeatureValue(cfg *files.BotConfig, guildID, featureID string) bool {
	resolved := cfg.ResolveFeatures(guildID)
	switch featureID {
	case "services.monitoring":
		return resolved.Services.Monitoring
	case "services.automod":
		return resolved.Services.Automod
	case "moderation.mute_role":
		return resolved.MuteRole
	case "moderation.ban":
		return resolved.Moderation.Ban
	case "moderation.massban":
		return resolved.Moderation.MassBan
	case "moderation.kick":
		return resolved.Moderation.Kick
	case "moderation.timeout":
		return resolved.Moderation.Timeout
	case "moderation.warn":
		return resolved.Moderation.Warn
	case "moderation.warnings":
		return resolved.Moderation.Warnings
	case "services.commands":
		return resolved.Services.Commands
	case "services.admin_commands":
		return resolved.Services.AdminCommands
	case "logging.avatar_logging":
		return resolved.Logging.AvatarLogging
	case "logging.role_update":
		return resolved.Logging.RoleUpdate
	case "logging.member_join":
		return resolved.Logging.MemberJoin
	case "logging.member_leave":
		return resolved.Logging.MemberLeave
	case "logging.message_process":
		return resolved.Logging.MessageProcess
	case "logging.message_edit":
		return resolved.Logging.MessageEdit
	case "logging.message_delete":
		return resolved.Logging.MessageDelete
	case "logging.reaction_metric":
		return resolved.Logging.ReactionMetric
	case "logging.automod_action":
		return resolved.Logging.AutomodAction
	case "logging.moderation_case":
		return resolved.Logging.ModerationCase
	case "logging.clean_action":
		return resolved.Logging.CleanAction
	case "message_cache.cleanup_on_startup":
		return resolved.MessageCache.CleanupOnStartup
	case "message_cache.delete_on_log":
		return resolved.MessageCache.DeleteOnLog
	case "presence_watch.bot":
		return resolved.PresenceWatch.Bot
	case "presence_watch.user":
		return resolved.PresenceWatch.User
	case "maintenance.db_cleanup":
		return resolved.Maintenance.DBCleanup
	case "safety.bot_role_perm_mirror":
		return resolved.Safety.BotRolePermMirror
	case "backfill.enabled":
		return resolved.Backfill.Enabled
	case "stats_channels":
		return resolved.StatsChannels
	case "auto_role_assignment":
		return resolved.AutoRoleAssign
	case "user_prune":
		return resolved.UserPrune
	default:
		return false
	}
}

func getGlobalFeatureToggle(ft files.FeatureToggles, featureID string) *bool {
	switch featureID {
	case "services.monitoring":
		return ft.Services.Monitoring
	case "services.automod":
		return ft.Services.Automod
	case "moderation.mute_role":
		return ft.MuteRole
	case "moderation.ban":
		return ft.Moderation.Ban
	case "moderation.massban":
		return ft.Moderation.MassBan
	case "moderation.kick":
		return ft.Moderation.Kick
	case "moderation.timeout":
		return ft.Moderation.Timeout
	case "moderation.warn":
		return ft.Moderation.Warn
	case "moderation.warnings":
		return ft.Moderation.Warnings
	case "services.commands":
		return ft.Services.Commands
	case "services.admin_commands":
		return ft.Services.AdminCommands
	case "logging.avatar_logging":
		return ft.Logging.AvatarLogging
	case "logging.role_update":
		return ft.Logging.RoleUpdate
	case "logging.member_join":
		return ft.Logging.MemberJoin
	case "logging.member_leave":
		return ft.Logging.MemberLeave
	case "logging.message_process":
		return ft.Logging.MessageProcess
	case "logging.message_edit":
		return ft.Logging.MessageEdit
	case "logging.message_delete":
		return ft.Logging.MessageDelete
	case "logging.reaction_metric":
		return ft.Logging.ReactionMetric
	case "logging.automod_action":
		return ft.Logging.AutomodAction
	case "logging.moderation_case":
		return ft.Logging.ModerationCase
	case "logging.clean_action":
		return ft.Logging.CleanAction
	case "message_cache.cleanup_on_startup":
		return ft.MessageCache.CleanupOnStartup
	case "message_cache.delete_on_log":
		return ft.MessageCache.DeleteOnLog
	case "presence_watch.bot":
		return ft.PresenceWatch.Bot
	case "presence_watch.user":
		return ft.PresenceWatch.User
	case "maintenance.db_cleanup":
		return ft.Maintenance.DBCleanup
	case "safety.bot_role_perm_mirror":
		return ft.Safety.BotRolePermMirror
	case "backfill.enabled":
		return ft.Backfill.Enabled
	case "stats_channels":
		return ft.StatsChannels
	case "auto_role_assignment":
		return ft.AutoRoleAssign
	case "user_prune":
		return ft.UserPrune
	default:
		return nil
	}
}

func setGlobalFeatureToggle(ft *files.FeatureToggles, featureID string, value *bool) {
	switch featureID {
	case "services.monitoring":
		ft.Services.Monitoring = cloneBool(value)
	case "services.automod":
		ft.Services.Automod = cloneBool(value)
	case "moderation.mute_role":
		ft.MuteRole = cloneBool(value)
	case "moderation.ban":
		ft.Moderation.Ban = cloneBool(value)
	case "moderation.massban":
		ft.Moderation.MassBan = cloneBool(value)
	case "moderation.kick":
		ft.Moderation.Kick = cloneBool(value)
	case "moderation.timeout":
		ft.Moderation.Timeout = cloneBool(value)
	case "moderation.warn":
		ft.Moderation.Warn = cloneBool(value)
	case "moderation.warnings":
		ft.Moderation.Warnings = cloneBool(value)
	case "services.commands":
		ft.Services.Commands = cloneBool(value)
	case "services.admin_commands":
		ft.Services.AdminCommands = cloneBool(value)
	case "logging.avatar_logging":
		ft.Logging.AvatarLogging = cloneBool(value)
	case "logging.role_update":
		ft.Logging.RoleUpdate = cloneBool(value)
	case "logging.member_join":
		ft.Logging.MemberJoin = cloneBool(value)
	case "logging.member_leave":
		ft.Logging.MemberLeave = cloneBool(value)
	case "logging.message_process":
		ft.Logging.MessageProcess = cloneBool(value)
	case "logging.message_edit":
		ft.Logging.MessageEdit = cloneBool(value)
	case "logging.message_delete":
		ft.Logging.MessageDelete = cloneBool(value)
	case "logging.reaction_metric":
		ft.Logging.ReactionMetric = cloneBool(value)
	case "logging.automod_action":
		ft.Logging.AutomodAction = cloneBool(value)
	case "logging.moderation_case":
		ft.Logging.ModerationCase = cloneBool(value)
	case "logging.clean_action":
		ft.Logging.CleanAction = cloneBool(value)
	case "message_cache.cleanup_on_startup":
		ft.MessageCache.CleanupOnStartup = cloneBool(value)
	case "message_cache.delete_on_log":
		ft.MessageCache.DeleteOnLog = cloneBool(value)
	case "presence_watch.bot":
		ft.PresenceWatch.Bot = cloneBool(value)
	case "presence_watch.user":
		ft.PresenceWatch.User = cloneBool(value)
	case "maintenance.db_cleanup":
		ft.Maintenance.DBCleanup = cloneBool(value)
	case "safety.bot_role_perm_mirror":
		ft.Safety.BotRolePermMirror = cloneBool(value)
	case "backfill.enabled":
		ft.Backfill.Enabled = cloneBool(value)
	case "stats_channels":
		ft.StatsChannels = cloneBool(value)
	case "auto_role_assignment":
		ft.AutoRoleAssign = cloneBool(value)
	case "user_prune":
		ft.UserPrune = cloneBool(value)
	}
}

func getGuildFeatureToggle(guild *files.GuildConfig, featureID string) *bool {
	if guild == nil {
		return nil
	}
	return getGlobalFeatureToggle(guild.Features, featureID)
}

func setGuildFeatureToggle(guild *files.GuildConfig, featureID string, value *bool) {
	if guild == nil {
		return
	}
	setGlobalFeatureToggle(&guild.Features, featureID, value)
}
