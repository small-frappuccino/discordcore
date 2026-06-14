package stats

import (
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

// RegisterDiscordGoEventHandlers registers the necessary gateway event handlers
// to keep the stats service updated using DiscordGo.
// This is used to maintain rock-solid stability during the atomic migration,
// reusing the existing websocket connection for events while the business logic
// is fully decoupled.
func RegisterDiscordGoEventHandlers(session *discordgo.Session, svc *domain.StatsService) {
	session.AddHandler(func(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
		if m == nil || m.Member == nil || m.Member.User == nil || svc == nil {
			return
		}
		svc.ApplyMemberAdd(m.GuildID, m.User.ID, m.JoinedAt, m.User.Bot, m.Roles)
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
		svc.ApplyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, m.Roles)
	})
}
