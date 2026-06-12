package logging

import (
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logpolicy"
	"github.com/small-frappuccino/discordgo"
)

// ResolveModerationLogChannel validates and returns the configured moderation log channel.
func ResolveModerationLogChannel(session *discordgo.Session, configManager *files.ConfigManager, guildID string) (string, bool) {
	if configManager == nil {
		return "", false
	}
	gcfg := configManager.GuildConfig(guildID)
	if gcfg == nil {
		return "", false
	}
	channelID := logpolicy.ResolveLogChannel(logpolicy.LogEventModerationCase, guildID, configManager)
	if channelID == "" {
		return "", false
	}

	if logpolicy.IsSharedModerationChannel(channelID, gcfg) {
		log.ErrorLoggerRaw().Error("Moderation log channel must be exclusive (not shared with other log channels)", "guildID", guildID, "channelID", channelID)
		return "", false
	}

	botID := ""
	if session != nil && session.State != nil && session.State.User != nil {
		botID = session.State.User.ID
	}

	if err := logpolicy.ValidateModerationLogChannel(session, guildID, channelID, botID); err != nil {
		log.ErrorLoggerRaw().Error("Moderation log channel validation failed", "guildID", guildID, "channelID", channelID, "err", err)
		return "", false
	}

	return channelID, true
}
