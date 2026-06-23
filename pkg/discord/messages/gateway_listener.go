package messages

import (
	"context"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/messages"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// GatewayListener listens to Arikawa message events and forwards them to the pure messages domain.
type GatewayListener struct {
	state          *state.State
	messageService *messages.MessageEventService
	cancels        []func()
}

// NewGatewayListener creates a new listener.
func NewGatewayListener(s *state.State, msgSvc *messages.MessageEventService) *GatewayListener {
	return &GatewayListener{
		state:          s,
		messageService: msgSvc,
		cancels:        make([]func(), 0, 3),
	}
}

// Start registers the Arikawa event handlers.
func (l *GatewayListener) Start(ctx context.Context) error {
	l.cancels = append(l.cancels,
		l.state.AddHandler(func(e *gateway.MessageCreateEvent) {
			l.messageService.IngestMessageCreate(context.Background(), e)
		}),
		l.state.AddHandler(func(e *gateway.MessageUpdateEvent) {
			l.messageService.IngestMessageUpdate(context.Background(), e)
		}),
		l.state.AddHandler(func(e *gateway.MessageDeleteEvent) {
			l.messageService.IngestMessageDelete(context.Background(), e)
		}),
	)
	return nil
}

// Stop unregisters the handlers.
func (l *GatewayListener) Stop(ctx context.Context) error {
	for _, cancel := range l.cancels {
		if cancel != nil {
			cancel()
		}
	}
	l.cancels = nil
	return nil
}

// Name returns the service name.
func (l *GatewayListener) Name() string { return "discord_messages_listener" }

// Type returns the service type.
func (l *GatewayListener) Type() service.ServiceType { return service.ServiceType("gateway_listener") }

// Priority returns the startup priority.
func (l *GatewayListener) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns a list of dependencies.
func (l *GatewayListener) Dependencies() []string { return []string{"messages"} }

// IsRunning returns whether the service is running.
func (l *GatewayListener) IsRunning() bool { return len(l.cancels) > 0 }

// HealthCheck returns the health status of the service.
func (l *GatewayListener) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK"}
}

// Stats returns runtime statistics.
func (l *GatewayListener) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
