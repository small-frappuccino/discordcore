package qotd

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

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

type Publisher interface {
	PublishOfficialPost(ctx context.Context, session *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error)
	SetThreadState(ctx context.Context, session *discordgo.Session, threadID string, state discordqotd.ThreadState) error
}

type QuestionMutation struct {
	DeckID string
	Body   string
	Status QuestionStatus
}

type QuestionCounts struct {
	Total    int `json:"total"`
	Draft    int `json:"draft"`
	Ready    int `json:"ready"`
	Reserved int `json:"reserved"`
	Used     int `json:"used"`
	Disabled int `json:"disabled"`
}

type Summary struct {
	Settings                files.QOTDConfig
	Counts                  QuestionCounts
	Decks                   []DeckSummary
	CurrentPublishDateUTC   time.Time
	PublishedForCurrentSlot bool
	CurrentPost             *OfficialPostRecord
	PreviousPost            *OfficialPostRecord
}

type PublishResult struct {
	Question     QuestionRecord
	OfficialPost OfficialPostRecord
	PostURL      string
}

type AutomaticQueueSlotStatus string

const (
	AutomaticQueueSlotStatusDisabled   AutomaticQueueSlotStatus = "disabled"
	AutomaticQueueSlotStatusWaiting    AutomaticQueueSlotStatus = "waiting"
	AutomaticQueueSlotStatusDue        AutomaticQueueSlotStatus = "due"
	AutomaticQueueSlotStatusReserved   AutomaticQueueSlotStatus = "reserved"
	AutomaticQueueSlotStatusRecovering AutomaticQueueSlotStatus = "recovering"
	AutomaticQueueSlotStatusPublished  AutomaticQueueSlotStatus = "published"
)

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

type Service struct {
	configManager       *files.ConfigManager
	store               *storage.Store
	publisher           Publisher
	metrics             Metrics
	now                 func() time.Time
	guildLifecycleLocks sync.Map
	// unmanageableThreadLogs deduplicates the WARN log emitted when Discord
	// rejects a thread state edit with 403/Missing Access despite the bot
	// otherwise being able to publish. Without dedup the reconcile loop floods
	// the log every minute for as long as the post stays in current/previous.
	// Keyed by "guildID|threadID|targetState".
	unmanageableThreadLogs sync.Map
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
	if publisher == nil {
		publisher = discordqotd.NewPublisher()
	}
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
