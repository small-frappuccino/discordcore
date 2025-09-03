package cache

import (
	"sync"
	"time"
)

// TTLMap is a concurrent in-memory cache with per-key TTL that satisfies TTLCache.
// - Safe for concurrent use.
// - Optional default TTL applied when Set is called with ttl <= 0.
// - Periodic cleanup to remove expired entries.
// - Provides basic hit/miss stats and rough memory estimation.
type TTLMap struct {
	mu              sync.RWMutex
	data            map[string]*ttlEntry
	defaultTTL      time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time

	// stats
	hits   uint64
	misses uint64

	// control
	stopOnce sync.Once
	stopCh   chan struct{}

	// optional metadata
	name    string
	maxSize int
}

type ttlEntry struct {
	value       any
	expiresAt   time.Time
	hasExpiry   bool
	keyOverhead int // rough size estimate for memory usage
}

// NewTTLMap creates a new TTLMap.
// - name is a human-friendly identifier exposed via Stats().CustomMetrics["name"]
// - defaultTTL is used when Set(key, value, ttl) is called with ttl <= 0
// - cleanupInterval defines how often expired items are purged. If 0, no background cleanup goroutine runs.
// - maxSize when > 0, upper-bounds the number of entries (soft cap: when exceeded, cleanup is triggered; if still exceeded new sets are allowed)
func NewTTLMap(name string, defaultTTL, cleanupInterval time.Duration, maxSize int) *TTLMap {
	m := &TTLMap{
		data:            make(map[string]*ttlEntry),
		defaultTTL:      defaultTTL,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
		name:            name,
		maxSize:         maxSize,
	}

	if cleanupInterval > 0 {
		go m.cleanupLoop()
	}

	return m
}

// Close stops the background cleanup goroutine, if any.
func (m *TTLMap) Close() {
	m.stopOnce.Do(func() {
		close(m.stopCh)
	})
}

// Get retrieves a value by key. If the key is expired or not present, returns (nil, false).
func (m *TTLMap) Get(key string) (any, bool) {
	now := time.Now()

	m.mu.RLock()
	entry, ok := m.data[key]
	if ok && entry != nil && (!entry.hasExpiry || now.Before(entry.expiresAt)) {
		// hit
		m.mu.RUnlock()
		m.incrementHit()
		return entry.value, true
	}
	m.mu.RUnlock()

	// miss or expired
	m.incrementMiss()

	if ok {
		// remove expired lazily
		m.mu.Lock()
		if cur, exists := m.data[key]; exists && cur != nil && cur.hasExpiry && !now.Before(cur.expiresAt) {
			delete(m.data, key)
		}
		m.mu.Unlock()
	}

	return nil, false
}

// Set stores a value by key with the provided TTL.
// - ttl <= 0 applies the default TTL.
// - ttl < 0 and defaultTTL <= 0 results in no expiration.
func (m *TTLMap) Set(key string, value any, ttl time.Duration) error {
	var expiresAt time.Time
	var hasExpiry bool

	if ttl <= 0 {
		ttl = m.defaultTTL
	}
	if ttl > 0 {
		hasExpiry = true
		expiresAt = time.Now().Add(ttl)
	}

	entry := &ttlEntry{
		value:       value,
		expiresAt:   expiresAt,
		hasExpiry:   hasExpiry,
		keyOverhead: roughKeyOverhead(key),
	}

	m.mu.Lock()
	m.data[key] = entry
	// Soft check for max size: try to cleanup if exceeded
	if m.maxSize > 0 && len(m.data) > m.maxSize {
		m.cleanupExpiredLocked(time.Now())
	}
	m.mu.Unlock()

	return nil
}

// Delete removes a key, returning no error for non-existent keys.
func (m *TTLMap) Delete(key string) error {
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
	return nil
}

// Has checks whether a key exists and is not expired. Does not update hit/miss counters.
func (m *TTLMap) Has(key string) bool {
	now := time.Now()

	m.mu.RLock()
	entry, ok := m.data[key]
	m.mu.RUnlock()

	if !ok || entry == nil {
		return false
	}
	if entry.hasExpiry && !now.Before(entry.expiresAt) {
		// cleanup lazily
		m.mu.Lock()
		if cur, exists := m.data[key]; exists && cur != nil && cur.hasExpiry && !now.Before(cur.expiresAt) {
			delete(m.data, key)
		}
		m.mu.Unlock()
		return false
	}
	return true
}

// Stats returns cache statistics.
// Note: MemoryUsage is a rough estimate based on key size + per-entry overhead.
func (m *TTLMap) Stats() CacheStats {
	now := time.Now()
	var totalEntries int
	var memory int64

	m.mu.RLock()
	for k, v := range m.data {
		if v == nil {
			continue
		}
		if v.hasExpiry && !now.Before(v.expiresAt) {
			continue
		}
		totalEntries++
		// rough estimate: key bytes + small fixed overhead
		memory += int64(v.keyOverhead + 64)
	}
	hits := m.hits
	misses := m.misses
	lastCleanup := m.lastCleanup
	m.mu.RUnlock()

	totalAccess := hits + misses
	hitRate := 0.0
	if totalAccess > 0 {
		hitRate = float64(hits) / float64(totalAccess)
	}

	return CacheStats{
		TotalEntries:  totalEntries,
		MemoryUsage:   memory,
		HitRate:       hitRate,
		MissRate:      1 - hitRate,
		LastCleanup:   lastCleanup,
		TTLEnabled:    true,
		PerGuildStats: nil,
		CustomMetrics: map[string]any{
			"name":              m.name,
			"default_ttl_ms":    m.defaultTTL.Milliseconds(),
			"cleanup_interval":  m.cleanupInterval.String(),
			"max_size":          m.maxSize,
			"total_accesses":    totalAccess,
			"implementation":    "TTLMap",
			"entries_raw_count": len(m.data),
		},
	}
}

// Cleanup removes expired entries immediately.
func (m *TTLMap) Cleanup() error {
	m.mu.Lock()
	m.cleanupExpiredLocked(time.Now())
	m.lastCleanup = time.Now()
	m.mu.Unlock()
	return nil
}

// Clear removes all entries from the cache.
func (m *TTLMap) Clear() error {
	m.mu.Lock()
	m.data = make(map[string]*ttlEntry)
	m.mu.Unlock()
	return nil
}

// Size returns the number of non-expired entries.
func (m *TTLMap) Size() int {
	now := time.Now()
	count := 0

	m.mu.RLock()
	for _, v := range m.data {
		if v == nil {
			continue
		}
		if v.hasExpiry && !now.Before(v.expiresAt) {
			continue
		}
		count++
	}
	m.mu.RUnlock()

	return count
}

// Keys returns all keys (non-expired) at the time of calling.
func (m *TTLMap) Keys() []string {
	now := time.Now()

	m.mu.RLock()
	keys := make([]string, 0, len(m.data))
	for k, v := range m.data {
		if v == nil {
			continue
		}
		if v.hasExpiry && !now.Before(v.expiresAt) {
			continue
		}
		keys = append(keys, k)
	}
	m.mu.RUnlock()

	return keys
}

// SetTTL updates the TTL for an existing key. A ttl <= 0 removes expiration.
func (m *TTLMap) SetTTL(key string, ttl time.Duration) error {
	var expiresAt time.Time
	var hasExpiry bool

	if ttl <= 0 {
		hasExpiry = false
	} else {
		hasExpiry = true
		expiresAt = time.Now().Add(ttl)
	}

	m.mu.Lock()
	if entry, ok := m.data[key]; ok && entry != nil {
		entry.hasExpiry = hasExpiry
		entry.expiresAt = expiresAt
	}
	m.mu.Unlock()
	return nil
}

// GetTTL returns the remaining time-to-live for a key.
// Returns (0, false) if the key does not exist or has no expiration.
func (m *TTLMap) GetTTL(key string) (time.Duration, bool) {
	now := time.Now()

	m.mu.RLock()
	entry, ok := m.data[key]
	m.mu.RUnlock()

	if !ok || entry == nil || !entry.hasExpiry {
		return 0, false
	}
	if !now.Before(entry.expiresAt) {
		// key expired, remove lazily
		m.mu.Lock()
		if cur, exists := m.data[key]; exists && cur != nil && cur.hasExpiry && !now.Before(cur.expiresAt) {
			delete(m.data, key)
		}
		m.mu.Unlock()
		return 0, false
	}

	return time.Until(entry.expiresAt), true
}

// GetExpiration returns the absolute expiration time for a key.
// Returns (time.Time{}, false) if the key does not exist or has no expiration.
func (m *TTLMap) GetExpiration(key string) (time.Time, bool) {
	now := time.Now()

	m.mu.RLock()
	entry, ok := m.data[key]
	m.mu.RUnlock()

	if !ok || entry == nil || !entry.hasExpiry {
		return time.Time{}, false
	}
	if !now.Before(entry.expiresAt) {
		// expired, remove lazily
		m.mu.Lock()
		if cur, exists := m.data[key]; exists && cur != nil && cur.hasExpiry && !now.Before(cur.expiresAt) {
			delete(m.data, key)
		}
		m.mu.Unlock()
		return time.Time{}, false
	}

	return entry.expiresAt, true
}

func (m *TTLMap) cleanupLoop() {
	t := time.NewTicker(m.cleanupInterval)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			_ = m.Cleanup()
		case <-m.stopCh:
			return
		}
	}
}

func (m *TTLMap) cleanupExpiredLocked(now time.Time) {
	for k, v := range m.data {
		if v == nil {
			delete(m.data, k)
			continue
		}
		if v.hasExpiry && !now.Before(v.expiresAt) {
			delete(m.data, k)
		}
	}
}

func (m *TTLMap) incrementHit() {
	// relaxed: do not lock, stats can be eventually consistent
	m.hits++
}

func (m *TTLMap) incrementMiss() {
	// relaxed: do not lock, stats can be eventually consistent
	m.misses++
}

func roughKeyOverhead(key string) int {
	// Very rough approximation: key bytes + small constant overhead
	return len(key) + 16
}
