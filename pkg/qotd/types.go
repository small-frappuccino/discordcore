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
