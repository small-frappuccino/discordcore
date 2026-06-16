package automod

import (
	"context"

	"github.com/diamondburned/arikawa/v3/discord"
)

// Sink receives validated automod events.
type Sink interface {
	// OnAutomodBlock is called when Discord executes an AutoMod action.
	OnAutomodBlock(ctx context.Context, guildID discord.GuildID, entry *ExecutionEvent)
}

// NopSink is a no-op implementation of Sink.
type NopSink struct{}

func (NopSink) OnAutomodBlock(ctx context.Context, guildID discord.GuildID, entry *ExecutionEvent) {
}
