package members

import (
	"context"
	"sync"

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

	updateQueue chan memberUpdatePayload
	wg          sync.WaitGroup
}

type memberUpdatePayload struct {
	e         *gateway.GuildMemberUpdateEvent
	oldMember *discord.Member
}

// NewGatewayListener creates a new listener.
func NewGatewayListener(s *state.State, memberSvc *members.MemberEventService) *GatewayListener {
	return &GatewayListener{
		state:         s,
		memberService: memberSvc,
		cancels:       make([]func(), 0, 3),
		updateQueue:   make(chan memberUpdatePayload, 1024),
	}
}

// Start registers the Arikawa event handlers.
func (l *GatewayListener) Start(ctx context.Context) error {
	l.cancels = append(l.cancels,
		l.state.AddHandler(func(e *gateway.GuildMemberAddEvent) {
			roles := make([]string, len(e.RoleIDs))
			for i, r := range e.RoleIDs {
				roles[i] = r.String()
			}
			intent := members.MemberJoinIntent{
				GuildID:    e.GuildID.String(),
				UserID:     e.User.ID.String(),
				Username:   e.User.Username,
				Bot:        e.User.Bot,
				AvatarHash: e.User.Avatar,
				RoleIDs:    roles,
				JoinedAt:   e.Joined.Time(),
			}
			l.memberService.IngestGuildMemberAdd(context.Background(), intent)
		}),
		l.state.AddHandler(func(e *gateway.GuildMemberRemoveEvent) {
			intent := members.MemberLeaveIntent{
				GuildID:    e.GuildID.String(),
				UserID:     e.User.ID.String(),
				Username:   e.User.Username,
				Bot:        e.User.Bot,
				AvatarHash: e.User.Avatar,
			}
			l.memberService.IngestGuildMemberRemove(context.Background(), intent)
		}),
		l.state.PreHandler.AddSyncHandler(func(e *gateway.GuildMemberUpdateEvent) {
			oldMember, _ := l.state.Cabinet.Member(e.GuildID, e.User.ID)
			var oldMemberCopy *discord.Member
			if oldMember != nil {
				copied := *oldMember
				oldMemberCopy = &copied
			}
			select {
			case l.updateQueue <- memberUpdatePayload{e: e, oldMember: oldMemberCopy}:
			default:
				// If queue is full, we drop the event to avoid blocking gateway
			}
		}),
	)

	l.wg.Add(1)
	go l.worker()

	return nil
}

func (l *GatewayListener) worker() {
	defer l.wg.Done()
	for payload := range l.updateQueue {
		e := payload.e
		oldMember := payload.oldMember

		roles := make([]string, len(e.RoleIDs))
		for i, r := range e.RoleIDs {
			roles[i] = r.String()
		}

		intent := members.MemberUpdateIntent{
			GuildID:    e.GuildID.String(),
			UserID:     e.User.ID.String(),
			Username:   e.User.Username,
			Bot:        e.User.Bot,
			RoleIDs:    roles,
			AvatarHash: e.User.Avatar,
		}

		if oldMember != nil {
			oldRoles := make([]string, len(oldMember.RoleIDs))
			for i, r := range oldMember.RoleIDs {
				oldRoles[i] = r.String()
			}
			intent.OldRoleIDs = oldRoles
			intent.OldAvatar = oldMember.User.Avatar
		}

		l.memberService.IngestGuildMemberUpdate(context.Background(), intent)
	}
}

// Stop unregisters the handlers.
func (l *GatewayListener) Stop(ctx context.Context) error {
	for _, cancel := range l.cancels {
		if cancel != nil {
			cancel()
		}
	}
	l.cancels = nil

	if l.updateQueue != nil {
		close(l.updateQueue)
		l.wg.Wait()
	}

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
func (l *GatewayListener) IsRunning() bool { return len(l.cancels) > 0 }

// HealthCheck returns the health status of the service.
func (l *GatewayListener) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK"}
}

// Stats returns runtime statistics.
func (l *GatewayListener) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
