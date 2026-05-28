package qotd

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) reconcileOfficialPostWindow(ctx context.Context, guildID string, session *discordgo.Session, now time.Time, currentOfficialPostID int64) error {
	posts, err := s.store.GetCurrentAndPreviousQOTDPosts(ctx, guildID, now)
	if err != nil {
		return err
	}

	for _, post := range posts {
		lifecycle := EvaluateOfficialPostWindow(post.PublishDateUTC, derefTime(post.PublishedAt), post.GraceUntil, post.ArchiveAt, now)
		if err := s.syncLiveOfficialPost(ctx, session, post, lifecycle); err != nil {
			return err
		}
	}

	candidates, err := s.store.ListQOTDOfficialPostsNeedingArchive(ctx, now)
	if err != nil {
		return err
	}
	for _, post := range candidates {
		if post.GuildID != guildID || post.ID == currentOfficialPostID {
			continue
		}
		if err := s.archiveOfficialPost(ctx, session, post, now.UTC()); err != nil {
			return err
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
