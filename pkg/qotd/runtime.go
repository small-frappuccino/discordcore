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
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return false, err
	}
	schedule, err := resolvePublishSchedule(cfg)
	if err != nil {
		return false, ErrQOTDDisabled
	}
	publishDate := CurrentPublishDateUTC(schedule, now)
	if now.Before(PublishTimeUTC(schedule, publishDate)) {
		return false, nil
	}

	existing, err := s.store.GetQOTDOfficialPostByDate(ctx, guildID, publishDate)
	if err != nil {
		return false, err
	}
	if existing != nil {
		if !isOfficialPostPublished(*existing) {
			recovered, err := s.resumeOfficialPostProvisioning(ctx, session, *existing, now)
			if err != nil {
				return false, err
			}
			if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, recovered.OfficialPost.ID); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, existing.ID); err != nil {
			return false, err
		}
		return false, nil
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || !canPublishQOTD(deck) {
		return false, ErrQOTDDisabled
	}
	if isScheduledPublishSuppressed(cfg, publishDate) {
		return false, nil
	}

	question, err := s.store.ReserveNextQOTDQuestion(ctx, guildID, deck.ID, publishDate)
	if err != nil {
		return false, err
	}
	if question == nil {
		return false, ErrNoQuestionsAvailable
	}
	availableQuestions, err := s.availableQuestionCount(ctx, guildID, deck.ID)
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return false, err
	}

	lifecycle := EvaluateOfficialPost(schedule, publishDate, now)
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               deck.ID,
		DeckNameSnapshot:     deck.Name,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       publishDate,
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            strings.TrimSpace(deck.ChannelID),
		QuestionTextSnapshot: question.Body,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		existing, conflictErr := s.lookupPublishConflictPost(ctx, guildID, publishDate, err)
		if conflictErr != nil {
			return false, conflictErr
		}
		if !isOfficialPostPublished(*existing) {
			recovered, recoverErr := s.resumeOfficialPostProvisioning(ctx, session, *existing, now)
			if recoverErr != nil {
				return false, recoverErr
			}
			if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, recovered.OfficialPost.ID); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
	}

	finalized, updatedQuestion, _, err := s.completeOfficialPostProvisioning(
		ctx,
		session,
		*provisioned,
		question,
		availableQuestions,
		buildOfficialThreadName(question.DisplayID),
		now,
	)
	if err != nil {
		return false, err
	}
	if updatedQuestion != nil {
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

	now := s.clock()
	if err := s.reconcilePendingOfficialPosts(ctx, guildID, session, now); err != nil {
		return err
	}
	return s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
}

func (s *Service) syncLiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, lifecycle OfficialPostLifecycle) error {
	if strings.TrimSpace(post.DiscordThreadID) == "" {
		_, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(lifecycle.State), nil, nil)
		return err
	}

	if missing, err := s.setThreadState(ctx, session, post.DiscordThreadID, discordqotd.ThreadState{
		Pinned:   false,
		Locked:   !lifecycle.AnswerWindow.IsOpen,
		Archived: false,
	}); err != nil {
		return err
	} else if missing {
		_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateMissingDiscord), nil, nil)
		return err
	}

	_, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(lifecycle.State), nil, nil)
	return err
}

func (s *Service) archiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateArchiving), nil, nil); err != nil {
		return err
	}

	messageMode := strings.TrimSpace(post.DiscordThreadID) == ""
	missingOfficial := false
	if !messageMode {
		var err error
		missingOfficial, err = s.archiveThreadMessages(ctx, session, storage.QOTDThreadArchiveRecord{
			GuildID:         post.GuildID,
			OfficialPostID:  post.ID,
			SourceKind:      qotdArchiveSourceOfficial,
			DiscordThreadID: post.DiscordThreadID,
			ArchivedAt:      archivedAt,
		})
		if err != nil {
			return err
		}
	}

	answerRecords, err := s.store.ListQOTDAnswerMessagesByOfficialPost(ctx, post.ID)
	if err != nil {
		return err
	}
	for _, answerRecord := range answerRecords {
		if err := s.archiveAnswerRecord(ctx, answerRecord, archivedAt); err != nil {
			return err
		}
	}

	state := string(OfficialPostStateArchived)
	if messageMode {
		_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, state, &archivedAt, &archivedAt)
		return err
	}
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

	_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, state, &archivedAt, &archivedAt)
	return err
}

func (s *Service) archiveAnswerRecord(ctx context.Context, answerRecord storage.QOTDAnswerMessageRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDAnswerMessageState(ctx, answerRecord.ID, string(AnswerRecordStateArchiving), nil, nil); err != nil {
		return err
	}
	_, err := s.store.UpdateQOTDAnswerMessageState(ctx, answerRecord.ID, string(AnswerRecordStateArchived), &archivedAt, &archivedAt)
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
		if !isQOTDThreadArchiveConflict(err) {
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
