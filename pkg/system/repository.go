package system

import (
	"context"
	"iter"
	"time"
)

type Repository interface {
	SetBotSince(ctx context.Context, guildID string, t time.Time) error
	BotSince(ctx context.Context, guildID string) (time.Time, bool, error)
	SetHeartbeatForBot(ctx context.Context, instanceID string, t time.Time) error
	SetLastEventForBot(ctx context.Context, instanceID string, t time.Time) error
	Heartbeat(ctx context.Context) (time.Time, bool, error)
	NextTicketID(ctx context.Context, guildID string) (int64, error)
	UpsertCacheEntriesContext(ctx context.Context, entries []CacheEntryRecord) error
	GetCacheEntry(ctx context.Context, key string) (cacheType, data string, expiresAt time.Time, ok bool, err error)
	GetCacheEntriesByType(ctx context.Context, cacheType string) iter.Seq2[CacheEntry, error]
	CleanupExpiredCacheEntries(ctx context.Context) error
	GetCacheStatsContext(ctx context.Context) (PersistentCacheStats, error)
	PurgeGuildModerationData(ctx context.Context, guildID string) error
	IncrementDailyMemberJoinContext(ctx context.Context, guildID, userID string, timestamp time.Time) error
	IncrementDailyMemberLeaveContext(ctx context.Context, guildID, userID string, timestamp time.Time) error
	HeartbeatForBot(ctx context.Context, instanceID string) (time.Time, bool, error)
	LastEventForBot(ctx context.Context, instanceID string) (time.Time, bool, error)
	Metadata(ctx context.Context, key string) (time.Time, bool, error)
	SetMetadata(ctx context.Context, key string, at time.Time) error
}
