package qotd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	qotdArchiveSourceOfficial = "official"
	qotdArchiveSourceReply    = "reply"
)

// PublishScheduledIfDue publishes the scheduled QOTD for the active slot when
// the publish boundary has passed and no scheduled post exists yet.
func (s *Service) PublishScheduledIfDue(ctx context.Context, guildID string, session *discordgo.Session) (bool, error) {
	if err := s.validate(); err != nil {
		return false, err
	}
	if session == nil {
		return false, ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	now := s.clock()
	publishDate := CurrentPublishDateUTC(now)
	if now.Before(PublishTimeUTC(publishDate)) {
		return false, nil
	}

	existing, err := s.store.GetQOTDOfficialPostByDate(ctx, guildID, publishDate)
	if err != nil {
		return false, err
	}
	if existing != nil {
		if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, existing.ID); err != nil {
			return false, err
		}
		return false, nil
	}

	cfg, err := s.configManager.GetQOTDConfig(guildID)
	if err != nil {
		return false, err
	}
	if !cfg.Enabled || strings.TrimSpace(cfg.ForumChannelID) == "" || strings.TrimSpace(cfg.QuestionTagID) == "" {
		return false, ErrQOTDDisabled
	}

	question, err := s.store.ReserveNextQOTDQuestion(ctx, guildID, publishDate)
	if err != nil {
		return false, err
	}
	if question == nil {
		return false, ErrNoQuestionsAvailable
	}

	lifecycle := EvaluateOfficialPost(publishDate, now)
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateProvisioning),
		ForumChannelID:       cfg.ForumChannelID,
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		if isQOTDUniqueConstraintError(err) {
			return false, s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
		}
		return false, err
	}

	published, err := s.publisher.PublishOfficialPost(ctx, session, discordqotd.PublishOfficialPostParams{
		GuildID:        guildID,
		OfficialPostID: provisioned.ID,
		ForumChannelID: cfg.ForumChannelID,
		QuestionTagID:  cfg.QuestionTagID,
		QuestionText:   question.Body,
		PublishDateUTC: publishDate,
		Pinned:         lifecycle.State == OfficialPostStateCurrent,
	})
	if err != nil {
		if deleteErr := s.store.DeleteQOTDOfficialPost(ctx, provisioned.ID); deleteErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled provisioning cleanup failed", "guildID", guildID, "officialPostID", provisioned.ID, "err", deleteErr)
		}
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return false, err
	}

	finalized, err := s.store.FinalizeQOTDOfficialPost(ctx, provisioned.ID, published.ThreadID, published.StarterMessageID, published.PublishedAt)
	if err != nil {
		return false, err
	}
	finalized, err = s.store.UpdateQOTDOfficialPostState(ctx, finalized.ID, string(lifecycle.State), lifecycle.State == OfficialPostStateCurrent, nil, nil)
	if err != nil {
		return false, err
	}

	question.Status = string(QuestionStatusUsed)
	question.UsedAt = &published.PublishedAt
	if updatedQuestion, err := s.store.UpdateQOTDQuestion(ctx, *question); err != nil {
		return false, err
	} else {
		question = updatedQuestion
	}

	if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, finalized.ID); err != nil {
		return false, err
	}

	return true, nil
}

// ReconcileGuild realigns QOTD thread state and snapshots/archive records for a guild.
func (s *Service) ReconcileGuild(ctx context.Context, guildID string, session *discordgo.Session) error {
	if err := s.validate(); err != nil {
		return err
	}
	if session == nil {
		return ErrDiscordUnavailable
	}

	guildID = strings.TrimSpace(guildID)
	lifecycleLock := s.guildLifecycleLock(guildID)
	lifecycleLock.Lock()
	defer lifecycleLock.Unlock()

	return s.reconcileOfficialPostWindow(ctx, guildID, session, s.clock(), 0)
}

func (s *Service) syncLiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, lifecycle OfficialPostLifecycle) error {
	if missing, err := s.setThreadState(ctx, session, post.DiscordThreadID, discordqotd.ThreadState{
		Pinned:   lifecycle.State == OfficialPostStateCurrent,
		Locked:   true,
		Archived: false,
	}); err != nil {
		return err
	} else if missing {
		_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateMissingDiscord), false, nil, nil)
		return err
	}

	_, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(lifecycle.State), lifecycle.State == OfficialPostStateCurrent, nil, nil)
	return err
}

func (s *Service) archiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateArchiving), false, nil, nil); err != nil {
		return err
	}

	missingOfficial, err := s.archiveThreadMessages(ctx, session, storage.QOTDThreadArchiveRecord{
		GuildID:         post.GuildID,
		OfficialPostID:  post.ID,
		SourceKind:      qotdArchiveSourceOfficial,
		DiscordThreadID: post.DiscordThreadID,
		ArchivedAt:      archivedAt,
	})
	if err != nil {
		return err
	}

	replyThreads, err := s.store.ListQOTDReplyThreadsByOfficialPost(ctx, post.ID)
	if err != nil {
		return err
	}
	for _, replyThread := range replyThreads {
		if err := s.archiveReplyThread(ctx, session, post, replyThread, archivedAt); err != nil {
			return err
		}
	}

	state := string(OfficialPostStateArchived)
	if missingOfficial {
		state = string(OfficialPostStateMissingDiscord)
	} else if missing, err := s.setThreadState(ctx, session, post.DiscordThreadID, discordqotd.ThreadState{
		Pinned:   false,
		Locked:   true,
		Archived: true,
	}); err != nil {
		return err
	} else if missing {
		state = string(OfficialPostStateMissingDiscord)
	}

	_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, state, false, &archivedAt, &archivedAt)
	return err
}

func (s *Service) archiveReplyThread(ctx context.Context, session *discordgo.Session, officialPost storage.QOTDOfficialPostRecord, replyThread storage.QOTDReplyThreadRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDReplyThreadState(ctx, replyThread.ID, string(ReplyThreadStateArchiving), nil, nil); err != nil {
		return err
	}

	state := string(ReplyThreadStateArchived)
	if strings.TrimSpace(replyThread.DiscordThreadID) != "" {
		missingReply, err := s.archiveThreadMessages(ctx, session, storage.QOTDThreadArchiveRecord{
			GuildID:         replyThread.GuildID,
			OfficialPostID:  officialPost.ID,
			ReplyThreadID:   &replyThread.ID,
			SourceKind:      qotdArchiveSourceReply,
			DiscordThreadID: replyThread.DiscordThreadID,
			ArchivedAt:      archivedAt,
		})
		if err != nil {
			return err
		}

		if missingReply {
			state = string(ReplyThreadStateMissingDiscord)
		} else if missing, err := s.setThreadState(ctx, session, replyThread.DiscordThreadID, discordqotd.ThreadState{
			Pinned:   false,
			Locked:   true,
			Archived: true,
		}); err != nil {
			return err
		} else if missing {
			state = string(ReplyThreadStateMissingDiscord)
		}
	}

	_, err := s.store.UpdateQOTDReplyThreadState(ctx, replyThread.ID, state, &archivedAt, &archivedAt)
	return err
}

func (s *Service) archiveThreadMessages(ctx context.Context, session *discordgo.Session, record storage.QOTDThreadArchiveRecord) (bool, error) {
	threadID := strings.TrimSpace(record.DiscordThreadID)
	if threadID == "" {
		return true, nil
	}

	archive, err := s.store.GetQOTDThreadArchiveByThreadID(ctx, threadID)
	if err != nil {
		return false, err
	}

	messages, err := s.publisher.FetchThreadMessages(ctx, session, threadID)
	if err != nil {
		if isMissingDiscordThreadError(err) {
			return archive == nil, nil
		}
		return false, err
	}

	if archive == nil {
		archive, err = s.ensureThreadArchive(ctx, record)
		if err != nil {
			return false, err
		}
	}

	if err := s.store.AppendQOTDArchivedMessages(ctx, archive.ID, buildArchivedMessageRecords(messages)); err != nil {
		return false, err
	}
	return false, nil
}

func (s *Service) ensureThreadArchive(ctx context.Context, record storage.QOTDThreadArchiveRecord) (*storage.QOTDThreadArchiveRecord, error) {
	threadID := strings.TrimSpace(record.DiscordThreadID)
	if threadID == "" {
		return nil, fmt.Errorf("ensure qotd thread archive: discord_thread_id is required")
	}

	existing, err := s.store.GetQOTDThreadArchiveByThreadID(ctx, threadID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	created, err := s.store.CreateQOTDThreadArchive(ctx, record)
	if err != nil {
		if !isQOTDUniqueConstraintError(err) {
			return nil, err
		}
		return s.store.GetQOTDThreadArchiveByThreadID(ctx, threadID)
	}
	return created, nil
}

func (s *Service) setThreadState(ctx context.Context, session *discordgo.Session, threadID string, state discordqotd.ThreadState) (bool, error) {
	threadID = strings.TrimSpace(threadID)
	if threadID == "" {
		return true, nil
	}
	if err := s.publisher.SetThreadState(ctx, session, threadID, state); err != nil {
		if isMissingDiscordThreadError(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

func buildArchivedMessageRecords(messages []discordqotd.ArchivedMessage) []storage.QOTDMessageArchiveRecord {
	if len(messages) == 0 {
		return nil
	}
	records := make([]storage.QOTDMessageArchiveRecord, 0, len(messages))
	for _, message := range messages {
		records = append(records, storage.QOTDMessageArchiveRecord{
			DiscordMessageID:   strings.TrimSpace(message.MessageID),
			AuthorID:           strings.TrimSpace(message.AuthorID),
			AuthorNameSnapshot: strings.TrimSpace(message.AuthorNameSnapshot),
			AuthorIsBot:        message.AuthorIsBot,
			Content:            message.Content,
			EmbedsJSON:         cloneArchiveJSON(message.EmbedsJSON),
			AttachmentsJSON:    cloneArchiveJSON(message.AttachmentsJSON),
			CreatedAt:          normalizeArchivedMessageTime(message.CreatedAt),
		})
	}
	return records
}

func cloneArchiveJSON(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(raw))
	copy(out, raw)
	return out
}

func normalizeArchivedMessageTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value.UTC()
}

func isMissingDiscordThreadError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response != nil && restErr.Response.StatusCode == http.StatusNotFound {
		return true
	}
	return restErr.Message != nil && restErr.Message.Code == discordgo.ErrCodeUnknownChannel
}
