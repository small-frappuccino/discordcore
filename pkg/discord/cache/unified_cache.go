package cache

import (
	"container/list"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// UnifiedCache provides an in-memory cache layer for frequently accessed Discord data
// to reduce API calls and improve performance. It includes TTL-based expiration, LRU eviction,
// and optional SQLite persistence.
type UnifiedCache struct {
	// Member cache: guildID:userID -> cachedMember
	members     map[string]*lruEntry
	membersList *list.List
	membersMu   sync.RWMutex

	// Guild cache: guildID -> cachedGuild
	guilds     map[string]*lruEntry
	guildsList *list.List
	guildsMu   sync.RWMutex

	// Roles cache: guildID -> cachedRoles
	roles     map[string]*lruEntry
	rolesList *list.List
	rolesMu   sync.RWMutex

	// Channel cache: channelID -> cachedChannel
	channels     map[string]*lruEntry
	channelsList *list.List
	channelsMu   sync.RWMutex

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

	// Metrics
	memberHits       uint64
	memberMisses     uint64
	memberEvictions  uint64
	guildHits        uint64
	guildMisses      uint64
	guildEvictions   uint64
	rolesHits        uint64
	rolesMisses      uint64
	rolesEvictions   uint64
	channelHits      uint64
	channelMisses    uint64
	channelEvictions uint64

	// SQLite persistence (optional)
	store          *storage.Store
	persistEnabled bool

	// Cleanup
	stopCleanup chan struct{}
	cleanupOnce sync.Once
}

// lruEntry wraps a cache entry with LRU list element
type lruEntry struct {
	key       string
	value     interface{}
	expiresAt time.Time
	element   *list.Element
}

// Cached value types
type cachedMember struct {
	member    *discordgo.Member
	expiresAt time.Time
}

type cachedGuild struct {
	guild     *discordgo.Guild
	expiresAt time.Time
}

type cachedRoles struct {
	roles     []*discordgo.Role
	expiresAt time.Time
}

type cachedChannel struct {
	channel   *discordgo.Channel
	expiresAt time.Time
}

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
		CleanupInterval: 2 * time.Minute,

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
		members:      make(map[string]*lruEntry),
		membersList:  list.New(),
		guilds:       make(map[string]*lruEntry),
		guildsList:   list.New(),
		roles:        make(map[string]*lruEntry),
		rolesList:    list.New(),
		channels:     make(map[string]*lruEntry),
		channelsList: list.New(),

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
	key := guildID + ":" + userID
	uc.membersMu.Lock()
	defer uc.membersMu.Unlock()

	entry, ok := uc.members[key]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			// Expired, remove it
			uc.membersList.Remove(entry.element)
			delete(uc.members, key)
		}
		atomic.AddUint64(&uc.memberMisses, 1)
		return nil, false
	}

	// Move to front (LRU)
	uc.membersList.MoveToFront(entry.element)

	atomic.AddUint64(&uc.memberHits, 1)
	cached := entry.value.(*cachedMember)
	return cached.member, true
}

// SetMember stores a member in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetMember(guildID, userID string, member *discordgo.Member) {
	if member == nil {
		return
	}
	key := guildID + ":" + userID
	cached := &cachedMember{
		member:    member,
		expiresAt: time.Now().Add(uc.memberTTL),
	}

	uc.membersMu.Lock()
	defer uc.membersMu.Unlock()

	// Update existing entry
	if entry, ok := uc.members[key]; ok {
		entry.value = cached
		entry.expiresAt = cached.expiresAt
		uc.membersList.MoveToFront(entry.element)
		return
	}

	// Check size limit and evict LRU if needed
	if uc.maxMemberSize > 0 && len(uc.members) >= uc.maxMemberSize {
		uc.evictMemberLRU()
	}

	// Add new entry
	element := uc.membersList.PushFront(key)
	uc.members[key] = &lruEntry{
		key:       key,
		value:     cached,
		expiresAt: cached.expiresAt,
		element:   element,
	}
}

// evictMemberLRU removes the least recently used member (must hold lock)
func (uc *UnifiedCache) evictMemberLRU() {
	element := uc.membersList.Back()
	if element != nil {
		key := element.Value.(string)
		uc.membersList.Remove(element)
		delete(uc.members, key)
		atomic.AddUint64(&uc.memberEvictions, 1)
	}
}

// InvalidateMember removes a member from the cache
func (uc *UnifiedCache) InvalidateMember(guildID, userID string) {
	key := guildID + ":" + userID
	uc.membersMu.Lock()
	if entry, ok := uc.members[key]; ok {
		uc.membersList.Remove(entry.element)
		delete(uc.members, key)
	}
	uc.membersMu.Unlock()
}

// GetGuild retrieves a cached guild or returns nil if not found/expired
func (uc *UnifiedCache) GetGuild(guildID string) (*discordgo.Guild, bool) {
	uc.guildsMu.Lock()
	defer uc.guildsMu.Unlock()

	entry, ok := uc.guilds[guildID]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			uc.guildsList.Remove(entry.element)
			delete(uc.guilds, guildID)
		}
		atomic.AddUint64(&uc.guildMisses, 1)
		return nil, false
	}

	// Move to front (LRU)
	uc.guildsList.MoveToFront(entry.element)

	atomic.AddUint64(&uc.guildHits, 1)
	cached := entry.value.(*cachedGuild)
	return cached.guild, true
}

// SetGuild stores a guild in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetGuild(guildID string, guild *discordgo.Guild) {
	if guild == nil {
		return
	}
	cached := &cachedGuild{
		guild:     guild,
		expiresAt: time.Now().Add(uc.guildTTL),
	}

	uc.guildsMu.Lock()
	defer uc.guildsMu.Unlock()

	// Update existing entry
	if entry, ok := uc.guilds[guildID]; ok {
		entry.value = cached
		entry.expiresAt = cached.expiresAt
		uc.guildsList.MoveToFront(entry.element)
		return
	}

	// Check size limit and evict LRU if needed
	if uc.maxGuildSize > 0 && len(uc.guilds) >= uc.maxGuildSize {
		uc.evictGuildLRU()
	}

	// Add new entry
	element := uc.guildsList.PushFront(guildID)
	uc.guilds[guildID] = &lruEntry{
		key:       guildID,
		value:     cached,
		expiresAt: cached.expiresAt,
		element:   element,
	}
}

// evictGuildLRU removes the least recently used guild (must hold lock)
func (uc *UnifiedCache) evictGuildLRU() {
	element := uc.guildsList.Back()
	if element != nil {
		key := element.Value.(string)
		uc.guildsList.Remove(element)
		delete(uc.guilds, key)
		atomic.AddUint64(&uc.guildEvictions, 1)
	}
}

// InvalidateGuild removes a guild from the cache
func (uc *UnifiedCache) InvalidateGuild(guildID string) {
	uc.guildsMu.Lock()
	if entry, ok := uc.guilds[guildID]; ok {
		uc.guildsList.Remove(entry.element)
		delete(uc.guilds, guildID)
	}
	uc.guildsMu.Unlock()
}

// GetRoles retrieves cached roles for a guild or returns nil if not found/expired
func (uc *UnifiedCache) GetRoles(guildID string) ([]*discordgo.Role, bool) {
	uc.rolesMu.Lock()
	defer uc.rolesMu.Unlock()

	entry, ok := uc.roles[guildID]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			uc.rolesList.Remove(entry.element)
			delete(uc.roles, guildID)
		}
		atomic.AddUint64(&uc.rolesMisses, 1)
		return nil, false
	}

	// Move to front (LRU)
	uc.rolesList.MoveToFront(entry.element)

	atomic.AddUint64(&uc.rolesHits, 1)
	cached := entry.value.(*cachedRoles)
	return cached.roles, true
}

// SetRoles stores guild roles in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetRoles(guildID string, roles []*discordgo.Role) {
	if roles == nil {
		return
	}
	cached := &cachedRoles{
		roles:     roles,
		expiresAt: time.Now().Add(uc.rolesTTL),
	}

	uc.rolesMu.Lock()
	defer uc.rolesMu.Unlock()

	// Update existing entry
	if entry, ok := uc.roles[guildID]; ok {
		entry.value = cached
		entry.expiresAt = cached.expiresAt
		uc.rolesList.MoveToFront(entry.element)
		return
	}

	// Check size limit and evict LRU if needed
	if uc.maxRolesSize > 0 && len(uc.roles) >= uc.maxRolesSize {
		uc.evictRolesLRU()
	}

	// Add new entry
	element := uc.rolesList.PushFront(guildID)
	uc.roles[guildID] = &lruEntry{
		key:       guildID,
		value:     cached,
		expiresAt: cached.expiresAt,
		element:   element,
	}
}

// evictRolesLRU removes the least recently used roles (must hold lock)
func (uc *UnifiedCache) evictRolesLRU() {
	element := uc.rolesList.Back()
	if element != nil {
		key := element.Value.(string)
		uc.rolesList.Remove(element)
		delete(uc.roles, key)
		atomic.AddUint64(&uc.rolesEvictions, 1)
	}
}

// InvalidateRoles removes guild roles from the cache
func (uc *UnifiedCache) InvalidateRoles(guildID string) {
	uc.rolesMu.Lock()
	if entry, ok := uc.roles[guildID]; ok {
		uc.rolesList.Remove(entry.element)
		delete(uc.roles, guildID)
	}
	uc.rolesMu.Unlock()
}

// GetChannel retrieves a cached channel or returns nil if not found/expired
func (uc *UnifiedCache) GetChannel(channelID string) (*discordgo.Channel, bool) {
	uc.channelsMu.Lock()
	defer uc.channelsMu.Unlock()

	entry, ok := uc.channels[channelID]
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			uc.channelsList.Remove(entry.element)
			delete(uc.channels, channelID)
		}
		atomic.AddUint64(&uc.channelMisses, 1)
		return nil, false
	}

	// Move to front (LRU)
	uc.channelsList.MoveToFront(entry.element)

	atomic.AddUint64(&uc.channelHits, 1)
	cached := entry.value.(*cachedChannel)
	return cached.channel, true
}

// SetChannel stores a channel in the cache with TTL and LRU eviction
func (uc *UnifiedCache) SetChannel(channelID string, channel *discordgo.Channel) {
	if channel == nil {
		return
	}
	cached := &cachedChannel{
		channel:   channel,
		expiresAt: time.Now().Add(uc.channelTTL),
	}

	uc.channelsMu.Lock()
	defer uc.channelsMu.Unlock()

	// Update existing entry
	if entry, ok := uc.channels[channelID]; ok {
		entry.value = cached
		entry.expiresAt = cached.expiresAt
		uc.channelsList.MoveToFront(entry.element)
		return
	}

	// Check size limit and evict LRU if needed
	if uc.maxChannelSize > 0 && len(uc.channels) >= uc.maxChannelSize {
		uc.evictChannelLRU()
	}

	// Add new entry
	element := uc.channelsList.PushFront(channelID)
	uc.channels[channelID] = &lruEntry{
		key:       channelID,
		value:     cached,
		expiresAt: cached.expiresAt,
		element:   element,
	}
}

// evictChannelLRU removes the least recently used channel (must hold lock)
func (uc *UnifiedCache) evictChannelLRU() {
	element := uc.channelsList.Back()
	if element != nil {
		key := element.Value.(string)
		uc.channelsList.Remove(element)
		delete(uc.channels, key)
		atomic.AddUint64(&uc.channelEvictions, 1)
	}
}

// InvalidateChannel removes a channel from the cache
func (uc *UnifiedCache) InvalidateChannel(channelID string) {
	uc.channelsMu.Lock()
	if entry, ok := uc.channels[channelID]; ok {
		uc.channelsList.Remove(entry.element)
		delete(uc.channels, channelID)
	}
	uc.channelsMu.Unlock()
}

// GetStats returns cache statistics
func (uc *UnifiedCache) GetStats() CacheStats {
	uc.membersMu.RLock()
	memberCount := len(uc.members)
	uc.membersMu.RUnlock()

	uc.guildsMu.RLock()
	guildCount := len(uc.guilds)
	uc.guildsMu.RUnlock()

	uc.rolesMu.RLock()
	rolesCount := len(uc.roles)
	uc.rolesMu.RUnlock()

	uc.channelsMu.RLock()
	channelCount := len(uc.channels)
	uc.channelsMu.RUnlock()

	return CacheStats{
		MemberEntries:    memberCount,
		GuildEntries:     guildCount,
		RolesEntries:     rolesCount,
		ChannelEntries:   channelCount,
		MemberHits:       atomic.LoadUint64(&uc.memberHits),
		MemberMisses:     atomic.LoadUint64(&uc.memberMisses),
		MemberEvictions:  atomic.LoadUint64(&uc.memberEvictions),
		GuildHits:        atomic.LoadUint64(&uc.guildHits),
		GuildMisses:      atomic.LoadUint64(&uc.guildMisses),
		GuildEvictions:   atomic.LoadUint64(&uc.guildEvictions),
		RolesHits:        atomic.LoadUint64(&uc.rolesHits),
		RolesMisses:      atomic.LoadUint64(&uc.rolesMisses),
		RolesEvictions:   atomic.LoadUint64(&uc.rolesEvictions),
		ChannelHits:      atomic.LoadUint64(&uc.channelHits),
		ChannelMisses:    atomic.LoadUint64(&uc.channelMisses),
		ChannelEvictions: atomic.LoadUint64(&uc.channelEvictions),
	}
}

// CacheStats holds cache statistics
type CacheStats struct {
	MemberEntries    int    `json:"member_entries"`
	GuildEntries     int    `json:"guild_entries"`
	RolesEntries     int    `json:"roles_entries"`
	ChannelEntries   int    `json:"channel_entries"`
	MemberHits       uint64 `json:"member_hits"`
	MemberMisses     uint64 `json:"member_misses"`
	MemberEvictions  uint64 `json:"member_evictions"`
	GuildHits        uint64 `json:"guild_hits"`
	GuildMisses      uint64 `json:"guild_misses"`
	GuildEvictions   uint64 `json:"guild_evictions"`
	RolesHits        uint64 `json:"roles_hits"`
	RolesMisses      uint64 `json:"roles_misses"`
	RolesEvictions   uint64 `json:"roles_evictions"`
	ChannelHits      uint64 `json:"channel_hits"`
	ChannelMisses    uint64 `json:"channel_misses"`
	ChannelEvictions uint64 `json:"channel_evictions"`
}

// Clear removes all entries from the cache
func (uc *UnifiedCache) Clear() {
	uc.membersMu.Lock()
	uc.members = make(map[string]*lruEntry)
	uc.membersList = list.New()
	uc.membersMu.Unlock()

	uc.guildsMu.Lock()
	uc.guilds = make(map[string]*lruEntry)
	uc.guildsList = list.New()
	uc.guildsMu.Unlock()

	uc.rolesMu.Lock()
	uc.roles = make(map[string]*lruEntry)
	uc.rolesList = list.New()
	uc.rolesMu.Unlock()

	uc.channelsMu.Lock()
	uc.channels = make(map[string]*lruEntry)
	uc.channelsList = list.New()
	uc.channelsMu.Unlock()
}

// Stop stops the background cleanup goroutine
func (uc *UnifiedCache) Stop() {
	uc.cleanupOnce.Do(func() {
		close(uc.stopCleanup)
	})
}

// cleanupLoop periodically removes expired entries
func (uc *UnifiedCache) cleanupLoop(interval time.Duration) {
	if interval <= 0 {
		interval = 2 * time.Minute
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

	// Cleanup members
	uc.membersMu.Lock()
	for key, entry := range uc.members {
		if now.After(entry.expiresAt) {
			uc.membersList.Remove(entry.element)
			delete(uc.members, key)
		}
	}
	uc.membersMu.Unlock()

	// Cleanup guilds
	uc.guildsMu.Lock()
	for key, entry := range uc.guilds {
		if now.After(entry.expiresAt) {
			uc.guildsList.Remove(entry.element)
			delete(uc.guilds, key)
		}
	}
	uc.guildsMu.Unlock()

	// Cleanup roles
	uc.rolesMu.Lock()
	for key, entry := range uc.roles {
		if now.After(entry.expiresAt) {
			uc.rolesList.Remove(entry.element)
			delete(uc.roles, key)
		}
	}
	uc.rolesMu.Unlock()

	// Cleanup channels
	uc.channelsMu.Lock()
	for key, entry := range uc.channels {
		if now.After(entry.expiresAt) {
			uc.channelsList.Remove(entry.element)
			delete(uc.channels, key)
		}
	}
	uc.channelsMu.Unlock()
}

// Persist saves current cache state to SQLite (if enabled)
func (uc *UnifiedCache) Persist() error {
	if !uc.persistEnabled || uc.store == nil {
		return nil
	}

	var errs []error

	// Persist members
	uc.membersMu.RLock()
	for key, entry := range uc.members {
		if cached, ok := entry.value.(*cachedMember); ok {
			data, err := encodeEntity(cached.member)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode member %s: %w", key, err))
				continue
			}
			if err := uc.store.UpsertCacheEntry(key, "member", data, entry.expiresAt); err != nil {
				errs = append(errs, fmt.Errorf("persist member %s: %w", key, err))
			}
		}
	}
	uc.membersMu.RUnlock()

	// Persist guilds
	uc.guildsMu.RLock()
	for key, entry := range uc.guilds {
		if cached, ok := entry.value.(*cachedGuild); ok {
			data, err := encodeEntity(cached.guild)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode guild %s: %w", key, err))
				continue
			}
			if err := uc.store.UpsertCacheEntry(key, "guild", data, entry.expiresAt); err != nil {
				errs = append(errs, fmt.Errorf("persist guild %s: %w", key, err))
			}
		}
	}
	uc.guildsMu.RUnlock()

	// Persist roles
	uc.rolesMu.RLock()
	for key, entry := range uc.roles {
		if cached, ok := entry.value.(*cachedRoles); ok {
			data, err := encodeEntity(cached.roles)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode roles %s: %w", key, err))
				continue
			}
			if err := uc.store.UpsertCacheEntry(key, "roles", data, entry.expiresAt); err != nil {
				errs = append(errs, fmt.Errorf("persist roles %s: %w", key, err))
			}
		}
	}
	uc.rolesMu.RUnlock()

	// Persist channels
	uc.channelsMu.RLock()
	for key, entry := range uc.channels {
		if cached, ok := entry.value.(*cachedChannel); ok {
			data, err := encodeEntity(cached.channel)
			if err != nil {
				errs = append(errs, fmt.Errorf("encode channel %s: %w", key, err))
				continue
			}
			if err := uc.store.UpsertCacheEntry(key, "channel", data, entry.expiresAt); err != nil {
				errs = append(errs, fmt.Errorf("persist channel %s: %w", key, err))
			}
		}
	}
	uc.channelsMu.RUnlock()

	if len(errs) > 0 {
		return fmt.Errorf("persist cache: %d errors occurred", len(errs))
	}
	return nil
}

// Warmup pre-populates the cache from SQLite (if enabled)
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
		uc.setChannelInternal(entry.Key, &channel, entry.ExpiresAt)
		totalLoaded++
	}

	return nil
}

// Internal setters for warmup (bypass LRU eviction during initial load)
func (uc *UnifiedCache) setMemberInternal(key string, member *discordgo.Member, expiresAt time.Time) {
	cached := &cachedMember{
		member:    member,
		expiresAt: expiresAt,
	}
	uc.membersMu.Lock()
	defer uc.membersMu.Unlock()

	if _, ok := uc.members[key]; !ok {
		element := uc.membersList.PushFront(key)
		uc.members[key] = &lruEntry{
			key:       key,
			value:     cached,
			expiresAt: expiresAt,
			element:   element,
		}
	}
}

func (uc *UnifiedCache) setGuildInternal(key string, guild *discordgo.Guild, expiresAt time.Time) {
	cached := &cachedGuild{
		guild:     guild,
		expiresAt: expiresAt,
	}
	uc.guildsMu.Lock()
	defer uc.guildsMu.Unlock()

	if _, ok := uc.guilds[key]; !ok {
		element := uc.guildsList.PushFront(key)
		uc.guilds[key] = &lruEntry{
			key:       key,
			value:     cached,
			expiresAt: expiresAt,
			element:   element,
		}
	}
}

func (uc *UnifiedCache) setRolesInternal(key string, roles []*discordgo.Role, expiresAt time.Time) {
	cached := &cachedRoles{
		roles:     roles,
		expiresAt: expiresAt,
	}
	uc.rolesMu.Lock()
	defer uc.rolesMu.Unlock()

	if _, ok := uc.roles[key]; !ok {
		element := uc.rolesList.PushFront(key)
		uc.roles[key] = &lruEntry{
			key:       key,
			value:     cached,
			expiresAt: expiresAt,
			element:   element,
		}
	}
}

func (uc *UnifiedCache) setChannelInternal(key string, channel *discordgo.Channel, expiresAt time.Time) {
	cached := &cachedChannel{
		channel:   channel,
		expiresAt: expiresAt,
	}
	uc.channelsMu.Lock()
	defer uc.channelsMu.Unlock()

	if _, ok := uc.channels[key]; !ok {
		element := uc.channelsList.PushFront(key)
		uc.channels[key] = &lruEntry{
			key:       key,
			value:     cached,
			expiresAt: expiresAt,
			element:   element,
		}
	}
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
