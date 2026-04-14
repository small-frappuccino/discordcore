package qotd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func isOfficialPostProvisioningComplete(post storage.QOTDOfficialPostRecord) bool {
	return strings.TrimSpace(post.QuestionListThreadID) != "" &&
		strings.TrimSpace(post.QuestionListEntryMessageID) != "" &&
		strings.TrimSpace(post.DiscordStarterMessageID) != "" &&
		strings.TrimSpace(post.AnswerChannelID) != ""
}

func isOfficialPostPublished(post storage.QOTDOfficialPostRecord) bool {
	return post.PublishedAt != nil && isOfficialPostProvisioningComplete(post)
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
	if strings.TrimSpace(post.State) != string(OfficialPostStateProvisioning) {
		updated, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateProvisioning), nil, nil)
		if err != nil {
			return nil, nil, "", err
		}
		post = *updated
	}

	surfaceThreadID := strings.TrimSpace(post.QuestionListThreadID)
	if surfaceThreadID == "" {
		surface, err := s.store.GetQOTDForumSurfaceByDeck(ctx, post.GuildID, post.DeckID)
		if err != nil {
			return nil, nil, "", err
		}
		surfaceThreadID = qotdForumSurfaceQuestionListThreadID(surface)
	}

	queuePosition := int64(0)
	if question != nil {
		queuePosition = question.QueuePosition
	}

	published, publishErr := s.publisher.PublishOfficialPost(ctx, session, discordqotd.PublishOfficialPostParams{
		GuildID:                    post.GuildID,
		OfficialPostID:             post.ID,
		QueuePosition:              queuePosition,
		DeckName:                   post.DeckNameSnapshot,
		AvailableQuestions:         availableQuestions,
		ForumChannelID:             strings.TrimSpace(post.ForumChannelID),
		QuestionListThreadID:       surfaceThreadID,
		QuestionListEntryMessageID: post.QuestionListEntryMessageID,
		OfficialThreadID:           post.DiscordThreadID,
		OfficialStarterMessageID:   post.DiscordStarterMessageID,
		OfficialAnswerChannelID:    post.AnswerChannelID,
		ExistingPublishedAt:        derefTime(post.PublishedAt),
		QuestionText:               post.QuestionTextSnapshot,
		PublishDateUTC:             post.PublishDateUTC,
		ThreadName:                 threadName,
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
			if _, err := s.store.UpsertQOTDForumSurface(ctx, storage.QOTDForumSurfaceRecord{
				GuildID:              post.GuildID,
				DeckID:               post.DeckID,
				ForumChannelID:       post.ForumChannelID,
				QuestionListThreadID: post.QuestionListThreadID,
			}); err != nil {
				return nil, nil, "", err
			}
		}
	}
	if publishErr != nil {
		if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateFailed), nil, nil); err != nil {
			return nil, nil, "", fmt.Errorf("publish official qotd post: %w (mark failed: %v)", publishErr, err)
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
	if _, err := s.store.UpsertQOTDForumSurface(ctx, storage.QOTDForumSurfaceRecord{
		GuildID:              finalized.GuildID,
		DeckID:               finalized.DeckID,
		ForumChannelID:       finalized.ForumChannelID,
		QuestionListThreadID: finalized.QuestionListThreadID,
	}); err != nil {
		if _, markErr := s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(OfficialPostStateFailed), nil, nil); markErr != nil {
			return nil, nil, "", fmt.Errorf("upsert qotd forum surface: %w (mark failed: %v)", err, markErr)
		}
		return nil, nil, "", err
	}

	finalState := StateWithinWindow(finalized.GraceUntil, finalized.ArchiveAt, now)
	finalized, err = s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(finalState), nil, nil)
	if err != nil {
		return nil, nil, "", err
	}

	var updatedQuestion *storage.QOTDQuestionRecord
	if question != nil && question.Status != string(QuestionStatusUsed) {
		question.Status = string(QuestionStatusUsed)
		question.UsedAt = &publishedAt
		updatedQuestion, err = s.store.UpdateQOTDQuestion(ctx, *question)
		if err != nil {
			return nil, nil, "", err
		}
	} else {
		updatedQuestion = question
	}

	postURL := officialPostJumpURL(*finalized)
	if published != nil && strings.TrimSpace(published.PostURL) != "" {
		postURL = published.PostURL
	}
	return finalized, updatedQuestion, postURL, nil
}

func (s *Service) resumeOfficialPostProvisioning(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, now time.Time) (*PublishResult, error) {
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

	threadName := buildOfficialThreadName(post.QuestionTextSnapshot, 0)
	if question != nil {
		threadName = buildOfficialThreadName(question.Body, question.QueuePosition)
	}

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
