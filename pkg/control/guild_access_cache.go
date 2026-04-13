package control

import (
	"strings"
	"sync"
	"time"
)

type accessibleGuildCacheEntry struct {
	guilds    []cachedAccessibleGuild
	expiresAt time.Time
}

type cachedAccessibleGuild struct {
	guild      discordOAuthGuild
	botPresent bool
}

type accessibleGuildCache struct {
	mu      sync.RWMutex
	ttl     time.Duration
	now     func() time.Time
	entries map[string]accessibleGuildCacheEntry
}

func newAccessibleGuildCache(ttl time.Duration, now func() time.Time) *accessibleGuildCache {
	if now == nil {
		now = time.Now
	}

	return &accessibleGuildCache{
		ttl: ttl,
		now: func() time.Time {
			return now().UTC()
		},
		entries: make(map[string]accessibleGuildCacheEntry),
	}
}

func (cache *accessibleGuildCache) SetTTL(ttl time.Duration) {
	if cache == nil {
		return
	}

	cache.mu.Lock()
	cache.ttl = ttl
	cache.mu.Unlock()
}

func (cache *accessibleGuildCache) Get(sessionID string) ([]cachedAccessibleGuild, bool) {
	if cache == nil {
		return nil, false
	}

	cache.mu.RLock()
	ttl := cache.ttl
	now := cache.now
	cache.mu.RUnlock()
	if ttl <= 0 {
		return nil, false
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	cache.mu.RLock()
	entry, ok := cache.entries[sessionID]
	cache.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && !now().Before(entry.expiresAt) {
		cache.mu.Lock()
		delete(cache.entries, sessionID)
		cache.mu.Unlock()
		return nil, false
	}
	return cloneCachedAccessibleGuilds(entry.guilds), true
}

func (cache *accessibleGuildCache) Put(session discordOAuthSession, guilds []cachedAccessibleGuild) {
	if cache == nil {
		return
	}

	cache.mu.RLock()
	ttl := cache.ttl
	now := cache.now
	cache.mu.RUnlock()
	if ttl <= 0 {
		return
	}

	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return
	}

	expiresAt := now().Add(ttl)
	if !session.ExpiresAt.IsZero() && session.ExpiresAt.Before(expiresAt) {
		expiresAt = session.ExpiresAt
	}

	cache.mu.Lock()
	cache.entries[sessionID] = accessibleGuildCacheEntry{
		guilds:    cloneCachedAccessibleGuilds(guilds),
		expiresAt: expiresAt,
	}
	cache.mu.Unlock()
}

func (cache *accessibleGuildCache) InvalidateAll() {
	if cache == nil {
		return
	}

	cache.mu.Lock()
	cache.entries = make(map[string]accessibleGuildCacheEntry)
	cache.mu.Unlock()
}

func cloneCachedAccessibleGuilds(guilds []cachedAccessibleGuild) []cachedAccessibleGuild {
	if len(guilds) == 0 {
		return nil
	}

	out := make([]cachedAccessibleGuild, len(guilds))
	copy(out, guilds)
	return out
}
