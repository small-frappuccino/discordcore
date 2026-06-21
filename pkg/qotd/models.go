package qotd

import (
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// QuestionRecord represents question record.
type QuestionRecord = storage.QOTDQuestionRecord

// OfficialPostRecord represents official post record.
type OfficialPostRecord = storage.QOTDOfficialPostRecord

// QuestionStatus is the lifecycle state of a QOTD question as it moves from
// authoring through reservation to publication.
type QuestionStatus string

// QuestionStatusDraft defines question status draft.
// QuestionStatusReady defines question status ready.
// QuestionStatusReserved defines question status reserved.
// QuestionStatusUsed defines question status used.
// QuestionStatusDisabled defines question status disabled.
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

// PublishModeScheduled defines publish mode scheduled.
// PublishModeManual defines publish mode manual.
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
// while Failed is retryable and Abandoned is terminal.
type OfficialPostState string

// OfficialPostStateArchiving defines official post state archiving.
// OfficialPostStatePrevious defines official post state previous.
// OfficialPostStateArchived defines official post state archived.
// OfficialPostStateProvisioning defines official post state provisioning.
// OfficialPostStateMissingDiscord defines official post state missing discord.
// OfficialPostStateCurrent defines official post state current.
// OfficialPostStateFailed defines official post state failed.
// OfficialPostStateAbandoned defines official post state abandoned.
const (
	OfficialPostStateProvisioning   OfficialPostState = "provisioning"
	OfficialPostStateCurrent        OfficialPostState = "current"
	OfficialPostStatePrevious       OfficialPostState = "previous"
	OfficialPostStateArchiving      OfficialPostState = "archiving"
	OfficialPostStateArchived       OfficialPostState = "archived"
	OfficialPostStateMissingDiscord OfficialPostState = "missing_discord"
	OfficialPostStateFailed         OfficialPostState = "failed"
	OfficialPostStateAbandoned      OfficialPostState = "abandoned"
)

// AnswerRecordState is the lifecycle state of the answer surface attached to an
// official post, tracked separately from the post itself so the answer thread
// can be reconciled independently.
type AnswerRecordState string

// AnswerRecordStateActive defines answer record state active.
// AnswerRecordStateArchiving defines answer record state archiving.
// AnswerRecordStateArchived defines answer record state archived.
// AnswerRecordStateMissingDiscord defines answer record state missing discord.
// AnswerRecordStateFailed defines answer record state failed.
// AnswerRecordStateProvisioning defines answer record state provisioning.
const (
	AnswerRecordStateProvisioning   AnswerRecordState = "provisioning"
	AnswerRecordStateActive         AnswerRecordState = "active"
	AnswerRecordStateArchiving      AnswerRecordState = "archiving"
	AnswerRecordStateArchived       AnswerRecordState = "archived"
	AnswerRecordStateMissingDiscord AnswerRecordState = "missing_discord"
	AnswerRecordStateFailed         AnswerRecordState = "failed"
)

// AnswerWindow describes whether answers are currently accepted for a post and,
// when open, the moment the window closes.
type AnswerWindow struct {
	IsOpen   bool
	State    OfficialPostState
	ClosesAt time.Time
}

// OfficialPostLifecycle holds the computed timeline of a post: when it
// publishes, becomes the previous post, and is archived, plus the derived state
// and answer window.
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

// QuestionCounts holds aggregated statistics for a QOTD deck.
type QuestionCounts struct {
	Draft    int
	Ready    int
	Reserved int
	Used     int
	Disabled int
}

// PublishOfficialPostParams is the full set of inputs needed to publish (or
// idempotently re-publish) an official QOTD post to Discord.
type PublishOfficialPostParams struct {
	GuildID                    string
	OfficialPostID             int64
	DisplayID                  int64
	DeckName                   string
	AvailableQuestions         int
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	OfficialThreadID           string
	OfficialStarterMessageID   string
	OfficialAnswerChannelID    string
	ExistingPublishedAt        time.Time
	QuestionText               string
	PublishDateUTC             time.Time
	ThreadName                 string
	Nonce                      string
}

// PublishedOfficialPost reports the Discord-side identifiers produced by a
// successful publish, which the caller persists back onto the official-post
// record.
type PublishedOfficialPost struct {
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	ThreadID                   string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
	PostURL                    string
}

// ThreadState is the desired pin/lock/archive state applied to a QOTD thread.
type ThreadState struct {
	Pinned   bool
	Locked   bool
	Archived bool
}

// DeleteOfficialPostParams carries the parameters to best-effort delete a QOTD official post from Discord.
type DeleteOfficialPostParams struct {
	GuildID                    string
	DiscordThreadID            string
	DiscordStarterMessageID    string
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
}
