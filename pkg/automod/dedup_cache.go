package automod

import (
	"sync"
	"time"
)

// automodFallbackDedupTTL mirrors the router-level IdempotencyTTL configured in
// EnqueueAutomodAction. The fallback map only kicks in when the router-backed
// adapter is unavailable or has failed, so dedup behavior stays consistent
// across the normal and fallback paths.
const automodFallbackDedupTTL = 10 * time.Second

// automodFallbackDedupCleanupThreshold caps the in-process fallback map size
// before lazy cleanup runs.
const automodFallbackDedupCleanupThreshold = 64

// FallbackDedupCache manages a time-based deduplication cache used by the Automod
// synchronous fallback path when the router is unavailable or fails. It drops
// duplicate raw events based on an idempotency key.
type FallbackDedupCache struct {
	mu    sync.Mutex
	cache map[string]time.Time
}

// NewFallbackDedupCache constructs a new deduplication cache.
func NewFallbackDedupCache() *FallbackDedupCache {
	return &FallbackDedupCache{
		cache: make(map[string]time.Time),
	}
}

// ShouldDedup reports whether key was seen within automodFallbackDedupTTL.
// Empty keys never dedup (no stable identifier available).
func (c *FallbackDedupCache) ShouldDedup(key string, isRunning bool) bool {
	return c.ShouldDedupAt(key, time.Now(), isRunning)
}

// ShouldDedupAt performs the dedup check against a specific time.
func (c *FallbackDedupCache) ShouldDedupAt(key string, now time.Time, isRunning bool) bool {
	if !isRunning || key == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		c.cache = make(map[string]time.Time)
	}

	if len(c.cache) > automodFallbackDedupCleanupThreshold {
		for k, expiry := range c.cache {
			if now.After(expiry) {
				delete(c.cache, k)
			}
		}
	}

	if expiry, exists := c.cache[key]; exists && now.Before(expiry) {
		return true
	}

	c.cache[key] = now.Add(automodFallbackDedupTTL)
	return false
}
