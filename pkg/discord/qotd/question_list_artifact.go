package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	questionListThreadWritableState = ThreadState{
		Pinned:   false,
		Locked:   false,
		Archived: false,
	}
	questionListThreadSealedState = ThreadState{
		Pinned:   false,
		Locked:   true,
		Archived: false,
	}
)

type questionListArtifactPublishParams struct {
	ForumChannelID      string
	PreferredThreadID   string
	EntryMessageID      string
	OfficialPostID      int64
	QuestionEmbed       *discordgo.MessageEmbed
	ExistingPublishedAt time.Time
}

type questionListArtifactPublishResult struct {
	ThreadID       string
	EntryMessageID string
	PublishedAt    time.Time
}

type questionListArtifactTransport interface {
	EnsureThread(ctx context.Context, forumChannelID, preferredThreadID string) (string, error)
	SetThreadState(ctx context.Context, threadID string, state ThreadState) error
	SendEntry(ctx context.Context, threadID string, message *discordgo.MessageSend) (*discordgo.Message, error)
}

type questionListArtifactPublisher struct {
	transport questionListArtifactTransport
	now       func() time.Time
}

func newQuestionListArtifactPublisher(p *Publisher, session *discordgo.Session) questionListArtifactPublisher {
	return questionListArtifactPublisher{
		transport: discordQuestionListArtifactTransport{
			publisher: p,
			session:   session,
		},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (p questionListArtifactPublisher) Publish(ctx context.Context, params questionListArtifactPublishParams) (*questionListArtifactPublishResult, error) {
	normalized, err := normalizeQuestionListArtifactPublishParams(params)
	if err != nil {
		return nil, err
	}
	if p.transport == nil {
		return nil, fmt.Errorf("question list artifact publisher: transport is required")
	}
	if p.now == nil {
		p.now = func() time.Time { return time.Now().UTC() }
	}

	threadID, err := p.transport.EnsureThread(ctx, normalized.ForumChannelID, normalized.PreferredThreadID)
	if err != nil {
		return nil, err
	}

	result := &questionListArtifactPublishResult{
		ThreadID:       threadID,
		EntryMessageID: normalized.EntryMessageID,
		PublishedAt:    normalized.ExistingPublishedAt,
	}

	if result.EntryMessageID == "" {
		entryMessageID, appendErr := p.appendEntry(ctx, threadID, normalized)
		if entryMessageID != "" {
			result.EntryMessageID = entryMessageID
			if result.PublishedAt.IsZero() {
				result.PublishedAt = p.now().UTC()
			}
		}
		if appendErr != nil {
			return result, appendErr
		}
		return result, nil
	}

	if err := p.transport.SetThreadState(ctx, threadID, questionListThreadSealedState); err != nil {
		return result, fmt.Errorf("lock qotd questions list thread: %w", err)
	}
	return result, nil
}

func (p questionListArtifactPublisher) EnsureSealedThread(ctx context.Context, forumChannelID, preferredThreadID string) (string, error) {
	forumChannelID = strings.TrimSpace(forumChannelID)
	preferredThreadID = strings.TrimSpace(preferredThreadID)
	if forumChannelID == "" {
		return "", fmt.Errorf("forum channel id is required")
	}
	if p.transport == nil {
		return "", fmt.Errorf("question list artifact publisher: transport is required")
	}

	threadID, err := p.transport.EnsureThread(ctx, forumChannelID, preferredThreadID)
	if err != nil {
		return "", err
	}
	if err := p.transport.SetThreadState(ctx, threadID, questionListThreadSealedState); err != nil {
		return "", fmt.Errorf("lock qotd questions list thread: %w", err)
	}
	return threadID, nil
}

func (p questionListArtifactPublisher) appendEntry(ctx context.Context, threadID string, params questionListArtifactPublishParams) (string, error) {
	var entryMessageID string
	err := p.withWritableThread(ctx, threadID, func() error {
		message, err := p.transport.SendEntry(ctx, threadID, buildQuestionListEntryMessage(params.QuestionEmbed, params.OfficialPostID))
		if message != nil {
			entryMessageID = strings.TrimSpace(message.ID)
		}
		if err != nil {
			return fmt.Errorf("append qotd message to questions list: %w", err)
		}
		if entryMessageID == "" {
			return fmt.Errorf("append qotd message to questions list: missing message id")
		}
		return nil
	})
	if err != nil {
		return entryMessageID, err
	}
	return entryMessageID, nil
}

func (p questionListArtifactPublisher) withWritableThread(ctx context.Context, threadID string, run func() error) error {
	if err := p.transport.SetThreadState(ctx, threadID, questionListThreadWritableState); err != nil {
		return fmt.Errorf("prepare qotd questions list thread: %w", err)
	}

	runErr := run()
	lockErr := p.transport.SetThreadState(ctx, threadID, questionListThreadSealedState)
	switch {
	case runErr != nil && lockErr != nil:
		return fmt.Errorf("%w (lock qotd questions list thread: %v)", runErr, lockErr)
	case runErr != nil:
		return runErr
	case lockErr != nil:
		return fmt.Errorf("lock qotd questions list thread: %w", lockErr)
	default:
		return nil
	}
}

func normalizeQuestionListArtifactPublishParams(params questionListArtifactPublishParams) (questionListArtifactPublishParams, error) {
	params.ForumChannelID = strings.TrimSpace(params.ForumChannelID)
	params.PreferredThreadID = strings.TrimSpace(params.PreferredThreadID)
	params.EntryMessageID = strings.TrimSpace(params.EntryMessageID)
	if !params.ExistingPublishedAt.IsZero() {
		params.ExistingPublishedAt = params.ExistingPublishedAt.UTC()
	}

	switch {
	case params.ForumChannelID == "":
		return questionListArtifactPublishParams{}, fmt.Errorf("forum channel id is required")
	case params.QuestionEmbed == nil:
		return questionListArtifactPublishParams{}, fmt.Errorf("question embed is required")
	case params.OfficialPostID <= 0:
		return questionListArtifactPublishParams{}, fmt.Errorf("official post id is required")
	default:
		return params, nil
	}
}

func buildQuestionListEntryMessage(embed *discordgo.MessageEmbed, officialPostID int64) *discordgo.MessageSend {
	return &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: buildAnswerButtonComponents(officialPostID),
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	}
}

type discordQuestionListArtifactTransport struct {
	publisher *Publisher
	session   *discordgo.Session
}

func (t discordQuestionListArtifactTransport) EnsureThread(ctx context.Context, forumChannelID, preferredThreadID string) (string, error) {
	if t.publisher == nil {
		return "", fmt.Errorf("ensure qotd questions list thread: publisher is required")
	}
	return t.publisher.ensureOfficialQuestionListThread(ctx, t.session, forumChannelID, preferredThreadID)
}

func (t discordQuestionListArtifactTransport) SetThreadState(ctx context.Context, threadID string, state ThreadState) error {
	if t.publisher == nil {
		return fmt.Errorf("set qotd questions list thread state: publisher is required")
	}
	return t.publisher.SetThreadState(ctx, t.session, threadID, state)
}

func (t discordQuestionListArtifactTransport) SendEntry(_ context.Context, threadID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	if t.session == nil {
		return nil, fmt.Errorf("append qotd message to questions list: discord session is required")
	}
	return t.session.ChannelMessageSendComplex(threadID, message)
}
