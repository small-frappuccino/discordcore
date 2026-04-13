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
	replyNonceFooterKey  = "QOTD reply ref:"
	replyNonceNamePrefix = "qrp-"
)

type PublishOfficialPostParams struct {
	GuildID           string
	OfficialPostID    int64
	QuestionChannelID string
	QuestionText      string
	PublishDateUTC    time.Time
	ThreadName        string
	Pinned            bool
}

type PublishedOfficialPost struct {
	ThreadID         string
	StarterMessageID string
	PublishedAt      time.Time
	PostURL          string
}

type CreateReplyPostParams struct {
	GuildID           string
	OfficialPostID    int64
	OfficialThreadID  string
	ForumChannelID    string
	ReplyTagID        string
	QuestionText      string
	PublishDateUTC    time.Time
	UserID            string
	UserDisplayName   string
	ProvisioningNonce string
}

type CreatedReplyPost struct {
	ThreadID         string
	StarterMessageID string
	ThreadURL        string
}

type FindReplyPostByNonceParams struct {
	GuildID           string
	ForumChannelID    string
	ProvisioningNonce string
	Since             time.Time
}

type FoundReplyPost struct {
	ThreadID         string
	StarterMessageID string
	ThreadURL        string
}

type UpsertAnswerMessageParams struct {
	GuildID           string
	OfficialPostID    int64
	ResponseChannelID string
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

	message, err := session.ChannelMessageSendComplex(
		normalized.QuestionChannelID,
		&discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				buildOfficialQuestionEmbed(normalized.OfficialPostID, normalized.QuestionText, normalized.PublishDateUTC),
			},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label:    answerButtonLabel,
							Style:    discordgo.PrimaryButton,
							CustomID: fmt.Sprintf(answerButtonCustomID, normalized.OfficialPostID),
						},
					},
				},
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create qotd message: %w", err)
	}

	messageID := ""
	if message != nil {
		messageID = strings.TrimSpace(message.ID)
	}
	if messageID == "" {
		return nil, fmt.Errorf("create qotd message: missing message id")
	}

	publishedAt := time.Now().UTC()
	return &PublishedOfficialPost{
		StarterMessageID: messageID,
		PublishedAt:      publishedAt,
		PostURL:          BuildMessageJumpURL(normalized.GuildID, normalized.QuestionChannelID, messageID),
	}, nil
}

func (p *Publisher) CreateReplyPost(ctx context.Context, session *discordgo.Session, params CreateReplyPostParams) (*CreatedReplyPost, error) {
	if session == nil {
		return nil, fmt.Errorf("create qotd reply post: discord session is required")
	}

	normalized, err := normalizeCreateReplyPostParams(params)
	if err != nil {
		return nil, fmt.Errorf("create qotd reply post: %w", err)
	}

	thread, err := session.ForumThreadStartComplex(
		normalized.ForumChannelID,
		&discordgo.ThreadStart{
			Name:                buildReplyThreadName(normalized.PublishDateUTC, normalized.UserDisplayName, normalized.UserID, normalized.ProvisioningNonce),
			AutoArchiveDuration: 4320,
			AppliedTags:         []string{normalized.ReplyTagID},
		},
		&discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				buildReplyThreadEmbed(normalized.QuestionText, BuildThreadJumpURL(normalized.GuildID, normalized.OfficialThreadID), normalized.ProvisioningNonce),
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create qotd reply forum thread: %w", err)
	}

	threadID := strings.TrimSpace(thread.ID)
	if threadID == "" {
		return nil, fmt.Errorf("create qotd reply forum thread: missing thread id")
	}

	starterMessageID := strings.TrimSpace(thread.LastMessageID)
	if starterMessageID == "" {
		msgs, fetchErr := session.ChannelMessages(threadID, 1, "", "", "")
		if fetchErr != nil {
			return nil, fmt.Errorf("resolve qotd reply starter message: %w", fetchErr)
		}
		if len(msgs) == 0 || strings.TrimSpace(msgs[0].ID) == "" {
			return nil, fmt.Errorf("resolve qotd reply starter message: discord returned no starter message")
		}
		starterMessageID = strings.TrimSpace(msgs[0].ID)
	}

	return &CreatedReplyPost{
		ThreadID:         threadID,
		StarterMessageID: starterMessageID,
		ThreadURL:        BuildThreadJumpURL(normalized.GuildID, threadID),
	}, nil
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
		normalized.OfficialPostID,
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
			Channel: normalized.ResponseChannelID,
			Embeds:  &[]*discordgo.MessageEmbed{embed},
		})
		if err == nil {
			updated = true
		}
	}
	if !updated {
		message, createErr := session.ChannelMessageSendComplex(
			normalized.ResponseChannelID,
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
		ChannelID:  normalized.ResponseChannelID,
		MessageID:  messageID,
		MessageURL: BuildMessageJumpURL(normalized.GuildID, normalized.ResponseChannelID, messageID),
		Updated:    updated,
	}, nil
}

func (p *Publisher) FindReplyPostByNonce(ctx context.Context, session *discordgo.Session, params FindReplyPostByNonceParams) (*FoundReplyPost, error) {
	if session == nil {
		return nil, fmt.Errorf("find qotd reply post by nonce: discord session is required")
	}
	normalized, err := normalizeFindReplyPostByNonceParams(params)
	if err != nil {
		return nil, fmt.Errorf("find qotd reply post by nonce: %w", err)
	}

	candidates, err := listRecoveryCandidateThreads(ctx, session, normalized.ForumChannelID, normalized.Since, replyNonceNameFragment(normalized.ProvisioningNonce))
	if err != nil {
		return nil, err
	}
	footerMarker := replyNonceFooterText(normalized.ProvisioningNonce)
	for _, thread := range candidates {
		if thread == nil {
			continue
		}
		messages, err := fetchThreadMessagesRaw(ctx, session, thread.ID)
		if err != nil {
			if isNotFoundRESTError(err) {
				continue
			}
			return nil, fmt.Errorf("find qotd reply post by nonce: fetch thread %s: %w", strings.TrimSpace(thread.ID), err)
		}
		for idx := range messages {
			message := messages[idx]
			if starterMessageMatchesReplyNonce(message, footerMarker) {
				threadID := strings.TrimSpace(thread.ID)
				starterMessageID := strings.TrimSpace(message.ID)
				if threadID == "" || starterMessageID == "" {
					continue
				}
				return &FoundReplyPost{
					ThreadID:         threadID,
					StarterMessageID: starterMessageID,
					ThreadURL:        BuildThreadJumpURL(normalized.GuildID, threadID),
				}, nil
			}
		}
	}

	return nil, nil
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

func BuildMessageJumpURL(guildID, channelID, messageID string) string {
	guildID = strings.TrimSpace(guildID)
	channelID = strings.TrimSpace(channelID)
	messageID = strings.TrimSpace(messageID)
	if guildID == "" || channelID == "" || messageID == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, channelID, messageID)
}

func buildOfficialQuestionEmbed(officialPostID int64, questionText string, publishDateUTC time.Time) *discordgo.MessageEmbed {
	description := strings.Join([]string{
		quoteEmbedText(questionText, 2800),
		"*Daily prompt*",
		fmt.Sprintf("Use **%s** below to open the reply form and send your answer.", answerButtonLabel),
	}, "\n\n")

	return &discordgo.MessageEmbed{
		Title:       "Question Of The Day",
		Description: description,
		Color:       0x89E5D1,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Prompt Type",
				Value:  "Daily question",
				Inline: true,
			},
			{
				Name:   "Question ID",
				Value:  fmt.Sprintf("`#%d`", officialPostID),
				Inline: true,
			},
			{
				Name:   "Publish Date",
				Value:  fmt.Sprintf("`%s`", publishDateUTC.UTC().Format("2006-01-02")),
				Inline: true,
			},
			{
				Name:   "How It Works",
				Value:  "The button below opens a private form so members can write a complete answer before it is posted in the response channel.",
				Inline: false,
			},
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("Official QOTD #%d", officialPostID),
		},
		Timestamp: publishDateUTC.UTC().Format(time.RFC3339),
	}
}

func buildReplyThreadEmbed(questionText, officialPostURL, provisioningNonce string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "Your QOTD Reply Thread",
		Description: fmt.Sprintf("Write your answer in this thread.\n\n%s", questionText),
		Color:       0x3BA55D,
		Footer: &discordgo.MessageEmbedFooter{
			Text: replyNonceFooterText(provisioningNonce),
		},
	}
	if officialPostURL != "" {
		embed.Fields = []*discordgo.MessageEmbedField{{
			Name:   "Official Question",
			Value:  officialPostURL,
			Inline: false,
		}}
	}
	return embed
}

func buildAnswerEmbed(officialPostID int64, questionText, questionURL, answerText, userID, userDisplayName, userAvatarURL string) *discordgo.MessageEmbed {
	userDisplayName = strings.TrimSpace(userDisplayName)
	if userDisplayName == "" {
		userDisplayName = strings.TrimSpace(userID)
	}
	if userDisplayName == "" {
		userDisplayName = "Member"
	}

	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Responder",
			Value:  formatQOTDResponder(userDisplayName, userID),
			Inline: false,
		},
	}
	if trimmedQuestion := strings.TrimSpace(questionText); trimmedQuestion != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Question",
			Value:  truncateEmbedText(trimmedQuestion, 1024),
			Inline: false,
		})
	}
	if trimmedQuestionURL := strings.TrimSpace(questionURL); trimmedQuestionURL != "" {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Original Post",
			Value:  "[Jump to the official question](" + trimmedQuestionURL + ")",
			Inline: false,
		})
	}
	fields = append(fields,
		&discordgo.MessageEmbedField{
			Name:   "Response Type",
			Value:  "Member response",
			Inline: true,
		},
		&discordgo.MessageEmbedField{
			Name:   "Question ID",
			Value:  fmt.Sprintf("`#%d`", officialPostID),
			Inline: true,
		},
	)

	embed := &discordgo.MessageEmbed{
		Title: "QOTD Answer",
		Author: &discordgo.MessageEmbedAuthor{
			Name:    "Submitted by " + userDisplayName,
			IconURL: strings.TrimSpace(userAvatarURL),
		},
		Description: strings.Join([]string{
			"**Submitted answer**",
			quoteEmbedText(answerText, 3600),
			"*Shared through the QOTD answer form.*",
		}, "\n\n"),
		Color:     0x68C77C,
		Fields:    fields,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("QOTD response for question #%d", officialPostID),
		},
	}
	if strings.TrimSpace(userAvatarURL) != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: strings.TrimSpace(userAvatarURL),
		}
	}
	return embed
}

func buildOfficialPostName(publishDateUTC time.Time, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return explicitName
	}
	return fmt.Sprintf("QOTD - %s", publishDateUTC.UTC().Format("2006-01-02"))
}

func buildReplyThreadName(publishDateUTC time.Time, userDisplayName, userID, provisioningNonce string) string {
	userDisplayName = strings.TrimSpace(userDisplayName)
	if userDisplayName == "" {
		userDisplayName = strings.TrimSpace(userID)
	}
	if userDisplayName == "" {
		userDisplayName = "Member"
	}

	base := fmt.Sprintf("Reply - %s", userDisplayName)
	if !publishDateUTC.IsZero() {
		base = fmt.Sprintf("Reply - %s - %s", publishDateUTC.UTC().Format("2006-01-02"), userDisplayName)
	}
	suffix := replyNonceNameSuffix(provisioningNonce)
	maxLen := 100 - len(suffix)
	if maxLen < 1 {
		maxLen = 100
	}
	if len(base) > maxLen {
		base = strings.TrimSpace(base[:maxLen])
	}
	return strings.TrimSpace(base) + suffix
}

func normalizePublishOfficialPostParams(params PublishOfficialPostParams) (PublishOfficialPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.QuestionChannelID = strings.TrimSpace(params.QuestionChannelID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.ThreadName = strings.TrimSpace(params.ThreadName)
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
	case params.QuestionChannelID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("question channel id is required")
	default:
		return params, nil
	}
}

func normalizeCreateReplyPostParams(params CreateReplyPostParams) (CreateReplyPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.ForumChannelID = strings.TrimSpace(params.ForumChannelID)
	params.ReplyTagID = strings.TrimSpace(params.ReplyTagID)
	params.OfficialThreadID = strings.TrimSpace(params.OfficialThreadID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.UserID = strings.TrimSpace(params.UserID)
	params.UserDisplayName = strings.TrimSpace(params.UserDisplayName)
	params.ProvisioningNonce = strings.TrimSpace(params.ProvisioningNonce)
	params.PublishDateUTC = params.PublishDateUTC.UTC()

	switch {
	case params.GuildID == "":
		return CreateReplyPostParams{}, fmt.Errorf("guild id is required")
	case params.OfficialPostID <= 0:
		return CreateReplyPostParams{}, fmt.Errorf("official post id is required")
	case params.ForumChannelID == "":
		return CreateReplyPostParams{}, fmt.Errorf("forum channel id is required")
	case params.ReplyTagID == "":
		return CreateReplyPostParams{}, fmt.Errorf("reply tag id is required")
	case params.OfficialThreadID == "":
		return CreateReplyPostParams{}, fmt.Errorf("official thread id is required")
	case params.QuestionText == "":
		return CreateReplyPostParams{}, fmt.Errorf("question text is required")
	case params.UserID == "":
		return CreateReplyPostParams{}, fmt.Errorf("user id is required")
	case params.ProvisioningNonce == "":
		return CreateReplyPostParams{}, fmt.Errorf("provisioning nonce is required")
	default:
		return params, nil
	}
}

func normalizeFindReplyPostByNonceParams(params FindReplyPostByNonceParams) (FindReplyPostByNonceParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.ForumChannelID = strings.TrimSpace(params.ForumChannelID)
	params.ProvisioningNonce = strings.TrimSpace(params.ProvisioningNonce)
	if params.GuildID == "" {
		return FindReplyPostByNonceParams{}, fmt.Errorf("guild id is required")
	}
	if params.ForumChannelID == "" {
		return FindReplyPostByNonceParams{}, fmt.Errorf("forum channel id is required")
	}
	if params.ProvisioningNonce == "" {
		return FindReplyPostByNonceParams{}, fmt.Errorf("provisioning nonce is required")
	}
	if !params.Since.IsZero() {
		params.Since = params.Since.UTC()
	}
	return params, nil
}

func normalizeUpsertAnswerMessageParams(params UpsertAnswerMessageParams) (UpsertAnswerMessageParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.ResponseChannelID = strings.TrimSpace(params.ResponseChannelID)
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
	case params.ResponseChannelID == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("response channel id is required")
	case params.AnswerText == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("answer text is required")
	case params.UserID == "":
		return UpsertAnswerMessageParams{}, fmt.Errorf("user id is required")
	default:
		return params, nil
	}
}

func replyNonceFooterText(provisioningNonce string) string {
	provisioningNonce = strings.TrimSpace(provisioningNonce)
	if provisioningNonce == "" {
		return ""
	}
	return fmt.Sprintf("%s %s", replyNonceFooterKey, provisioningNonce)
}

func replyNonceNameSuffix(provisioningNonce string) string {
	fragment := replyNonceNameFragment(provisioningNonce)
	if fragment == "" {
		return ""
	}
	return " [" + fragment + "]"
}

func replyNonceNameFragment(provisioningNonce string) string {
	provisioningNonce = strings.TrimSpace(strings.ToLower(provisioningNonce))
	if provisioningNonce == "" {
		return ""
	}
	if len(provisioningNonce) > 8 {
		provisioningNonce = provisioningNonce[:8]
	}
	return replyNonceNamePrefix + provisioningNonce
}

func starterMessageMatchesReplyNonce(message *discordgo.Message, footerMarker string) bool {
	if message == nil || footerMarker == "" {
		return false
	}
	for _, embed := range message.Embeds {
		if embed == nil || embed.Footer == nil {
			continue
		}
		if strings.TrimSpace(embed.Footer.Text) == footerMarker {
			return true
		}
	}
	return false
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

func formatQOTDResponder(userDisplayName, userID string) string {
	userDisplayName = strings.TrimSpace(userDisplayName)
	userID = strings.TrimSpace(userID)
	if userDisplayName == "" && userID == "" {
		return "Member"
	}
	if userID == "" {
		return "**" + userDisplayName + "**"
	}
	if userDisplayName == "" {
		return "<@" + userID + ">\n`" + userID + "`"
	}
	return fmt.Sprintf("**%s**\n<@%s> · `%s`", userDisplayName, userID, userID)
}
