package cache

import (
	"errors"
	"fmt"
	"time"
)

// CompositeCache aggregates multiple caches behind a single CacheManager (and GuildCache when possible).
// - Get: returns the first hit across children in order
// - Set/Delete/Clear/Cleanup: broadcasts to all children (aggregates errors)
// - Stats/Size/Keys: aggregated across children
// - Guild methods: applied to children that implement GuildCache, results aggregated
//
// Notes on Stats aggregation:
// - TotalEntries, MemoryUsage: summed
// - HitRate: averaged across children that report it (simple average). MissRate = 1 - HitRate
// - TTLEnabled: true if any child reports TTLEnabled
// - LastCleanup: latest across children
// - PerGuildStats: summed per guild across children
// - CustomMetrics: includes a "composite" flag and per-child metrics summary under "children"
type CompositeCache struct {
	name   string
	caches []CacheManager
}

// NewCompositeCache constructs a composite cache with the given name and child caches.
// The order of caches matters for Get() priority.
func NewCompositeCache(name string, caches ...CacheManager) *CompositeCache {
	return &CompositeCache{
		name:   name,
		caches: append([]CacheManager(nil), caches...),
	}
}

// AddCache appends a child cache (at lowest priority for Get()).
func (cc *CompositeCache) AddCache(c CacheManager) {
	cc.caches = append(cc.caches, c)
}

// ReplaceCaches replaces the current child caches.
func (cc *CompositeCache) ReplaceCaches(caches ...CacheManager) {
	cc.caches = append([]CacheManager(nil), caches...)
}

// Children returns a copy of the underlying child caches slice.
func (cc *CompositeCache) Children() []CacheManager {
	out := make([]CacheManager, len(cc.caches))
	copy(out, cc.caches)
	return out
}

// Get retrieves a value by searching child caches in order, returning the first hit.
func (cc *CompositeCache) Get(key string) (any, bool) {
	for _, c := range cc.caches {
		if v, ok := c.Get(key); ok {
			return v, true
		}
	}
	return nil, false
}

// Set broadcasts the value to all child caches.
// If any Set fails, returns an aggregated error (but attempts all).
func (cc *CompositeCache) Set(key string, value any, ttl time.Duration) error {
	var errs []error
	for _, c := range cc.caches {
		if err := c.Set(key, value, ttl); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors("composite set", errs...)
}

// Delete removes a key from all child caches.
// Returns aggregated error if any child fails.
func (cc *CompositeCache) Delete(key string) error {
	var errs []error
	for _, c := range cc.caches {
		if err := c.Delete(key); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors("composite delete", errs...)
}

// Has returns true if any child has the key (and it is not expired in TTL caches).
func (cc *CompositeCache) Has(key string) bool {
	for _, c := range cc.caches {
		if c.Has(key) {
			return true
		}
	}
	return false
}

// Stats returns aggregated stats across all child caches.
func (cc *CompositeCache) Stats() CacheStats {
	var totalEntries int
	var memory int64
	var hitRateSum float64
	var hitRateCount int
	var lastCleanup time.Time
	ttlEnabledAny := false
	perGuild := make(map[string]int)

	children := make([]map[string]any, 0, len(cc.caches))

	for idx, c := range cc.caches {
		s := c.Stats()

		totalEntries += s.TotalEntries
		memory += s.MemoryUsage
		if s.HitRate > 0 || s.MissRate > 0 {
			hitRateSum += s.HitRate
			hitRateCount++
		}
		if s.LastCleanup.After(lastCleanup) {
			lastCleanup = s.LastCleanup
		}
		if s.TTLEnabled {
			ttlEnabledAny = true
		}
		for gid, count := range s.PerGuildStats {
			perGuild[gid] += count
		}

		childMeta := map[string]any{
			"index":          idx,
			"total_entries":  s.TotalEntries,
			"memory_bytes":   s.MemoryUsage,
			"hit_rate":       s.HitRate,
			"ttl_enabled":    s.TTLEnabled,
			"last_cleanup":   s.LastCleanup,
			"custom_metrics": s.CustomMetrics,
		}
		children = append(children, childMeta)
	}

	avgHit := 0.0
	if hitRateCount > 0 {
		avgHit = hitRateSum / float64(hitRateCount)
	}
	avgMiss := 1 - avgHit

	custom := map[string]any{
		"composite": true,
		"name":      cc.name,
		"children":  children,
		"children_count": func() int {
			return len(cc.caches)
		}(),
	}

	return CacheStats{
		TotalEntries:  totalEntries,
		MemoryUsage:   memory,
		HitRate:       avgHit,
		MissRate:      avgMiss,
		LastCleanup:   lastCleanup,
		TTLEnabled:    ttlEnabledAny,
		PerGuildStats: perGuild,
		CustomMetrics: custom,
	}
}

// Cleanup triggers Cleanup on all child caches.
// Returns aggregated error if any child fails.
func (cc *CompositeCache) Cleanup() error {
	var errs []error
	for _, c := range cc.caches {
		if err := c.Cleanup(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors("composite cleanup", errs...)
}

// Clear clears all entries from all child caches.
// Returns aggregated error if any child fails.
func (cc *CompositeCache) Clear() error {
	var errs []error
	for _, c := range cc.caches {
		if err := c.Clear(); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors("composite clear", errs...)
}

// Size sums the number of entries across all child caches (best-effort).
func (cc *CompositeCache) Size() int {
	total := 0
	for _, c := range cc.caches {
		total += c.Size()
	}
	return total
}

// Keys returns the union of keys across all child caches (deduplicated).
// For large caches, this can be expensive.
func (cc *CompositeCache) Keys() []string {
	seen := make(map[string]struct{}, 1024)
	for _, c := range cc.caches {
		for _, k := range c.Keys() {
			seen[k] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// --- GuildCache passthrough (when children implement it) ---

// Ensure CompositeCache satisfies GuildCache when methods are used.
var _ GuildCache = (*CompositeCache)(nil)

// GetGuildKeys aggregates guild keys across children that implement GuildCache.
func (cc *CompositeCache) GetGuildKeys(guildID string) []string {
	seen := make(map[string]struct{}, 256)
	for _, c := range cc.caches {
		if gc, ok := c.(GuildCache); ok {
			for _, k := range gc.GetGuildKeys(guildID) {
				seen[k] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

// ClearGuild clears guild-scoped entries across children that implement GuildCache.
// Returns aggregated error if any child fails.
func (cc *CompositeCache) ClearGuild(guildID string) error {
	var errs []error
	for _, c := range cc.caches {
		if gc, ok := c.(GuildCache); ok {
			if err := gc.ClearGuild(guildID); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return joinErrors("composite clear_guild", errs...)
}

// GuildStats aggregates per-guild stats across children that implement GuildCache.
func (cc *CompositeCache) GuildStats(guildID string) CacheStats {
	total := 0
	memory := int64(0)
	lastCleanup := time.Time{}
	ttlAny := false

	for _, c := range cc.caches {
		if gc, ok := c.(GuildCache); ok {
			s := gc.GuildStats(guildID)
			total += s.TotalEntries
			memory += s.MemoryUsage
			if s.LastCleanup.After(lastCleanup) {
				lastCleanup = s.LastCleanup
			}
			if s.TTLEnabled {
				ttlAny = true
			}
		}
	}

	return CacheStats{
		TotalEntries:  total,
		MemoryUsage:   memory,
		HitRate:       0, // not tracked per guild
		MissRate:      0, // not tracked per guild
		LastCleanup:   lastCleanup,
		TTLEnabled:    ttlAny,
		PerGuildStats: map[string]int{guildID: total},
		CustomMetrics: map[string]any{
			"composite": true,
			"name":      cc.name + ".guild",
			"guild_id":  guildID,
		},
	}
}

// joinErrors aggregates multiple errors into a single error value.
func joinErrors(operation string, errs ...error) error {
	var filtered []error
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	if len(filtered) == 1 {
		return filtered[0]
	}

	msg := operation + " encountered multiple errors:"
	for i, e := range filtered {
		msg += fmt.Sprintf(" [%d] %v", i+1, e)
	}
	return errors.New(msg)
}
