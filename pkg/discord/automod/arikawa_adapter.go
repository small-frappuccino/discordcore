package automod

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/diamondburned/arikawa/v3/state"
	"github.com/diamondburned/arikawa/v3/utils/ws"
	"github.com/small-frappuccino/discordcore/pkg/automod"
	"github.com/small-frappuccino/discordcore/pkg/discord/perf"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// ArikawaAdapter listens for Discord native AutoMod executions directly
// via Arikawa's low-level WebSocket Operations, bypassing missing types
// in the SDK, and routing them to the automod.Sink.
type ArikawaAdapter struct {
	state         *state.State
	sink          automod.Sink
	isRunning     bool
	handlerCancel func()

	mu        sync.Mutex
	startTime time.Time

	logger *slog.Logger
}

// NewArikawaAdapter initializes a new raw WebSocket adapter for AutoMod.
func NewArikawaAdapter(state *state.State, sink automod.Sink, logger *slog.Logger) *ArikawaAdapter {
	if sink == nil {
		sink = automod.NopSink{}
	}
	return &ArikawaAdapter{
		state:  state,
		sink:   sink,
		logger: logger,
	}
}

// Name implements the service.Service interface.
func (a *ArikawaAdapter) Name() string { return "discord_automod_adapter" }

// Type implements the service.Service interface.
func (a *ArikawaAdapter) Type() service.ServiceType { return service.TypeAutomod }

// Priority implements the service.Service interface.
func (a *ArikawaAdapter) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies implements the service.Service interface.
func (a *ArikawaAdapter) Dependencies() []string { return nil }

// IsRunning safely reports the current execution state of the service.
func (a *ArikawaAdapter) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.isRunning
}

// HealthCheck reports the operational status of the service.
func (a *ArikawaAdapter) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "Arikawa Automod Adapter is active",
		LastCheck: time.Now(),
	}
}

// Stats provides runtime telemetry for the adapter.
func (a *ArikawaAdapter) Stats() service.ServiceStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	var uptime time.Duration
	if a.isRunning {
		uptime = time.Since(a.startTime)
	}

	return service.ServiceStats{
		StartTime: a.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start binds the raw WebSocket handler to Arikawa's Session.
func (a *ArikawaAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.isRunning {
		return nil
	}
	a.isRunning = true
	a.startTime = time.Now()

	if a.state != nil {
		// Use Session.AddHandler to intercept raw generic payloads (like *ws.Op)
		// Or AddHandler accepts raw *ws.Op directly?
		// Note: Arikawa's Handler does not invoke for *ws.Op natively if we use standard typed AddHandler.
		// However, we can use a generic handler `func(interface{})` to catch everything, OR `func(*ws.Op)`
		// if we attach to `state.Session`. Wait, `Session.AddHandler(func(v interface{}))` catches ALL parsed events.
		// But unknown events are discarded before the generic handler.
		// Actually, `PreHandler` or `Session` might not emit unknown.
		// Let's hook into the low-level `Session.AddHandler` with `func(*ws.Op)`.
		// Arikawa's `EventCreator` runs BEFORE `Session` handlers?
		// No, `state.Session` is `gateway.Session`.
		a.handlerCancel = a.state.AddHandler(a.handleRawOp)
	}
	return nil
}

// Stop deregisters gateway handlers.
func (a *ArikawaAdapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.isRunning {
		return nil
	}
	if a.handlerCancel != nil {
		a.handlerCancel()
		a.handlerCancel = nil
	}
	a.isRunning = false
	return nil
}

// handleRawOp intercepts WebSocket operations.
func (a *ArikawaAdapter) handleRawOp(op *ws.Op) {
	if op == nil || op.Type != "AUTO_MODERATION_ACTION_EXECUTION" {
		return
	}

	b, err := json.Marshal(op.Data)
	if err != nil {
		a.logger.Error("Failed to re-marshal AUTO_MODERATION_ACTION_EXECUTION payload", "error", err)
		return
	}

	var e automod.ExecutionEvent
	if err := json.Unmarshal(b, &e); err != nil {
		a.logger.Error("Failed to unmarshal AUTO_MODERATION_ACTION_EXECUTION", "error", err)
		return
	}

	done := perf.StartGatewayEvent(
		"auto_moderation_action_execution",
		slog.String("guildID", e.GuildID.String()),
		slog.String("ruleID", e.RuleID.String()),
		slog.String("userID", e.UserID.String()),
	)
	defer done()

	// Pure emission to the Sink
	a.sink.OnAutomodBlock(context.Background(), e.GuildID, &e)
}
