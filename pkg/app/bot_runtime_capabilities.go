package app

import (
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type botRuntimeCapabilities struct {
	monitoring  bool
	admin       bool
	automod     bool
	userPrune   bool
	qotdRuntime bool
	warmup      bool
	intents     discordgo.Intent
	hasCommands bool
}

// hasCommands reports whether any command catalog should be installed.
func (c botRuntimeCapabilities) HasCommands() bool { return c.hasCommands }

func resolveBotRuntimeCapabilities(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
) botRuntimeCapabilities {
	capabilities := botRuntimeCapabilities{
		intents: discordgo.IntentsGuilds,
	}
	if cfg == nil {
		return capabilities
	}

	guilds := cfg.GuildsForBotInstance(botInstanceID)
	for _, guild := range guilds {
		features := cfg.ResolveFeatures(guild.GuildID)
		runtimeConfig := cfg.ResolveRuntimeConfig(guild.GuildID)

		if !guild.QOTD.IsZero() {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("qotd", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.qotdRuntime = true
			}
		}

		if features.Services.Commands {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("commands", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.hasCommands = true
				if features.Services.AdminCommands {
					capabilities.admin = true
				}
			}
		}

		if features.Services.Automod && features.Logging.AutomodAction && !runtimeConfig.DisableAutomodLogs {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("automod", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.automod = true
				capabilities.intents |= discordgo.IntentAutoModerationExecution
			}
		}

		if features.UserPrune && guild.UserPrune.Enabled {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("user_prune", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.userPrune = true
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if !features.Services.Monitoring {
			continue
		}

		monitoringResolvedID, _ := guild.ResolveFeatureBotInstanceID("monitoring", defaultBotInstanceID)
		if monitoringResolvedID != botInstanceID {
			continue
		}

		if botRuntimeNeedsMonitoring(features, runtimeConfig, guild) {
			capabilities.monitoring = true
		}
		if botRuntimeNeedsMemberData(features, runtimeConfig, guild) {
			capabilities.intents |= discordgo.IntentsGuildMembers
			capabilities.warmup = true
		}
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

	return capabilities
}

// Clone creates a deep copy of the botRuntimeCapabilities.
// While currently composed of primitive types (where pass-by-value is naturally a deep copy),
// this method establishes the architectural contract for memory isolation, ensuring that
// any future reference types (like slices or maps) added to this struct are properly
// decoupled from the original array backing.
func (c botRuntimeCapabilities) Clone() botRuntimeCapabilities {
	return botRuntimeCapabilities{
		monitoring:  c.monitoring,
		admin:       c.admin,
		automod:     c.automod,
		userPrune:   c.userPrune,
		qotdRuntime: c.qotdRuntime,
		warmup:      c.warmup,
		intents:     c.intents,
		hasCommands: c.hasCommands,
	}
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
