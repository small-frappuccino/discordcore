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
	ctx            context.Context

	cancelCreate func()
	cancelUpdate func()
	cancelDelete func()
}

// NewGatewayListener creates a new listener.
func NewGatewayListener(s *state.State, msgSvc *messages.MessageEventService) *GatewayListener {
	return &GatewayListener{
		state:          s,
		messageService: msgSvc,
		ctx:            context.Background(),
	}
}

// Start registers the Arikawa event handlers.
func (l *GatewayListener) Start(ctx context.Context) error {
	l.cancelCreate = l.state.AddHandler(l.handleMessageCreate)
	l.cancelUpdate = l.state.AddHandler(l.handleMessageUpdate)
	l.cancelDelete = l.state.AddHandler(l.handleMessageDelete)
	return nil
}

func (l *GatewayListener) handleMessageCreate(e *gateway.MessageCreateEvent) {
	if !e.ID.IsValid() || !e.GuildID.IsValid() || !e.ChannelID.IsValid() || !e.Author.ID.IsValid() {
		return
	}
	intent := messages.MessageCreateIntent{
		MessageID:      e.ID.String(),
		GuildID:        e.GuildID.String(),
		ChannelID:      e.ChannelID.String(),
		AuthorID:       e.Author.ID.String(),
		AuthorUsername: e.Author.Username,
		AuthorBot:      e.Author.Bot,
		Content:        e.Content,
		Timestamp:      e.Timestamp.Time(),
	}
	l.messageService.IngestMessageCreate(l.ctx, intent)
}

func (l *GatewayListener) handleMessageUpdate(e *gateway.MessageUpdateEvent) {
	if !e.ID.IsValid() || !e.GuildID.IsValid() || !e.ChannelID.IsValid() {
		return
	}
	intent := messages.MessageUpdateIntent{
		MessageID: e.ID.String(),
		GuildID:   e.GuildID.String(),
		ChannelID: e.ChannelID.String(),
		Content:   e.Content,
	}
	l.messageService.IngestMessageUpdate(l.ctx, intent)
}

func (l *GatewayListener) handleMessageDelete(e *gateway.MessageDeleteEvent) {
	if !e.ID.IsValid() || !e.GuildID.IsValid() || !e.ChannelID.IsValid() {
		return
	}
	intent := messages.MessageDeleteIntent{
		MessageID: e.ID.String(),
		GuildID:   e.GuildID.String(),
		ChannelID: e.ChannelID.String(),
	}
	l.messageService.IngestMessageDelete(l.ctx, intent)
}

// Stop unregisters the handlers.
func (l *GatewayListener) Stop(ctx context.Context) error {
	if l.cancelCreate != nil {
		l.cancelCreate()
	}
	if l.cancelUpdate != nil {
		l.cancelUpdate()
	}
	if l.cancelDelete != nil {
		l.cancelDelete()
	}
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
func (l *GatewayListener) IsRunning() bool { return l.cancelCreate != nil }

// HealthCheck returns the health status of the service.
func (l *GatewayListener) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK"}
}

// Stats returns runtime statistics.
func (l *GatewayListener) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
