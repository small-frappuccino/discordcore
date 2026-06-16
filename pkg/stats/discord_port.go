package stats

import (
	"context"
	"iter"
)

// Gateway defines the contract for Discord API and Gateway interactions
// required by the stats domain.
type Gateway interface {
	// UpdateChannelName updates the name of a voice channel used for stats.
	UpdateChannelName(ctx context.Context, channelID, newName string) error

	// GetChannel retrieves information about a specific channel.
	GetChannel(ctx context.Context, channelID string) (*Channel, error)

	// StreamGuildMembers returns an iterator over all members in a guild.
	StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[MemberSnapshot, error]
}

// Channel represents a Discord channel's basic metadata needed for stats.
type Channel struct {
	ID      string
	Name    string
	GuildID string
}

// MemberSnapshot represents the state of a member needed for stats reconciliation.
type MemberSnapshot struct {
	UserID string
	IsBot  bool
	Roles  iter.Seq[string]
}
