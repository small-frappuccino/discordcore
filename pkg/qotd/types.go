package qotd

import (
	"errors"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

type QuestionRecord = storage.QOTDQuestionRecord
type OfficialPostRecord = storage.QOTDOfficialPostRecord

// QuestionStatus is the lifecycle state of a QOTD question as it moves from
// authoring through reservation to publication. See the QuestionStatus*
// constants.
type QuestionStatus string

const (
	QuestionStatusDraft    QuestionStatus = "draft"
	QuestionStatusReady    QuestionStatus = "ready"
	QuestionStatusReserved QuestionStatus = "reserved"
	QuestionStatusUsed     QuestionStatus = "used"
	QuestionStatusDisabled QuestionStatus = "disabled"
)

// PublishMode distinguishes a post produced by the automatic scheduler
// (PublishModeScheduled) from one triggered by an operator (PublishModeManual).
// It participates in the scheduled-publish uniqueness index.
type PublishMode string

const (
	PublishModeScheduled PublishMode = "scheduled"
	PublishModeManual    PublishMode = "manual"
)

// PublishNowParams tunes a manual publish. A nil ConsumeAutomaticSlot defaults
// to consuming the slot; PublishDateOverride and IsReplacement support replacing
// an existing slot's post rather than advancing the queue.
type PublishNowParams struct {
	ConsumeAutomaticSlot *bool      `json:"consume_automatic_slot,omitempty"`
	PublishDateOverride  *time.Time `json:"-"`
	IsReplacement        bool       `json:"-"`
}

// ShouldConsumeAutomaticSlot reports whether the publish should consume the
// automatic queue slot, defaulting to true when ConsumeAutomaticSlot is unset.
func (p PublishNowParams) ShouldConsumeAutomaticSlot() bool {
	return p.ConsumeAutomaticSlot == nil || *p.ConsumeAutomaticSlot
}

// OfficialPostState is the lifecycle state of an official QOTD post. It drives
// reconcile behavior: most states are transient and advance automatically,
// while Failed is retryable and Abandoned is terminal (see the constants below
// and isUnrecoverableDiscordPublishError).
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

// AnswerRecordState is the lifecycle state of the answer surface attached to an
// official post, tracked separately from the post itself so the answer thread
// can be reconciled independently. See the AnswerRecordState* constants.
type AnswerRecordState string

const (
	AnswerRecordStateProvisioning   AnswerRecordState = "provisioning"
	AnswerRecordStateActive         AnswerRecordState = "active"
	AnswerRecordStateArchiving      AnswerRecordState = "archiving"
	AnswerRecordStateArchived       AnswerRecordState = "archived"
	AnswerRecordStateMissingDiscord AnswerRecordState = "missing_discord"
	AnswerRecordStateFailed         AnswerRecordState = "failed"
)

var ErrNoCurrentPublish = errors.New("no current qotd publish found to replace")

// AnswerWindow describes whether answers are currently accepted for a post and,
// when open, the moment the window closes.
type AnswerWindow struct {
	IsOpen   bool
	State    OfficialPostState
	ClosesAt time.Time
}

// OfficialPostLifecycle holds the computed timeline of a post: when it
// publishes, becomes the previous post, and is archived, plus the derived state
// and answer window. The times are deterministic functions of PublishDateUTC
// and the schedule.
type OfficialPostLifecycle struct {
	PublishDateUTC    time.Time
	PublishAt         time.Time
	BecomesPreviousAt time.Time
	ArchiveAt         time.Time
	State             OfficialPostState
	AnswerWindow      AnswerWindow
}

// DeckSummary reports a single deck's authoring state: its config, question
// counts, how many cards remain to publish, and whether it is active and
// currently publishable.
type DeckSummary struct {
	Deck           files.QOTDDeckConfig
	Counts         QuestionCounts
	CardsRemaining int
	IsActive       bool
	CanPublish     bool
}
