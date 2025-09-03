package cache

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/alice-bnuy/discordcore/v2/internal/files"
	"github.com/alice-bnuy/discordcore/v2/internal/util"
)

// AvatarCacheManager is a persistent, guild-aware cache for user avatar hashes.
// It implements CacheManager, PersistentCache, and GuildCache.
//
// Key format for the generic CacheManager methods is "guildID:userID".
type AvatarCacheManager struct {
	mu          sync.RWMutex
	saveMu      sync.Mutex
	path        string
	version     string
	lastUpdated time.Time
	lastCleanup time.Time
	lastSave    time.Time
	hits        uint64
	misses      uint64
	guilds      map[string]*avatarCache // guildID -> avatarCache
	jsonManager *util.JSONManager
}

// Internal persistence structures (compatible with previous format under files.AvatarMultiGuildCache)
type avatarCache struct {
	Avatars     map[string]string `json:"avatars"`
	LastUpdated time.Time         `json:"last_updated"`
	GuildID     string            `json:"guild_id"`
}

type avatarMultiGuildCache struct {
	Guilds      map[string]*avatarCache `json:"guilds"`
	LastUpdated time.Time               `json:"last_updated"`
	Version     string                  `json:"version"`
}

// NewAvatarCacheManager creates a new avatar cache manager with the provided path.
func NewAvatarCacheManager(path string) *AvatarCacheManager {
	if path == "" {
		path = files.GetApplicationCacheFilePath()
	}
	return &AvatarCacheManager{
		path:        path,
		version:     "2.0",
		guilds:      make(map[string]*avatarCache),
		jsonManager: util.NewJSONManager(path),
	}
}

// NewDefaultAvatarCacheManager creates a new avatar cache manager using the standard application cache path.
func NewDefaultAvatarCacheManager() *AvatarCacheManager {
	return NewAvatarCacheManager(files.GetApplicationCacheFilePath())
}

// --------------- CacheManager implementation ---------------

// Get retrieves the avatar hash using a composite key "guildID:userID".
func (m *AvatarCacheManager) Get(key string) (any, bool) {
	guildID, userID := parseKey(key)
	if guildID == "" || userID == "" {
		m.incrementMiss()
		return nil, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	g, ok := m.guilds[guildID]
	if !ok || g == nil {
		m.incrementMiss()
		return nil, false
	}

	val, ok := g.Avatars[userID]
	if ok {
		m.incrementHit()
		return val, true
	}
	m.incrementMiss()
	return nil, false
}

// Set stores an avatar hash using a composite key "guildID:userID".
// TTL is ignored for this persistent cache.
func (m *AvatarCacheManager) Set(key string, value any, _ time.Duration) error {
	guildID, userID := parseKey(key)
	if guildID == "" || userID == "" {
		return NewCacheError("set", key, fmt.Errorf("invalid key; expected 'guildID:userID'"))
	}

	strVal, ok := value.(string)
	if !ok {
		return NewCacheError("set", key, fmt.Errorf("value must be a string (avatar hash)"))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	g := m.ensureGuildLocked(guildID)
	g.Avatars[userID] = strVal
	g.LastUpdated = time.Now()
	m.lastUpdated = g.LastUpdated

	return nil
}

// Delete removes a single entry by composite key "guildID:userID".
func (m *AvatarCacheManager) Delete(key string) error {
	guildID, userID := parseKey(key)
	if guildID == "" || userID == "" {
		return NewCacheError("delete", key, fmt.Errorf("invalid key; expected 'guildID:userID'"))
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if g, ok := m.guilds[guildID]; ok && g != nil {
		delete(g.Avatars, userID)
		g.LastUpdated = time.Now()
		m.lastUpdated = g.LastUpdated
	}
	return nil
}

// Has checks if a composite key "guildID:userID" exists.
func (m *AvatarCacheManager) Has(key string) bool {
	guildID, userID := parseKey(key)
	if guildID == "" || userID == "" {
		return false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	g, ok := m.guilds[guildID]
	if !ok || g == nil {
		return false
	}
	_, ok = g.Avatars[userID]
	return ok
}

// Stats returns aggregate cache statistics.
func (m *AvatarCacheManager) Stats() CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var total int
	perGuild := make(map[string]int, len(m.guilds))

	for gid, g := range m.guilds {
		count := len(g.Avatars)
		perGuild[gid] = count
		total += count
	}

	totalAccess := m.hits + m.misses
	hitRate := 0.0
	if totalAccess > 0 {
		hitRate = float64(m.hits) / float64(totalAccess)
	}
	memory := m.estimateMemoryLocked()

	return CacheStats{
		TotalEntries:  total,
		MemoryUsage:   memory,
		HitRate:       hitRate,
		MissRate:      1 - hitRate,
		LastCleanup:   m.lastCleanup,
		TTLEnabled:    false,
		PerGuildStats: perGuild,
		CustomMetrics: map[string]any{
			"version":      m.version,
			"last_updated": m.lastUpdated,
			"path":         m.path,
		},
	}
}

// Cleanup is a no-op for this cache (no TTL). It records the cleanup time.
func (m *AvatarCacheManager) Cleanup() error {
	m.mu.Lock()
	m.lastCleanup = time.Now()
	m.mu.Unlock()
	return nil
}

// Clear removes all entries from all guilds.
func (m *AvatarCacheManager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.guilds = make(map[string]*avatarCache)
	m.lastUpdated = time.Now()
	return nil
}

// Size returns the total number of cached entries across all guilds.
func (m *AvatarCacheManager) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, g := range m.guilds {
		if g != nil {
			total += len(g.Avatars)
		}
	}
	return total
}

// Keys returns all composite keys "guildID:userID".
func (m *AvatarCacheManager) Keys() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]string, 0, m.Size())
	for gid, g := range m.guilds {
		for uid := range g.Avatars {
			out = append(out, makeKey(gid, uid))
		}
	}
	return out
}

// --------------- PersistentCache implementation ---------------

// Save persists the cache to disk in JSON format (compatible with previous structure).
func (m *AvatarCacheManager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	payload := avatarMultiGuildCache{
		Guilds:      m.guilds,
		LastUpdated: time.Now(),
		Version:     m.version,
	}
	if err := m.ensureParentDir(); err != nil {
		return NewCacheError("save", "", err)
	}
	if err := m.jsonManager.Save(payload); err != nil {
		return NewCacheError("save", "", err)
	}
	return nil
}

// Load loads the cache from disk. If the file does not exist, this is a no-op.
func (m *AvatarCacheManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try to read the file
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			// initialize empty
			m.guilds = make(map[string]*avatarCache)
			return nil
		}
		return NewCacheError("load", "", err)
	}

	// Attempt to parse as multi-guild payload
	var multi avatarMultiGuildCache
	if err := json.Unmarshal(data, &multi); err == nil && multi.Guilds != nil {
		m.guilds = multi.Guilds
		m.version = coalesce(multi.Version, "2.0")
		m.lastUpdated = multi.LastUpdated
		return nil
	}

	// Fallback: attempt to parse as single-guild legacy structure (not typical, but safe)
	var legacy avatarCache
	if err := json.Unmarshal(data, &legacy); err == nil && len(legacy.Avatars) > 0 && legacy.GuildID != "" {
		m.guilds = map[string]*avatarCache{
			legacy.GuildID: &legacy,
		}
		m.version = "2.0"
		m.lastUpdated = time.Now()
		return nil
	}

	// If both fail, reinitialize (avoid corrupt state)
	m.guilds = make(map[string]*avatarCache)
	m.version = "2.0"
	m.lastUpdated = time.Now()
	return nil
}

// SaveThrottled saves the cache if the last save was older than minInterval.
func (m *AvatarCacheManager) SaveThrottled(minInterval time.Duration) error {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	if time.Since(m.lastSave) < minInterval {
		return nil
	}
	if err := m.Save(); err != nil {
		return err
	}
	m.lastSave = time.Now()
	return nil
}

// --------------- GuildCache implementation ---------------

// GetGuildKeys returns composite keys "guildID:userID" for a specific guild.
func (m *AvatarCacheManager) GetGuildKeys(guildID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	g, ok := m.guilds[guildID]
	if !ok || g == nil {
		return []string{}
	}
	out := make([]string, 0, len(g.Avatars))
	for uid := range g.Avatars {
		out = append(out, makeKey(guildID, uid))
	}
	return out
}

// ClearGuild removes all entries for a specific guild.
func (m *AvatarCacheManager) ClearGuild(guildID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.guilds, guildID)
	m.lastUpdated = time.Now()
	return nil
}

// GuildStats returns statistics scoped to a single guild.
func (m *AvatarCacheManager) GuildStats(guildID string) CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	g, ok := m.guilds[guildID]
	count := 0
	if ok && g != nil {
		count = len(g.Avatars)
	}

	return CacheStats{
		TotalEntries:  count,
		MemoryUsage:   m.estimateGuildMemoryLocked(guildID),
		HitRate:       0, // not tracked per guild
		MissRate:      0, // not tracked per guild
		LastCleanup:   m.lastCleanup,
		TTLEnabled:    false,
		PerGuildStats: map[string]int{guildID: count},
		CustomMetrics: map[string]any{
			"version":      m.version,
			"last_updated": m.lastUpdated,
			"path":         m.path,
		},
	}
}

// --------------- Convenience helpers (non-interface) ---------------

// UpdateAvatar sets the avatar hash for a user within a guild.
func (m *AvatarCacheManager) UpdateAvatar(guildID, userID, avatarHash string) {
	_ = m.Set(makeKey(guildID, userID), avatarHash, 0)
}

// AvatarHash retrieves the avatar hash for a user within a guild.
func (m *AvatarCacheManager) AvatarHash(guildID, userID string) string {
	if v, ok := m.Get(makeKey(guildID, userID)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// AvatarChanged checks if the current hash differs from the cached one.
func (m *AvatarCacheManager) AvatarChanged(guildID, userID, currentAvatarHash string) bool {
	return m.AvatarHash(guildID, userID) != currentAvatarHash
}

// Path returns the cache file path being used.
func (m *AvatarCacheManager) Path() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.path
}

// SetPath updates the cache file path (does not save automatically).
func (m *AvatarCacheManager) SetPath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if path == "" {
		return
	}
	m.path = path
	m.jsonManager = util.NewJSONManager(path)
}

// --------------- Internal helpers ---------------

func (m *AvatarCacheManager) ensureGuildLocked(guildID string) *avatarCache {
	if g, ok := m.guilds[guildID]; ok && g != nil {
		return g
	}
	g := &avatarCache{
		Avatars:     make(map[string]string),
		LastUpdated: time.Now(),
		GuildID:     guildID,
	}
	m.guilds[guildID] = g
	return g
}

func (m *AvatarCacheManager) ensureParentDir() error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	return nil
}

func (m *AvatarCacheManager) incrementHit() {
	// Not locking for performance; eventual consistency is OK for stats
	m.hits++
}

func (m *AvatarCacheManager) incrementMiss() {
	// Not locking for performance; eventual consistency is OK for stats
	m.misses++
}

func (m *AvatarCacheManager) estimateMemoryLocked() int64 {
	var total int64
	for gid, g := range m.guilds {
		total += int64(len(gid))
		for uid, hash := range g.Avatars {
			// very rough estimate: key + value + small overhead
			total += int64(len(uid) + len(hash) + 32)
		}
	}
	return total
}

func (m *AvatarCacheManager) estimateGuildMemoryLocked(guildID string) int64 {
	g, ok := m.guilds[guildID]
	if !ok || g == nil {
		return 0
	}
	var total int64 = int64(len(guildID))
	for uid, hash := range g.Avatars {
		total += int64(len(uid) + len(hash) + 32)
	}
	return total
}

func makeKey(guildID, userID string) string {
	return guildID + ":" + userID
}

func parseKey(key string) (guildID, userID string) {
	i := strings.IndexByte(key, ':')
	if i <= 0 || i >= len(key)-1 {
		return "", ""
	}
	return key[:i], key[i+1:]
}

func coalesce(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
