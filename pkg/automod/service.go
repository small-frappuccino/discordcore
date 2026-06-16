package automod

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordgo"
)

// Service listens for Discord native AutoMod executions via discordgo
// (since Arikawa v3 lacks AutoModerationActionExecutionEvent) and routes them to a Sink.
type Service struct {
	session       *discordgo.Session
	sink          Sink
	isRunning     bool
	handlerCancel func()

	mu        sync.Mutex
	startTime time.Time

	logger *slog.Logger
}

// NewService initializes a new AutoMod logging service.
func NewService(session *discordgo.Session, sink Sink, logger *slog.Logger) *Service {
	if sink == nil {
		sink = NopSink{}
	}
	return &Service{
		session: session,
		sink:    sink,
		logger:  logger,
	}
}

// Name implements the service.Service interface.
func (s *Service) Name() string { return "automod" }

// Type implements the service.Service interface.
func (s *Service) Type() service.ServiceType { return service.TypeAutomod }

// Priority implements the service.Service interface.
func (s *Service) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies implements the service.Service interface.
func (s *Service) Dependencies() []string { return nil }

// IsRunning safely reports the current execution state of the service.
func (s *Service) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// HealthCheck reports the operational status of the service.
func (s *Service) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "Automod Service is active",
		LastCheck: time.Now(),
	}
}

// Stats provides runtime telemetry for the AutoMod service.
func (s *Service) Stats() service.ServiceStats {
	s.mu.Lock()
	defer s.mu.Unlock()

	var uptime time.Duration
	if s.isRunning {
		uptime = time.Since(s.startTime)
	}

	return service.ServiceStats{
		StartTime: s.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start binds the discordgo event handlers.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return nil
	}
	s.isRunning = true
	s.startTime = time.Now()

	if s.session != nil {
		s.handlerCancel = s.session.AddHandler(s.handleAutoModerationActionExecution)
	}
	return nil
}

// Stop deregisters gateway handlers.
func (s *Service) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}
	if s.handlerCancel != nil {
		s.handlerCancel()
		s.handlerCancel = nil
	}
	s.isRunning = false
	return nil
}

// handleAutoModerationActionExecution filters for AutoMod execution audit logs and emits them.
func (s *Service) handleAutoModerationActionExecution(sess *discordgo.Session, e *discordgo.AutoModerationActionExecution) {
	if e == nil {
		return
	}

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", e.GuildID),
		slog.String("ruleID", e.RuleID),
		slog.String("userID", e.UserID),
	)
	defer done()

	// Parse GuildID for Arikawa compatibility in the Sink
	parsedGuildID, err := discord.ParseSnowflake(e.GuildID)
	if err != nil {
		return
	}

	// Pure emission to the Sink
	s.sink.OnAutomodBlock(context.Background(), discord.GuildID(parsedGuildID), e)
}
