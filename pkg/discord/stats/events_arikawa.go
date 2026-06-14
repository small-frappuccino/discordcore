package stats

import (
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
)

// RegisterEventHandlers registers the necessary gateway event handlers
// to keep the stats service updated using Arikawa.
func RegisterEventHandlers(s *state.State, svc *domain.StatsService) {
	s.AddHandler(func(e *gateway.GuildMemberAddEvent) {
		if e == nil || svc == nil {
			return
		}
		roles := make([]string, len(e.RoleIDs))
		for i, r := range e.RoleIDs {
			roles[i] = r.String()
		}
		// Assuming member.Joined is available in Arikawa
		svc.ApplyMemberAdd(e.GuildID.String(), e.User.ID.String(), e.Joined.Time(), e.User.Bot, roles)
	})

	s.AddHandler(func(e *gateway.GuildMemberRemoveEvent) {
		if e == nil || svc == nil {
			return
		}
		svc.ApplyMemberRemove(e.GuildID.String(), e.User.ID.String())
	})

	s.AddHandler(func(e *gateway.GuildMemberUpdateEvent) {
		if e == nil || svc == nil {
			return
		}
		roles := make([]string, len(e.RoleIDs))
		for i, r := range e.RoleIDs {
			roles[i] = r.String()
		}
		svc.ApplyStatsMemberUpdate(e.GuildID.String(), e.User.ID.String(), e.User.Bot, roles)
	})
}
