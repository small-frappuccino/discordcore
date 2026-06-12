package qotd

import (
	"context"
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) reconcileOfficialPostWindow(ctx context.Context, guildID string, now time.Time, currentOfficialPostID int64) error {
	for post, err := range s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now) {
		if err != nil {
			return fmt.Errorf("Service.reconcileOfficialPostWindow: %w", err)
		}
		lifecycle := EvaluateOfficialPostWindow(post.PublishDateUTC, derefTime(post.PublishedAt), post.GraceUntil, post.ArchiveAt, now)
		if err := s.syncLiveOfficialPost(ctx, post, lifecycle); err != nil {
			return fmt.Errorf("Service.reconcileOfficialPostWindow: %w", err)
		}
	}

	for post, err := range s.store.ListQOTDOfficialPostsNeedingArchive(ctx, now) {
		if err != nil {
			return fmt.Errorf("Service.reconcileOfficialPostWindow: %w", err)
		}
		if post.GuildID != guildID || post.ID == currentOfficialPostID {
			continue
		}
		if err := s.archiveOfficialPost(ctx, post, now.UTC()); err != nil {
			return fmt.Errorf("Service.reconcileOfficialPostWindow: %w", err)
		}
	}

	return nil
}

func (s *Service) releaseReservedQuestion(ctx context.Context, question storage.QOTDQuestionRecord) error {
	question.Status = string(QuestionStatusReady)
	question.ScheduledForDateUTC = nil
	question.UsedAt = nil
	question.PublishedOnceAt = nil
	_, err := s.store.UpdateQOTDQuestion(ctx, question)
	return err
}
