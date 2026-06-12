package qotd

import (
	"fmt"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// OfficialPostJumpURL returns the canonical Discord jump URL for one official post.
func OfficialPostJumpURL(post storage.QOTDOfficialPostRecord) string {
	if channelID := strings.TrimSpace(post.ChannelID); channelID != "" {
		if starterMessageID := strings.TrimSpace(post.DiscordStarterMessageID); starterMessageID != "" {
			return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", post.GuildID, channelID, starterMessageID)
		}
	}
	if threadID := strings.TrimSpace(post.DiscordThreadID); threadID != "" {
		return fmt.Sprintf("https://discord.com/channels/%s/%s", post.GuildID, threadID)
	}
	channelID := strings.TrimSpace(post.QuestionListThreadID)
	messageID := strings.TrimSpace(post.QuestionListEntryMessageID)
	if channelID == "" || messageID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", post.GuildID, channelID, messageID)
}

// BuildThreadJumpURL builds thread jump url.
func BuildThreadJumpURL(guildID, threadID string) string {
	guildID = strings.TrimSpace(guildID)
	threadID = strings.TrimSpace(threadID)
	if guildID == "" || threadID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s", guildID, threadID)
}

// BuildChannelJumpURL builds channel jump url.
func BuildChannelJumpURL(guildID, channelID string) string {
	return BuildThreadJumpURL(guildID, channelID)
}

// BuildMessageJumpURL builds message jump url.
func BuildMessageJumpURL(guildID, channelID, messageID string) string {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if guildID == "" || channelID == "" || messageID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}
