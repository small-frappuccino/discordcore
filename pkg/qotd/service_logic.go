package qotd

import (
	"fmt"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

func (s *Service) validate() error {
	if s == nil {
		return ErrServiceUnavailable
	}
	if s.configManager == nil || s.store == nil || s.publisher == nil {
		return ErrServiceUnavailable
	}
	return nil
}

func (s *Service) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

// observability returns the Metrics sink to write to. Falls back to
// NopMetrics so the publish and reconcile paths can call
// s.observability().RecordX without nil-checking, including in unit
// tests that build &Service{} directly without going through a
// constructor.
func (s *Service) observability() Metrics {
	if s == nil || s.metrics == nil {
		return NopMetrics{}
	}
	return s.metrics
}

// Metrics returns the Metrics implementation currently attached to the
// service. Exported so external readers (the /v1/health/qotd route
// handler) can pull a snapshot via the SnapshotProvider type assertion
// without granting write access to the same surface.
func (s *Service) Metrics() Metrics {
	return s.observability()
}

func normalizeActorID(actorID string) string {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return "control_api"
	}
	return actorID
}

func derefTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

// buildOfficialThreadName renders the Discord thread title shown in the QOTD
// channel sidebar. The visible number is the deck's count of used questions,
// so the sidebar reflects "this is the Nth question this deck has ever used".
// A non-positive number degrades to a bare "Question".
func buildOfficialThreadName(displayNumber int64) string {
	if displayNumber <= 0 {
		return "Question"
	}
	return fmt.Sprintf("Question #%03d", displayNumber)
}

// deckQuestionSelector translates the deck's user-facing strategy setting
// into the storage-layer selector consumed by ReserveNextQOTDQuestion. It
// lives next to the publish wiring so callers do not need to reach across
// the files / storage package boundary themselves.
func deckQuestionSelector(deck files.QOTDDeckConfig) storage.QOTDQuestionSelector {
	if deck.EffectiveSelectionStrategy() == files.QOTDSelectionStrategyRandom {
		return storage.QOTDQuestionSelectorRandom
	}
	return storage.QOTDQuestionSelectorQueue
}

func normalizeQuestionMutation(mutation QuestionMutation) (string, QuestionStatus, error) {
	body := strings.TrimSpace(mutation.Body)
	if body == "" {
		return "", "", fmt.Errorf("%w: question body is required", files.ErrInvalidQOTDInput)
	}

	status := mutation.Status
	if status == "" {
		status = QuestionStatusReady
	}
	switch status {
	case QuestionStatusDraft, QuestionStatusReady, QuestionStatusDisabled:
		return body, status, nil
	default:
		return "", "", fmt.Errorf("%w: question status must be draft, ready, or disabled", files.ErrInvalidQOTDInput)
	}
}

func isImmutableQuestion(question storage.QOTDQuestionRecord) bool {
	if question.PublishedOnceAt != nil && !question.PublishedOnceAt.IsZero() {
		return true
	}
	if question.ScheduledForDateUTC != nil && !question.ScheduledForDateUTC.IsZero() {
		return true
	}
	switch QuestionStatus(strings.TrimSpace(question.Status)) {
	case QuestionStatusReserved, QuestionStatusUsed:
		return true
	default:
		return false
	}
}

// threadDisplayNumberFromUsedCount renders the thread title number from the
// deck's used-question count. The publishing question is in Reserved state
// at title-render time and will transition to Used as part of finalization,
// so we add 1 to anticipate that transition. On resume after a crash where
// the question was already flipped to Used, the count already includes it
// and we pass through unchanged.
func threadDisplayNumberFromUsedCount(usedCount int, question *storage.QOTDQuestionRecord) int64 {
	display := int64(usedCount)
	if question == nil || QuestionStatus(strings.TrimSpace(question.Status)) != QuestionStatusUsed {
		display++
	}
	return display
}

func canPublishQOTD(deck files.QOTDDeckConfig) bool {
	return strings.TrimSpace(deck.ChannelID) != ""
}

func hasPublishedOfficialPostTarget(post *storage.QOTDOfficialPostRecord) bool {
	if post == nil {
		return false
	}
	return isOfficialPostPublished(*post)
}
