package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	answerButtonLabel    = "Answer"
	answerButtonCustomID = "qotd:answer:%d"

	officialQuestionListThreadName    = "questions list!"
	officialQuestionListThreadMessage = "Daily QOTD prompts are posted here."
	defaultThreadAutoArchiveMinutes   = 4320
)

type PublishOfficialPostParams struct {
	GuildID                    string
	OfficialPostID             int64
	QueuePosition              int64
	DeckName                   string
	AvailableQuestions         int
	ForumChannelID             string
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

type UpsertAnswerMessageParams struct {
	GuildID           string
	OfficialPostID    int64
	DeckName          string
	PublishDateUTC    time.Time
	AnswerChannelID   string
	QuestionText      string
	QuestionURL       string
	AnswerText        string
	UserID            string
	UserDisplayName   string
	UserAvatarURL     string
	ExistingMessageID string
}

type UpsertedAnswerMessage struct {
	ChannelID  string
	MessageID  string
	MessageURL string
	Updated    bool
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

	questionEmbed := buildOfficialQuestionEmbed(normalized.DeckName, normalized.AvailableQuestions, normalized.QuestionText, normalized.QueuePosition)
	result := &PublishedOfficialPost{
		QuestionListThreadID:       normalized.QuestionListThreadID,
		QuestionListEntryMessageID: normalized.QuestionListEntryMessageID,
		ThreadID:                   normalized.OfficialThreadID,
		StarterMessageID:           normalized.OfficialStarterMessageID,
		AnswerChannelID:            normalized.OfficialAnswerChannelID,
		PublishedAt:                normalized.ExistingPublishedAt,
	}

	if result.ThreadID == "" {
		thread, err := session.ForumThreadStartComplex(
			normalized.ForumChannelID,
			&discordgo.ThreadStart{
				Name:                buildOfficialPostName(normalized.PublishDateUTC, normalized.QuestionText, normalized.QueuePosition, normalized.ThreadName),
				AutoArchiveDuration: defaultThreadAutoArchiveMinutes,
			},
			&discordgo.MessageSend{
				Embeds:          []*discordgo.MessageEmbed{questionEmbed},
				AllowedMentions: &discordgo.MessageAllowedMentions{},
			},
		)
		if err != nil {
			return result.withPostURL(normalized.GuildID), fmt.Errorf("create qotd forum thread: %w", err)
		}
		if thread != nil {
			result.ThreadID = strings.TrimSpace(thread.ID)
			if result.StarterMessageID == "" {
				result.StarterMessageID = strings.TrimSpace(thread.LastMessageID)
			}
		}
		if result.ThreadID == "" {
			return result.withPostURL(normalized.GuildID), fmt.Errorf("create qotd forum thread: missing thread id")
		}
		if result.AnswerChannelID == "" {
			result.AnswerChannelID = result.ThreadID
		}
		if result.PublishedAt.IsZero() {
			result.PublishedAt = time.Now().UTC()
		}
	}

	if result.StarterMessageID == "" && result.ThreadID != "" {
		msgs, fetchErr := session.ChannelMessages(result.ThreadID, 1, "", "", "")
		if fetchErr != nil {
			return result.withPostURL(normalized.GuildID), fmt.Errorf("resolve qotd starter message: %w", fetchErr)
		}
		if len(msgs) == 0 || strings.TrimSpace(msgs[0].ID) == "" {
			return result.withPostURL(normalized.GuildID), fmt.Errorf("resolve qotd starter message: discord returned no starter message")
		}
		result.StarterMessageID = strings.TrimSpace(msgs[0].ID)
	}

	if result.AnswerChannelID == "" && result.ThreadID != "" {
		result.AnswerChannelID = result.ThreadID
	}

	listArtifact, err := newQuestionListArtifactPublisher(p, session).Publish(ctx, questionListArtifactPublishParams{
		ForumChannelID:      normalized.ForumChannelID,
		PreferredThreadID:   result.QuestionListThreadID,
		EntryMessageID:      result.QuestionListEntryMessageID,
		OfficialPostID:      normalized.OfficialPostID,
		QuestionEmbed:       questionEmbed,
		ExistingPublishedAt: result.PublishedAt,
	})
	if listArtifact != nil {
		result.QuestionListThreadID = listArtifact.ThreadID
		result.QuestionListEntryMessageID = listArtifact.EntryMessageID
		if result.PublishedAt.IsZero() {
			result.PublishedAt = listArtifact.PublishedAt
		}
	}
	if err != nil {
		return result.withPostURL(normalized.GuildID), fmt.Errorf("publish qotd questions list artifact: %w", err)
	}

	return result.withPostURL(normalized.GuildID), nil
}

func (post *PublishedOfficialPost) withPostURL(guildID string) *PublishedOfficialPost {
	if post == nil {
		return nil
	}
	if strings.TrimSpace(post.PostURL) == "" {
		if threadID := strings.TrimSpace(post.ThreadID); threadID != "" {
			post.PostURL = BuildThreadJumpURL(guildID, threadID)
		}
	}
	return post
}

func (p *Publisher) UpsertAnswerMessage(ctx context.Context, session *discordgo.Session, params UpsertAnswerMessageParams) (*UpsertedAnswerMessage, error) {
	if session == nil {
		return nil, fmt.Errorf("upsert qotd answer message: discord session is required")
	}

	normalized, err := normalizeUpsertAnswerMessageParams(params)
	if err != nil {
		return nil, fmt.Errorf("upsert qotd answer message: %w", err)
	}

	embed := buildAnswerEmbed(
		normalized.DeckName,
		normalized.PublishDateUTC,
		normalized.QuestionText,
		normalized.QuestionURL,
		normalized.AnswerText,
		normalized.UserID,
		normalized.UserDisplayName,
		normalized.UserAvatarURL,
	)

	messageID := normalized.ExistingMessageID
	updated := false
	if messageID != "" {
		_, err = session.ChannelMessageEditComplex(&discordgo.MessageEdit{
			ID:      messageID,
			Channel: normalized.AnswerChannelID,
			Embeds:  &[]*discordgo.MessageEmbed{embed},
		})
		if err == nil {
			updated = true
		}
	}
	if !updated {
		message, createErr := session.ChannelMessageSendComplex(
			normalized.AnswerChannelID,
			&discordgo.MessageSend{
				Embeds:          []*discordgo.MessageEmbed{embed},
				AllowedMentions: &discordgo.MessageAllowedMentions{},
			},
		)
		if createErr != nil {
			if err != nil {
				return nil, fmt.Errorf("update answer message: %w", err)
			}
			return nil, fmt.Errorf("create answer message: %w", createErr)
		}
		if message == nil || strings.TrimSpace(message.ID) == "" {
			return nil, fmt.Errorf("create answer message: missing message id")
		}
		messageID = strings.TrimSpace(message.ID)
	}

	return &UpsertedAnswerMessage{
		ChannelID:  normalized.AnswerChannelID,
		MessageID:  messageID,
		MessageURL: BuildMessageJumpURL(normalized.GuildID, normalized.AnswerChannelID, messageID),
		Updated:    updated,
	}, nil
}

func (p *Publisher) SetThreadState(ctx context.Context, session *discordgo.Session, threadID string, state ThreadState) error {
	if session == nil {
		return fmt.Errorf("set qotd thread state: discord session is required")
	}
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return fmt.Errorf("set qotd thread state: thread id is required")
	}

	flags := discordgo.ChannelFlags(0)
	if state.Pinned {
		flags = discordgo.ChannelFlagPinned
	}
	locked := state.Locked
	archived := state.Archived
	if _, err := session.ChannelEditComplex(
		threadID,
		&discordgo.ChannelEdit{
			Flags:    &flags,
			Locked:   &locked,
			Archived: &archived,
		},
	); err != nil {
		return fmt.Errorf("set qotd thread state: %w", err)
	}
	return nil
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

func buildOfficialQuestionEmbed(deckName string, availableQuestions int, questionText string, queuePosition int64) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "☆ question!! ☆",
		Description: quoteEmbedText(questionText, 3800),
		Color:       0x89E5D1,
		Footer: &discordgo.MessageEmbedFooter{
			Text: buildOfficialQuestionFooter(deckName, availableQuestions, queuePosition),
		},
	}
}

func buildAnswerEmbed(deckName string, publishDateUTC time.Time, questionText, questionURL, answerText, userID, userDisplayName, userAvatarURL string) *discordgo.MessageEmbed {
	userDisplayName = strings.TrimSpace(userDisplayName)
	if userDisplayName == "" {
		userDisplayName = strings.TrimSpace(userID)
	}
	if userDisplayName == "" {
		userDisplayName = "Member"
	}

	description := ""
	if trimmedQuestion := strings.TrimSpace(questionText); trimmedQuestion != "" {
		compactQuestion := strings.ReplaceAll(trimmedQuestion, "\n", " ")
		description += fmt.Sprintf("*%s*\n\n", truncateEmbedText(compactQuestion, 256))
	}
	description += quoteEmbedText(answerText, 3600)

	embed := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    userDisplayName,
			IconURL: strings.TrimSpace(userAvatarURL),
		},
		Description: description,
		Color:       0x68C77C,
		Footer: &discordgo.MessageEmbedFooter{
			Text: buildAnswerFooter(deckName, publishDateUTC),
		},
	}
	return embed
}

func buildOfficialPostName(publishDateUTC time.Time, questionText string, queuePosition int64, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return truncateThreadName(explicitName)
	}
	base := compactThreadNameBase(questionText)
	if base == "" {
		base = "Question of the Day"
	}
	if queuePosition > 0 {
		return truncateThreadName(fmt.Sprintf("%s - qotd #%d", base, queuePosition))
	}
	if !publishDateUTC.IsZero() {
		return truncateThreadName(fmt.Sprintf("%s - qotd %s", base, publishDateUTC.UTC().Format("2006-01-02")))
	}
	return truncateThreadName(base + " - qotd")
}

func normalizePublishOfficialPostParams(params PublishOfficialPostParams) (PublishOfficialPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.DeckName = strings.TrimSpace(params.DeckName)
	params.ForumChannelID = strings.TrimSpace(params.ForumChannelID)
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
	case params.ForumChannelID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("forum channel id is required")
	default:
		return params, nil
	}
}

func normalizeUpsertAnswerMessageParams(params UpsertAnswerMessageParams) (UpsertAnswerMessageParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.DeckName = strings.TrimSpace(params.DeckName)
	if !params.PublishDateUTC.IsZero() {
		params.PublishDateUTC = params.PublishDateUTC.UTC()
	}
	params.AnswerChannelID = strings.TrimSpace(params.AnswerChannelID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.QuestionURL = strings.TrimSpace(params.QuestionURL)
	params.AnswerText = strings.TrimSpace(params.AnswerText)
	params.UserID = strings.TrimSpace(params.UserID)
	params.UserDisplayName = strings.TrimSpace(params.UserDisplayName)
	params.UserAvatarURL = strings.TrimSpace(params.UserAvatarURL)
	params.ExistingMessageID = strings.TrimSpace(params.ExistingMessageID)

	switch {
	case params.GuildID == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("guild id is required")
	case params.OfficialPostID <= 0:
		return UpsertAnswerMessageParams{}, fmt.Errorf("official post id is required")
	case params.AnswerChannelID == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("answer channel id is required")
	case params.AnswerText == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("answer text is required")
	case params.UserID == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("user id is required")
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
	if limit <= 0 || len(text) <= limit {
		return text
	}
	if limit <= 3 {
		return text[:limit]
	}
	return strings.TrimSpace(text[:limit-3]) + "..."
}

func buildOfficialQuestionFooter(deckName string, availableQuestions int, queuePosition int64) string {
	deckName = strings.TrimSpace(deckName)
	if deckName == "" {
		deckName = "Default"
	}
	if availableQuestions < 0 {
		availableQuestions = 0
	}
	if queuePosition > 0 {
		return fmt.Sprintf("Deck: %s | Question #%d -- %d Cards Remaining", deckName, queuePosition, availableQuestions)
	}
	return fmt.Sprintf("Deck: %s -- %d Cards Remaining", deckName, availableQuestions)
}

func buildAnswerFooter(deckName string, publishDateUTC time.Time) string {
	deckName = strings.TrimSpace(deckName)
	if !publishDateUTC.IsZero() {
		publishDateUTC = publishDateUTC.UTC()
	}
	publishDateText := ""
	if !publishDateUTC.IsZero() {
		publishDateText = publishDateUTC.Format("2006-01-02")
	}

	switch {
	case deckName != "" && publishDateText != "":
		return fmt.Sprintf("%s | %s", deckName, publishDateText)
	case deckName != "":
		return deckName
	case publishDateText != "":
		return publishDateText
	default:
		return "Official QOTD"
	}
}

func (p *Publisher) ensureOfficialQuestionListThread(ctx context.Context, session *discordgo.Session, forumChannelID, preferredThreadID string) (string, error) {
	thread, err := resolveForumThreadByID(ctx, session, forumChannelID, preferredThreadID)
	if err != nil {
		return "", err
	}
	if thread == nil {
		thread, err = session.ForumThreadStartComplex(
			forumChannelID,
			&discordgo.ThreadStart{
				Name:                officialQuestionListThreadName,
				AutoArchiveDuration: defaultThreadAutoArchiveMinutes,
			},
			&discordgo.MessageSend{
				Content:         officialQuestionListThreadMessage,
				AllowedMentions: &discordgo.MessageAllowedMentions{},
			},
		)
		if err != nil {
			return "", fmt.Errorf("create qotd questions list thread: %w", err)
		}
	}

	threadID := ""
	if thread != nil {
		threadID = strings.TrimSpace(thread.ID)
	}
	if threadID == "" {
		return "", fmt.Errorf("resolve qotd questions list thread: missing thread id")
	}
	return threadID, nil
}

func resolveForumThreadByID(ctx context.Context, session *discordgo.Session, forumChannelID, threadID string) (*discordgo.Channel, error) {
	forumChannelID = strings.TrimSpace(forumChannelID)
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return nil, nil
	}
	if forumChannelID == "" {
		return nil, fmt.Errorf("resolve qotd forum thread by id: forum channel id is required")
	}
	if session == nil {
		return nil, fmt.Errorf("resolve qotd forum thread by id: discord session is required")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	thread, err := session.Channel(threadID)
	if err != nil {
		if isNotFoundRESTError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("resolve qotd forum thread by id: %w", err)
	}
	if thread == nil || strings.TrimSpace(thread.ParentID) != forumChannelID {
		return nil, nil
	}
	return thread, nil
}

func compactThreadNameBase(questionText string) string {
	questionText = strings.ReplaceAll(questionText, "\n", " ")
	return strings.Join(strings.Fields(strings.TrimSpace(questionText)), " ")
}

func truncateThreadName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) <= 100 {
		return name
	}
	return strings.TrimSpace(name[:97]) + "..."
}
