# Domain Architecture: qotd

## Layout Topology
```text
qotd/
├── doc.go
├── errors.go
├── lifecycle.go
├── models.go
├── repository.go
└── service.go
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

// ServiceOption configures a Service.
type ServiceOption func(*Service)

// WithRepository sets the repository for the service.
func WithRepository(repo Repository) ServiceOption {
	return func(s *Service) {
		s.repo = repo
	}
}

// WithPublisher sets the publisher for the service.
func WithPublisher(publisher Publisher) ServiceOption {
	return func(s *Service) {
		s.publisher = publisher
	}
}

// WithMetrics sets the metrics for the service.
func WithMetrics(metrics Metrics) ServiceOption {
	return func(s *Service) {
		if metrics != nil {
			s.metrics = metrics
		}
	}
}

// NewService constructs the QOTD service.
func NewService(configManager *files.ConfigManager, opts ...ServiceOption) *Service {
	s := &Service{
		configManager: configManager,
		metrics:       NopMetrics{},
		now: func() time.Time {
			return time.Now().UTC()
		},
		guildActors: make(map[string]*sync.Mutex),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
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

