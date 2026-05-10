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
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Service) completeOfficialPostProvisioning(
	ctx context.Context,
	session *discordgo.Session,
	post storage.QOTDOfficialPostRecord,
	question *storage.QOTDQuestionRecord,
	availableQuestions int,
	threadName string,
	now time.Time,
) (*storage.QOTDOfficialPostRecord, *storage.QOTDQuestionRecord, string, error) {
	// Always reassert provisioning in storage before touching Discord. This
	// guards cross-instance races where maintenance deleted the row after we
	// loaded it but before publish starts.
	updated, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateProvisioning), nil, nil)
	if err != nil {
		return nil, nil, "", err
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
		progress, err := s.store.UpdateQOTDOfficialPostProgress(ctx, post.ID, storage.QOTDOfficialPostRecord{
			QuestionListThreadID:       published.QuestionListThreadID,
			QuestionListEntryMessageID: published.QuestionListEntryMessageID,
			DiscordThreadID:            published.ThreadID,
			DiscordStarterMessageID:    published.StarterMessageID,
			AnswerChannelID:            published.AnswerChannelID,
		})
		if err != nil {
			return nil, nil, "", err
		}
		post = *progress
		if strings.TrimSpace(post.QuestionListThreadID) != "" {
			if _, err := s.store.UpsertQOTDSurface(ctx, storage.QOTDSurfaceRecord{
				GuildID:              post.GuildID,
				DeckID:               post.DeckID,
				ChannelID:            post.ChannelID,
				QuestionListThreadID: post.QuestionListThreadID,
			}); err != nil {
				return nil, nil, "", err
			}
		}
	}
	if publishErr != nil {
		failureState := OfficialPostStateFailed
		if isUnrecoverableDiscordPublishError(publishErr) {
			failureState = OfficialPostStateAbandoned
		}
		if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(failureState), nil, nil); err != nil {
			return nil, nil, "", fmt.Errorf("publish official qotd post: %w (mark %s: %v)", publishErr, failureState, err)
		}
		if failureState == OfficialPostStateAbandoned {
			log.ApplicationLogger().Warn(
				"QOTD publish abandoned (unrecoverable Discord error)",
				"officialPostID", post.ID,
				"guildID", post.GuildID,
				"channelID", strings.TrimSpace(post.ChannelID),
				"err", publishErr,
			)
		}
		return nil, nil, "", publishErr
	}
	if !isOfficialPostProvisioningComplete(post) {
		incompleteErr := fmt.Errorf("publish official qotd post: incomplete provisioning state for official post %d", post.ID)
		if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateFailed), nil, nil); err != nil {
			return nil, nil, "", fmt.Errorf("%w (mark failed: %v)", incompleteErr, err)
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

	finalized, err := s.store.FinalizeQOTDOfficialPost(
		ctx,
		post.ID,
		post.QuestionListThreadID,
		post.QuestionListEntryMessageID,
		post.DiscordThreadID,
		post.DiscordStarterMessageID,
		post.AnswerChannelID,
		publishedAt,
	)
	if err != nil {
		if _, markErr := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateFailed), nil, nil); markErr != nil {
			return nil, nil, "", fmt.Errorf("finalize qotd official post: %w (mark failed: %v)", err, markErr)
		}
		return nil, nil, "", err
	}
	if _, err := s.store.UpsertQOTDSurface(ctx, storage.QOTDSurfaceRecord{
		GuildID:              finalized.GuildID,
		DeckID:               finalized.DeckID,
		ChannelID:            finalized.ChannelID,
		QuestionListThreadID: finalized.QuestionListThreadID,
	}); err != nil {
		if _, markErr := s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(OfficialPostStateFailed), nil, nil); markErr != nil {
			return nil, nil, "", fmt.Errorf("upsert qotd surface: %w (mark failed: %v)", err, markErr)
		}
		return nil, nil, "", err
	}

	finalState := StateWithinWindow(finalized.GraceUntil, finalized.ArchiveAt, now)
	finalized, err = s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(finalState), nil, nil)
	if err != nil {
		return nil, nil, "", err
	}

	var updatedQuestion *storage.QOTDQuestionRecord
	if question != nil {
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
		if needsQuestionUpdate {
			updatedQuestion, err = s.store.UpdateQOTDQuestion(ctx, *question)
			if err != nil {
				return nil, nil, "", err
			}
		} else {
			updatedQuestion = question
		}
	}

	postURL := OfficialPostJumpURL(*finalized)
	if published != nil && strings.TrimSpace(published.PostURL) != "" {
		postURL = published.PostURL
	}
	return finalized, updatedQuestion, postURL, nil
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
		return nil, err
	}

	availableQuestions := 0
	if question != nil {
		availableQuestions, err = s.availableQuestionCount(ctx, post.GuildID, post.DeckID)
		if err != nil {
			return nil, err
		}
	}

	// On resume, the post already owns its publish ordinal. We deliberately
	// do not re-derive the title from the question's queue_position here:
	// the ordinal is the source of truth for the visible thread name and is
	// stable across the question being shifted, deleted, or republished.
	threadName := buildOfficialThreadName(post.PublishOrdinal)

	finalized, updatedQuestion, postURL, err := s.completeOfficialPostProvisioning(ctx, session, post, question, availableQuestions, threadName, now)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if len(pending) == 0 {
		return nil, nil
	}
	return s.resumeOfficialPostProvisioning(ctx, session, pending[0], now)
}

func (s *Service) reconcilePendingOfficialPosts(ctx context.Context, guildID string, session *discordgo.Session, now time.Time) error {
	pending, err := s.store.ListQOTDOfficialPostsPendingRecovery(ctx, guildID)
	if err != nil {
		return err
	}
	for _, post := range pending {
		if _, err := s.resumeOfficialPostProvisioning(ctx, session, post, now); err != nil {
			return err
		}
	}
	return nil
}
