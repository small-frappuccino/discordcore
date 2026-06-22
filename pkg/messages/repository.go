package messages

import (
	"context"
)

type Repository interface {
	UpsertMessage(m Record) error
	UpsertMessagesContext(ctx context.Context, records []Record) error
	GetMessage(ctx context.Context, guildID, messageID string) (*Record, error)
	DeleteMessagesContext(ctx context.Context, keys []DeleteKey) error
	InsertMessageVersionsMixedBatchContext(ctx context.Context, versions []Version) error
	CleanupExpiredMessages() error
	IncrementDailyMessageCountsContext(ctx context.Context, deltas []DailyCountDelta) error
	DeleteMessage(ctx context.Context, guildID, messageID string) error
	InsertMessageVersion(ctx context.Context, v Version) error
	IncrementDailyMessageCount(ctx context.Context, guildID string) error
}
