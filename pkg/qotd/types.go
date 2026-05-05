package qotd

import (
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
	OfficialPostStateFailed         OfficialPostState = "failed"
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
