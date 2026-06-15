package app

import (
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type botRuntimeCapabilities struct {
	monitoring  bool
	automod     bool
	userPrune   bool
	qotdRuntime bool
	stats       bool
	warmup      bool
	intents     discordgo.Intent
	hasCommands bool
}

// hasCommands reports whether any command catalog should be installed.
func (c botRuntimeCapabilities) HasCommands() bool { return c.hasCommands }

func resolveBotRuntimeCapabilities(
	cfg *files.BotConfig,
	botInstanceID string,
) botRuntimeCapabilities {
	capabilities := botRuntimeCapabilities{
		intents: discordgo.IntentsGuilds,
	}

	if cfg == nil {
		slog.Warn("Mitigated service degradation: Configuration reference resolves to nil; enforcing basal gateway intents",
			slog.String("bot_instance_id", botInstanceID),
			slog.Int("basal_intents", int(capabilities.intents)),
		)
		return capabilities
	}

	guilds := cfg.GuildsForBotInstance(botInstanceID)
	for _, guild := range guilds {
		features := cfg.ResolveFeatures(guild.GuildID)
		runtimeConfig := cfg.ResolveRuntimeConfig(guild.GuildID)

		if !guild.QOTD.IsZero() {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("qotd")
			if resolvedID == botInstanceID {
				capabilities.qotdRuntime = true
			}
		}

		if features.Services.Commands {
			capabilities.hasCommands = true

			rolesResolvedID, _ := guild.ResolveFeatureBotInstanceID("roles")
			if rolesResolvedID == botInstanceID {
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}

			statsResolvedID, _ := guild.ResolveFeatureBotInstanceID("stats")
			if statsResolvedID == botInstanceID {
				capabilities.stats = true
			}
		}

		if features.Services.Automod && features.Logging.AutomodAction && !runtimeConfig.DisableAutomodLogs {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation")
			if resolvedID == botInstanceID {
				capabilities.automod = true
				capabilities.intents |= discordgo.IntentAutoModerationExecution
			}
		}

		if features.UserPrune && guild.UserPrune.Enabled {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation")
			if resolvedID == botInstanceID {
				capabilities.userPrune = true
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if !features.Services.Monitoring {
			continue
		}

		rolesResolvedID, _ := guild.ResolveFeatureBotInstanceID("roles")
		modResolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation")

		isRolesBot := rolesResolvedID == botInstanceID
		isModBot := modResolvedID == botInstanceID

		if !isRolesBot && !isModBot {
			continue
		}

		slog.Debug("Tracking complex conditional branch: Evaluating monitoring sub-capabilities for target runtime",
			slog.String("guild_id", guild.GuildID),
			slog.String("bot_instance_id", botInstanceID),
			slog.Bool("is_roles_bot", isRolesBot),
			slog.Bool("is_mod_bot", isModBot),
		)

		if botRuntimeNeedsMonitoring(features, runtimeConfig, guild) {
			capabilities.monitoring = true
		}

		if isRolesBot {
			if botRuntimeNeedsMemberData(features, runtimeConfig, guild) {
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if isModBot {
			if botRuntimeNeedsPresence(features, runtimeConfig) {
				capabilities.intents |= discordgo.IntentsGuildPresences
				capabilities.warmup = true
			}
			if botRuntimeNeedsMessages(features, runtimeConfig) {
				capabilities.intents |= discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
			}
			if botRuntimeNeedsReactions(features, runtimeConfig) {
				capabilities.intents |= discordgo.IntentsGuildMessageReactions
			}
		}
	}

	slog.Info("Architectural state transition: Gateway intent bitmask and runtime capabilities computed",
		slog.String("bot_instance_id", botInstanceID),
		slog.Int("intents_bitmask", int(capabilities.intents)),
		slog.Bool("has_commands", capabilities.hasCommands),
		slog.Bool("monitoring_enabled", capabilities.monitoring),
	)

	return capabilities
}

func botRuntimeNeedsMonitoring(
	features files.ResolvedFeatureToggles,
	runtimeConfig files.RuntimeConfig,
	guild files.GuildConfig,
) bool {
	return botRuntimeNeedsMessages(features, runtimeConfig) ||
		botRuntimeNeedsReactions(features, runtimeConfig) ||
		botRuntimeNeedsPresence(features, runtimeConfig) ||
		botRuntimeNeedsMemberData(features, runtimeConfig, guild) ||
		botRuntimeNeedsBotPermMirror(features, runtimeConfig)
}

func botRuntimeNeedsMessages(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig) bool {
	if runtimeConfig.DisableMessageLogs {
		return false
	}
	return features.Logging.MessageProcess || features.Logging.MessageEdit || features.Logging.MessageDelete
}

func botRuntimeNeedsReactions(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig) bool {
	return !runtimeConfig.DisableReactionLogs && features.Logging.ReactionMetric
}

func botRuntimeNeedsPresence(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig) bool {
	if !runtimeConfig.DisableUserLogs && features.Logging.AvatarLogging {
		return true
	}
	if features.PresenceWatch.User && strings.TrimSpace(runtimeConfig.PresenceWatchUserID) != "" {
		return true
	}
	return features.PresenceWatch.Bot && runtimeConfig.PresenceWatchBot
}

func botRuntimeNeedsMemberData(
	features files.ResolvedFeatureToggles,
	runtimeConfig files.RuntimeConfig,
	guild files.GuildConfig,
) bool {
	if !runtimeConfig.DisableUserLogs && features.Logging.RoleUpdate {
		return true
	}
	if !runtimeConfig.DisableEntryExitLogs && (features.Logging.MemberJoin || features.Logging.MemberLeave) {
		return true
	}
	if features.AutoRoleAssign && guild.Roles.AutoAssignment.Enabled {
		return true
	}
	if features.StatsChannels && len(guild.Stats.Channels) > 0 {
		return true
	}
	return features.Backfill.Enabled && strings.TrimSpace(runtimeConfig.BackfillChannelID) != ""
}

func botRuntimeNeedsBotPermMirror(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig) bool {
	return features.Safety.BotRolePermMirror && !runtimeConfig.DisableBotRolePermMirror
}
