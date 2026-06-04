package app

import (
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// commandDomainSet records which command domains a runtime serves. Domain keys
// are normalized; the empty string represents the implicit default domain.
type commandDomainSet map[string]struct{}

func (s *commandDomainSet) add(domain string) {
	if *s == nil {
		*s = make(commandDomainSet)
	}
	(*s)[domain] = struct{}{}
}

func (s commandDomainSet) has(domain string) bool {
	_, ok := s[domain]
	return ok
}

func (s commandDomainSet) isNotEmpty() bool { return len(s) > 0 }

// sorted returns the contained domains in stable order so that callers and
// tests get deterministic output.
func (s commandDomainSet) sorted() []string {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, 0, len(s))
	for d := range s {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

type botRuntimeCapabilities struct {
	monitoring     bool
	admin          bool
	automod        bool
	userPrune      bool
	qotdRuntime    bool
	warmup         bool
	intents        discordgo.Intent
	commandDomains commandDomainSet
}

// hasCommands reports whether any command catalog should be installed.
func (c botRuntimeCapabilities) hasCommands() bool { return c.commandDomains.isNotEmpty() }

// hasCommandDomain reports whether the runtime serves the given domain.
func (c botRuntimeCapabilities) hasCommandDomain(domain string) bool {
	return c.commandDomains.has(domain)
}

// commandDomainList returns the served command domains in stable order.
func (c botRuntimeCapabilities) commandDomainList() []string {
	return c.commandDomains.sorted()
}

func resolveBotRuntimeCapabilities(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
) botRuntimeCapabilities {
	return resolveBotRuntimeCapabilitiesForDomains(
		cfg,
		botInstanceID,
		defaultBotInstanceID,
		newRuntimeDomainSupport(nil),
	)
}

func resolveBotRuntimeCapabilitiesForDomains(
	cfg *files.BotConfig,
	botInstanceID string,
	defaultBotInstanceID string,
	domainSupport runtimeDomainSupport,
) botRuntimeCapabilities {
	capabilities := botRuntimeCapabilities{
		intents: discordgo.IntentsGuilds,
	}
	if cfg == nil {
		return capabilities
	}

	if domainSupport.supports(files.BotDomainQOTD) {
		qotdGuilds := cfg.GuildsForBotInstanceForDomain(files.BotDomainQOTD, botInstanceID, defaultBotInstanceID)
		for _, guild := range qotdGuilds {
			capabilities.commandDomains.add(files.BotDomainQOTD)
			if botRuntimeNeedsQOTDRuntime(guild) {
				capabilities.qotdRuntime = true
			}
		}
	}

	if !domainSupport.supportsDefaultDomain() {
		return capabilities
	}

	guilds := cfg.GuildsForBotInstance(botInstanceID, defaultBotInstanceID)
	for _, guild := range guilds {
		features := cfg.ResolveFeatures(guild.GuildID)
		runtimeConfig := cfg.ResolveRuntimeConfig(guild.GuildID)

		if features.Services.Commands {
			capabilities.commandDomains.add("")
			if features.Services.AdminCommands {
				capabilities.admin = true
			}
		}

		if features.Services.Automod && features.Logging.AutomodAction && !runtimeConfig.DisableAutomodLogs {
			capabilities.automod = true
			capabilities.intents |= discordgo.IntentAutoModerationExecution
		}

		if features.UserPrune && guild.UserPrune.Enabled {
			capabilities.userPrune = true
			capabilities.intents |= discordgo.IntentsGuildMembers
			capabilities.warmup = true
		}

		if !features.Services.Monitoring {
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

func botRuntimeNeedsQOTDRuntime(guild files.GuildConfig) bool {
	return !guild.QOTD.IsZero()
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
