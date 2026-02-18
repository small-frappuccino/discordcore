package logging

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
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
	channelID := ResolveLogChannel(LogEventModerationCase, guildID, configManager)
	if channelID == "" {
		return "", false
	}

	if isSharedModerationChannel(channelID, gcfg) {
		log.ErrorLoggerRaw().Error("Moderation log channel must be exclusive (not shared with other log channels)", "guildID", guildID, "channelID", channelID)
		return "", false
	}

	botID := ""
	if session != nil && session.State != nil && session.State.User != nil {
		botID = session.State.User.ID
	}

	if err := validateModerationLogChannel(session, guildID, channelID, botID); err != nil {
		log.ErrorLoggerRaw().Error("Moderation log channel validation failed", "guildID", guildID, "channelID", channelID, "err", err)
		return "", false
	}

	return channelID, true
}

func isSharedModerationChannel(channelID string, gcfg *files.GuildConfig) bool {
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

func validateModerationLogChannel(session *discordgo.Session, guildID, channelID, botID string) error {
	if session == nil {
		return fmt.Errorf("session is nil")
	}
	if guildID == "" || channelID == "" {
		return fmt.Errorf("missing guildID or channelID")
	}

	var ch *discordgo.Channel
	if session.State != nil {
		if cached, _ := session.State.Channel(channelID); cached != nil {
			ch = cached
		}
	}
	if ch == nil {
		c, err := session.Channel(channelID)
		if err != nil {
			return fmt.Errorf("channel lookup failed: %w", err)
		}
		ch = c
	}

	if ch == nil {
		return fmt.Errorf("channel not found")
	}
	if ch.GuildID != "" && ch.GuildID != guildID {
		return fmt.Errorf("channel guild mismatch")
	}
	if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
		return fmt.Errorf("channel is not a guild text channel")
	}

	if botID == "" {
		return fmt.Errorf("bot identity not available")
	}

	perms, err := session.UserChannelPermissions(botID, channelID)
	if err != nil {
		return fmt.Errorf("permission check failed: %w", err)
	}

	required := int64(discordgo.PermissionViewChannel | discordgo.PermissionSendMessages | discordgo.PermissionEmbedLinks)
	if perms&required != required {
		return fmt.Errorf("missing permissions (need view/send/embed)")
	}
	return nil
}
