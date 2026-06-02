package logging

import (
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

// rolesCacheStore is an in-memory, TTL-bounded cache of per-(guild,user) role
// ID lists used to avoid REST/DB lookups during member updates. Access is
// serialized by mu. The zero value is ready to use: the entry map is created
// lazily on first write. ttl is the default entry lifetime applied when a
// caller does not supply a per-guild override; a zero ttl falls back to five
// minutes at write time.
type rolesCacheStore struct {
	mu      sync.RWMutex
	entries map[string]cachedRoles
	ttl     time.Duration
}

// get returns a copy of the cached role IDs for (guildID,userID) when present
// and unexpired. Expired entries are deleted on read.
func (c *rolesCacheStore) get(guildID, userID string) ([]string, bool) {
	key := guildID + ":" + userID
	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return nil, false
	}
	return append([]string(nil), entry.roles...), true
}

// set stores roles for (guildID,userID) with the supplied ttl, copying the
// slice. An empty roles slice deletes the entry. A non-positive ttl falls back
// to five minutes.
func (c *rolesCacheStore) set(guildID, userID string, roles []string, ttl time.Duration) {
	key := guildID + ":" + userID
	if len(roles) == 0 {
		c.mu.Lock()
		delete(c.entries, key)
		c.mu.Unlock()
		return
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.mu.Lock()
	if c.entries == nil {
		c.entries = make(map[string]cachedRoles)
	}
	c.entries[key] = cachedRoles{
		roles:     append([]string(nil), roles...),
		expiresAt: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// evictExpired removes all entries whose TTL has elapsed.
func (c *rolesCacheStore) evictExpired() {
	now := time.Now()
	var toDelete []string

	c.mu.RLock()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			toDelete = append(toDelete, key)
		}
	}
	c.mu.RUnlock()

	if len(toDelete) > 0 {
		c.mu.Lock()
		for _, key := range toDelete {
			delete(c.entries, key)
		}
		c.mu.Unlock()
		log.ApplicationLogger().Info("Cleaned up expired roles cache entries", "count", len(toDelete))
	}
}

// size returns the current number of cached entries for status display.
func (c *rolesCacheStore) size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
