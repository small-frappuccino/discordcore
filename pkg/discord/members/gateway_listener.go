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
	ctx           context.Context

	cancelMemberAdd    func()
	cancelMemberRemove func()
	cancelMemberUpdate func()

	updateQueue chan memberUpdatePayload
	wg          sync.WaitGroup
}

type memberUpdatePayload struct {
	e            *gateway.GuildMemberUpdateEvent
	oldMember    discord.Member
	hasOldMember bool
}

// NewGatewayListener creates a new listener.
func NewGatewayListener(s *state.State, memberSvc *members.MemberEventService) *GatewayListener {
	return &GatewayListener{
		state:         s,
		memberService: memberSvc,
		ctx:           context.Background(),
		updateQueue:   make(chan memberUpdatePayload, 1024),
	}
}

// Start registers the Arikawa event handlers.
func (l *GatewayListener) Start(ctx context.Context) error {
	l.cancelMemberAdd = l.state.AddHandler(l.handleMemberAdd)
	l.cancelMemberRemove = l.state.AddHandler(l.handleMemberRemove)
	l.cancelMemberUpdate = l.state.PreHandler.AddSyncHandler(l.handleMemberUpdate)

	l.wg.Add(1)
	go l.worker()

	return nil
}

func (l *GatewayListener) handleMemberAdd(e *gateway.GuildMemberAddEvent) {
	if !e.GuildID.IsValid() || !e.User.ID.IsValid() {
		return
	}
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
	l.memberService.IngestGuildMemberAdd(l.ctx, intent)
}

func (l *GatewayListener) handleMemberRemove(e *gateway.GuildMemberRemoveEvent) {
	if !e.GuildID.IsValid() || !e.User.ID.IsValid() {
		return
	}
	intent := members.MemberLeaveIntent{
		GuildID:    e.GuildID.String(),
		UserID:     e.User.ID.String(),
		Username:   e.User.Username,
		Bot:        e.User.Bot,
		AvatarHash: e.User.Avatar,
	}
	l.memberService.IngestGuildMemberRemove(l.ctx, intent)
}

func (l *GatewayListener) handleMemberUpdate(e *gateway.GuildMemberUpdateEvent) {
	if !e.GuildID.IsValid() || !e.User.ID.IsValid() {
		return
	}
	oldMember, _ := l.state.Cabinet.Member(e.GuildID, e.User.ID)
	payload := memberUpdatePayload{e: e}
	if oldMember != nil {
		payload.oldMember = *oldMember
		payload.hasOldMember = true
	}
	select {
	case l.updateQueue <- payload:
	default:
		// If queue is full, we drop the event to avoid blocking gateway
	}
}

func (l *GatewayListener) worker() {
	defer l.wg.Done()
	for payload := range l.updateQueue {
		e := payload.e

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

		if payload.hasOldMember {
			oldMember := &payload.oldMember
			oldRoles := make([]string, len(oldMember.RoleIDs))
			for i, r := range oldMember.RoleIDs {
				oldRoles[i] = r.String()
			}
			intent.OldRoleIDs = oldRoles
			intent.OldAvatar = oldMember.User.Avatar
		}

		l.memberService.IngestGuildMemberUpdate(l.ctx, intent)
	}
}

// Stop unregisters the handlers.
func (l *GatewayListener) Stop(ctx context.Context) error {
	if l.cancelMemberAdd != nil {
		l.cancelMemberAdd()
	}
	if l.cancelMemberRemove != nil {
		l.cancelMemberRemove()
	}
	if l.cancelMemberUpdate != nil {
		l.cancelMemberUpdate()
	}

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
func (l *GatewayListener) IsRunning() bool { return l.cancelMemberAdd != nil }

// HealthCheck returns the health status of the service.
func (l *GatewayListener) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{Healthy: true, Message: "OK"}
}

// Stats returns runtime statistics.
func (l *GatewayListener) Stats() service.ServiceStats {
	return service.ServiceStats{}
}
