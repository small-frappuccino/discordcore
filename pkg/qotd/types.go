package qotd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type QuestionStatus string

const (
	QuestionStatusDraft    QuestionStatus = "draft"
	QuestionStatusReady    QuestionStatus = "ready"
	QuestionStatusReserved QuestionStatus = "reserved"
	QuestionStatusUsed     QuestionStatus = "used"
	QuestionStatusDisabled QuestionStatus = "disabled"
)

type PublishMode string

const (
	PublishModeScheduled PublishMode = "scheduled"
	PublishModeManual    PublishMode = "manual"
)

type PublishNowParams struct {
	ConsumeAutomaticSlot *bool `json:"consume_automatic_slot,omitempty"`
}

type SlotMaintenanceParams struct {
	DateUTC *time.Time `json:"date_utc,omitempty"`
}

type SlotMaintenanceResult struct {
	PublishDateUTC       time.Time `json:"publish_date_utc"`
	OfficialPostsCleared int       `json:"official_posts_cleared"`
	QuestionsReleased    int       `json:"questions_released"`
	ClearedSuppression   bool      `json:"cleared_suppression"`
}

type SlotMaintenancePartialError struct {
	Action                string
	Result                SlotMaintenanceResult
	FailedOfficialPostIDs []int64
	Cause                 error
}

func (e *SlotMaintenancePartialError) Error() string {
	if e == nil {
		return ""
	}
	action := strings.TrimSpace(e.Action)
	if action == "" {
		action = "maintenance"
	}
	dateLabel := "unknown-date"
	if !e.Result.PublishDateUTC.IsZero() {
		dateLabel = e.Result.PublishDateUTC.Format("2006-01-02")
	}
	failed := len(e.FailedOfficialPostIDs)
	if failed == 0 {
		return fmt.Sprintf("%s partial for %s: cleared=%d released=%d", action, dateLabel, e.Result.OfficialPostsCleared, e.Result.QuestionsReleased)
	}
	return fmt.Sprintf("%s partial for %s: cleared=%d released=%d failed=%d (post_ids=%s)",
		action,
		dateLabel,
		e.Result.OfficialPostsCleared,
		e.Result.QuestionsReleased,
		failed,
		joinInt64CSV(e.FailedOfficialPostIDs),
	)
}

func (e *SlotMaintenancePartialError) Unwrap() error {
	if e == nil {
		return nil
	}
	if e.Cause == nil {
		return ErrSlotMaintenancePartial
	}
	return errors.Join(ErrSlotMaintenancePartial, e.Cause)
}

func joinInt64CSV(values []int64) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.FormatInt(value, 10))
	}
	return strings.Join(parts, ",")
}

func (p PublishNowParams) ShouldConsumeAutomaticSlot() bool {
	return p.ConsumeAutomaticSlot == nil || *p.ConsumeAutomaticSlot
}

type OfficialPostState string

const (
	OfficialPostStateProvisioning   OfficialPostState = "provisioning"
	OfficialPostStateCurrent        OfficialPostState = "current"
	OfficialPostStatePrevious       OfficialPostState = "previous"
	OfficialPostStateArchiving      OfficialPostState = "archiving"
	OfficialPostStateArchived       OfficialPostState = "archived"
	OfficialPostStateMissingDiscord OfficialPostState = "missing_discord"
	// OfficialPostStateFailed is a transient failure (DB hiccup, DNS blip,
	// 5xx from Discord). The reconcile loop retries it every cycle.
	OfficialPostStateFailed OfficialPostState = "failed"
	// OfficialPostStateAbandoned is a terminal failure that the bot cannot
	// recover from on its own — the channel was deleted, the bot was kicked
	// from the guild, or it lost the permissions to post. The reconcile loop
	// must NOT retry these or it spams Discord every 15 minutes forever; an
	// admin has to fix the Discord-side state and re-trigger publishing
	// manually.
	OfficialPostStateAbandoned OfficialPostState = "abandoned"
)

type AnswerRecordState string

const (
	AnswerRecordStateProvisioning   AnswerRecordState = "provisioning"
	AnswerRecordStateActive         AnswerRecordState = "active"
	AnswerRecordStateArchiving      AnswerRecordState = "archiving"
	AnswerRecordStateArchived       AnswerRecordState = "archived"
	AnswerRecordStateMissingDiscord AnswerRecordState = "missing_discord"
	AnswerRecordStateFailed         AnswerRecordState = "failed"
)

type AnswerWindow struct {
	IsOpen   bool
	State    OfficialPostState
	ClosesAt time.Time
}

type OfficialPostLifecycle struct {
	PublishDateUTC    time.Time
	PublishAt         time.Time
	BecomesPreviousAt time.Time
	ArchiveAt         time.Time
	State             OfficialPostState
	AnswerWindow      AnswerWindow
}

type DeckSummary struct {
	Deck           files.QOTDDeckConfig
	Counts         QuestionCounts
	CardsRemaining int
	IsActive       bool
	CanPublish     bool
}
