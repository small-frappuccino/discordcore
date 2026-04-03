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
)

type PublishOfficialPostParams struct {
	GuildID        string
	OfficialPostID int64
	ForumChannelID string
	QuestionTagID  string
	QuestionText   string
	PublishDateUTC time.Time
	ThreadName     string
	Pinned         bool
}

type PublishedOfficialPost struct {
	ThreadID         string
	StarterMessageID string
	PublishedAt      time.Time
	ThreadURL        string
}

type CreateReplyPostParams struct {
	GuildID          string
	OfficialPostID   int64
	OfficialThreadID string
	ForumChannelID   string
	ReplyTagID       string
	QuestionText     string
	PublishDateUTC   time.Time
	UserID           string
	UserDisplayName  string
}

type CreatedReplyPost struct {
	ThreadID         string
	StarterMessageID string
	ThreadURL        string
}

type ThreadState struct {
	Pinned   bool
	Locked   bool
	Archived bool
}

// Publisher wraps forum-thread creation and thread state transitions for QOTD.
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

	thread, err := session.ForumThreadStartComplex(
		normalized.ForumChannelID,
		&discordgo.ThreadStart{
			Name:                buildOfficialPostName(normalized.PublishDateUTC, normalized.ThreadName),
			AutoArchiveDuration: 4320,
			AppliedTags:         []string{normalized.QuestionTagID},
		},
		&discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				buildOfficialQuestionEmbed(normalized.QuestionText, normalized.PublishDateUTC),
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
		return nil, fmt.Errorf("create forum thread: %w", err)
	}

	threadID := strings.TrimSpace(thread.ID)
	if threadID == "" {
		return nil, fmt.Errorf("create forum thread: missing thread id")
	}

	starterMessageID := strings.TrimSpace(thread.LastMessageID)
	if starterMessageID == "" {
		msgs, fetchErr := session.ChannelMessages(threadID, 1, "", "", "")
		if fetchErr != nil {
			return nil, fmt.Errorf("resolve starter message: %w", fetchErr)
		}
		if len(msgs) == 0 || strings.TrimSpace(msgs[0].ID) == "" {
			return nil, fmt.Errorf("resolve starter message: discord returned no starter message")
		}
		starterMessageID = strings.TrimSpace(msgs[0].ID)
	}

	state := ThreadState{
		Pinned: normalized.Pinned,
		Locked: true,
	}
	if err := p.SetThreadState(ctx, session, threadID, state); err != nil {
		return nil, err
	}

	publishedAt := time.Now().UTC()
	return &PublishedOfficialPost{
		ThreadID:         threadID,
		StarterMessageID: starterMessageID,
		PublishedAt:      publishedAt,
		ThreadURL:        BuildThreadJumpURL(normalized.GuildID, threadID),
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
			Name:                buildReplyThreadName(normalized.PublishDateUTC, normalized.UserDisplayName, normalized.UserID),
			AutoArchiveDuration: 4320,
			AppliedTags:         []string{normalized.ReplyTagID},
		},
		&discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{
				buildReplyThreadEmbed(normalized.QuestionText, BuildThreadJumpURL(normalized.GuildID, normalized.OfficialThreadID)),
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

func buildOfficialQuestionEmbed(questionText string, publishDateUTC time.Time) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       "Question of the Day",
		Description: questionText,
		Color:       0x5B9CF6,
		Footer: &discordgo.MessageEmbedFooter{
			Text: publishDateUTC.UTC().Format("2006-01-02"),
		},
	}
}

func buildReplyThreadEmbed(questionText, officialPostURL string) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "Your QOTD Reply Thread",
		Description: fmt.Sprintf("Write your answer in this thread.\n\n%s", questionText),
		Color:       0x3BA55D,
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

func buildOfficialPostName(publishDateUTC time.Time, explicitName string) string {
	explicitName = strings.TrimSpace(explicitName)
	if explicitName != "" {
		return explicitName
	}
	return fmt.Sprintf("QOTD - %s", publishDateUTC.UTC().Format("2006-01-02"))
}

func buildReplyThreadName(publishDateUTC time.Time, userDisplayName, userID string) string {
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
	if len(base) <= 100 {
		return base
	}
	return strings.TrimSpace(base[:100])
}

func normalizePublishOfficialPostParams(params PublishOfficialPostParams) (PublishOfficialPostParams, error) {
	params.GuildID = strings.TrimSpace(params.GuildID)
	params.ForumChannelID = strings.TrimSpace(params.ForumChannelID)
	params.QuestionTagID = strings.TrimSpace(params.QuestionTagID)
	params.QuestionText = strings.TrimSpace(params.QuestionText)
	params.ThreadName = strings.TrimSpace(params.ThreadName)
	params.PublishDateUTC = params.PublishDateUTC.UTC()

	switch {
	case params.GuildID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("guild id is required")
	case params.OfficialPostID <= 0:
		return PublishOfficialPostParams{}, fmt.Errorf("official post id is required")
	case params.ForumChannelID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("forum channel id is required")
	case params.QuestionTagID == "":
		return PublishOfficialPostParams{}, fmt.Errorf("question tag id is required")
	case params.QuestionText == "":
		return PublishOfficialPostParams{}, fmt.Errorf("question text is required")
	case params.PublishDateUTC.IsZero():
		return PublishOfficialPostParams{}, fmt.Errorf("publish date is required")
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
	default:
		return params, nil
	}
}
