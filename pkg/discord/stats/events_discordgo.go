package stats

import (
	"log/slog"

	domain "github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// RegisterDiscordGoEventHandlers registers the necessary gateway event handlers
// to keep the stats service updated using DiscordGo.
// This is used to maintain rock-solid stability during the atomic migration,
// reusing the existing websocket connection for events while the business logic
// is fully decoupled.
func RegisterDiscordGoEventHandlers(session *discordgo.Session, svc *domain.StatsService, logger *slog.Logger) {
	if logger != nil {
		logger.Info("Registered DiscordGo event handlers for stats")
	}
	session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
		if m == nil || m.Member == nil || m.Member.User == nil || svc == nil {
			return
		}
		svc.ApplyMemberAdd(m.GuildID, m.User.ID, m.JoinedAt, m.User.Bot, func(yield func(string) bool) {
			for _, r := range m.Roles {
				if !yield(r) {
					return
				}
			}
		})
	})

	session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
		if m == nil || m.User == nil || svc == nil {
			return
		}
		svc.ApplyMemberRemove(m.GuildID, m.User.ID)
	})

	session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberUpdate) {
		if m == nil || m.User == nil || svc == nil {
			return
		}
		svc.ApplyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, func(yield func(string) bool) {
			for _, r := range m.Roles {
				if !yield(r) {
					return
				}
			}
		})
	})
}
