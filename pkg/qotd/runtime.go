package qotd

import (
	"context"
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

// PublishScheduledIfDue publishes the scheduled QOTD for the active slot when
// the publish boundary has passed and no official post exists yet.
func (s *Service) PublishScheduledIfDue(ctx context.Context, guildID string, session *discordgo.Session) (published bool, err error) {
	// publishAttempted flips to true once we have passed the "is anything
	// due / are we configured" gates and have actually committed to a
	// publish path (fresh provision OR resume of a pending provision).
	// Early returns from gate failures are NOT publish attempts and do
	// not contribute to the publish success/failure ratio; they only
	// surface via structured logs and reconcile-cycle metrics elsewhere.
	publishStart := time.Now()
	publishAttempted := false
	defer func() {
		if !publishAttempted {
			return
		}
		duration := time.Since(publishStart)
		if err != nil {
			s.observability().RecordPublishFailure(PublishModeScheduled, ClassifyPublishFailure(err), duration)
			return
		}
		if published {
			s.observability().RecordPublishSuccess(PublishModeScheduled, duration)
		}
	}()

	if err = s.validate(); err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	if session == nil {
		err = ErrDiscordUnavailable
		return false, err
	}

	guildID = strings.TrimSpace(guildID)

	// recordAttempt commits this cycle to publish accounting: it flips the local
	// flag the metrics defer reads and emits the attempt counter. It is invoked
	// only past the gates, on the fresh-provision and resume-provision paths.
	recordAttempt := func() {
		publishAttempted = true
		s.observability().RecordPublishAttempt(PublishModeScheduled)
	}

	val, err := s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		return s.publishScheduledInActor(ctx, guildID, session, recordAttempt)
	})
	if err != nil {
		return false, err
	}
	return val.(bool), nil
}

// publishScheduledInActor runs the gated scheduled-publish decision inside the guild
// actor: it loads config and the due slot, applies the schedule/boundary gates, and
// dispatches to the existing-record or fresh-provision path. recordAttempt is invoked
// once a path commits to talking to Discord.
func (s *Service) publishScheduledInActor(ctx context.Context, guildID string, session *discordgo.Session, recordAttempt func()) (bool, error) {
	now := s.clock()
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	s.clearExpiredScheduledPublishSuppression(guildID, cfg, now)
	slotState, err := s.loadDueSlotState(ctx, guildID, cfg, now)
	if err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	if !slotState.ScheduleConfigured {
		return false, ErrQOTDDisabled
	}
	if !slotState.BoundaryPassed(now) {
		return false, nil
	}

	scope := publishScope{GuildID: guildID, Session: session, Now: now}
	if slotState.HasOfficialPostRecord() {
		return s.resumeOrReconcileExistingPost(ctx, scope, slotState, recordAttempt)
	}
	return s.provisionScheduledOfficialPost(ctx, scope, cfg, slotState, recordAttempt)
}

// publishScope carries the per-cycle ambient values threaded unchanged
// through the scheduled/manual publish helpers: the guild being published,
// the Discord session used for the API calls, and the cycle clock captured
// once by the actor.
type publishScope struct {
	GuildID string
	Session *discordgo.Session
	Now     time.Time
}

// resumeOrReconcileExistingPost handles a due slot that already has an official-post
// record: resume it if still provisioning (a publish attempt), otherwise just realign
// the window.
func (s *Service) resumeOrReconcileExistingPost(ctx context.Context, scope publishScope, slotState currentSlotState, recordAttempt func()) (bool, error) {
	if slotState.HasProvisioningOfficialPost() {
		// Resuming a half-provisioned record IS a publish attempt from the
		// metrics perspective: we are about to talk to Discord and may
		// transition the post to current.
		recordAttempt()
		recovered, resumeErr := s.resumeOfficialPostProvisioning(ctx, scope.Session, *slotState.OfficialPost, scope.Now)
		if resumeErr != nil {
			return false, resumeErr
		}
		if err := s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, recovered.OfficialPost.ID); err != nil {
			return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
		}
		return true, nil
	}
	if err := s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, slotState.OfficialPost.ID); err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	return false, nil
}

// provisionScheduledOfficialPost applies the deck/suppression gates and, once committed,
// reserves a question and provisions a fresh official post, recovering through
// handleProvisioningConflict when the provisioning insert conflicts.
func (s *Service) provisionScheduledOfficialPost(ctx context.Context, scope publishScope, cfg files.QOTDConfig, slotState currentSlotState, recordAttempt func()) (bool, error) {
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
	if projected := CurrentPublishDateUTC(slotState.Schedule, scope.Now); !projected.Equal(slotState.PublishDateUTC) && isScheduledPublishSuppressed(cfg, projected) {
		return false, nil
	}

	// Past every gate — this cycle is committing to a publish attempt. recordAttempt
	// flips the metrics flag so the defer accounting captures the
	// reservation/provisioning/Discord path even when an early DB error
	// short-circuits before completeOfficialPostProvisioning runs.
	recordAttempt()

	question, err := s.store.ReserveNextQOTDQuestion(ctx, scope.GuildID, deck.ID, slotState.PublishDateUTC, deckQuestionSelector(deck))
	if err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	if question == nil {
		return false, ErrNoQuestionsAvailable
	}
	counts, err := s.deckQuestionCounts(ctx, scope.GuildID, deck.ID)
	if err != nil {
		s.releaseReservedQuestionBestEffort(ctx, scope.GuildID, *question)
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	availableQuestions := counts.Ready + counts.Draft

	lifecycle := EvaluateOfficialPost(slotState.Schedule, slotState.PublishDateUTC, scope.Now)
	nonce, err := generatePublishNonce()
	if err != nil {
		s.releaseReservedQuestionBestEffort(ctx, scope.GuildID, *question)
		return false, fmt.Errorf("generate qotd publish nonce: %w", err)
	}
	provisioned, err := s.store.CreateQOTDOfficialPostProvisioning(ctx, storage.QOTDOfficialPostRecord{
		GuildID:              scope.GuildID,
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
		s.releaseReservedQuestionBestEffort(ctx, scope.GuildID, *question)
		return s.handleProvisioningConflict(ctx, scope, slotState.PublishDateUTC, err)
	}

	finalized, updatedQuestion, _, err := s.completeOfficialPostProvisioning(ctx, scope.Session, officialPostProvisioningParams{
		Post:               *provisioned,
		Question:           question,
		AvailableQuestions: availableQuestions,
		ThreadName:         buildOfficialThreadName(threadDisplayNumberFromUsedCount(counts.Used, question)),
		Now:                scope.Now,
	})
	if err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}
	if updatedQuestion != nil {
		question = updatedQuestion
	}

	if err := s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, finalized.ID); err != nil {
		return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
	}

	return true, nil
}

// handleProvisioningConflict recovers when CreateQOTDOfficialPostProvisioning fails,
// typically due to a unique-index conflict with a concurrently created record. It
// resumes a still-unpublished record, defers a permanently abandoned one to admin
// action, or simply realigns the window for an already-published record.
func (s *Service) handleProvisioningConflict(ctx context.Context, scope publishScope, publishDateUTC time.Time, createErr error) (bool, error) {
	existing, conflictErr := s.lookupPublishConflictPost(ctx, scope.GuildID, publishDateUTC, createErr)
	if conflictErr != nil {
		return false, conflictErr
	}
	if isOfficialPostAbandoned(*existing) {
		// Existing record was permanently abandoned by a previous attempt
		// (unrecoverable Discord error). Don't resume it — admin must
		// intervene. Just keep the window state coherent.
		return false, s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, 0)
	}
	if !isOfficialPostPublished(*existing) {
		recovered, recoverErr := s.resumeOfficialPostProvisioning(ctx, scope.Session, *existing, scope.Now)
		if recoverErr != nil {
			return false, recoverErr
		}
		if err := s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, recovered.OfficialPost.ID); err != nil {
			return false, fmt.Errorf("Service.PublishScheduledIfDue: %w", err)
		}
		return true, nil
	}
	return false, s.reconcileOfficialPostWindow(ctx, scope.GuildID, scope.Session, scope.Now, 0)
}

// releaseReservedQuestionBestEffort returns a reserved question to the pool, logging but
// not failing on release errors — callers invoke it while already on an error path.
func (s *Service) releaseReservedQuestionBestEffort(ctx context.Context, guildID string, question storage.QOTDQuestionRecord) {
	if releaseErr := s.releaseReservedQuestion(ctx, question); releaseErr != nil {
		log.ApplicationLogger().Warn("QOTD scheduled reservation release failed", "guildID", guildID, "questionID", question.ID, "err", releaseErr)
	}
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
func (s *Service) ReconcileGuild(ctx context.Context, guildID string, session *discordgo.Session) (err error) {
	// Named return + defer captures every exit path uniformly so the
	// reconcile cycle counter is exact regardless of which gate trips
	// first. Duration includes the lock acquisition; spikes there are
	// operationally interesting (they mean publish or another reconcile
	// is holding the guild lock).
	reconcileStart := time.Now()
	defer func() {
		s.observability().RecordReconcileCycle(time.Since(reconcileStart), err)
	}()

	if err = s.validate(); err != nil {
		return fmt.Errorf("Service.ReconcileGuild: %w", err)
	}
	if session == nil {
		err = ErrDiscordUnavailable
		return err
	}

	guildID = strings.TrimSpace(guildID)

	_, err = s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		now := s.clock()
		cfg, err := s.configManager.QOTDConfig(guildID)
		if err != nil {
			return nil, fmt.Errorf("Service.ReconcileGuild: %w", err)
		}
		s.clearExpiredScheduledPublishSuppression(guildID, cfg, now)
		if err := s.reconcilePendingOfficialPosts(ctx, guildID, session, now); err != nil {
			return nil, fmt.Errorf("Service.ReconcileGuild: %w", err)
		}
		if err := s.reclaimOrphanReservedQuestions(ctx, guildID, now); err != nil {
			return nil, fmt.Errorf("Service.ReconcileGuild: %w", err)
		}
		return nil, s.reconcileOfficialPostWindow(ctx, guildID, session, now, 0)
	})

	return err
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
		return fmt.Errorf("Service.reclaimOrphanReservedQuestions: %w", err)
	}
	if len(ids) > 0 {
		s.observability().RecordOrphanReclaim(len(ids))
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

	// Discord-then-DB transition is funnelled through
	// applyOfficialPostThreadTransition. That helper documents the
	// divergence-window contract; the reconcile loop is the recovery path
	// if the DB write fails after Discord succeeded.
	_, err := s.applyOfficialPostThreadTransition(ctx, session, officialPostThreadTransition{
		Post:          post,
		ThreadState:   discordqotd.ThreadState{Pinned: false, Locked: false, Archived: false},
		TargetDBState: lifecycle.State,
		ClosedAt:      nil,
		ArchivedAt:    nil,
	})
	return err
}

func (s *Service) archiveOfficialPost(ctx context.Context, session *discordgo.Session, post storage.QOTDOfficialPostRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateArchiving), nil, nil); err != nil {
		return fmt.Errorf("Service.archiveOfficialPost: %w", err)
	}

	answerRecords, err := s.store.ListQOTDAnswerMessagesByOfficialPost(ctx, post.ID)
	if err != nil {
		return fmt.Errorf("Service.archiveOfficialPost: %w", err)
	}
	for _, answerRecord := range answerRecords {
		if err := s.archiveAnswerRecord(ctx, answerRecord, archivedAt); err != nil {
			return fmt.Errorf("Service.archiveOfficialPost: %w", err)
		}
	}

	// Message-mode posts skip Discord entirely; commit the archived state
	// directly. The Discord-thread path below routes through
	// applyOfficialPostThreadTransition so the divergence semantics stay
	// in one place.
	if strings.TrimSpace(post.DiscordThreadID) == "" {
		_, err = s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(OfficialPostStateArchived), &archivedAt, &archivedAt)
		return err
	}

	// At ArchiveAt (publish + 48h) we only lock the Discord thread.
	// Discord auto-archives it independently because the thread was
	// created with auto_archive_duration = 48h (equivalent to a mod
	// hitting "Close" in the UI). Setting Archived=true ourselves used to
	// be required, but now it would race the Discord auto-archive and
	// risk unarchiving a thread the platform just closed. The lock stays
	// so reply traffic on the past QOTD cannot reopen the thread from
	// the archived state. applyOfficialPostThreadTransition flips the
	// final state to OfficialPostStateMissingDiscord when the thread is
	// gone from Discord's side.
	_, err = s.applyOfficialPostThreadTransition(ctx, session, officialPostThreadTransition{
		Post:          post,
		ThreadState:   discordqotd.ThreadState{Pinned: false, Locked: true, Archived: false},
		TargetDBState: OfficialPostStateArchived,
		ClosedAt:      &archivedAt,
		ArchivedAt:    &archivedAt,
	})
	return err
}

func (s *Service) archiveAnswerRecord(ctx context.Context, answerRecord storage.QOTDAnswerMessageRecord, archivedAt time.Time) error {
	if _, err := s.store.UpdateQOTDAnswerMessageState(ctx, answerRecord.ID, string(AnswerRecordStateArchiving), nil, nil); err != nil {
		return fmt.Errorf("Service.archiveAnswerRecord: %w", err)
	}
	_, err := s.store.UpdateQOTDAnswerMessageState(ctx, answerRecord.ID, string(AnswerRecordStateArchived), &archivedAt, &archivedAt)
	return err
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
		return false, fmt.Errorf("Service.setThreadState: %w", err)
	}
	return false, nil
}

// logUnmanageableThreadOnce emits a single WARN per (thread, target state)
// pair so operators see the issue without the reconcile loop flooding logs.
// The metrics counter increments once per unique pair, matching the log
// dedup — operators reading /v1/health/qotd see the same "distinct
// rejection" cardinality as the WARN stream.
func (s *Service) logUnmanageableThreadOnce(threadID string, state discordqotd.ThreadState, cause error) {
	if s == nil {
		return
	}
	key := fmt.Sprintf("%s|%t|%t|%t", threadID, state.Pinned, state.Locked, state.Archived)
	if _, loaded := s.unmanageableThreadLogs.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	s.observability().RecordUnmanageableThread()
	log.ApplicationLogger().Warn(
		"QOTD thread state edit rejected by Discord; continuing without grooming the thread",
		"threadID", threadID,
		"targetPinned", state.Pinned,
		"targetLocked", state.Locked,
		"targetArchived", state.Archived,
		"err", cause,
	)
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
