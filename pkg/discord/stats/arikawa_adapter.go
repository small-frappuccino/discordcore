package stats

import (
	"context"
	"fmt"
	"iter"
	"log/slog"

	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	domain "github.com/small-frappuccino/discordcore/pkg/stats"
)

// ArikawaGateway implements the domain.Gateway interface using Arikawa.
type ArikawaGateway struct {
	state  *state.State
	logger *slog.Logger
}

// NewArikawaGateway creates a new ArikawaGateway.
func NewArikawaGateway(s *state.State, logger *slog.Logger) *ArikawaGateway {
	return &ArikawaGateway{
		state:  s,
		logger: logger,
	}
}

// UpdateChannelName implements domain.Gateway.
func (g *ArikawaGateway) UpdateChannelName(ctx context.Context, channelID, newName string) error {
	id, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return fmt.Errorf("invalid channel ID %q: %w", channelID, err)
	}

	data := api.ModifyChannelData{
		Name: newName,
	}

	c := g.state.Client.WithContext(ctx)
	if err := c.ModifyChannel(discord.ChannelID(id), data); err != nil {
		return fmt.Errorf("arikawa modify channel: %w", err)
	}
	return nil
}

// GetChannel implements domain.Gateway.
func (g *ArikawaGateway) GetChannel(ctx context.Context, channelID string) (*domain.Channel, error) {
	id, err := discord.ParseSnowflake(channelID)
	if err != nil {
		return nil, fmt.Errorf("invalid channel ID %q: %w", channelID, err)
	}

	ch, err := g.state.Channel(discord.ChannelID(id))
	if err != nil {
		return nil, fmt.Errorf("arikawa get channel: %w", err)
	}

	return &domain.Channel{
		ID:      ch.ID.String(),
		Name:    ch.Name,
		GuildID: ch.GuildID.String(),
	}, nil
}

// StreamGuildMembers implements domain.Gateway.
func (g *ArikawaGateway) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[domain.MemberSnapshot, error] {
	return func(yield func(domain.MemberSnapshot, error) bool) {
		id, err := discord.ParseSnowflake(guildID)
		if err != nil {
			yield(domain.MemberSnapshot{}, fmt.Errorf("invalid guild ID %q: %w", guildID, err))
			return
		}

		c := g.state.Client.WithContext(ctx)
		limit := uint(1000)
		var after discord.UserID

		for {
			if ctx.Err() != nil {
				yield(domain.MemberSnapshot{}, ctx.Err())
				return
			}

			members, err := c.MembersAfter(discord.GuildID(id), after, limit)
			if err != nil {
				yield(domain.MemberSnapshot{}, fmt.Errorf("arikawa fetch members: %w", err))
				return
			}

			// Retorno antecipado absoluto: esgotamento da paginação.
			if len(members) == 0 {
				return
			}

			for _, m := range members {
				// Isolamento da construção do iterador aninhado.
				roleIter := func(roleYield func(string) bool) {
					for _, r := range m.RoleIDs {
						if !roleYield(r.String()) {
							return
						}
					}
				}

				snap := domain.MemberSnapshot{
					UserID: m.User.ID.String(),
					IsBot:  m.User.Bot,
					Roles:  roleIter,
				}

				if !yield(snap, nil) {
					return
				}
			}

			if len(members) < int(limit) {
				return
			}
			after = members[len(members)-1].User.ID
		}
	}
}
