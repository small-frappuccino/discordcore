package qotd

import (
	"context"
	"errors"
	"strings"
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
	configManager          *files.ConfigManager
	store                  *storage.Store
	publisher              Publisher
	metrics                Metrics
	now                    func() time.Time
	unmanageableThreadLogs sync.Map

	guildActorsMu sync.Mutex
	guildActors   map[string]chan func()
}

func (s *Service) ExecuteInGuildActor(guildID string, fn func()) {
	s.ExecuteInGuildActorWithResult(guildID, func() (any, error) {
		fn()
		return nil, nil
	})
}

func (s *Service) ExecuteInGuildActorWithResult(guildID string, fn func() (any, error)) (any, error) {
	guildID = strings.TrimSpace(guildID)
	type result struct {
		val any
		err error
	}
	done := make(chan result, 1)

	wrapper := func() {
		val, err := fn()
		done <- result{val, err}
		close(done)
	}

	s.guildActorsMu.Lock()
	if s.guildActors == nil {
		s.guildActors = make(map[string]chan func())
	}
	ch, ok := s.guildActors[guildID]
	if !ok {
		ch = make(chan func(), 10)
		s.guildActors[guildID] = ch
		go s.runGuildActor(guildID, ch)
	}
	ch <- wrapper
	s.guildActorsMu.Unlock()

	res := <-done
	return res.val, res.err
}

func (s *Service) runGuildActor(guildID string, ch chan func()) {
	idleTimer := time.NewTimer(5 * time.Minute)
	defer idleTimer.Stop()

	for {
		select {
		case fn := <-ch:
			idleTimer.Stop()
			fn()
			idleTimer.Reset(5 * time.Minute)
		case <-idleTimer.C:
			s.guildActorsMu.Lock()
			if len(ch) > 0 {
				s.guildActorsMu.Unlock()
				idleTimer.Reset(5 * time.Minute)
				continue
			}
			delete(s.guildActors, guildID)
			s.guildActorsMu.Unlock()
			return
		}
	}
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
