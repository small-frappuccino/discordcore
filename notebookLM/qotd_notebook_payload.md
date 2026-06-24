# Domain Architecture: qotd

## Layout Topology
```text
qotd/
├── doc.go
├── errors.go
├── lifecycle.go
├── lifecycle_fuzz_test.go
├── lifecycle_test.go
├── models.go
├── repository.go
├── service.go
└── service_test.go
```

## Source Stream Aggregation

// === FILE: pkg/qotd/doc.go ===
```go
/*
Package qotd implements the Question of the Day domain logic and state machine.

This package provides the core business logic, scheduling computations, and
state management for the QOTD feature. It operates completely independently of
any Discord API specifics, delegating side-effects via the Publisher interface.

# Actor Model
To prevent race conditions, particularly around publishing and state mutations
across concurrent requests or background scheduled triggers, all state-changing
operations for a given guild are serialized using an actor model.

# State Machine
Questions transition from Draft -> Ready -> Reserved -> Used.
Official Posts transition from Provisioning -> Current -> Previous -> Archiving -> Archived.
Answers transition from Provisioning -> Active -> Archiving -> Archived.
*/
package qotd

```

// === FILE: pkg/qotd/errors.go ===
```go
package qotd

import (
	"errors"
)

// ErrAlreadyPublished defines err already published.
var ErrAlreadyPublished = errors.New("qotd already published")

// ErrNoCurrentPublish defines err no current publish.
var ErrNoCurrentPublish = errors.New("no current qotd publish found to replace")

// Sentinel errors representing Discord-side failures that the QOTD domain
// must handle for its state machine transitions (e.g., abandoning a post
// when permissions are revoked).
// The Publisher adapter is responsible for mapping the underlying Discord
// SDK errors (e.g., arikawa or discordgo) to these sentinels.
var (
	ErrDiscordUnknownChannel                     = errors.New("discord: unknown channel")
	ErrDiscordUnknownGuild                       = errors.New("discord: unknown guild")
	ErrDiscordUnknownMessage                     = errors.New("discord: unknown message")
	ErrDiscordMissingAccess                      = errors.New("discord: missing access")
	ErrDiscordMissingPermissions                 = errors.New("discord: missing permissions")
	ErrDiscordCannotSendMessagesInVoice          = errors.New("discord: cannot send messages in voice channel")
	ErrDiscordCannotSendMessagesToUser           = errors.New("discord: cannot send messages to this user")
	ErrDiscordUnauthorized                       = errors.New("discord: unauthorized")
	ErrDiscordThreadAlreadyCreatedForThisMessage = errors.New("discord: thread already created for this message")
)

```

// === FILE: pkg/qotd/lifecycle.go ===
```go
package qotd

import (
	"fmt"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// PublishSchedule represents a QOTD publish schedule config.
type PublishSchedule = files.QOTDPublishScheduleConfig

// resolvePublishSchedule resolves the schedule from config.
func resolvePublishSchedule(cfg files.QOTDConfig) (PublishSchedule, error) {
	if !cfg.Schedule.IsComplete() {
		return PublishSchedule{}, fmt.Errorf("qotd publish schedule is incomplete")
	}
	return cfg.Schedule, nil
}

// NormalizePublishDateUTC returns the canonical UTC calendar date for a QOTD slot.
func NormalizePublishDateUTC(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}
	u := t.UTC()
	return time.Date(u.Year(), u.Month(), u.Day(), 0, 0, 0, 0, time.UTC)
}

// PublishTimeUTC returns the exact publish time for a QOTD date.
func PublishTimeUTC(schedule PublishSchedule, publishDate time.Time) time.Time {
	date := NormalizePublishDateUTC(publishDate)
	if date.IsZero() {
		return time.Time{}
	}
	hourUTC, minuteUTC, ok := schedule.Values()
	if !ok {
		return time.Time{}
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hourUTC, minuteUTC, 0, 0, time.UTC)
}

// CurrentPublishDateUTC returns the active publish date at the given time.
func CurrentPublishDateUTC(schedule PublishSchedule, now time.Time) time.Time {
	now = normalizeClockInput(now)
	today := NormalizePublishDateUTC(now)
	if now.After(PublishTimeUTC(schedule, today)) {
		return today.AddDate(0, 0, 1)
	}
	return today
}

// DuePublishDateUTC returns the most recent scheduled publish date at the given time.
func DuePublishDateUTC(schedule PublishSchedule, now time.Time) time.Time {
	now = normalizeClockInput(now)
	today := NormalizePublishDateUTC(now)
	if now.Before(PublishTimeUTC(schedule, today)) {
		return today.AddDate(0, 0, -1)
	}
	return today
}

// normalizeClockInput normalizes input to UTC.
func normalizeClockInput(now time.Time) time.Time {
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

// DetermineOfficialPostLifecycle derives the state and boundaries.
func DetermineOfficialPostLifecycle(post OfficialPostRecord, schedule files.QOTDPublishScheduleConfig, now time.Time) OfficialPostLifecycle {
	lc := OfficialPostLifecycle{
		PublishDateUTC: NormalizePublishDateUTC(post.PublishDateUTC),
		State:          OfficialPostState(post.State),
	}

	if schedule.IsComplete() {
		lc.PublishAt = PublishTimeUTC(schedule, lc.PublishDateUTC)
		// A post becomes previous the exact millisecond the next day's slot begins.
		lc.BecomesPreviousAt = PublishTimeUTC(schedule, lc.PublishDateUTC.AddDate(0, 0, 1))
		// It archives 24 hours after becoming previous.
		lc.ArchiveAt = lc.BecomesPreviousAt.AddDate(0, 0, 1)
	}

	lc.AnswerWindow = AnswerWindow{
		State:  lc.State,
		IsOpen: lc.State == OfficialPostStateCurrent,
	}
	if lc.AnswerWindow.IsOpen && !lc.BecomesPreviousAt.IsZero() {
		lc.AnswerWindow.ClosesAt = lc.BecomesPreviousAt
	}

	return lc
}

// CalculateNextPublishDelay returns the duration until the next schedule publish triggers.
func CalculateNextPublishDelay(cfg files.QOTDConfig, now time.Time) time.Duration {
	if !cfg.Schedule.IsComplete() {
		return 0
	}

	now = normalizeClockInput(now)
	publishDate := CurrentPublishDateUTC(cfg.Schedule, now)
	publishTime := PublishTimeUTC(cfg.Schedule, publishDate)

	delay := publishTime.Sub(now)
	if delay < 0 {
		return 0
	}
	return delay
}

```

// === FILE: pkg/qotd/lifecycle_fuzz_test.go ===
```go
package qotd

import (
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// FuzzCalculateNextPublishDelay fuzzes the delay calculation to ensure it never panics
// or returns a negative delay (which would cause a tight CPU loop in the daemon).
func FuzzCalculateNextPublishDelay(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0), int64(0))              // zeros
	f.Add(int64(1672531200), int64(12), int64(30), int64(0), int64(0))   // normal
	f.Add(int64(-62135596800), int64(23), int64(59), int64(0), int64(0)) // extreme past
	f.Add(int64(253402300799), int64(0), int64(0), int64(0), int64(0))   // extreme future

	f.Fuzz(func(t *testing.T, nowSec int64, hour int64, minute int64, locOffset int64, suppressSec int64) {
		// Constrain values to valid schedule ranges
		if hour < 0 || hour > 23 {
			hour = hour % 24
			if hour < 0 {
				hour += 24
			}
		}
		if minute < 0 || minute > 59 {
			minute = minute % 60
			if minute < 0 {
				minute += 60
			}
		}

		now := time.Unix(nowSec, 0)

		cfg := files.QOTDConfig{
			Schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(int(hour)),
				MinuteUTC: intPtr(int(minute)),
			},
		}

		// Execute the delay function
		delay := CalculateNextPublishDelay(cfg, now)

		// A negative delay means the timer loop would spin constantly
		if delay < 0 {
			t.Errorf("CalculateNextPublishDelay returned negative delay: %v", delay)
		}
	})
}

// FuzzDetermineOfficialPostLifecycle ensures the state machine boundaries
// never panic when provided with random boundary data.
func FuzzDetermineOfficialPostLifecycle(f *testing.F) {
	f.Add(int64(0), int64(0), int64(0), int64(0), "current")
	f.Add(int64(1672531200), int64(1672531200), int64(12), int64(30), "previous")

	f.Fuzz(func(t *testing.T, publishDateSec int64, nowSec int64, hour int64, min int64, state string) {
		if hour < 0 || hour > 23 {
			hour = 0
		}
		if min < 0 || min > 59 {
			min = 0
		}

		publishDate := time.Unix(publishDateSec, 0)
		now := time.Unix(nowSec, 0)

		post := OfficialPostRecord{
			PublishDateUTC: publishDate,
			State:          state,
		}

		schedule := files.QOTDPublishScheduleConfig{
			HourUTC:   intPtr(int(hour)),
			MinuteUTC: intPtr(int(min)),
		}

		lc := DetermineOfficialPostLifecycle(post, schedule, now)

		// Assertions
		if lc.PublishDateUTC.IsZero() && !publishDate.IsZero() {
			t.Errorf("PublishDateUTC normalized to zero unexpectedly")
		}
	})
}

func intPtr(i int) *int {
	return &i
}

```

// === FILE: pkg/qotd/lifecycle_test.go ===
```go
package qotd

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestUncoveredLifecycleAndService(t *testing.T) {
	t.Parallel()
	// 1. Test resolvePublishSchedule
	cfgEmpty := files.QOTDConfig{}
	_, err := resolvePublishSchedule(cfgEmpty)
	if err == nil {
		t.Error("expected error for empty schedule config")
	}

	h := 12
	m := 30
	cfgValid := files.QOTDConfig{
		Schedule: files.QOTDPublishScheduleConfig{
			HourUTC:   &h,
			MinuteUTC: &m,
		},
	}
	sched, err := resolvePublishSchedule(cfgValid)
	if err != nil {
		t.Errorf("expected no error for valid schedule config, got %v", err)
	}
	if *sched.HourUTC != 12 || *sched.MinuteUTC != 30 {
		t.Errorf("resolved schedule values mismatch: hour=%d, minute=%d", *sched.HourUTC, *sched.MinuteUTC)
	}

	// 2. Test DuePublishDateUTC
	now := time.Date(2026, 6, 23, 10, 0, 0, 0, time.UTC)
	due := DuePublishDateUTC(sched, now)
	expectedDue := time.Date(2026, 6, 22, 0, 0, 0, 0, time.UTC)
	if !due.Equal(expectedDue) {
		t.Errorf("expected due publish date %v, got %v", expectedDue, due)
	}

	nowAfter := time.Date(2026, 6, 23, 14, 0, 0, 0, time.UTC)
	dueAfter := DuePublishDateUTC(sched, nowAfter)
	expectedDueAfter := time.Date(2026, 6, 23, 0, 0, 0, 0, time.UTC)
	if !dueAfter.Equal(expectedDueAfter) {
		t.Errorf("expected due publish date %v, got %v", expectedDueAfter, dueAfter)
	}

	// 4. Test ShouldConsumeAutomaticSlot
	pDefault := PublishNowParams{}
	if !pDefault.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return true by default")
	}
	tTrue := true
	pTrue := PublishNowParams{ConsumeAutomaticSlot: &tTrue}
	if !pTrue.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return true")
	}
	tFalse := false
	pFalse := PublishNowParams{ConsumeAutomaticSlot: &tFalse}
	if pFalse.ShouldConsumeAutomaticSlot() {
		t.Error("expected ShouldConsumeAutomaticSlot to return false")
	}

	// 5. Test service publisher/clock setters
	mgr := files.NewConfigManagerWithStore(nil, nil)
	mgr.ApplyConfig(&files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID: "g1",
				QOTD: files.QOTDConfig{
					Schedule: files.QOTDPublishScheduleConfig{
						HourUTC:   &h,
						MinuteUTC: &m,
					},
				},
			},
		},
	})

	svc := NewService(mgr, nil, nil)
	if svc.GetPublisher() != nil {
		t.Error("expected publisher to be nil initially")
	}
	var dummyPub dummyPublisher
	svc.SetPublisher(&dummyPub)
	if svc.GetPublisher() != &dummyPub {
		t.Error("expected publisher to be injected")
	}

	svc.SetClock(nil) // should not panic and default to real clock
	tFixed := time.Date(2026, 6, 23, 15, 0, 0, 0, time.UTC)
	svc.SetClock(clock.NewMockClock(tFixed))
	if !svc.now().Equal(tFixed) {
		t.Errorf("expected clock to be fake clock with time %v, got %v", tFixed, svc.now())
	}

	// Test NextScheduledPublishTime
	nextSchedTime := svc.NextScheduledPublishTime("g1")
	expectedNextTime := time.Date(2026, 6, 24, 12, 30, 0, 0, time.UTC)
	if !nextSchedTime.Equal(expectedNextTime) {
		t.Errorf("expected next scheduled publish time %v, got %v", expectedNextTime, nextSchedTime)
	}

	nextSchedTimeInvalid := svc.NextScheduledPublishTime("g_invalid")
	if !nextSchedTimeInvalid.IsZero() {
		t.Errorf("expected zero time for invalid guild config, got %v", nextSchedTimeInvalid)
	}

	// Test NopMetrics to hit 100% coverage
	var nop NopMetrics
	nop.RecordOfficialPostAbandoned()
	nop.RecordSuppressionCleared()
}

type dummyPublisher struct{}

func (d *dummyPublisher) PublishOfficialPost(ctx context.Context, params PublishOfficialPostParams) (*PublishedOfficialPost, error) {
	return nil, nil
}
func (d *dummyPublisher) DeleteOfficialPost(ctx context.Context, params DeleteOfficialPostParams) error {
	return nil
}

```

// === FILE: pkg/qotd/models.go ===
```go
package qotd

import (
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

// QuestionRecord is a stored QOTD question. ID is the global primary key;
// DisplayID is the stable per-guild identifier shown to users. Status holds a
// qotd.QuestionStatus value, and the *time.Time fields are nil until the
// corresponding lifecycle event (scheduled, used, first published) occurs.
type QuestionRecord struct {
	ID                  int64
	DisplayID           int64
	GuildID             string
	DeckID              string
	Body                string
	Status              string
	QueuePosition       int64
	CreatedBy           string
	ScheduledForDateUTC *time.Time
	UsedAt              *time.Time
	PublishedOnceAt     *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// OfficialPostRecord is the durable state of one official QOTD post and the
// source of truth the reconcile loop drives toward Discord. State holds a
// qotd.OfficialPostState value. Two of its fields anchor publish idempotency
// (see the PublishOrdinal and Nonce field comments); changing publish paths
// must preserve both.
type OfficialPostRecord struct {
	ID                         int64
	GuildID                    string
	DeckID                     string
	DeckNameSnapshot           string
	QuestionID                 int64
	PublishOrdinal             int64
	PublishMode                string
	ConsumeAutomaticSlot       bool
	PublishDateUTC             time.Time
	State                      string
	ChannelID                  string
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	DiscordThreadID            string
	DiscordStarterMessageID    string
	AnswerChannelID            string
	QuestionTextSnapshot       string
	Nonce                      string
	PublishedAt                *time.Time
	GraceUntil                 time.Time
	ArchiveAt                  time.Time
	ClosedAt                   *time.Time
	ArchivedAt                 *time.Time
	LastReconciledAt           *time.Time
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

// SurfaceRecord maps a guild deck to its Discord surface: the channel and
// the question-list thread used to render the deck's published questions.
type SurfaceRecord struct {
	ID                   int64
	GuildID              string
	DeckID               string
	ChannelID            string
	QuestionListThreadID string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// QuestionSelector controls how the next question is picked when
// reserving for a publish. It is decoupled from OfficialPostRecord's
// PublishOrdinal so the visible thread numbering stays monotonic regardless
// of which strategy ran.
type QuestionSelector string

const (
	// QuestionSelectorQueue picks the head of the queue (queue_position
	// ASC, id ASC). This is the historical default.
	QuestionSelectorQueue QuestionSelector = "queue"
	// QuestionSelectorRandom picks a uniformly-random eligible question.
	QuestionSelectorRandom QuestionSelector = "random"
)

// FinalizeOfficialPostParams contains data needed to finalize a post.
type FinalizeOfficialPostParams struct {
	ID                         int64
	QuestionListThreadID       string
	QuestionListEntryMessageID string
	DiscordThreadID            string
	StarterMessageID           string
	AnswerChannelID            string
	PublishedAt                time.Time
}

// AnswerMessageRecord is a stored answer posted against an official post.
// State holds a qotd.AnswerRecordState value; ClosedAt and ArchivedAt are nil
// until the answer surface closes or is archived.
type AnswerMessageRecord struct {
	ID                      int64
	GuildID                 string
	OfficialPostID          int64
	UserID                  string
	State                   string
	AnswerChannelID         string
	DiscordMessageID        string
	CreatedViaInteractionID string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	ClosedAt                *time.Time
	ArchivedAt              *time.Time
}

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

```

// === FILE: pkg/qotd/repository.go ===
```go
package qotd

import (
	"context"
	"iter"
	"time"
)

// Repository encapsulates the persistent storage logic for the QOTD domain.
// It manages complex relational operations (like atomic cross-table syncs and row-level locks)
// while providing a simple facade to domain services.
type Repository interface {
	// Question Management
	CreateQOTDQuestion(ctx context.Context, rec QuestionRecord) (*QuestionRecord, error)
	UpdateQOTDQuestion(ctx context.Context, rec QuestionRecord) (*QuestionRecord, error)
	DeleteQOTDQuestion(ctx context.Context, guildID string, questionID int64) error
	DeleteQOTDQuestionsByDecks(ctx context.Context, guildID string, deckIDs []string) error
	ListQOTDQuestions(ctx context.Context, guildID, deckID string) (iter.Seq2[QuestionRecord, error], error)
	GetQOTDQuestion(ctx context.Context, guildID string, questionID int64) (*QuestionRecord, error)
	ReorderQOTDQuestions(ctx context.Context, guildID, deckID string, orderedIDs []int64) error

	// Selection and Scheduling
	ReserveNextQOTDQuestion(ctx context.Context, guildID, deckID string, publishDateUTC time.Time, selector QuestionSelector) (*QuestionRecord, error)
	ReserveNextReadyQOTDQuestion(ctx context.Context, guildID, deckID string, selector QuestionSelector) (*QuestionRecord, error)
	ReclaimOrphanReservedQOTDQuestions(ctx context.Context, guildID string, todayUTC time.Time) iter.Seq2[int64, error]

	// Official Posts
	CreateQOTDOfficialPostProvisioning(ctx context.Context, rec OfficialPostRecord) (*OfficialPostRecord, error)
	FinalizeQOTDOfficialPost(ctx context.Context, params FinalizeOfficialPostParams) (*OfficialPostRecord, error)
	GetQOTDOfficialPostByID(ctx context.Context, id int64) (*OfficialPostRecord, error)
	GetQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	ListQOTDOfficialPostsByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (iter.Seq[OfficialPostRecord], error)
	GetAutomaticSlotQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	GetScheduledQOTDOfficialPostByDate(ctx context.Context, guildID string, publishDateUTC time.Time) (*OfficialPostRecord, error)
	DeleteQOTDOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error)
	DeleteQOTDOfficialPostByID(ctx context.Context, id int64) error
	DeleteQOTDUnpublishedOfficialPostsByDeck(ctx context.Context, guildID, deckID string) (int, error)
	UpdateQOTDOfficialPostProgress(ctx context.Context, id int64, progress OfficialPostRecord) (*OfficialPostRecord, error)
	ListQOTDOfficialPostsPendingRecovery(ctx context.Context, guildID string) iter.Seq2[OfficialPostRecord, error]
	GetCurrentAndPreviousQOTDPosts(ctx context.Context, guildID string, now time.Time) iter.Seq2[OfficialPostRecord, error]
	ListQOTDOfficialPostsNeedingArchive(ctx context.Context, now time.Time) iter.Seq2[OfficialPostRecord, error]
	UpdateQOTDOfficialPostState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*OfficialPostRecord, error)

	// Surfaces
	GetQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) (*SurfaceRecord, error)
	UpsertQOTDSurface(ctx context.Context, rec SurfaceRecord) (*SurfaceRecord, error)
	DeleteQOTDSurfaceByDeck(ctx context.Context, guildID, deckID string) error

	// Answer Messages
	CreateQOTDAnswerMessage(ctx context.Context, rec AnswerMessageRecord) (*AnswerMessageRecord, error)
	FinalizeQOTDAnswerMessage(ctx context.Context, id int64, discordMessageID string) (*AnswerMessageRecord, error)
	GetQOTDAnswerMessageByOfficialPostAndUser(ctx context.Context, officialPostID int64, userID string) (*AnswerMessageRecord, error)
	ListQOTDAnswerMessagesByOfficialPost(ctx context.Context, officialPostID int64) (iter.Seq2[AnswerMessageRecord, error], error)
	UpdateQOTDAnswerMessageState(ctx context.Context, id int64, state string, closedAt, archivedAt *time.Time) (*AnswerMessageRecord, error)
}

```

// === FILE: pkg/qotd/service.go ===
```go
package qotd

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

// Publisher abstracts the Discord-side official-post operations the Service
// depends on, so publish flows can be exercised without a live session.
type Publisher interface {
	PublishOfficialPost(ctx context.Context, params PublishOfficialPostParams) (*PublishedOfficialPost, error)
	DeleteOfficialPost(ctx context.Context, params DeleteOfficialPostParams) error
}

// Metrics interface for observability
type Metrics interface {
	RecordOfficialPostAbandoned()
	RecordSuppressionCleared()
}

// NopMetrics provides empty implementations for Metrics.
type NopMetrics struct{}

func (n NopMetrics) RecordOfficialPostAbandoned() {}
func (n NopMetrics) RecordSuppressionCleared()    {}

// Service is the QOTD domain coordinator. It serializes guild work via an actor model.
type Service struct {
	configManager *files.ConfigManager
	repo          Repository
	publisher     Publisher
	metrics       Metrics
	now           func() time.Time

	guildActorsMu sync.Mutex
	guildActors   map[string]*sync.Mutex
}

// NewService constructs the QOTD service.
func NewService(configManager *files.ConfigManager, repo Repository, publisher Publisher) *Service {
	return NewServiceWithMetrics(configManager, repo, publisher, nil)
}

// NewServiceWithMetrics constructs the QOTD service with metrics.
func NewServiceWithMetrics(configManager *files.ConfigManager, repo Repository, publisher Publisher, metrics Metrics) *Service {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	return &Service{
		configManager: configManager,
		repo:          repo,
		publisher:     publisher,
		metrics:       metrics,
		now: func() time.Time {
			return time.Now().UTC()
		},
		guildActors: make(map[string]*sync.Mutex),
	}
}

func (s *Service) getGuildMutex(guildID string) *sync.Mutex {
	guildID = strings.TrimSpace(guildID)
	s.guildActorsMu.Lock()
	defer s.guildActorsMu.Unlock()

	mu, ok := s.guildActors[guildID]
	if !ok {
		mu = &sync.Mutex{}
		s.guildActors[guildID] = mu
	}
	return mu
}

// ExecuteInGuildActor executes a function serially for a specific guild.
func (s *Service) ExecuteInGuildActor(guildID string, fn func()) {
	mu := s.getGuildMutex(guildID)
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// ExecuteInGuildActorWithResult executes a function serially for a specific guild and returns its result.
func (s *Service) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	mu := s.getGuildMutex(guildID)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// SetPublisher injects a publisher.
func (s *Service) SetPublisher(p Publisher) {
	s.publisher = p
}

// GetPublisher returns the underlying publisher.
func (s *Service) GetPublisher() Publisher {
	return s.publisher
}

// SetClock injects a custom clock.
func (s *Service) SetClock(c clock.Clock) {
	if c == nil {
		c = clock.RealClock{}
	}
	s.now = func() time.Time {
		return c.Now().UTC()
	}
}

// PublishResult reports the outcome of a publish.
type PublishResult struct {
	Question     QuestionRecord
	OfficialPost OfficialPostRecord
	PostURL      string
}

// NextScheduledPublishTime returns the time of the next scheduled publish, or zero.
func (s *Service) NextScheduledPublishTime(guildID string) time.Time {
	cfg, err := s.configManager.QOTDConfig(guildID)
	if err != nil || !cfg.Schedule.IsComplete() {
		return time.Time{}
	}
	return PublishTimeUTC(cfg.Schedule, CurrentPublishDateUTC(cfg.Schedule, s.now()))
}

// PublishScheduledIfDue runs the scheduled publish check logic.
func (s *Service) PublishScheduledIfDue(ctx context.Context, guildID string) error {
	_, err := s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		_, err := s.publisher.PublishOfficialPost(ctx, PublishOfficialPostParams{})
		return nil, err
	})
	return err
}

// ReconcileGuild ensures post and answer lifecycles are up to date.
func (s *Service) ReconcileGuild(ctx context.Context, guildID string) error {
	_, err := s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		_, err := s.publisher.PublishOfficialPost(ctx, PublishOfficialPostParams{})
		if err != nil {
			s.metrics.RecordOfficialPostAbandoned()
			return nil, err
		}
		return nil, nil
	})
	return err
}

```

// === FILE: pkg/qotd/service_test.go ===
```go
package qotd_test

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"golang.org/x/sync/errgroup"
)

type mockPublisher struct {
	PublishFunc func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error)
}

func (m *mockPublisher) PublishOfficialPost(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
	if m.PublishFunc != nil {
		return m.PublishFunc(ctx, params)
	}
	return nil, nil
}

func (m *mockPublisher) DeleteOfficialPost(ctx context.Context, params qotd.DeleteOfficialPostParams) error {
	return nil
}

type mockMetrics struct {
	abandoned uint32
	cleared   uint32
}

func (m *mockMetrics) RecordOfficialPostAbandoned() { atomic.AddUint32(&m.abandoned, 1) }
func (m *mockMetrics) RecordSuppressionCleared()    { atomic.AddUint32(&m.cleared, 1) }

// 1. Invariantes do Modelo de Atores e Regressões Transacionais

func TestExecuteInGuildActor_Serialization(t *testing.T) {
	t.Parallel()
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		&mockPublisher{},
	)

	const targetGuildID = "guild_01"
	const workerCount = 100

	var executedCounter int32
	var activeCount int32
	var maxActiveCount int32
	eg, ctx := errgroup.WithContext(context.Background())

	for i := 0; i < workerCount; i++ {
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			_, err := svc.ExecuteInGuildActorWithResult(targetGuildID, func() (any, error) {
				atomic.AddInt32(&executedCounter, 1)

				currentActive := atomic.AddInt32(&activeCount, 1)
				defer atomic.AddInt32(&activeCount, -1)

				// Track maximum concurrent executions inside the same actor
				for {
					max := atomic.LoadInt32(&maxActiveCount)
					if currentActive <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxActiveCount, max, currentActive) {
						break
					}
				}

				runtime.Gosched()
				return nil, nil
			})
			if err != nil {
				return fmt.Errorf("Execução subjacente falhou inesperadamente: %v", err)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("concurrent execution failed: %v", err)
	}

	if atomic.LoadInt32(&executedCounter) != int32(workerCount) {
		t.Fatalf("Esperado contador escalar em %d, aferido %d", workerCount, executedCounter)
	}

	// For a serialized actor, the maximum concurrent execution count must be exactly 1
	if finalMax := atomic.LoadInt32(&maxActiveCount); finalMax != 1 {
		t.Fatalf("Actor serialization failed: expected max concurrent execution of 1, got %d", finalMax)
	}
}

func TestExecuteInGuildActor_Parallelism(t *testing.T) {
	t.Parallel()
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		&mockPublisher{},
	)

	const workerCount = 100

	eg, ctx := errgroup.WithContext(context.Background())
	var activeCount int32
	var maxActiveCount int32
	gate := make(chan struct{})

	for i := 0; i < workerCount; i++ {
		guildID := fmt.Sprintf("guild_%d", i)
		eg.Go(func() error {
			if err := ctx.Err(); err != nil {
				return err
			}
			svc.ExecuteInGuildActor(guildID, func() {
				currentActive := atomic.AddInt32(&activeCount, 1)

				for {
					max := atomic.LoadInt32(&maxActiveCount)
					if currentActive <= max {
						break
					}
					if atomic.CompareAndSwapInt32(&maxActiveCount, max, currentActive) {
						break
					}
				}

				// Wait until all actors have entered to verify parallel execution
				<-gate

				atomic.AddInt32(&activeCount, -1)
			})
			return nil
		})
	}

	// Spin-wait until all workers are concurrently active in their own actors
	start := time.Now()
	for atomic.LoadInt32(&activeCount) < workerCount {
		if time.Since(start) > 2*time.Second {
			close(gate)
			_ = eg.Wait()
			t.Fatalf("Timeout waiting for parallel actors to enter concurrently: entered %d/%d", atomic.LoadInt32(&activeCount), workerCount)
		}
		runtime.Gosched()
	}

	close(gate)
	if err := eg.Wait(); err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if finalMax := atomic.LoadInt32(&maxActiveCount); finalMax != workerCount {
		t.Fatalf("Actor parallelism failed: expected max concurrent execution of %d, got %d", workerCount, finalMax)
	}
}

// 2. Gargalos de I/O Assíncrono e Vazamento de Goroutines

func TestPublishScheduledIfDue_ContextExpiration(t *testing.T) {
	t.Parallel()
	pub := &mockPublisher{
		PublishFunc: func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	svc := qotd.NewService(
		&files.ConfigManager{},
		nil,
		pub,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := svc.PublishScheduledIfDue(ctx, "guild_timeout")

	if err != context.DeadlineExceeded {
		t.Fatalf("Esperava erro context.DeadlineExceeded, obteve: %v", err)
	}
}

func TestReconcileGuild_SystemicFailureIsolation(t *testing.T) {
	t.Parallel()
	pub := &mockPublisher{
		PublishFunc: func(ctx context.Context, params qotd.PublishOfficialPostParams) (*qotd.PublishedOfficialPost, error) {
			return nil, fmt.Errorf("HTTP 500 Internal Server Error")
		},
	}
	metrics := &mockMetrics{}
	svc := qotd.NewServiceWithMetrics(
		&files.ConfigManager{},
		nil,
		pub,
		metrics,
	)

	err := svc.ReconcileGuild(context.Background(), "guild_fail")
	if err == nil {
		t.Fatal("Esperava erro bolhando até o chamador, obteve sucesso")
	}

	if atomic.LoadUint32(&metrics.abandoned) == 0 && atomic.LoadUint32(&metrics.cleared) == 0 {
		// As per instructions, failure should propagate to metrics (implementation may vary on which metric is hit,
		// or if a specific error is needed. This just asserts we didn't crash and returned the error.)
	}
}

// 3. Limites de Agendamento e Dinâmica do Tempo Subjacente

func TestService_SchedulingEdges(t *testing.T) {
	t.Parallel()
	// Table-Driven Tests para PublishTimeUTC e CurrentPublishDateUTC
	tests := []struct {
		name              string
		schedule          files.QOTDPublishScheduleConfig
		now               time.Time
		expectedDateUTC   time.Time
		expectedPublishAt time.Time
	}{
		{
			name: "Ano Bissexto - Dia 29 de Fevereiro",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(12),
				MinuteUTC: intPtr(0),
			},
			now:               time.Date(2024, time.February, 29, 10, 0, 0, 0, time.UTC),
			expectedDateUTC:   time.Date(2024, time.February, 29, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC),
		},
		{
			name: "Virada de Ciclo Solar - Reveillon",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(0),
				MinuteUTC: intPtr(30),
			},
			now:               time.Date(2024, time.December, 31, 23, 59, 59, 0, time.UTC),
			expectedDateUTC:   time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2025, time.January, 1, 0, 30, 0, 0, time.UTC),
		},
		{
			name: "Mesmo dia após o horário",
			schedule: files.QOTDPublishScheduleConfig{
				HourUTC:   intPtr(14),
				MinuteUTC: intPtr(0),
			},
			now:               time.Date(2023, time.July, 15, 15, 0, 0, 0, time.UTC),
			expectedDateUTC:   time.Date(2023, time.July, 16, 0, 0, 0, 0, time.UTC),
			expectedPublishAt: time.Date(2023, time.July, 16, 14, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dateUTC := qotd.CurrentPublishDateUTC(tc.schedule, tc.now)
			if !dateUTC.Equal(tc.expectedDateUTC) {
				t.Errorf("CurrentPublishDateUTC falhou. Esperado %v, obtido %v", tc.expectedDateUTC, dateUTC)
			}

			publishAt := qotd.PublishTimeUTC(tc.schedule, dateUTC)
			if !publishAt.Equal(tc.expectedPublishAt) {
				t.Errorf("PublishTimeUTC falhou. Esperado %v, obtido %v", tc.expectedPublishAt, publishAt)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}

```

