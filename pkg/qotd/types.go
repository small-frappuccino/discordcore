package qotd

import "time"

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

type ReplyThreadState string

const (
	ReplyThreadStateProvisioning   ReplyThreadState = "provisioning"
	ReplyThreadStateActive         ReplyThreadState = "active"
	ReplyThreadStateArchiving      ReplyThreadState = "archiving"
	ReplyThreadStateArchived       ReplyThreadState = "archived"
	ReplyThreadStateMissingDiscord ReplyThreadState = "missing_discord"
	ReplyThreadStateFailed         ReplyThreadState = "failed"
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
