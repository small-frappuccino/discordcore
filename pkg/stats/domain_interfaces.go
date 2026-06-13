package stats

import (
	"context"
	"iter"
	"time"
)

type GuildMember struct {
	UserID   string
	IsBot    bool
	Roles    []string
	JoinedAt time.Time
}

type Channel struct {
	ID      string
	GuildID string
	Name    string
}

type Publisher interface {
	ChannelEditName(ctx context.Context, channelID, newName string) error
	Channel(ctx context.Context, channelID string) (Channel, error)
	StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[GuildMember, error]
}
