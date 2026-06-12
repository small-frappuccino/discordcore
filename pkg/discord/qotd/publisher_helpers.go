package qotd

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	defaultThreadAutoArchiveMinutes  = 2880
	fallbackThreadAutoArchiveMinutes = 4320
	officialQuestionEmbedColor       = 0xF48FB1
)

func buildOfficialPostName(_ time.Time, _ int64, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return truncateThreadName(explicitName)
	}
	return "Question of the Day"
}

func truncateThreadName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= 100 {
		return name
	}
	return strings.TrimSpace(string([]rune(name)[:97])) + "..."
}

func buildOfficialQuestionFooter(deckName string, availableQuestions int, displayID int64) string {
	deckName = strings.TrimSpace(deckName)
	if deckName == "" {
		deckName = "Default"
	}
	if availableQuestions < 0 {
		availableQuestions = 0
	}
	if displayID > 0 {
		return fmt.Sprintf("Question ID %d from %s -- %d questions remaining", displayID, deckName, availableQuestions)
	}
	return fmt.Sprintf("%s -- %d questions remaining", deckName, availableQuestions)
}

func normalizeOfficialQuestionText(questionText string) string {
	return strings.ToLower(strings.TrimSpace(questionText))
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

func quoteEmbedText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "> -"
	}
	lines := strings.Split(text, "\n")
	for idx := range lines {
		line := strings.TrimSpace(lines[idx])
		if line == "" {
			lines[idx] = ">"
			continue
		}
		lines[idx] = "> " + line
	}
	return truncateEmbedText(strings.Join(lines, "\n"), limit)
}

func truncateEmbedText(text string, limit int) string {
	if limit <= 0 || utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	if limit <= 3 {
		return string(runes[:limit])
	}
	return strings.TrimSpace(string(runes[:limit-3])) + "..."
}
