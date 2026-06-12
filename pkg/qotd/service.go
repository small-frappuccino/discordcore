package qotd

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// ErrQOTDDisabled defines err qotddisabled.
// ErrAlreadyPublished defines err already published.
// ErrPublishInProgress defines err publish in progress.
// ErrServiceUnavailable defines err service unavailable.
// ErrImmutableQuestion defines err immutable question.
// ErrQuestionNotFound defines err question not found.
// ErrQuestionNotUsed defines err question not used.
// ErrQuestionNotReady defines err question not ready.
// ErrDiscordUnavailable defines err discord unavailable.
// ErrDeckNotFound defines err deck not found.
// ErrNoQuestionsAvailable defines err no questions available.
var (
	ErrServiceUnavailable   = errors.New("qotd service unavailable")
	ErrQOTDDisabled         = errors.New("qotd is disabled")
	ErrAlreadyPublished     = errors.New("qotd already published for the current slot")
	ErrPublishInProgress    = errors.New("qotd publish already in progress for the current slot")
	ErrNoQuestionsAvailable = errors.New("no qotd questions available")
	ErrImmutableQuestion    = errors.New("qotd question is already scheduled or used")
	ErrQuestionNotFound     = errors.New("qotd question not found")
	ErrQuestionNotUsed      = errors.New("qotd question is not used")
	ErrQuestionNotReady     = errors.New("qotd question is not ready")
	ErrDeckNotFound         = errors.New("qotd deck not found")
	ErrDiscordUnavailable   = errors.New("discord session unavailable")
)

// Publisher abstracts the Discord-side official-post operations the Service
// depends on, so publish flows can be exercised without a live session.
type Publisher interface {
	PublishOfficialPost(ctx context.Context, params PublishOfficialPostParams) (*PublishedOfficialPost, error)
	SetThreadState(ctx context.Context, guildID string, threadID string, state ThreadState) error
	DeleteOfficialPost(ctx context.Context, params DeleteOfficialPostParams) error
}

// QuestionMutation carries the fields accepted when creating or updating a
// question; an empty Status leaves the existing status unchanged.
type QuestionMutation struct {
	DeckID string
	Body   string
	Status QuestionStatus
}

// QuestionCounts breaks down a deck's questions by status. Total is the sum of
// the per-status fields.
type QuestionCounts struct {
	Total    int `json:"total"`
	Draft    int `json:"draft"`
	Ready    int `json:"ready"`
	Reserved int `json:"reserved"`
	Used     int `json:"used"`
	Disabled int `json:"disabled"`
}

// Summary is the aggregated QOTD state for a guild: effective settings,
// per-deck question counts, and the current/previous official posts for the
// active publish slot.
type Summary struct {
	Settings                files.QOTDConfig
	Counts                  QuestionCounts
	Decks                   []DeckSummary
	CurrentPublishDateUTC   time.Time
	PublishedForCurrentSlot bool
	CurrentPost             *OfficialPostRecord
	PreviousPost            *OfficialPostRecord
}

// PublishResult reports the outcome of a successful publish: the question that
// was consumed, the resulting official-post record, and the jump URL to it.
type PublishResult struct {
	Question     QuestionRecord
	OfficialPost OfficialPostRecord
	PostURL      string
}

// AutomaticQueueSlotStatus describes where the current scheduled slot stands in
// the automatic publish pipeline. See the AutomaticQueueSlotStatus* constants.
type AutomaticQueueSlotStatus string

// AutomaticQueueSlotStatusWaiting defines automatic queue slot status waiting.
// AutomaticQueueSlotStatusDue defines automatic queue slot status due.
// AutomaticQueueSlotStatusReserved defines automatic queue slot status reserved.
// AutomaticQueueSlotStatusRecovering defines automatic queue slot status recovering.
// AutomaticQueueSlotStatusPublished defines automatic queue slot status published.
// AutomaticQueueSlotStatusDisabled defines automatic queue slot status disabled.
const (
	AutomaticQueueSlotStatusDisabled   AutomaticQueueSlotStatus = "disabled"
	AutomaticQueueSlotStatusWaiting    AutomaticQueueSlotStatus = "waiting"
	AutomaticQueueSlotStatusDue        AutomaticQueueSlotStatus = "due"
	AutomaticQueueSlotStatusReserved   AutomaticQueueSlotStatus = "reserved"
	AutomaticQueueSlotStatusRecovering AutomaticQueueSlotStatus = "recovering"
	AutomaticQueueSlotStatusPublished  AutomaticQueueSlotStatus = "published"
)

// AutomaticQueueState is a snapshot of one deck's automatic-publish slot: the
// resolved schedule, whether publishing is currently possible, and the records
// involved in the slot (existing post, reserved question, next ready question).
type AutomaticQueueState struct {
	Deck               files.QOTDDeckConfig
	Schedule           PublishSchedule
	ScheduleConfigured bool
	CanPublish         bool
	SlotDateUTC        time.Time
	SlotPublishAtUTC   time.Time
	SlotStatus         AutomaticQueueSlotStatus
	SlotOfficialPost   *storage.QOTDOfficialPostRecord
	SlotQuestion       *storage.QOTDQuestionRecord
	NextReadyQuestion  *storage.QOTDQuestionRecord
}

// Service is the QOTD domain coordinator: it owns question mutations, publish
// flows, and lifecycle reconciliation on top of the config manager, storage,
// and a Publisher. Per-guild work is serialized through guild actor goroutines
// (see ExecuteInGuildActor), so handlers must not assume concurrent access to a
// single guild's state.
type Service struct {
	configManager          *files.ConfigManager
	store                  *storage.Store
	publisher              Publisher
	metrics                Metrics
	now                    func() time.Time
	unmanageableThreadLogs sync.Map

	guildActorsMu sync.Mutex
	guildActors   map[string]*sync.Mutex
}

func (s *Service) getGuildMutex(guildID string) *sync.Mutex {
	guildID = strings.TrimSpace(guildID)
	s.guildActorsMu.Lock()
	defer s.guildActorsMu.Unlock()
	if s.guildActors == nil {
		s.guildActors = make(map[string]*sync.Mutex)
	}
	mu, ok := s.guildActors[guildID]
	if !ok {
		mu = &sync.Mutex{}
		s.guildActors[guildID] = mu
	}
	return mu
}

// ExecuteInGuildActor executes in guild actor.
func (s *Service) ExecuteInGuildActor(guildID string, fn func()) {
	mu := s.getGuildMutex(guildID)
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// ExecuteInGuildActorWithResult executes in guild actor with result.
func (s *Service) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	mu := s.getGuildMutex(guildID)
	mu.Lock()
	defer mu.Unlock()
	return fn()
}

// NewService constructs the QOTD service with no metrics wired (defaults
// to NopMetrics). Use NewServiceWithMetrics when an operational metrics
// sink should be threaded through.
func NewService(configManager *files.ConfigManager, store *storage.Store, publisher Publisher) *Service {
	return NewServiceWithMetrics(configManager, store, publisher, nil)
}

// NewServiceWithMetrics is the canonical constructor when the bot is
// running with /v1/health/qotd exposed. Passing a nil metrics value
// falls back to NopMetrics so library tests that don't care about
// observability stay clean.
func NewServiceWithMetrics(configManager *files.ConfigManager, store *storage.Store, publisher Publisher, metrics Metrics) *Service {
	if metrics == nil {
		metrics = NopMetrics{}
	}
	return &Service{
		configManager: configManager,
		store:         store,
		publisher:     publisher,
		metrics:       metrics,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

// SetPublisher injects the Publisher after service construction.
func (s *Service) SetPublisher(p Publisher) {
	s.publisher = p
}

// SetClock injects a custom clock into the service.
func (s *Service) SetClock(c clock.Clock) {
	if c == nil {
		c = clock.RealClock{}
	}
	s.now = func() time.Time {
		return c.Now().UTC()
	}
}
