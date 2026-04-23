package qotd

import (
	"strings"

	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// OfficialPostJumpURL returns the canonical Discord jump URL for one official post.
func OfficialPostJumpURL(post storage.QOTDOfficialPostRecord) string {
	if channelID := strings.TrimSpace(post.ChannelID); channelID != "" {
		if starterMessageID := strings.TrimSpace(post.DiscordStarterMessageID); starterMessageID != "" {
			return discordqotd.BuildMessageJumpURL(post.GuildID, channelID, starterMessageID)
		}
	}
	if threadID := strings.TrimSpace(post.DiscordThreadID); threadID != "" {
		return discordqotd.BuildThreadJumpURL(post.GuildID, threadID)
	}
	channelID := strings.TrimSpace(post.QuestionListThreadID)
	messageID := strings.TrimSpace(post.QuestionListEntryMessageID)
	if channelID == "" || messageID == "" {
		return ""
	}
	return discordqotd.BuildMessageJumpURL(post.GuildID, channelID, messageID)
}
