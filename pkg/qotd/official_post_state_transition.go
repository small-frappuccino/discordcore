package qotd

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// ErrOfficialPostStateDivergence is the sentinel wrapped by
// OfficialPostStateDivergenceError so callers can branch with errors.Is
// without depending on the concrete error type.
var ErrOfficialPostStateDivergence = errors.New("qotd official post state divergence")

// OfficialPostStateDivergenceError signals the asymmetric failure mode of
// a thread-state transition: Discord accepted the new thread state but
// the subsequent DB write to record the new lifecycle state failed.
//
// This is the *expected* failure mode under the "best-effort with bounded
// convergence" contract documented on applyOfficialPostThreadTransition.
// The reconcile loop will pick up the post on the next cycle, see that
// the DB state still does not match the lifecycle target, reapply the
// (idempotent) thread state, and retry the DB write. The system
// converges; the divergence window is bounded by the reconcile interval
// (~1 minute in production).
//
// We expose the asymmetric failure as a distinct type for two reasons:
//  1. Callers can choose to swallow it (when the reconcile loop will
//     recover and surfacing the error would just be noise) or propagate
//     it (when an operator needs to see the transient state immediately).
//  2. A structured log on every divergence event gives operators a
//     single grep target ("qotd_official_post_state_divergence") to
//     distinguish transient blips from a persistent DB outage.
type OfficialPostStateDivergenceError struct {
	OfficialPostID int64
	IntendedState  OfficialPostState
	Cause          error
}

// Error errors.
func (e *OfficialPostStateDivergenceError) Error() string {
	if e == nil {
		return ""
	}
	intended := strings.TrimSpace(string(e.IntendedState))
	if intended == "" {
		intended = "<unknown>"
	}
	if e.Cause == nil {
		return fmt.Sprintf("qotd official post %d: thread state applied but DB transition to %s failed", e.OfficialPostID, intended)
	}
	return fmt.Sprintf("qotd official post %d: thread state applied but DB transition to %s failed: %v", e.OfficialPostID, intended, e.Cause)
}

// Unwrap unwraps.
func (e *OfficialPostStateDivergenceError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.Cause == nil {
		return ErrOfficialPostStateDivergence
	}
	return errors.Join(ErrOfficialPostStateDivergence, e.Cause)
}

// officialPostThreadTransition bundles the inputs to
// applyOfficialPostThreadTransition: the persisted post whose Discord thread
// is being transitioned, the target Discord thread state, the
// OfficialPostState to persist on success, and the optional closed/archived
// timestamps forwarded to the DB write.
type officialPostThreadTransition struct {
	Post          storage.QOTDOfficialPostRecord
	ThreadState   ThreadState
	TargetDBState OfficialPostState
	ClosedAt      *time.Time
	ArchivedAt    *time.Time
}

// applyOfficialPostThreadTransition is the canonical seam through which a
// QOTD official-post lifecycle transition touches BOTH Discord and the
// DB. Every code path that flips state on a post with a Discord thread
// should route through here so the divergence semantics live in exactly
// one place.
//
// # Contract
//
// Order: Discord first, DB second. Discord is the user-visible state;
// the DB row is our cache of it. If Discord succeeds and the DB write
// fails the user still sees the correct thread state — only our internal
// decisions lag by at most one reconcile cycle. Reversing the order
// would let the DB get ahead of Discord, which means we would make
// decisions assuming a transition completed when the user still sees
// the old thread state. That asymmetry is strictly worse.
//
// Missing thread (404 / Unknown Channel): setThreadState reports
// missing=true; we skip the requested target and flip the DB row to
// OfficialPostStateMissingDiscord. No divergence error.
//
// Unmanageable thread (403 / Missing Access / Missing Permissions on a
// thread the bot can otherwise post in): setThreadState already degrades
// silently with log-once dedup on Service.unmanageableThreadLogs and
// returns missing=false err=nil. This helper sees the success and
// advances the DB row to the requested target anyway — the lifecycle
// model is "the DB describes what *should* be true on Discord"; if
// Discord cannot be groomed, the model is still correct intent.
//
// DB write failure after a successful Discord call: return
// OfficialPostStateDivergenceError wrapping the underlying cause. The
// reconcile loop is the recovery path; on the next cycle the
// short-circuit in syncLiveOfficialPost will see DB.State !=
// lifecycle.State and reapply (idempotent) the same thread state, then
// retry the DB write.
//
// # Why not an outbox or two-phase commit
//
// The reconcile loop already provides bounded convergence (~1 minute).
// The bot is the only writer to qotd_official_posts and only one bot
// instance writes per guild (guildLifecycleLock serializes manual and
// scheduled publishes). A true outbox would require a new table, a
// drain worker, and additional observability — costs that exceed the
// benefit of shrinking the divergence window from one reconcile cycle
// to zero.
//
// # Parameters
//
// The transition inputs are bundled in an officialPostThreadTransition:
//
//   - Post: the current persisted record. Used for IDs and the
//     Discord thread ID; the helper does not mutate it.
//   - ThreadState: the target Discord thread state. For current/previous
//     transitions, all three flags are false. For archive, Locked=true
//     while Archived stays false (Discord's auto_archive_duration
//     handles the visible archive).
//   - TargetDBState: the OfficialPostState to persist on success. Will
//     be overridden to OfficialPostStateMissingDiscord if the thread is
//     gone from Discord's side.
//   - ClosedAt, ArchivedAt: passed through to UpdateQOTDOfficialPostState
//     unchanged. nil for live transitions, non-nil for archive.
func (s *Service) applyOfficialPostThreadTransition(ctx context.Context, params officialPostThreadTransition) (*storage.QOTDOfficialPostRecord, error) {
	post := params.Post
	threadState := params.ThreadState
	targetDBState := params.TargetDBState
	closedAt := params.ClosedAt
	archivedAt := params.ArchivedAt

	if strings.TrimSpace(post.DiscordThreadID) == "" {
		// Message-mode posts (no Discord thread, e.g. legacy/manual seed
		// rows) skip Discord entirely; there is no divergence to manage.
		updated, err := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(targetDBState), closedAt, archivedAt)
		if err != nil {
			return nil, fmt.Errorf("Service.applyOfficialPostThreadTransition: %w", err)
		}
		return updated, nil
	}

	missing, err := s.setThreadState(ctx, post.GuildID, post.DiscordThreadID, threadState)
	if err != nil {
		return nil, fmt.Errorf("Service.applyOfficialPostThreadTransition: %w", err)
	}

	finalState := targetDBState
	if missing {
		finalState = OfficialPostStateMissingDiscord
	}

	updated, dbErr := s.store.UpdateQOTDOfficialPostState(ctx, post.ID, string(finalState), closedAt, archivedAt)
	if dbErr != nil {
		s.observability().RecordStateDivergence()
		log.ApplicationLogger().Warn(
			"qotd_official_post_state_divergence",
			"officialPostID", post.ID,
			"guildID", post.GuildID,
			"threadID", post.DiscordThreadID,
			"intendedState", finalState,
			"previousState", strings.TrimSpace(post.State),
			"err", dbErr,
		)
		return nil, &OfficialPostStateDivergenceError{
			OfficialPostID: post.ID,
			IntendedState:  finalState,
			Cause:          dbErr,
		}
	}
	return updated, nil
}
