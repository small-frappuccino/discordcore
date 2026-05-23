package cache

import (
	"context"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// SegmentSnapshot is the per-segment view the /v1/health/cache endpoint and
// any future dashboard widget consume. Fields mirror segmentStats but with a
// stable JSON shape so scrapers can `jq` them without re-deriving rates from
// hits and misses.
type SegmentSnapshot struct {
	Entries    int     `json:"entries"`
	Hits       uint64  `json:"hits"`
	Misses     uint64  `json:"misses"`
	Evictions  uint64  `json:"evictions"`
	HitRate    float64 `json:"hit_rate"`
	TTLSeconds int     `json:"ttl_seconds"`
	Limit      int     `json:"limit"`
}

// CacheMetricsSnapshot is the JSON payload /v1/health/cache returns. It bundles
// the four in-memory segments, the persisted cache totals (queried via the
// optional store), and the last warmup timestamp.
type CacheMetricsSnapshot struct {
	Members    SegmentSnapshot              `json:"members"`
	Guilds     SegmentSnapshot              `json:"guilds"`
	Roles      SegmentSnapshot              `json:"roles"`
	Channels   SegmentSnapshot              `json:"channels"`
	Persisted  storage.PersistentCacheStats `json:"persisted"`
	LastWarmup time.Time                    `json:"last_warmup"`
}

// Snapshot returns a typed point-in-time view of every observable counter on
// the cache. When store is non-nil the persisted totals are queried under the
// caller's context; query errors are swallowed so the in-memory counters still
// reach the caller — the route layer can decide whether to surface the partial
// snapshot or fail loud.
//
// Snapshot is safe to call concurrently; segment Stats() takes its own locks
// and the Postgres query is read-only.
func (uc *UnifiedCache) Snapshot(ctx context.Context, store *storage.Store) CacheMetricsSnapshot {
	if uc == nil {
		return CacheMetricsSnapshot{}
	}
	out := CacheMetricsSnapshot{LastWarmup: uc.lastWarmup}
	if uc.members != nil {
		out.Members = buildSegmentSnapshot(uc.members.Stats())
	}
	if uc.guilds != nil {
		out.Guilds = buildSegmentSnapshot(uc.guilds.Stats())
	}
	if uc.roles != nil {
		out.Roles = buildSegmentSnapshot(uc.roles.Stats())
	}
	if uc.channels != nil {
		out.Channels = buildSegmentSnapshot(uc.channels.Stats())
	}
	if store != nil {
		if persisted, err := store.GetCacheStatsContext(ctx); err == nil {
			out.Persisted = persisted
		}
	}
	return out
}

func buildSegmentSnapshot(s segmentStats) SegmentSnapshot {
	return SegmentSnapshot{
		Entries:    s.Size,
		Hits:       s.Hits,
		Misses:     s.Misses,
		Evictions:  s.Evictions,
		HitRate:    s.HitRate,
		TTLSeconds: int(s.TTL / time.Second),
		Limit:      s.Limit,
	}
}
