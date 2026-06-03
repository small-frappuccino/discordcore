package qotd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func isOfficialPostProvisioningComplete(post storage.QOTDOfficialPostRecord) bool {
	return strings.TrimSpace(post.DiscordThreadID) != "" &&
		strings.TrimSpace(post.DiscordStarterMessageID) != "" &&
		strings.TrimSpace(post.AnswerChannelID) != ""
}

func isOfficialPostPublished(post storage.QOTDOfficialPostRecord) bool {
	return post.PublishedAt != nil && isOfficialPostProvisioningComplete(post)
}

// isOfficialPostAbandoned reports whether a publish attempt was permanently
// abandoned because of an unrecoverable Discord error (channel deleted, bot
// kicked, missing permission). The reconcile and publish loops MUST treat
// these as terminal — retrying spams Discord and never succeeds without
// admin intervention.
func isOfficialPostAbandoned(post storage.QOTDOfficialPostRecord) bool {
	return OfficialPostState(strings.TrimSpace(post.State)) == OfficialPostStateAbandoned
}

// generatePublishNonce returns a fresh idempotency token for a QOTD publish.
// The Discord nonce field accepts up to 25 characters; we use 16 hex chars
// (8 random bytes) which leaves margin and provides ~10^19 entropy — more
// than enough to be globally unique across all our publish attempts.
func generatePublishNonce() (string, error) {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generatePublishNonce: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// officialPostProvisioningParams carries the per-attempt inputs to
// completeOfficialPostProvisioning: the persisted post record being
// provisioned, the reserved question (nil when the row was seeded without
// one), the count of still-available questions used in the thread header,
// the pre-rendered thread name, and the reference time for window math.
type officialPostProvisioningParams struct {
	Post               storage.QOTDOfficialPostRecord
	Question           *storage.QOTDQuestionRecord
	AvailableQuestions int
	ThreadName         string
	Now                time.Time
}

func (s *Service) completeOfficialPostProvisioning(ctx context.Context, session *discordgo.Session, params officialPostProvisioningParams) (*storage.QOTDOfficialPostRecord, *storage.QOTDQuestionRecord, string, error) {
	post := params.Post
	question := params.Question
	availableQuestions := params.AvailableQuestions
	threadName := params.ThreadName
	now := params.Now

	// Always reassert provisioning in storage before touching Discord. This
	// guards cross-instance races where maintenance deleted the row after we
	// loaded it but before publish starts.
	updated, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateProvisioning), nil, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("Service.completeOfficialPostProvisioning: %w", err)
	}
	post = *updated

	displayID := int64(0)
	if question != nil {
		displayID = question.DisplayID
	}

	published, publishErr := s.publisher.PublishOfficialPost(ctx, session, discordqotd.PublishOfficialPostParams{
		GuildID:                    post.GuildID,
		OfficialPostID:             post.ID,
		DisplayID:                  displayID,
		DeckName:                   post.DeckNameSnapshot,
		AvailableQuestions:         availableQuestions,
		ChannelID:                  strings.TrimSpace(post.ChannelID),
		QuestionListThreadID:       post.QuestionListThreadID,
		QuestionListEntryMessageID: post.QuestionListEntryMessageID,
		OfficialThreadID:           post.DiscordThreadID,
		OfficialStarterMessageID:   post.DiscordStarterMessageID,
		OfficialAnswerChannelID:    post.AnswerChannelID,
		ExistingPublishedAt:        derefTime(post.PublishedAt),
		QuestionText:               post.QuestionTextSnapshot,
		PublishDateUTC:             post.PublishDateUTC,
		ThreadName:                 threadName,
		Nonce:                      post.Nonce,
	})
	if published != nil {
		post, err = s.persistPublishedProgress(ctx, post, published)
		if err != nil {
			return nil, nil, "", err
		}
	}
	if publishErr != nil {
		return nil, nil, "", s.handlePublishFailure(ctx, post, publishErr)
	}
	if !isOfficialPostProvisioningComplete(post) {
		incompleteErr := fmt.Errorf("publish official qotd post: incomplete provisioning state for official post %d", post.ID)
		if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateFailed), nil, nil); err != nil {
			return nil, nil, "", fmt.Errorf("%w (mark failed: %w)", incompleteErr, err)
		}
		return nil, nil, "", incompleteErr
	}

	publishedAt := derefTime(post.PublishedAt)
	if publishedAt.IsZero() && published != nil && !published.PublishedAt.IsZero() {
		publishedAt = published.PublishedAt.UTC()
	}
	if publishedAt.IsZero() {
		publishedAt = now.UTC()
	}

	finalized, err := s.finalizeOfficialPost(ctx, post, publishedAt)
	if err != nil {
		return nil, nil, "", err
	}

	finalState := StateWithinWindow(finalized.GraceUntil, finalized.ArchiveAt, now)
	finalized, err = s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(finalState), nil, nil)
	if err != nil {
		return nil, nil, "", fmt.Errorf("Service.completeOfficialPostProvisioning: %w", err)
	}

	updatedQuestion, err := s.updateQuestionAfterPublish(ctx, question, publishedAt)
	if err != nil {
		return nil, nil, "", err
	}

	postURL := OfficialPostJumpURL(*finalized)
	if published != nil && strings.TrimSpace(published.PostURL) != "" {
		postURL = published.PostURL
	}
	return finalized, updatedQuestion, postURL, nil
}

// persistPublishedProgress records the Discord IDs returned by a successful publish and,
// when a question-list thread exists, upserts the corresponding surface. It returns the
// progressed post record.
func (s *Service) persistPublishedProgress(ctx context.Context, post storage.QOTDOfficialPostRecord, published *discordqotd.PublishedOfficialPost) (storage.QOTDOfficialPostRecord, error) {
	progress, err := s.store.UpdateQOTDOfficialPostProgress(ctx, post.ID, storage.QOTDOfficialPostRecord{
		QuestionListThreadID:       published.QuestionListThreadID,
		QuestionListEntryMessageID: published.QuestionListEntryMessageID,
		DiscordThreadID:            published.ThreadID,
		DiscordStarterMessageID:    published.StarterMessageID,
		AnswerChannelID:            published.AnswerChannelID,
	})
	if err != nil {
		return post, fmt.Errorf("Service.completeOfficialPostProvisioning: %w", err)
	}
	post = *progress
	if strings.TrimSpace(post.QuestionListThreadID) != "" {
		if _, err := s.store.UpsertQOTDSurface(ctx, storage.QOTDSurfaceRecord{
			GuildID:              post.GuildID,
			DeckID:               post.DeckID,
			ChannelID:            post.ChannelID,
			QuestionListThreadID: post.QuestionListThreadID,
		}); err != nil {
			return post, err
		}
	}
	return post, nil
}

// handlePublishFailure marks the official post failed, or abandoned for an unrecoverable
// Discord error, then returns the publish error to propagate. The failed/abandoned split is
// load-bearing: abandoned is terminal, failed is retried by the reconcile loop.
func (s *Service) handlePublishFailure(ctx context.Context, post storage.QOTDOfficialPostRecord, publishErr error) error {
	failureState := OfficialPostStateFailed
	if isUnrecoverableDiscordPublishError(publishErr) {
		failureState = OfficialPostStateAbandoned
	}
	if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(failureState), nil, nil); err != nil {
		return fmt.Errorf("publish official qotd post: %w (mark %s: %w)", publishErr, failureState, err)
	}
	if failureState == OfficialPostStateAbandoned {
		s.observability().RecordOfficialPostAbandoned()
		log.ApplicationLogger().Warn(
			"QOTD publish abandoned (unrecoverable Discord error)",
			"officialPostID", post.ID,
			"guildID", post.GuildID,
			"channelID", strings.TrimSpace(post.ChannelID),
			"err", publishErr,
		)
	}
	return publishErr
}

// finalizeOfficialPost writes the finalized official-post record and upserts its surface,
// marking the post failed if either step errors.
func (s *Service) finalizeOfficialPost(ctx context.Context, post storage.QOTDOfficialPostRecord, publishedAt time.Time) (*storage.QOTDOfficialPostRecord, error) {
	finalized, err := s.store.FinalizeQOTDOfficialPost(ctx, storage.FinalizeQOTDOfficialPostParams{
		ID:                         post.ID,
		QuestionListThreadID:       post.QuestionListThreadID,
		QuestionListEntryMessageID: post.QuestionListEntryMessageID,
		DiscordThreadID:            post.DiscordThreadID,
		StarterMessageID:           post.DiscordStarterMessageID,
		AnswerChannelID:            post.AnswerChannelID,
		PublishedAt:                publishedAt,
	})
	if err != nil {
		if _, markErr := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateFailed), nil, nil); markErr != nil {
			return nil, fmt.Errorf("finalize qotd official post: %w (mark failed: %v)", err, markErr)
		}
		return nil, fmt.Errorf("Service.completeOfficialPostProvisioning: %w", err)
	}
	if _, err := s.store.UpsertQOTDSurface(ctx, storage.QOTDSurfaceRecord{
		GuildID:              finalized.GuildID,
		DeckID:               finalized.DeckID,
		ChannelID:            finalized.ChannelID,
		QuestionListThreadID: finalized.QuestionListThreadID,
	}); err != nil {
		if _, markErr := s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(OfficialPostStateFailed), nil, nil); markErr != nil {
			return nil, fmt.Errorf("upsert qotd surface: %w (mark failed: %v)", err, markErr)
		}
		return nil, err
	}
	return finalized, nil
}

// updateQuestionAfterPublish marks the published question used and stamps its first-publish
// timestamps, persisting only when something changed. A nil question is a no-op.
func (s *Service) updateQuestionAfterPublish(ctx context.Context, question *storage.QOTDQuestionRecord, publishedAt time.Time) (*storage.QOTDQuestionRecord, error) {
	if question == nil {
		return nil, nil
	}
	needsQuestionUpdate := false
	if question.Status != string(QuestionStatusUsed) {
		question.Status = string(QuestionStatusUsed)
		needsQuestionUpdate = true
	}
	if question.UsedAt == nil || question.UsedAt.IsZero() {
		question.UsedAt = &publishedAt
		needsQuestionUpdate = true
	}
	if question.PublishedOnceAt == nil || question.PublishedOnceAt.IsZero() {
		question.PublishedOnceAt = &publishedAt
		needsQuestionUpdate = true
	}
	if !needsQuestionUpdate {
		return question, nil
	}
	updated, err := s.store.UpdateQOTDQuestion(ctx, *question)
	if err != nil {
		return nil, fmt.Errorf("Service.completeOfficialPostProvisioning: %w", err)
	}
	return updated, nil
}

func (s *Service) resumeOfficialPostProvisioning(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, now time.Time) (*PublishResult, error) {
	if isOfficialPostAbandoned(post) {
		// Defensive guard: callers should already filter abandoned posts out.
		// If we got here, return the post as-is so the caller can keep the
		// window state coherent without retrying the failing publish.
		return &PublishResult{OfficialPost: post}, nil
	}
	question, err := s.store.GetQOTDQuestion(ctx, post.GuildID, post.QuestionID)
	if err != nil {
		return nil, fmt.Errorf("Service.resumeOfficialPostProvisioning: %w", err)
	}

	availableQuestions := 0
	displayNumber := int64(0)
	if question != nil {
		counts, countsErr := s.deckQuestionCounts(ctx, post.GuildID, post.DeckID)
		if countsErr != nil {
			return nil, countsErr
		}
		availableQuestions = counts.Ready + counts.Draft
		displayNumber = threadDisplayNumberFromUsedCount(counts.Used, question)
	}

	// On resume the displayed thread number is re-derived from the deck's
	// current used-question count. If the prior attempt already flipped this
	// question to Used, the count includes it (display matches what would
	// have been rendered). If the prior attempt crashed before that flip,
	// the question is still Reserved and we add 1 to anticipate the flip
	// that finalize will perform momentarily.
	threadName := buildOfficialThreadName(displayNumber)

	finalized, updatedQuestion, postURL, err := s.completeOfficialPostProvisioning(ctx, session, officialPostProvisioningParams{
		Post:               post,
		Question:           question,
		AvailableQuestions: availableQuestions,
		ThreadName:         threadName,
		Now:                now,
	})
	if err != nil {
		return nil, fmt.Errorf("Service.resumeOfficialPostProvisioning: %w", err)
	}

	result := &PublishResult{
		OfficialPost: *finalized,
		PostURL:      postURL,
	}
	if updatedQuestion != nil {
		result.Question = *updatedQuestion
	}
	return result, nil
}

func (s *Service) resumeOldestPendingOfficialPost(ctx context.Context, guildID string, session *discordgo.Session, now time.Time) (*PublishResult, error) {
	pending, err := s.store.ListQOTDOfficialPostsPendingRecovery(ctx, guildID)
	if err != nil {
		return nil, fmt.Errorf("Service.resumeOldestPendingOfficialPost: %w", err)
	}
	if len(pending) == 0 {
		return nil, nil
	}
	return s.resumeOfficialPostProvisioning(ctx, session, pending[0], now)
}

func (s *Service) reconcilePendingOfficialPosts(ctx context.Context, guildID string, session *discordgo.Session, now time.Time) error {
	pending, err := s.store.ListQOTDOfficialPostsPendingRecovery(ctx, guildID)
	if err != nil {
		return fmt.Errorf("Service.reconcilePendingOfficialPosts: %w", err)
	}
	for _, post := range pending {
		if _, err := s.resumeOfficialPostProvisioning(ctx, session, post, now); err != nil {
			return fmt.Errorf("Service.reconcilePendingOfficialPosts: %w", err)
		}
	}
	return nil
}
