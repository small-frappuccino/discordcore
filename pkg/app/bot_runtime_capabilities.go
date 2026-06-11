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

		if features.StatsChannels {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("stats", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.stats = true
				capabilities.hasCommands = true
			}
		}

		if features.Services.Commands {
			cmdResolvedID, _ := guild.ResolveFeatureBotInstanceID("commands", defaultBotInstanceID)
			if cmdResolvedID == botInstanceID {
				capabilities.hasCommands = true
				if features.Services.AdminCommands {
					capabilities.admin = true
				}
			}

			// Sub-domain feature routing grants command capability to the assigned bot.
			// If a sub-domain route is not explicitly configured, it falls back to the
			// base "commands" route (which itself falls back to the default bot).
			rolesResolvedID, _ := guild.ResolveFeatureBotInstanceID("roles", cmdResolvedID)
			if rolesResolvedID == botInstanceID {
				capabilities.hasCommands = true
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}

			modResolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation", cmdResolvedID)
			if modResolvedID == botInstanceID {
				capabilities.hasCommands = true
			}

			partnersResolvedID, _ := guild.ResolveFeatureBotInstanceID("partners", cmdResolvedID)
			if partnersResolvedID == botInstanceID {
				capabilities.hasCommands = true
			}

			embedsResolvedID, _ := guild.ResolveFeatureBotInstanceID("embeds", cmdResolvedID)
			if embedsResolvedID == botInstanceID {
				capabilities.hasCommands = true
			}

			ticketsResolvedID, _ := guild.ResolveFeatureBotInstanceID("tickets", cmdResolvedID)
			if ticketsResolvedID == botInstanceID {
				capabilities.hasCommands = true
			}
		}

		if features.Services.Automod && features.Logging.AutomodAction && !runtimeConfig.DisableAutomodLogs {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.automod = true
				capabilities.intents |= discordgo.IntentAutoModerationExecution
			}
		}

		if features.UserPrune && guild.UserPrune.Enabled {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation", defaultBotInstanceID)
			if resolvedID == botInstanceID {
				capabilities.userPrune = true
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if !features.Services.Monitoring {
			continue
		}

		rolesResolvedID, _ := guild.ResolveFeatureBotInstanceID("roles", defaultBotInstanceID)
		modResolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation", defaultBotInstanceID)

		isRolesBot := rolesResolvedID == botInstanceID
		isModBot := modResolvedID == botInstanceID

		if !isRolesBot && !isModBot {
			continue
		}

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
