package discordstats

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/small-frappuccino/discordcore/pkg/stats"
	"github.com/small-frappuccino/discordgo"
)

type DiscordgoPublisher struct {
	session *discordgo.Session
}

func NewDiscordgoPublisher(session *discordgo.Session) *DiscordgoPublisher {
	return &DiscordgoPublisher{
		session: session,
	}
}

func (p *DiscordgoPublisher) ChannelEditName(ctx context.Context, channelID, newName string) error {
	_, err := p.session.ChannelEdit(channelID, &discordgo.ChannelEdit{Name: newName})
	return err
}

func (p *DiscordgoPublisher) Channel(ctx context.Context, channelID string) (stats.Channel, error) {
	if p.session.State != nil {
		if ch, err := p.session.State.Channel(channelID); err == nil && ch != nil {
			return stats.Channel{
				ID:      ch.ID,
				GuildID: ch.GuildID,
				Name:    ch.Name,
			}, nil
		}
	}
	ch, err := p.session.Channel(channelID)
	if err != nil {
		return stats.Channel{}, err
	}
	return stats.Channel{
		ID:      ch.ID,
		GuildID: ch.GuildID,
		Name:    ch.Name,
	}, nil
}

func (p *DiscordgoPublisher) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[stats.GuildMember, error] {
	return func(yield func(stats.GuildMember, error) bool) {
		pageSize := 1000
		after := ""

		for {
			if err := ctx.Err(); err != nil {
				yield(stats.GuildMember{}, err)
				return
			}
			members, err := p.session.GuildMembers(guildID, after, pageSize)
			if err != nil {
				yield(stats.GuildMember{}, err)
				return
			}
			if len(members) == 0 {
				return
			}

			for _, m := range members {
				if m == nil || m.User == nil {
					continue
				}
				userID := strings.TrimSpace(m.User.ID)
				if userID == "" {
					continue
				}
				sm := stats.GuildMember{
					UserID:   userID,
					IsBot:    m.User.Bot,
					Roles:    m.Roles,
					JoinedAt: m.JoinedAt,
				}
				if !yield(sm, nil) {
					return
				}
			}

			if len(members) < pageSize {
				return
			}
			last := members[len(members)-1]
			if last == nil || last.User == nil || strings.TrimSpace(last.User.ID) == "" {
				yield(stats.GuildMember{}, fmt.Errorf("stream guild members: invalid page tail for guild %s", guildID))
				return
			}
			after = last.User.ID
		}
	}
}

func RegisterEventHandlers(session *discordgo.Session, statsService *stats.StatsService) {
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberAdd) {
		if m == nil || m.Member == nil || m.Member.User == nil {
			return
		}
		statsService.ApplyStatsMemberAdd(m.GuildID, m.Member.User.ID, m.Member.User.Bot, m.Member.Roles, m.Member.JoinedAt)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberRemove) {
		if m == nil || m.User == nil {
			return
		}
		statsService.ApplyStatsMemberRemove(m.GuildID, m.User.ID)
	})
	session.AddHandler(func(_ *discordgo.Session, m *discordgo.GuildMemberUpdate) {
		if m == nil || m.User == nil {
			return
		}
		statsService.ApplyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, m.Roles)
	})
}
