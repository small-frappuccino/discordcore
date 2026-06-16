package stats

import (
	"log/slog"

	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
)

// RegisterEventHandlers registers the necessary gateway event handlers
// to keep the stats service updated using Arikawa.
func RegisterEventHandlers(s *state.State, svc *domain.StatsService, logger *slog.Logger) {
	if logger != nil {
		logger.Info("Registered Arikawa event handlers for stats")
	}
	s.AddHandler(func(e *gateway.GuildMemberAddEvent) {
		if e == nil || svc == nil {
			return
		}
		// Assuming member.Joined is available in Arikawa
		svc.ApplyMemberAdd(e.GuildID.String(), e.User.ID.String(), e.Joined.Time(), e.User.Bot, func(yield func(string) bool) {
			for _, r := range e.RoleIDs {
				if !yield(r.String()) {
					return
				}
			}
		})
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
		svc.ApplyStatsMemberUpdate(e.GuildID.String(), e.User.ID.String(), e.User.Bot, func(yield func(string) bool) {
			for _, r := range e.RoleIDs {
				if !yield(r.String()) {
					return
				}
			}
		})
	})
}
