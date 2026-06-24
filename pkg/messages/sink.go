package messages

import (
	"context"
)

// MessageSink receives validated message domain events.
// This interface allows the domain module to be decoupled from logging/notification implementations.
type MessageSink interface {
	OnMessageDelete(ctx context.Context, intent MessageDeleteIntent, cachedMessage *CachedMessageData)
	OnMessageUpdate(ctx context.Context, intent MessageUpdateIntent, cachedMessage *CachedMessageData)
	OnMessageDeleteBulk(ctx context.Context, intent MessageDeleteBulkIntent)
}
