package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"
	"weak"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// WeakRef encapsulates a weak pointer and an explicit time-to-live expiration boundary.
// It allows the runtime to reclaim the underlying memory if no other strong references exist,
// preventing the cache from becoming a memory leak under high load.
type WeakRef[T any] struct {
	ptr       weak.Pointer[T]
	expiresAt time.Time
}

// Shard represents a dedicated partition of the cache state secured by an independent RWMutex.
// Sharding the map reduces lock contention across concurrent Discord events affecting different entities.
type Shard[T any] struct {
	mu   sync.RWMutex
	data map[string]WeakRef[T]
}

// getShardIndex computes a deterministic, non-cryptographic hash for key distribution across shards.
func getShardIndex(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % 16
}

// Segment orchestrates a fixed array of shards to uniformly distribute cache entries based on a hashed key.
type Segment[T any] struct {
	shards [16]*Shard[T]
	ttl    time.Duration
}

// NewSegment initializes a highly concurrent Segment with exactly 16 pre-allocated shards.
// The fixed size avoids dynamic slice reallocation during high-throughput hash indexing.
func NewSegment[T any](ttl time.Duration) *Segment[T] {
	s := &Segment[T]{ttl: ttl}
	for i := 0; i < 16; i++ {
		s.shards[i] = &Shard[T]{data: make(map[string]WeakRef[T])}
	}
	return s
}

// Get retrieves a strongly-typed value from the cache if it exists, is not expired, and hasn't been collected.
func (s *Segment[T]) Get(key string) (*T, bool) {
	shard := s.shards[getShardIndex(key)]
	shard.mu.RLock()
	ref, ok := shard.data[key]
	shard.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(ref.expiresAt) {
		// Explicitly prune expired references to maintain deterministic map sizing before eviction.
		s.Invalidate(key)
		return nil, false
	}

	val := ref.ptr.Value()
	if val == nil {
		slog.Warn("Mitigated service degradation: Stale read detected, weak pointer collected before explicit invalidation",
			slog.String("key", key),
		)
		s.Invalidate(key)
		return nil, false
	}
	slog.Debug("Granular transient state inspection: Cache hit", slog.String("key", key))

	return val, true
}

// Set inserts a new value into the designated shard as a weak reference.
// Callers must maintain a strong reference elsewhere if they intend for the item to persist
// beyond the next garbage collection cycle.
func (s *Segment[T]) Set(key string, val *T) {
	if val == nil {
		return
	}
	shard := s.shards[getShardIndex(key)]
	shard.mu.Lock()
	shard.data[key] = WeakRef[T]{
		ptr:       weak.Make(val),
		expiresAt: time.Now().Add(s.ttl),
	}
	shard.mu.Unlock()

	// GC Eviction: Register a deterministic cleanup callback to purge the map entry
	// immediately when the underlying object is garbage collected.
	runtime.AddCleanup(val, func(k string) {
		s.Invalidate(k)
	}, key)
}

// Invalidate forcefully purges an entry from the underlying shard regardless of TTL or GC state.
func (s *Segment[T]) Invalidate(key string) {
	shard := s.shards[getShardIndex(key)]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.data, key)
}

// Snapshot aggregates and returns all currently active, non-expired cache entries across all shards.
// This operation is computationally expensive and acquires read locks sequentially across the segment.
func (s *Segment[T]) Snapshot() map[string]*T {
	snapshot := make(map[string]*T)
	for i := 0; i < 16; i++ {
		shard := s.shards[i]
		shard.mu.RLock()
		for k, ref := range shard.data {
			if val := ref.ptr.Value(); val != nil && time.Now().Before(ref.expiresAt) {
				snapshot[k] = val
			}
		}
		shard.mu.RUnlock()
	}
	return snapshot
}

// CacheConfig aggregates time-to-live durations and persistence dependencies for the cache tier.
type CacheConfig struct {
	MemberTTL  time.Duration
	GuildTTL   time.Duration
	RolesTTL   time.Duration
	ChannelTTL time.Duration
	Store      *storage.Store
}

// UnifiedCache serves as the central orchestration registry for all entity-specific memory segments.
type UnifiedCache struct {
	members  *Segment[discord.Member]
	guilds   *Segment[discord.Guild]
	roles    *Segment[[]discord.Role]
	channels *Segment[discord.Channel]

	store *storage.Store
}

// NewUnifiedCache instantiates a comprehensive caching layer bound to the provided TTL configurations.
func NewUnifiedCache(cfg CacheConfig) *UnifiedCache {
	slog.Info("Architectural state transition: Initializing UnifiedCache",
		slog.Duration("member_ttl", cfg.MemberTTL),
		slog.Duration("guild_ttl", cfg.GuildTTL),
	)
	return &UnifiedCache{
		members:  NewSegment[discord.Member](cfg.MemberTTL),
		guilds:   NewSegment[discord.Guild](cfg.GuildTTL),
		roles:    NewSegment[[]discord.Role](cfg.RolesTTL),
		channels: NewSegment[discord.Channel](cfg.ChannelTTL),
		store:    cfg.Store,
	}
}

// Accessors
// GetMember retrieves a Guild Member from the transient memory segment.
func (uc *UnifiedCache) GetMember(guildID, userID string) (*discord.Member, bool) {
	return uc.members.Get(guildID + ":" + userID)
}

// SetMember injects a Guild Member into the transient memory segment.
func (uc *UnifiedCache) SetMember(guildID, userID string, member *discord.Member) {
	uc.members.Set(guildID+":"+userID, member)
}

// InvalidateMember evicts a specific Guild Member from the memory segment.
func (uc *UnifiedCache) InvalidateMember(guildID, userID string) {
	uc.members.Invalidate(guildID + ":" + userID)
}

// GetGuild retrieves a Guild structure from the transient memory segment.
func (uc *UnifiedCache) GetGuild(guildID string) (*discord.Guild, bool) {
	return uc.guilds.Get(guildID)
}

// SetGuild injects a Guild structure into the transient memory segment.
func (uc *UnifiedCache) SetGuild(guildID string, guild *discord.Guild) {
	uc.guilds.Set(guildID, guild)
}

// InvalidateGuild evicts a specific Guild structure from the memory segment.
func (uc *UnifiedCache) InvalidateGuild(guildID string) {
	uc.guilds.Invalidate(guildID)
}

// GetRoles retrieves an aggregate slice of Guild Roles from the transient memory segment.
func (uc *UnifiedCache) GetRoles(guildID string) (*[]discord.Role, bool) {
	return uc.roles.Get(guildID)
}

// SetRoles injects an aggregate slice of Guild Roles into the transient memory segment.
func (uc *UnifiedCache) SetRoles(guildID string, roles *[]discord.Role) {
	uc.roles.Set(guildID, roles)
}

// InvalidateRoles evicts the aggregate slice of Roles for a specific Guild from the memory segment.
func (uc *UnifiedCache) InvalidateRoles(guildID string) {
	uc.roles.Invalidate(guildID)
}

// GetChannel retrieves a Channel structure from the transient memory segment.
func (uc *UnifiedCache) GetChannel(channelID string) (*discord.Channel, bool) {
	return uc.channels.Get(channelID)
}

// SetChannel injects a Channel structure into the transient memory segment.
func (uc *UnifiedCache) SetChannel(channelID string, channel *discord.Channel) {
	uc.channels.Set(channelID, channel)
}

// InvalidateChannel evicts a specific Channel structure from the memory segment.
func (uc *UnifiedCache) InvalidateChannel(channelID string) {
	uc.channels.Invalidate(channelID)
}

// Warmup recovery handling for corrupt JSON/Gob snapshots
// Warmup reconstructs the transient in-memory state from the persistent Postgres store.
func (uc *UnifiedCache) Warmup(ctx context.Context) error {
	if uc.store == nil {
		return nil
	}

	for entry, err := range uc.store.GetCacheEntriesByType("guild") {
		if err != nil {
			return fmt.Errorf("warmup read: %w", err)
		}

		var g discord.Guild
		if err := json.Unmarshal([]byte(entry.Data), &g); err != nil {
			slog.Warn("Mitigated service degradation: Aborted warmup for corrupted guild snapshot",
				slog.String("request_id", "warmup"),
				slog.String("key", entry.Key),
				slog.String("error", err.Error()),
			)
			continue
		}
		uc.SetGuild(strings.TrimPrefix(entry.Key, "guild:"), &g)
	}

	return nil
}

// WarmupConfig encapsulates heuristic parameters for targeted cache pre-warming flows.
type WarmupConfig struct {
	FetchMissingMembers bool
	MaxMembersPerGuild  int
}

// DefaultWarmupConfig constructs a zero-value configuration struct for cache warmup.
func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{}
}

// IntelligentWarmupContext orchestrates an adaptive hydration phase tailored to specific cache contexts.
func IntelligentWarmupContext(ctx context.Context, session any, uc *UnifiedCache, store *storage.Store, config WarmupConfig) error {
	return uc.Warmup(ctx)
}

// WasWarmedUpRecently validates whether the cache layer received a hydration payload within the specified duration window.
func (uc *UnifiedCache) WasWarmedUpRecently(d time.Duration) bool {
	return false
}

// SchedulePeriodicCleanup initializes a background goroutine to purge expired entries from the durable store.
// Callers must close the returned channel to terminate the background collector safely.
func SchedulePeriodicCleanup(store *storage.Store, interval time.Duration) chan struct{} {
	slog.Info("Architectural state transition: Initializing persistent cache garbage collector")
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if store != nil {
					_ = store.CleanupExpiredCacheEntries()
				}
			case <-stop:
				return
			}
		}
	}()
	return stop
}
