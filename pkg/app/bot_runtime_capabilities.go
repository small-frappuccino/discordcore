package app

import (
	"log/slog"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

type botRuntimeCapabilities struct {
	monitoring          bool
	automod             bool
	userPrune           bool
	qotdRuntime         bool
	stats               bool
	warmup              bool
	intents             discordgo.Intent
	hasCommands         bool
	messageEventService bool
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

		if features.Services.Automod && guild.Channels.AutomodAction != "" && !runtimeConfig.DisableAutomodLogs {
			resolvedID, _ := guild.ResolveFeatureBotInstanceID("moderation")
			if resolvedID == botInstanceID {
				capabilities.automod = true
				capabilities.intents |= discordgo.IntentAutoModerationExecution
			}
		}

		if guild.UserPrune.Enabled {
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
		statsResolvedID, _ := guild.ResolveFeatureBotInstanceID("stats")
		loggingResolvedID, _ := guild.ResolveFeatureBotInstanceID("logging")

		isRolesBot := rolesResolvedID == botInstanceID
		isModBot := modResolvedID == botInstanceID
		isStatsBot := statsResolvedID == botInstanceID
		isLoggingBot := loggingResolvedID == botInstanceID

		if !isRolesBot && !isModBot && !isStatsBot && !isLoggingBot {
			continue
		}

		if isLoggingBot {
			capabilities.messageEventService = true
		}

		slog.Debug("Tracking complex conditional branch: Evaluating monitoring sub-capabilities for target runtime",
			slog.String("guild_id", guild.GuildID),
			slog.String("bot_instance_id", botInstanceID),
			slog.Bool("is_roles_bot", isRolesBot),
			slog.Bool("is_mod_bot", isModBot),
			slog.Bool("is_stats_bot", isStatsBot),
			slog.Bool("is_logging_bot", isLoggingBot),
		)

		if botRuntimeNeedsMonitoring(features, runtimeConfig, guild) {
			capabilities.monitoring = true
		}

		if isRolesBot || isStatsBot || isLoggingBot {
			if botRuntimeNeedsMemberData(features, runtimeConfig, guild) {
				capabilities.intents |= discordgo.IntentsGuildMembers
				capabilities.warmup = true
			}
		}

		if isModBot || isLoggingBot {
			if botRuntimeNeedsPresence(features, runtimeConfig, guild) {
				capabilities.intents |= discordgo.IntentsGuildPresences
				capabilities.warmup = true
			}
			if botRuntimeNeedsMessages(runtimeConfig, guild) {
				capabilities.intents |= discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
			}
			if botRuntimeNeedsReactions(runtimeConfig) {
				capabilities.intents |= discordgo.IntentsGuildMessageReactions
			}
		}

		if isLoggingBot {
			slog.Info("Logging bot runtime capability activated",
				slog.String("guild_id", guild.GuildID),
				slog.String("bot_instance_id", botInstanceID),
				slog.Int("intents_mask", int(capabilities.intents)),
			)
		}
	}

	slog.Debug("Computed gateway intent bitmask and runtime capabilities",
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
	return botRuntimeNeedsMessages(runtimeConfig, guild) ||
		botRuntimeNeedsReactions(runtimeConfig) ||
		botRuntimeNeedsPresence(features, runtimeConfig, guild) ||
		botRuntimeNeedsMemberData(features, runtimeConfig, guild) ||
		botRuntimeNeedsBotPermMirror(runtimeConfig)
}

func botRuntimeNeedsMessages(runtimeConfig files.RuntimeConfig, guild files.GuildConfig) bool {
	if runtimeConfig.DisableMessageLogs {
		return false
	}
	return guild.Channels.MessageEdit != "" || guild.Channels.MessageDelete != ""
}

func botRuntimeNeedsReactions(runtimeConfig files.RuntimeConfig) bool {
	// Reaction logs are metrics-only and don't require a channel configuration.
	return !runtimeConfig.DisableReactionLogs
}

func botRuntimeNeedsPresence(features files.ResolvedFeatureToggles, runtimeConfig files.RuntimeConfig, guild files.GuildConfig) bool {
	if !runtimeConfig.DisableUserLogs && guild.Channels.AvatarLogging != "" {
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
	if !runtimeConfig.DisableUserLogs && guild.Channels.RoleUpdate != "" {
		return true
	}
	if guild.Channels.MemberJoin != "" || guild.Channels.MemberLeave != "" {
		return true
	}

	if guild.Roles.AutoAssignment.Enabled {
		return true
	}
	if len(guild.Stats.Channels) > 0 {
		return true
	}
	return strings.TrimSpace(runtimeConfig.BackfillChannelID) != ""
}

func botRuntimeNeedsBotPermMirror(runtimeConfig files.RuntimeConfig) bool {
	return !runtimeConfig.DisableBotRolePermMirror && strings.TrimSpace(runtimeConfig.BotRolePermMirrorActorRoleID) != ""
}
