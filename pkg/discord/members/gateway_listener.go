package members

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/service"
)

// GatewayListener listens to Arikawa member events and forwards them to the pure members domain.
type GatewayListener struct {
	state         *state.State
	memberService *members.MemberEventService
	cancels       []func()
}

// NewGatewayListener creates a new listener.
func NewGatewayListener(s *state.State, memberSvc *members.MemberEventService) *GatewayListener {
	return &GatewayListener{
		state:         s,
		memberService: memberSvc,
		cancels:       make([]func(), 0, 3),
	}
}

// Start registers the Arikawa event handlers.
func (l *GatewayListener) Start(ctx context.Context) error {
	l.cancels = append(l.cancels,
		l.state.AddHandler(func(e *gateway.GuildMemberAddEvent) {
			l.memberService.IngestGuildMemberAdd(context.Background(), e)
		}),
		l.state.AddHandler(func(e *gateway.GuildMemberRemoveEvent) {
			l.memberService.IngestGuildMemberRemove(context.Background(), e)
		}),
		l.state.PreHandler.AddSyncHandler(func(e *gateway.GuildMemberUpdateEvent) {
			oldMember, _ := l.state.Cabinet.Member(e.GuildID, e.User.ID)
			var oldMemberCopy *discord.Member
			if oldMember != nil {
				copied := *oldMember
				oldMemberCopy = &copied
			}
			go l.memberService.IngestGuildMemberUpdate(context.Background(), e, oldMemberCopy)
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
func (l *GatewayListener) Name() string { return "discord_members_listener" }

// Type returns the service type.
func (l *GatewayListener) Type() service.ServiceType { return service.ServiceType("gateway_listener") }

// Priority returns the startup priority.
func (l *GatewayListener) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns a list of dependencies.
func (l *GatewayListener) Dependencies() []string { return []string{"members"} }

// IsRunning returns whether the service is running.
func (l *GatewayListener) IsRunning() bool { return l.cancels != nil }

// HealthCheck returns the health status of the service.
func (l *GatewayListener) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK"}
}

// Stats returns runtime statistics.
func (l *GatewayListener) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
