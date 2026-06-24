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
