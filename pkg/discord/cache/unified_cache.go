package cache

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	genericcache "github.com/small-frappuccino/discordcore/pkg/cache"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

// to reduce API calls and improve performance. It includes TTL-based expiration, LRU eviction,
// and optional SQLite persistence.
type UnifiedCache struct {
	// Member cache segment: guildID:userID -> *discordgo.Member
	members *segment[*discordgo.Member]

	// Guild cache segment: guildID -> *discordgo.Guild
	guilds *segment[*discordgo.Guild]

	// Roles cache segment: guildID -> []*discordgo.Role
	roles *segment[[]*discordgo.Role]

	// Channel cache segment: channelID -> *discordgo.Channel
	channels *segment[*discordgo.Channel]

	// Indices to relate channels and guilds for efficient guild-scoped operations
	guildToChannels   map[string]map[string]struct{}
	guildToChannelsMu sync.RWMutex
	channelToGuild    map[string]string
	channelToGuildMu  sync.RWMutex

	// TTL configurations (configurable per type)
	memberTTL  time.Duration
	guildTTL   time.Duration
	rolesTTL   time.Duration
	channelTTL time.Duration

	// LRU size limits (0 = unlimited)
	maxMemberSize  int
	maxGuildSize   int
	maxRolesSize   int
	maxChannelSize int

	// Metrics are tracked per-segment

	// SQLite persistence (optional)
	store          *storage.Store
	persistEnabled bool

	// Cleanup
	stopCleanup chan struct{}
	cleanupOnce sync.Once

	// Last cleanup timestamp for stats reporting
	lastCleanup time.Time
	// Last warmup timestamp for recency checks
	lastWarmup time.Time
}

// Small helpers to build keys/prefixes and centralize comparisons
func (uc *UnifiedCache) memberKey(guildID, userID string) string {
	if guildID == "" || userID == "" {
		return ""
	}
	return guildID + ":" + userID
}

func (uc *UnifiedCache) memberPrefix(guildID string) string {
	if guildID == "" {
		return ""
	}
	return guildID + ":"
}

// Cached value types

// Persistent cache entry for SQLite
type persistentCacheEntry struct {
	Key       string    `json:"key"`
	Type      string    `json:"type"` // "member", "guild", "roles", "channel"
	Data      string    `json:"data"` // JSON-encoded entity
	ExpiresAt time.Time `json:"expires_at"`
}

// CacheConfig holds configuration for the unified cache
type CacheConfig struct {
	MemberTTL       time.Duration
	GuildTTL        time.Duration
	RolesTTL        time.Duration
	ChannelTTL      time.Duration
	CleanupInterval time.Duration

	// LRU size limits (0 = unlimited)
	MaxMemberSize  int
	MaxGuildSize   int
	MaxRolesSize   int
	MaxChannelSize int

	// SQLite persistence
	Store          *storage.Store
	PersistEnabled bool
}

// DefaultCacheConfig returns sensible defaults for the cache
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		MemberTTL:       5 * time.Minute,
		GuildTTL:        15 * time.Minute,
		RolesTTL:        10 * time.Minute,
		ChannelTTL:      15 * time.Minute,
		CleanupInterval: 5 * time.Minute,

		// LRU limits (0 = unlimited)
		MaxMemberSize:  10000, // ~10k members per bot instance
		MaxGuildSize:   100,   // ~100 guilds
		MaxRolesSize:   100,   // Roles per guild
		MaxChannelSize: 1000,  // ~1k channels

		// Persistence disabled by default
		PersistEnabled: false,
	}
}

// NewUnifiedCache creates a new unified cache with the given configuration
func NewUnifiedCache(cfg CacheConfig) *UnifiedCache {
	if cfg.MemberTTL == 0 {
		cfg = DefaultCacheConfig()
	}

	uc := &UnifiedCache{
		members:         newSegment[*discordgo.Member](cfg.MemberTTL, cfg.MaxMemberSize),
		guilds:          newSegment[*discordgo.Guild](cfg.GuildTTL, cfg.MaxGuildSize),
		roles:           newSegment[[]*discordgo.Role](cfg.RolesTTL, cfg.MaxRolesSize),
		channels:        newSegment[*discordgo.Channel](cfg.ChannelTTL, cfg.MaxChannelSize),
		guildToChannels: make(map[string]map[string]struct{}),
		channelToGuild:  make(map[string]string),

		memberTTL:  cfg.MemberTTL,
		guildTTL:   cfg.GuildTTL,
		rolesTTL:   cfg.RolesTTL,
		channelTTL: cfg.ChannelTTL,

		maxMemberSize:  cfg.MaxMemberSize,
		maxGuildSize:   cfg.MaxGuildSize,
		maxRolesSize:   cfg.MaxRolesSize,
		maxChannelSize: cfg.MaxChannelSize,

		store:          cfg.Store,
		persistEnabled: cfg.PersistEnabled && cfg.Store != nil,

		stopCleanup: make(chan struct{}),
	}

	// Start background cleanup goroutine
	go uc.cleanupLoop(cfg.CleanupInterval)

	return uc
}

// GetMember retrieves a cached member or returns nil if not found/expired
func (uc *UnifiedCache) GetMember(guildID, userID string) (*discordgo.Member, bool) {
	key := uc.memberKey(guildID, userID)
	if key == "" || uc.members == nil {
		return nil, false
	}
	return uc.members.Get(key)
}

// SetMember stores a member in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetMember(guildID, userID string, member *discordgo.Member) {
	if member == nil || uc.members == nil {
		return
	}
	key := uc.memberKey(guildID, userID)
	if key == "" {
		return
	}
	uc.members.Set(key, member)
}

// evictMemberLRU removes the least recently used member (must hold lock)
func (uc *UnifiedCache) evictMemberLRU() {}

// InvalidateMember removes a member from the cache
func (uc *UnifiedCache) InvalidateMember(guildID, userID string) {
	key := uc.memberKey(guildID, userID)
	if key == "" || uc.members == nil {
		return
	}
	uc.members.Invalidate(key)
}

// GetGuild retrieves a cached guild or returns nil if not found/expired
func (uc *UnifiedCache) GetGuild(guildID string) (*discordgo.Guild, bool) {
	if guildID == "" || uc.guilds == nil {
		return nil, false
	}
	return uc.guilds.Get(guildID)
}

// SetGuild stores a guild in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetGuild(guildID string, guild *discordgo.Guild) {
	if guild == nil || uc.guilds == nil || guildID == "" {
		return
	}
	uc.guilds.Set(guildID, guild)
}

// evictGuildLRU removes the least recently used guild (must hold lock)
func (uc *UnifiedCache) evictGuildLRU() {}

// InvalidateGuild removes a guild from the cache
func (uc *UnifiedCache) InvalidateGuild(guildID string) {
	if guildID == "" || uc.guilds == nil {
		return
	}
	uc.guilds.Invalidate(guildID)
}

// GetRoles retrieves cached roles for a guild or returns nil if not found/expired
func (uc *UnifiedCache) GetRoles(guildID string) ([]*discordgo.Role, bool) {
	if guildID == "" || uc.roles == nil {
		return nil, false
	}
	return uc.roles.Get(guildID)
}

// SetRoles stores guild roles in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetRoles(guildID string, roles []*discordgo.Role) {
	if roles == nil || uc.roles == nil || guildID == "" {
		return
	}
	uc.roles.Set(guildID, roles)
}

// evictRolesLRU removes the least recently used roles (must hold lock)
func (uc *UnifiedCache) evictRolesLRU() {}

// InvalidateRoles removes guild roles from the cache
func (uc *UnifiedCache) InvalidateRoles(guildID string) {
	if guildID == "" || uc.roles == nil {
		return
	}
	uc.roles.Invalidate(guildID)
}

// GetChannel retrieves a cached channel or returns nil if not found/expired
func (uc *UnifiedCache) GetChannel(channelID string) (*discordgo.Channel, bool) {
	if channelID == "" || uc.channels == nil {
		return nil, false
	}
	return uc.channels.Get(channelID)
}

// SetChannel stores a channel in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetChannel(channelID string, channel *discordgo.Channel) {
	if channel == nil || uc.channels == nil || channelID == "" {
		return
	}

	// Update indices before inserting
	if channel.GuildID != "" {
		// channelID -> guildID
		uc.channelToGuildMu.Lock()
		if uc.channelToGuild == nil {
			uc.channelToGuild = make(map[string]string)
		}
		oldGuildID := uc.channelToGuild[channelID]
		uc.channelToGuild[channelID] = channel.GuildID
		uc.channelToGuildMu.Unlock()

		// guildID -> set(channelID)
		uc.guildToChannelsMu.Lock()
		if uc.guildToChannels == nil {
			uc.guildToChannels = make(map[string]map[string]struct{})
		}
		if oldGuildID != "" && oldGuildID != channel.GuildID {
			if set, ok := uc.guildToChannels[oldGuildID]; ok {
				delete(set, channelID)
				if len(set) == 0 {
					delete(uc.guildToChannels, oldGuildID)
				}
			}
		}
		set := uc.guildToChannels[channel.GuildID]
		if set == nil {
			set = make(map[string]struct{})
			uc.guildToChannels[channel.GuildID] = set
		}
		set[channelID] = struct{}{}
		uc.guildToChannelsMu.Unlock()
	} else {
		// No guild association
		uc.channelToGuildMu.Lock()
		if uc.channelToGuild != nil {
			delete(uc.channelToGuild, channelID)
		}
		uc.channelToGuildMu.Unlock()
	}

	uc.channels.Set(channelID, channel)
}

// evictChannelLRU removes the least recently used channel (must hold lock)
func (uc *UnifiedCache) evictChannelLRU() {}

// InvalidateChannel removes a channel from the cache
func (uc *UnifiedCache) InvalidateChannel(channelID string) {
	if channelID == "" || uc.channels == nil {
		return
	}
	// Update indices
	var guildID string
	uc.channelToGuildMu.RLock()
	if uc.channelToGuild != nil {
		guildID = uc.channelToGuild[channelID]
	}
	uc.channelToGuildMu.RUnlock()

	if guildID != "" {
		uc.guildToChannelsMu.Lock()
		if set, ok := uc.guildToChannels[guildID]; ok {
			delete(set, channelID)
			if len(set) == 0 {
				delete(uc.guildToChannels, guildID)
			}
		}
		uc.guildToChannelsMu.Unlock()
	}

	uc.channelToGuildMu.Lock()
	if uc.channelToGuild != nil {
		delete(uc.channelToGuild, channelID)
	}
	uc.channelToGuildMu.Unlock()

	uc.channels.Invalidate(channelID)
}

// GetStats returns cache statistics
func (uc *UnifiedCache) GetStats() genericcache.CacheStats {
	memberCount, guildCount, rolesCount, channelCount := 0, 0, 0, 0
	var memberHits, memberMisses, guildHits, guildMisses, rolesHits, rolesMisses, channelHits, channelMisses uint64

	if uc.members != nil {
		memberCount = uc.members.Len()
		ms := uc.members.Stats()
		memberHits = ms.Hits
		memberMisses = ms.Misses
	}
	if uc.guilds != nil {
		guildCount = uc.guilds.Len()
		gs := uc.guilds.Stats()
		guildHits = gs.Hits
		guildMisses = gs.Misses
	}
	if uc.roles != nil {
		rolesCount = uc.roles.Len()
		rs := uc.roles.Stats()
		rolesHits = rs.Hits
		rolesMisses = rs.Misses
	}
	if uc.channels != nil {
		channelCount = uc.channels.Len()
		cs := uc.channels.Stats()
		channelHits = cs.Hits
		channelMisses = cs.Misses
	}

	totalEntries := memberCount + guildCount + rolesCount + channelCount
	totalHits := float64(memberHits + guildHits + rolesHits + channelHits)
	totalMisses := float64(memberMisses + guildMisses + rolesMisses + channelMisses)
	var hitRate, missRate float64
	if (totalHits + totalMisses) > 0 {
		hitRate = totalHits / (totalHits + totalMisses)
		missRate = totalMisses / (totalHits + totalMisses)
	}

	return genericcache.CacheStats{
		TotalEntries:  totalEntries,
		MemoryUsage:   0,
		HitRate:       hitRate,
		MissRate:      missRate,
		LastCleanup:   uc.lastCleanup,
		TTLEnabled:    true,
		PerGuildStats: nil,
		CustomMetrics: map[string]any{
			"memberEntries":  memberCount,
			"guildEntries":   guildCount,
			"rolesEntries":   rolesCount,
			"channelEntries": channelCount,
			"memberHits":     memberHits,
			"memberMisses":   memberMisses,
			"guildHits":      guildHits,
			"guildMisses":    guildMisses,
			"rolesHits":      rolesHits,
			"rolesMisses":    rolesMisses,
			"channelHits":    channelHits,
			"channelMisses":  channelMisses,
		},
	}
}

// StatsGeneric returns generic cache statistics for external consumers
func (uc *UnifiedCache) StatsGeneric() genericcache.CacheStats {
	return uc.GetStats()
}

// MemberMetrics returns typed metrics for the member segment.
func (uc *UnifiedCache) MemberMetrics() (entries int, hits, misses, evictions uint64) {
	if uc.members == nil {
		return 0, 0, 0, 0
	}
	s := uc.members.Stats()
	return s.Size, s.Hits, s.Misses, s.Evictions
}

// GuildMetrics returns typed metrics for the guild segment.
func (uc *UnifiedCache) GuildMetrics() (entries int, hits, misses, evictions uint64) {
	if uc.guilds == nil {
		return 0, 0, 0, 0
	}
	s := uc.guilds.Stats()
	return s.Size, s.Hits, s.Misses, s.Evictions
}

// RolesMetrics returns typed metrics for the roles segment.
func (uc *UnifiedCache) RolesMetrics() (entries int, hits, misses, evictions uint64) {
	if uc.roles == nil {
		return 0, 0, 0, 0
	}
	s := uc.roles.Stats()
	return s.Size, s.Hits, s.Misses, s.Evictions
}

// ChannelMetrics returns typed metrics for the channel segment.
func (uc *UnifiedCache) ChannelMetrics() (entries int, hits, misses, evictions uint64) {
	if uc.channels == nil {
		return 0, 0, 0, 0
	}
	s := uc.channels.Stats()
	return s.Size, s.Hits, s.Misses, s.Evictions
}

// WasWarmedUpRecently returns whether Warmup was executed within the given duration
// WasWarmedUpRecently reports whether a warmup has occurred within the given duration.
// Semantics:
// - d <= 0 returns false (treats as disabled check).
// - If no warmup has ever occurred (zero timestamp), returns false.
// - Otherwise returns time.Since(lastWarmup) <= d.
func (uc *UnifiedCache) WasWarmedUpRecently(d time.Duration) bool {
	if d <= 0 {
		return false
	}
	if uc.lastWarmup.IsZero() {
		return false
	}
	return time.Since(uc.lastWarmup) <= d
}

// Clear removes all entries from the cache
// Clear removes all in-memory cache entries across all cache types.
//
// Semantics:
// - Only in-memory maps are reset (members, guilds, roles, channels).
// - No persistent storage rows are touched. Use PersistAndStop or ClearGuild for durable cleanup.
// - This is safe to call at any time; ongoing readers may miss entries immediately after.
func (uc *UnifiedCache) Clear() {
	if uc.members != nil {
		uc.members.Clear()
	}
	if uc.guilds != nil {
		uc.guilds.Clear()
	}
	if uc.roles != nil {
		uc.roles.Clear()
	}
	if uc.channels != nil {
		uc.channels.Clear()
	}

	// Reset indices
	uc.guildToChannelsMu.Lock()
	uc.guildToChannels = make(map[string]map[string]struct{})
	uc.guildToChannelsMu.Unlock()

	uc.channelToGuildMu.Lock()
	uc.channelToGuild = make(map[string]string)
	uc.channelToGuildMu.Unlock()
}

// ClearGuild removes all cached data for a specific guild.
//
// Semantics:
//   - In-memory: invalidates guild, roles, and all member entries whose key has prefix "<guildID>:".
//   - Persistent: if persistence is enabled and a store is configured, it deletes durable rows by
//     cache_type and key prefix for:
//   - members:   type "member", keys "<guildID>:<userID>"
//   - guild:     type "guild",  key  "<guildID>"
//   - roles:     type "roles",  key  "<guildID>"
//   - channels:  type "channel", keys "<guildID>:<channelID>"
//   - Idempotent: safe to call multiple times.
func (uc *UnifiedCache) ClearGuild(guildID string) error {
	// Clear guild entry
	uc.InvalidateGuild(guildID)

	// Clear roles for this guild
	uc.InvalidateRoles(guildID)

	// Clear all members for this guild (scan all member keys with guildID prefix)
	if uc.members != nil {
		prefix := uc.memberPrefix(guildID)
		for _, key := range uc.members.Keys() {
			if util.HasPrefix(key, prefix) {
				uc.members.Invalidate(key)
			}
		}
	}

	// Clear channels for this guild using the guild->channels index
	uc.guildToChannelsMu.RLock()
	var chIDs []string
	if set, ok := uc.guildToChannels[guildID]; ok {
		chIDs = make([]string, 0, len(set))
		for cid := range set {
			chIDs = append(chIDs, cid)
		}
	}
	uc.guildToChannelsMu.RUnlock()
	for _, cid := range chIDs {
		uc.InvalidateChannel(cid)
	}

	// Clear from persistent storage if enabled
	if uc.persistEnabled && uc.store != nil {
		// Delete persistent cache entries for this guild using precise type+prefix filtering
		if err := uc.store.DeleteCacheEntriesByTypeAndPrefix("member", guildID+":"); err != nil {
			return fmt.Errorf("delete member cache entries: %w", err)
		}
		if err := uc.store.DeleteCacheEntriesByTypeAndPrefix("guild", guildID); err != nil {
			return fmt.Errorf("delete guild cache entry: %w", err)
		}
		if err := uc.store.DeleteCacheEntriesByTypeAndPrefix("roles", guildID); err != nil {
			return fmt.Errorf("delete roles cache entry: %w", err)
		}
		if err := uc.store.DeleteCacheEntriesByTypeAndPrefix("channel", guildID+":"); err != nil {
			return fmt.Errorf("delete channel cache entries: %w", err)
		}
	}

	return nil
}

// Stop stops the background cleanup goroutine
func (uc *UnifiedCache) Stop() {
	uc.cleanupOnce.Do(func() {
		if uc.stopCleanup != nil {
			close(uc.stopCleanup)
			uc.stopCleanup = nil
		}
	})
}

// cleanupLoop periodically removes expired entries
func (uc *UnifiedCache) cleanupLoop(interval time.Duration) {
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			uc.cleanupExpired()
		case <-uc.stopCleanup:
			return
		}
	}
}

// cleanupExpired removes all expired entries from all caches
func (uc *UnifiedCache) cleanupExpired() {
	now := time.Now()

	// Cleanup segments
	if uc.members != nil {
		uc.members.CleanupExpired(now)
	}
	if uc.guilds != nil {
		uc.guilds.CleanupExpired(now)
	}
	if uc.roles != nil {
		uc.roles.CleanupExpired(now)
	}
	if uc.channels != nil {
		uc.channels.CleanupExpiredWithCallback(now, func(key string, _ *discordgo.Channel) {
			// Cleanup indices for channels
			uc.channelToGuildMu.RLock()
			guildID := uc.channelToGuild[key]
			uc.channelToGuildMu.RUnlock()
			if guildID != "" {
				uc.guildToChannelsMu.Lock()
				if set, ok := uc.guildToChannels[guildID]; ok {
					delete(set, key)
					if len(set) == 0 {
						delete(uc.guildToChannels, guildID)
					}
				}
				uc.guildToChannelsMu.Unlock()
			}
			uc.channelToGuildMu.Lock()
			delete(uc.channelToGuild, key)
			uc.channelToGuildMu.Unlock()
		})
	}

	// Track last cleanup time for stats
	uc.lastCleanup = now
}

// Persist saves current cache state to SQLite (if enabled)
// Persist writes all non-expired in-memory entries to the persistent store.
//
// Notes:
// - Each cache type is encoded to JSON and upserted with its TTL-derived expiry.
// - Errors are aggregated; the method returns a single error summarizing count, if any.
// - No entries are removed in-memory by this call.
func (uc *UnifiedCache) Persist() error {
	if !uc.persistEnabled || uc.store == nil {
		return nil
	}

	var errs []error

	// Persist members
	if uc.members != nil {
		for _, key := range uc.members.Keys() {
			member, ok := uc.members.Get(key)
			if !ok || member == nil {
				continue
			}
			data, err := encodeEntity(member)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode member %s: %w", key, err))
				continue
			}
			exp, _ := uc.members.GetExpiration(key)
			if err := uc.store.UpsertCacheEntry(key, "member", data, exp); err != nil {
				errs = append(errs, fmt.Errorf("persist member %s: %w", key, err))
			}
		}
	}

	// Persist guilds
	if uc.guilds != nil {
		for _, key := range uc.guilds.Keys() {
			guild, ok := uc.guilds.Get(key)
			if !ok || guild == nil {
				continue
			}
			data, err := encodeEntity(guild)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode guild %s: %w", key, err))
				continue
			}
			exp, _ := uc.guilds.GetExpiration(key)
			if err := uc.store.UpsertCacheEntry(key, "guild", data, exp); err != nil {
				errs = append(errs, fmt.Errorf("persist guild %s: %w", key, err))
			}
		}
	}

	// Persist roles
	if uc.roles != nil {
		for _, key := range uc.roles.Keys() {
			roles, ok := uc.roles.Get(key)
			if !ok || roles == nil {
				continue
			}
			data, err := encodeEntity(roles)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode roles %s: %w", key, err))
				continue
			}
			exp, _ := uc.roles.GetExpiration(key)
			if err := uc.store.UpsertCacheEntry(key, "roles", data, exp); err != nil {
				errs = append(errs, fmt.Errorf("persist roles %s: %w", key, err))
			}
		}
	}

	// Persist channels
	if uc.channels != nil {
		for _, key := range uc.channels.Keys() {
			ch, ok := uc.channels.Get(key)
			if !ok || ch == nil {
				continue
			}
			data, err := encodeEntity(ch)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode channel %s: %w", key, err))
				continue
			}
			persistKey := key
			if ch.GuildID != "" {
				persistKey = ch.GuildID + ":" + ch.ID
			}
			exp, _ := uc.channels.GetExpiration(key)
			if err := uc.store.UpsertCacheEntry(persistKey, "channel", data, exp); err != nil {
				errs = append(errs, fmt.Errorf("persist channel %s: %w", key, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("persist cache: %d errors occurred", len(errs))
	}
	return nil
}

// Warmup pre-populates the cache from SQLite (if enabled)
//
// Behavior and guarantees:
// - No-op if persistence is disabled or the store is nil.
// - Loads only non-expired entries for each cache type ("member", "guild", "roles", "channel").
// - Uses internal setters that bypass LRU side effects during initial load.
// - Does not evict existing in-memory entries; entries are upserted by key.
// - Corrupt rows are skipped (best-effort); errors per section return with context.
// - Updates the last warmup timestamp for WasWarmedUpRecently checks.
func (uc *UnifiedCache) Warmup() error {
	if !uc.persistEnabled || uc.store == nil {
		return nil
	}

	var totalLoaded int

	// Warmup members
	memberEntries, err := uc.store.GetCacheEntriesByType("member")
	if err != nil {
		return fmt.Errorf("warmup members: %w", err)
	}
	for _, entry := range memberEntries {
		var member discordgo.Member
		if err := decodeEntity(entry.Data, &member); err != nil {
			continue // Skip corrupted entries
		}
		// Use internal method to avoid LRU eviction during warmup
		uc.setMemberInternal(entry.Key, &member, entry.ExpiresAt)
		totalLoaded++
	}

	// Warmup guilds
	guildEntries, err := uc.store.GetCacheEntriesByType("guild")
	if err != nil {
		return fmt.Errorf("warmup guilds: %w", err)
	}
	for _, entry := range guildEntries {
		var guild discordgo.Guild
		if err := decodeEntity(entry.Data, &guild); err != nil {
			continue
		}
		uc.setGuildInternal(entry.Key, &guild, entry.ExpiresAt)
		totalLoaded++
	}

	// Warmup roles
	rolesEntries, err := uc.store.GetCacheEntriesByType("roles")
	if err != nil {
		return fmt.Errorf("warmup roles: %w", err)
	}
	for _, entry := range rolesEntries {
		var roles []*discordgo.Role
		if err := decodeEntity(entry.Data, &roles); err != nil {
			continue
		}
		uc.setRolesInternal(entry.Key, roles, entry.ExpiresAt)
		totalLoaded++
	}

	// Warmup channels
	channelEntries, err := uc.store.GetCacheEntriesByType("channel")
	if err != nil {
		return fmt.Errorf("warmup channels: %w", err)
	}
	for _, entry := range channelEntries {
		var channel discordgo.Channel
		if err := decodeEntity(entry.Data, &channel); err != nil {
			continue
		}
		// Store in-memory by channel.ID and populate indices via internal setter
		uc.setChannelInternal(channel.ID, &channel, entry.ExpiresAt)
		totalLoaded++
	}

	uc.lastWarmup = time.Now()
	return nil
}

// Internal setters for warmup (bypass LRU eviction during initial load)
func (uc *UnifiedCache) setMemberInternal(key string, member *discordgo.Member, expiresAt time.Time) {
	if key == "" || member == nil || uc.members == nil {
		return
	}
	uc.members.SetWithExpiration(key, member, expiresAt)
}

func (uc *UnifiedCache) setGuildInternal(key string, guild *discordgo.Guild, expiresAt time.Time) {
	if key == "" || guild == nil || uc.guilds == nil {
		return
	}
	uc.guilds.SetWithExpiration(key, guild, expiresAt)
}

func (uc *UnifiedCache) setRolesInternal(key string, roles []*discordgo.Role, expiresAt time.Time) {
	if key == "" || roles == nil || uc.roles == nil {
		return
	}
	uc.roles.SetWithExpiration(key, roles, expiresAt)
}

func (uc *UnifiedCache) setChannelInternal(key string, channel *discordgo.Channel, expiresAt time.Time) {
	if key == "" || channel == nil || uc.channels == nil {
		return
	}

	// Maintain indices
	if channel.GuildID != "" {
		uc.channelToGuildMu.Lock()
		if uc.channelToGuild == nil {
			uc.channelToGuild = make(map[string]string)
		}
		uc.channelToGuild[key] = channel.GuildID
		uc.channelToGuildMu.Unlock()

		uc.guildToChannelsMu.Lock()
		if uc.guildToChannels == nil {
			uc.guildToChannels = make(map[string]map[string]struct{})
		}
		set := uc.guildToChannels[channel.GuildID]
		if set == nil {
			set = make(map[string]struct{})
			uc.guildToChannels[channel.GuildID] = set
		}
		set[key] = struct{}{}
		uc.guildToChannelsMu.Unlock()
	} else {
		uc.channelToGuildMu.Lock()
		if uc.channelToGuild != nil {
			delete(uc.channelToGuild, key)
		}
		uc.channelToGuildMu.Unlock()
	}

	uc.channels.SetWithExpiration(key, channel, expiresAt)
}

// encodeEntity serializes a Discord entity to JSON
func encodeEntity(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// decodeEntity deserializes a Discord entity from JSON
func decodeEntity(data string, v interface{}) error {
	return json.Unmarshal([]byte(data), v)
}

// PersistAndStop gracefully shuts down the cache with persistence
func (uc *UnifiedCache) PersistAndStop() error {
	// Stop cleanup goroutine first
	uc.Stop()

	// Persist cache state if enabled
	if uc.persistEnabled {
		return uc.Persist()
	}
	return nil
}

// SetPersistInterval configures automatic persistence interval (0 disables auto-persist)
// SetPersistInterval starts a background ticker to periodically call Persist.
//
// Semantics:
// - interval <= 0 or missing store/persistence disables scheduling and returns nil.
// - Returns a channel; closing the returned channel stops the ticker goroutine.
// - The ticker is independent of cleanupLoop; both may run concurrently.
func (uc *UnifiedCache) SetPersistInterval(interval time.Duration) chan struct{} {
	if interval <= 0 || !uc.persistEnabled || uc.store == nil {
		return nil
	}

	stopChan := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := uc.Persist(); err != nil {
					// Log error but continue
				}
			case <-stopChan:
				return
			case <-uc.stopCleanup:
				return
			}
		}
	}()

	return stopChan
}
