package cache

import (
	"context"
	"time"

	"github.com/diamondburned/arikawa/v3/state/store"
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

// SnapshotCabinet returns a typed point-in-time view of every observable counter on
// the cache. When pgStore is non-nil the persisted totals are queried under the
// caller's context; query errors are swallowed so the in-memory counters still
// reach the caller — the route layer can decide whether to surface the partial
// snapshot or fail loud.
func SnapshotCabinet(ctx context.Context, cab *store.Cabinet, pgStore *storage.Store) CacheMetricsSnapshot {
	out := CacheMetricsSnapshot{}

	if cab != nil {
		gSlice, _ := cab.Guilds()
		out.Guilds.Entries = len(gSlice)

		for _, g := range gSlice {
			mSlice, _ := cab.Members(g.ID)
			out.Members.Entries += len(mSlice)
			rSlice, _ := cab.Roles(g.ID)
			out.Roles.Entries += len(rSlice)
			cSlice, _ := cab.Channels(g.ID)
			out.Channels.Entries += len(cSlice)
		}

		pcSlice, _ := cab.PrivateChannels()
		out.Channels.Entries += len(pcSlice)
	}

	if pgStore != nil {
		if persisted, err := pgStore.GetCacheStatsContext(ctx); err == nil {
			out.Persisted = persisted
		}
	}
	return out
}
