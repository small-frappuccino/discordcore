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
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	qotdArchiveSourceOfficial = "official"
)

// PublishScheduledIfDue publishes the scheduled QOTD for the active slot when
// the publish boundary has passed and no official post exists yet.
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
	s.clearExpiredScheduledPublishSuppression(guildID, cfg, now)
	slotState, err := s.loadDueSlotState(ctx, guildID, cfg, now)
	if err != nil {
		return false, err
	}
	if !slotState.ScheduleConfigured {
		return false, ErrQOTDDisabled
	}
	if !slotState.BoundaryPassed(now) {
		return false, nil
	}

	if slotState.HasOfficialPostRecord() {
		if slotState.HasProvisioningOfficialPost() {
			recovered, err := s.resumeOfficialPostProvisioning(ctx, session, *slotState.OfficialPost, now)
			if err != nil {
				return false, err
			}
			if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, recovered.OfficialPost.ID); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := s.reconcileOfficialPostWindow(ctx, guildID, session, now, slotState.OfficialPost.ID); err != nil {
			return false, err
		}
		return false, nil
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || !canPublishQOTD(deck) {
		return false, ErrQOTDDisabled
	}
	if isScheduledPublishSuppressed(cfg, slotState.PublishDateUTC) {
		return false, nil
	}
	// Also honor suppressions whose target is the forward-looking projection
	// (tomorrow once today's boundary has passed). Manual publish suppression
	// records the projection's date because that matches the manual post being
	// claimed; without this check, today's late publish would re-fire even
	// though the user just paused autopublishing.
	if projected := CurrentPublishDateUTC(slotState.Schedule, now); !projected.Equal(slotState.PublishDateUTC) && isScheduledPublishSuppressed(cfg, projected) {
		return false, nil
	}

	question, err := s.store.ReserveNextQOTDQuestion(ctx, guildID, deck.ID, slotState.PublishDateUTC, deckQuestionSelector(deck))
	if err != nil {
		return false, err
	}
	if question == nil {
		return false, ErrNoQuestionsAvailable
	}
	counts, err := s.deckQuestionCounts(ctx, guildID, deck.ID)
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return false, err
	}
	availableQuestions := counts.Ready + counts.Draft

	lifecycle := EvaluateOfficialPost(slotState.Schedule, slotState.PublishDateUTC, now)
	nonce, err := generatePublishNonce()
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		return false, fmt.Errorf("generate qotd publish nonce: %w", err)
	}
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              guildID,
		DeckID:               deck.ID,
		DeckNameSnapshot:     deck.Name,
		QuestionID:           question.ID,
		PublishMode:          string(PublishModeScheduled),
		PublishDateUTC:       slotState.PublishDateUTC,
		State:                string(OfficialPostStateProvisioning),
		ChannelID:            strings.TrimSpace(deck.ChannelID),
		QuestionTextSnapshot: question.Body,
		Nonce:                nonce,
		GraceUntil:           lifecycle.BecomesPreviousAt,
		ArchiveAt:            lifecycle.ArchiveAt,
	})
	if err != nil {
		if releaseErr := s.releaseReservedQuestion(ctx, *question); releaseErr != nil {
			log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
		}
		existing, conflictErr := s.lookupPublishConflictPost(ctx, guildID, slotState.PublishDateUTC, err)
		if conflictErr != nil {
			return false, conflictErr
		}
		if isOfficialPostAbandoned(*existing) {
			// Existing record was permanently abandoned by a previous
			// attempt (unrecoverable Discord error). Don't resume it —
			// admin must intervene. Just keep the window state coherent.
			return false, s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
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
		buildOfficialThreadName(threadDisplayNumberFromUsedCount(counts.Used, question)),
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

// NextScheduledPublishTime returns the next eligible scheduled publish moment
// for a guild, derived purely from configuration (schedule, deck enablement,
// channel target, suppression). Returns ok=false when the guild is not
// currently a candidate for autopublish. The runtime loop uses this to sleep
// directly until a slot is due instead of polling on a fixed interval; the
// authoritative "is this guild due" check still happens inside
// PublishScheduledIfDue via slotState.BoundaryPassed, so this projection is
// safe to use as a wake-up hint even when the wall clock is skewed.
//
// The pure projection logic lives in nextScheduledPublishTimeFromConfig so
// tests can exercise it without building a Service + ConfigManager.
func (s *Service) NextScheduledPublishTime(guildID string, now time.Time) (time.Time, bool) {
	if s == nil || s.configManager == nil {
		return time.Time{}, false
	}
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		return time.Time{}, false
	}
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return time.Time{}, false
	}
	return nextScheduledPublishTimeFromConfig(cfg, now)
}

// nextScheduledPublishTimeFromConfig is the pure projection used by
// NextScheduledPublishTime once the guild's config has been loaded. Keeping
// it separate from the Service method lets tests pin every branch
// (incomplete schedule, disabled deck, missing channel, suppression
// advance, zero projection) by passing a config struct directly, which is
// both faster and avoids depending on the in-memory ConfigManager wiring.
func nextScheduledPublishTimeFromConfig(cfg files.QOTDConfig, now time.Time) (time.Time, bool) {
	schedule, err := resolvePublishSchedule(cfg)
	if err != nil {
		return time.Time{}, false
	}
	deck, ok := cfg.ActiveDeck()
	if !ok || !deck.Enabled || strings.TrimSpace(deck.ChannelID) == "" {
		return time.Time{}, false
	}
	candidate := CurrentPublishDateUTC(schedule, now)
	// Suppression is a single-date token, so at most one date may be skipped.
	// We advance past it so the loop wakes for the next real slot rather than
	// burning a wakeup at a slot that PublishScheduledIfDue would refuse.
	if isScheduledPublishSuppressed(cfg, candidate) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	next := PublishTimeUTC(schedule, candidate)
	if next.IsZero() {
		return time.Time{}, false
	}
	return next, true
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
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return err
	}
	s.clearExpiredScheduledPublishSuppression(guildID, cfg, now)
	if err := s.reconcilePendingOfficialPosts(ctx, guildID, session, now); err != nil {
		return err
	}
	if err := s.reclaimOrphanReservedQuestions(ctx, guildID, now); err != nil {
		return err
	}
	return s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
}

// reclaimOrphanReservedQuestions returns "reserved" questions to "ready" when
// the publish flow crashed between ReserveNextQOTDQuestion and the
// CreateQOTDOfficialPostProvisioning insert: no qotd_official_posts row
// references the question and the reservation date is now in the past, so the
// in-process release path (releaseReservedQuestion) is no longer reachable.
// Without this sweep the deck silently leaks one ready question per crash.
func (s *Service) reclaimOrphanReservedQuestions(ctx context.Context, guildID string, now time.Time) error {
	todayUTC := NormalizePublishDateUTC(now)
	if todayUTC.IsZero() {
		return nil
	}
	ids, err := s.store.ReclaimOrphanReservedQOTDQuestions(ctx, guildID, todayUTC)
	if err != nil {
		return err
	}
	if len(ids) > 0 {
		log.ApplicationLogger().Info(
			"QOTD reclaimed orphan question reservations",
			"guildID", guildID,
			"questionIDs", ids,
			"todayUTC", todayUTC,
		)
	}
	return nil
}

func (s *Service) syncLiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, lifecycle OfficialPostLifecycle) error {
	// Short-circuit when the DB already reflects the lifecycle target. For
	// the current→previous transition the *thread* target is unchanged
	// (both states want unlocked+unarchived+unpinned), so once we've reached
	// the target state once we don't need to repeatedly poke Discord. Saves
	// rate-limit budget and eliminates the recurring 403 churn on guilds
	// where the bot can publish but lacks per-thread MANAGE_THREADS. The
	// transition to OfficialPostStateArchived is handled by
	// archiveOfficialPost (separate code path with its own thread edit), so
	// short-circuiting here does not block the archive flow.
	if string(lifecycle.State) == strings.TrimSpace(post.State) {
		return nil
	}

	if strings.TrimSpace(post.DiscordThreadID) == "" {
		_, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(lifecycle.State), nil, nil)
		return err
	}

	if missing, err := s.setThreadState(ctx, session, post.DiscordThreadID, discordqotd.ThreadState{
		Pinned:   false,
		Locked:   false,
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
		// At ArchiveAt (publish + 48h) we only lock the Discord thread.
		// Discord auto-archives it independently because the thread was
		// created with auto_archive_duration = 48h (equivalent to a mod
		// hitting "Close" in the UI). Setting Archived=true ourselves used
		// to be required, but now it would race the Discord auto-archive
		// and risk unarchiving a thread the platform just closed. The lock
		// stays so reply traffic on the past QOTD cannot reopen the thread
		// from the archived state.
		Pinned:   false,
		Locked:   true,
		Archived: false,
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
		// 403/Missing Access on a thread the bot CAN otherwise post in is
		// usually a per-thread overwrite or a forum tag/lock that prevents
		// MANAGE_THREADS even when the role grants it server-wide. Failing the
		// reconcile cycle here would re-emit the same WARN every minute for
		// the post's whole 48h lifecycle, drowning real publish failures.
		// Treat as a no-op (state already unmanageable from our side), log
		// once per (guild, thread, target state), and let the DB lifecycle
		// state advance.
		if isUnmanageableDiscordThreadError(err) {
			s.logUnmanageableThreadOnce(threadID, state, err)
			return false, nil
		}
		return false, err
	}
	return false, nil
}

// logUnmanageableThreadOnce emits a single WARN per (thread, target state)
// pair so operators see the issue without the reconcile loop flooding logs.
func (s *Service) logUnmanageableThreadOnce(threadID string, state discordqotd.ThreadState, cause error) {
	if s == nil {
		return
	}
	key := fmt.Sprintf("%s|%t|%t|%t", threadID, state.Pinned, state.Locked, state.Archived)
	if _, loaded := s.unmanageableThreadLogs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	log.ApplicationLogger().Warn(
		"QOTD thread state edit rejected by Discord; continuing without grooming the thread",
		"threadID", threadID,
		"targetPinned", state.Pinned,
		"targetLocked", state.Locked,
		"targetArchived", state.Archived,
		"err", cause,
	)
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

// isUnmanageableDiscordThreadError reports whether Discord rejected a thread
// state edit because the bot lacks the specific permissions required to
// manage *this* thread, even if it can post there. Distinct from
// isMissingDiscordThreadError (404 / Unknown Channel): the thread exists,
// the bot just cannot edit its lock/archive/pin flags. Distinct from
// isUnrecoverableDiscordPublishError: that one is for the publish path,
// where 403 means "give up and abandon"; here 403 means "skip grooming and
// move on with the lifecycle".
func isUnmanageableDiscordThreadError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response != nil && restErr.Response.StatusCode == http.StatusForbidden {
		return true
	}
	if restErr.Message != nil {
		switch restErr.Message.Code {
		case discordgo.ErrCodeMissingAccess, discordgo.ErrCodeMissingPermissions:
			return true
		}
	}
	return false
}

// isUnrecoverableDiscordPublishError reports whether the Discord error returned
// by the publish flow indicates a permanent condition: the channel was
// deleted, the bot is no longer in the guild, or it lost write permissions.
// These cases cannot be fixed by retrying — the reconcile loop must abandon
// the post and surface it to operators instead of hammering Discord every 15
// minutes. Transient failures (5xx, DNS, rate limits) intentionally do NOT
// match here so they keep retrying through the existing failed→retry loop.
func isUnrecoverableDiscordPublishError(err error) bool {
	var restErr *discordgo.RESTError
	if !errors.As(err, &restErr) || restErr == nil {
		return false
	}
	if restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case http.StatusNotFound, http.StatusForbidden, http.StatusUnauthorized:
			return true
		}
	}
	if restErr.Message != nil {
		switch restErr.Message.Code {
		case discordgo.ErrCodeUnknownChannel,
			discordgo.ErrCodeUnknownGuild,
			discordgo.ErrCodeUnknownMessage,
			discordgo.ErrCodeMissingAccess,
			discordgo.ErrCodeMissingPermissions,
			discordgo.ErrCodeCannotSendMessagesInVoiceChannel,
			discordgo.ErrCodeCannotSendMessagesToThisUser,
			discordgo.ErrCodeUnauthorized:
			return true
		}
	}
	return false
}
