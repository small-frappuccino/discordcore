package messages

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
)

// MessageSink receives validated message domain events.
// This interface allows the domain module to be decoupled from logging/notification implementations.
type MessageSink interface {
	OnMessageDelete(ctx context.Context, m *gateway.MessageDeleteEvent, cachedMessage *discord.Message, executor *discord.User)
	OnMessageUpdate(ctx context.Context, m *gateway.MessageUpdateEvent, cachedMessage *discord.Message)
	OnMessageDeleteBulk(ctx context.Context, guildID discord.GuildID, channelID discord.ChannelID, messages []string)
}
