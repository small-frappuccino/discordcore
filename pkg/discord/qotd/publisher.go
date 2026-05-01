package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

const (
	defaultThreadAutoArchiveMinutes  = 10080
	officialQuestionEmbedColor       = 0xF48FB1
)

type PublishOfficialPostParams struct {
	GuildID                    string
	OfficialPostID             int64
	DisplayID                  int64
	DeckName                   string
	AvailableQuestions         int
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	OfficialThreadID           string
	OfficialStarterMessageID   string
	OfficialAnswerChannelID    string
	ExistingPublishedAt        time.Time
	QuestionText               string
	PublishDateUTC             time.Time
	ThreadName                 string
}

type PublishedOfficialPost struct {
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	ThreadID                   string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
	PostURL                    string
}

type ThreadState struct {
	Pinned   bool
	Locked   bool
	Archived bool
}

// Publisher wraps Discord publishing and archive/state transitions for QOTD.
type Publisher struct{}

func NewPublisher() *Publisher {
	return &Publisher{}
}

func (p *Publisher) PublishOfficialPost(ctx context.Context, session *discordgo.Session, params PublishOfficialPostParams) (*PublishedOfficialPost, error) {
	if session == nil {
		return nil, fmt.Errorf("publish official qotd post: discord session is required")
	}

	normalized, err := normalizePublishOfficialPostParams(params)
	if err != nil {
		return nil, fmt.Errorf("publish official qotd post: %w", err)
	}

	questionEmbed := buildOfficialQuestionEmbed(normalized.DeckName, normalized.AvailableQuestions, normalized.QuestionText, normalized.DisplayID)
	result := &PublishedOfficialPost{
		QuestionListThreadID:       normalized.QuestionListThreadID,
		QuestionListEntryMessageID: normalized.QuestionListEntryMessageID,
		ThreadID:                   normalized.OfficialThreadID,
		StarterMessageID:           normalized.OfficialStarterMessageID,
		AnswerChannelID:            normalized.OfficialAnswerChannelID,
		PublishedAt:                normalized.ExistingPublishedAt,
	}

	if result.StarterMessageID == "" {
		message, err := session.ChannelMessageSendComplex(
			normalized.ChannelID,
			buildOfficialPostStarterMessage(questionEmbed),
		)
		if err != nil {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd starter message: %w", err)
		}
		if message == nil || strings.TrimSpace(message.ID) == "" {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd starter message: missing message id")
		}
		result.StarterMessageID = strings.TrimSpace(message.ID)
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.ThreadID == "" {
		thread, err := session.MessageThreadStartComplex(
			normalized.ChannelID,
			result.StarterMessageID,
			&discordgo.ThreadStart{
				Name:                buildOfficialPostName(normalized.PublishDateUTC, normalized.DisplayID, normalized.ThreadName),
				AutoArchiveDuration: defaultThreadAutoArchiveMinutes,
			},
		)
		if err != nil {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd daily thread: %w", err)
		}
		if thread != nil {
			result.ThreadID = strings.TrimSpace(thread.ID)
		}
		if result.ThreadID == "" {
			return result.withPostURL(normalized.GuildID, normalized.ChannelID), fmt.Errorf("create qotd daily thread: missing thread id")
		}
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.AnswerChannelID == "" && result.ThreadID != "" {
		result.AnswerChannelID = result.ThreadID
	}

	return result.withPostURL(normalized.GuildID, normalized.ChannelID), nil
}

func (post *PublishedOfficialPost) withPostURL(guildID, channelID string) *PublishedOfficialPost {
	if post == nil {
		return nil
	}
	if strings.TrimSpace(post.PostURL) == "" {
		if channelID = strings.TrimSpace(channelID); channelID != "" {
			if starterMessageID := strings.TrimSpace(post.StarterMessageID); starterMessageID != "" {
				post.PostURL = BuildMessageJumpURL(guildID, channelID, starterMessageID)
				return post
			}
		}
		if threadID := strings.TrimSpace(post.ThreadID); threadID != "" {
			post.PostURL = BuildThreadJumpURL(guildID, threadID)
		}
	}
	return post
}

func (p *Publisher) SetThreadState(ctx context.Context, session *discordgo.Session, threadID string, state ThreadState) error {
	if session == nil {
		return fmt.Errorf("set qotd thread state: discord session is required")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return fmt.Errorf("set qotd thread state: thread id is required")
	}

	if _, err := session.ChannelEditComplex(
		threadID,
		buildThreadStateChannelEdit(state),
	); err != nil {
		return fmt.Errorf("set qotd thread state: %w", err)
	}
	return nil
}

func buildThreadStateChannelEdit(state ThreadState) *discordgo.ChannelEdit {
	locked := state.Locked
	archived := state.Archived
	edit := &discordgo.ChannelEdit{
		Locked:   &locked,
		Archived: &archived,
	}
	if state.Pinned {
		flags := discordgo.ChannelFlagPinned
		edit.Flags = &flags
	}
	return edit
}

func BuildThreadJumpURL(guildID, threadID string) string {
	guildID = strings.TrimSpace(guildID)
	threadID = strings.TrimSpace(threadID)
	if guildID == "" || threadID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s", guildID, threadID)
}

func BuildChannelJumpURL(guildID, channelID string) string {
	return BuildThreadJumpURL(guildID, channelID)
}

func BuildMessageJumpURL(guildID, channelID, messageID string) string {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if guildID == "" || channelID == "" || messageID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

func buildOfficialQuestionEmbed(deckName string, availableQuestions int, questionText string, displayID int64) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "☆ question!! ☆",
		Description: normalizeOfficialQuestionText(questionText),
		Color:       officialQuestionEmbedColor,
		Footer: &discordgo.MessageEmbedFooter{
			Text: buildOfficialQuestionFooter(deckName, availableQuestions, displayID),
		},
	}
}

func buildOfficialPostName(_ time.Time, _ int64, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return truncateThreadName(explicitName)
	}
	return "Question of the Day"
}

func buildOfficialPostStarterMessage(embed *discordgo.MessageEmbed) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Embeds:          []*discordgo.MessageEmbed{embed},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

func normalizePublishOfficialPostParams(params PublishOfficialPostParams) (PublishOfficialPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.DeckName = strings.TrimSpace(params.DeckName)
	params.ChannelID = strings.TrimSpace(params.ChannelID)
	params.QuestionListThreadID = strings.TrimSpace(params.QuestionListThreadID)
	params.QuestionListEntryMessageID = strings.TrimSpace(params.QuestionListEntryMessageID)
	params.OfficialThreadID = strings.TrimSpace(params.OfficialThreadID)
	params.OfficialStarterMessageID = strings.TrimSpace(params.OfficialStarterMessageID)
	params.OfficialAnswerChannelID = strings.TrimSpace(params.OfficialAnswerChannelID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.ThreadName = strings.TrimSpace(params.ThreadName)
	if params.AvailableQuestions < 0 {
		params.AvailableQuestions = 0
	}
	if !params.ExistingPublishedAt.IsZero() {
		params.ExistingPublishedAt = params.ExistingPublishedAt.UTC()
	}
	params.PublishDateUTC = params.PublishDateUTC.UTC()

	switch {
	case params.GuildID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("guild id is required")
	case params.OfficialPostID <= 0:
		return PublishOfficialPostParams{}, fmt.Errorf("official post id is required")
	case params.QuestionText == "":
		return PublishOfficialPostParams{}, fmt.Errorf("question text is required")
	case params.PublishDateUTC.IsZero():
		return PublishOfficialPostParams{}, fmt.Errorf("publish date is required")
	case params.ChannelID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("channel id is required")
	default:
		return params, nil
	}
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

func truncateThreadName(name string) string {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) <= 100 {
		return name
	}
	return strings.TrimSpace(string([]rune(name)[:97])) + "..."
}
