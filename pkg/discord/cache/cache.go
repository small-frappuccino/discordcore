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

type WeakRef[T any] struct {
	ptr       weak.Pointer[T]
	expiresAt time.Time
}

type Shard[T any] struct {
	mu   sync.RWMutex
	data map[string]WeakRef[T]
}

func getShardIndex(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32() % 16
}

type Segment[T any] struct {
	shards [16]*Shard[T]
	ttl    time.Duration
}

func NewSegment[T any](ttl time.Duration) *Segment[T] {
	s := &Segment[T]{ttl: ttl}
	for i := 0; i < 16; i++ {
		s.shards[i] = &Shard[T]{data: make(map[string]WeakRef[T])}
	}
	return s
}

func (s *Segment[T]) Get(key string) (*T, bool) {
	shard := s.shards[getShardIndex(key)]
	shard.mu.RLock()
	ref, ok := shard.data[key]
	shard.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(ref.expiresAt) {
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

	// GC Eviction
	runtime.AddCleanup(val, func(k string) {
		s.Invalidate(k)
	}, key)
}

func (s *Segment[T]) Invalidate(key string) {
	shard := s.shards[getShardIndex(key)]
	shard.mu.Lock()
	defer shard.mu.Unlock()
	delete(shard.data, key)
}

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

type CacheConfig struct {
	MemberTTL  time.Duration
	GuildTTL   time.Duration
	RolesTTL   time.Duration
	ChannelTTL time.Duration
	Store      *storage.Store
}

type UnifiedCache struct {
	members  *Segment[discord.Member]
	guilds   *Segment[discord.Guild]
	roles    *Segment[[]discord.Role]
	channels *Segment[discord.Channel]

	store *storage.Store
}

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
func (uc *UnifiedCache) GetMember(guildID, userID string) (*discord.Member, bool) {
	return uc.members.Get(guildID + ":" + userID)
}

func (uc *UnifiedCache) SetMember(guildID, userID string, member *discord.Member) {
	uc.members.Set(guildID+":"+userID, member)
}

func (uc *UnifiedCache) InvalidateMember(guildID, userID string) {
	uc.members.Invalidate(guildID + ":" + userID)
}

func (uc *UnifiedCache) GetGuild(guildID string) (*discord.Guild, bool) {
	return uc.guilds.Get(guildID)
}

func (uc *UnifiedCache) SetGuild(guildID string, guild *discord.Guild) {
	uc.guilds.Set(guildID, guild)
}

func (uc *UnifiedCache) InvalidateGuild(guildID string) {
	uc.guilds.Invalidate(guildID)
}

func (uc *UnifiedCache) GetRoles(guildID string) (*[]discord.Role, bool) {
	return uc.roles.Get(guildID)
}

func (uc *UnifiedCache) SetRoles(guildID string, roles *[]discord.Role) {
	uc.roles.Set(guildID, roles)
}

func (uc *UnifiedCache) InvalidateRoles(guildID string) {
	uc.roles.Invalidate(guildID)
}

func (uc *UnifiedCache) GetChannel(channelID string) (*discord.Channel, bool) {
	return uc.channels.Get(channelID)
}

func (uc *UnifiedCache) SetChannel(channelID string, channel *discord.Channel) {
	uc.channels.Set(channelID, channel)
}

func (uc *UnifiedCache) InvalidateChannel(channelID string) {
	uc.channels.Invalidate(channelID)
}

// Warmup recovery handling for corrupt JSON/Gob snapshots
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

type WarmupConfig struct {
	FetchMissingMembers bool
	MaxMembersPerGuild  int
}

func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{}
}

func IntelligentWarmupContext(ctx context.Context, session any, uc *UnifiedCache, store *storage.Store, config WarmupConfig) error {
	return uc.Warmup(ctx)
}

func (uc *UnifiedCache) WasWarmedUpRecently(d time.Duration) bool {
	return false
}

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
