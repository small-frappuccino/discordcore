package cache

// Generic sharded LRU+TTL segment for reducing repetition across cache types (members/guilds/roles/channels).
//
// Each segment is split into independent shards to reduce lock contention on unrelated keys.
// Entries keep shard-local LRU state for eviction order. Expiration is uniform per segment unless
// explicitly overridden via SetWithExpiration.
//
// Concurrency: all operations are safe for concurrent access.

import (
	"container/list"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

const defaultSegmentShardCount = 16

// segment is a generic sharded LRU+TTL container.
type segment[T any] struct {
	shards []segmentShard[T]

	ttl        time.Duration
	ttlMu      sync.RWMutex
	limit      int
	limitMu    sync.RWMutex
	shardLimit int

	// Metrics
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

type segmentShard[T any] struct {
	mu    sync.RWMutex
	data  map[string]*entry[T]
	lru   *list.List
	dirty map[string]struct{}
}

type entry[T any] struct {
	key       string
	value     T
	expiresAt time.Time // zero means no expiration
	elem      *list.Element
}

type segmentSnapshot[T any] struct {
	Key       string
	Value     T
	ExpiresAt time.Time
}

// newSegment creates a new segment with the given TTL and capacity limit.
// ttl <= 0 disables expiration. limit <= 0 means unbounded size (no LRU evictions).
func newSegment[T any](ttl time.Duration, limit int) *segment[T] {
	shardCount := configuredSegmentShardCount(limit)
	shards := make([]segmentShard[T], shardCount)
	for i := range shards {
		shards[i] = segmentShard[T]{
			data:  make(map[string]*entry[T]),
			lru:   list.New(),
			dirty: make(map[string]struct{}),
		}
	}

	return &segment[T]{
		shards:     shards,
		ttl:        ttl,
		limit:      limit,
		shardLimit: shardScopedLimit(limit, len(shards)),
	}
}

// Get returns the value for key if present and not expired.
// Promotion to the front of the shard-local LRU is opportunistic and never blocks readers.
func (s *segment[T]) Get(key string) (T, bool) {
	var zero T
	if key == "" {
		s.misses.Add(1)
		return zero, false
	}

	now := time.Now()
	shard := s.shardFor(key)

	shard.mu.RLock()
	e, ok := shard.data[key]
	if !ok {
		shard.mu.RUnlock()
		s.misses.Add(1)
		return zero, false
	}
	value := e.value
	expiresAt := e.expiresAt
	shard.mu.RUnlock()

	if !expiresAt.IsZero() && now.After(expiresAt) {
		shard.mu.Lock()
		current, exists := shard.data[key]
		if exists && current == e && !current.expiresAt.IsZero() && now.After(current.expiresAt) {
			shard.removeEntry(current)
		}
		shard.mu.Unlock()
		s.misses.Add(1)
		return zero, false
	}

	s.hits.Add(1)

	if shard.mu.TryLock() {
		if current, exists := shard.data[key]; exists && current == e {
			shard.lru.MoveToFront(current.elem)
		}
		shard.mu.Unlock()
	}

	return value, true
}

// Set stores the value for key, updating TTL and promoting to the front of the shard-local LRU.
func (s *segment[T]) Set(key string, v T) {
	if key == "" {
		return
	}

	expiresAt := time.Time{}
	if ttl := s.currentTTL(); ttl > 0 {
		expiresAt = time.Now().Add(ttl)
	}
	s.setWithExpiration(key, v, expiresAt, true)
}

// SetWithExpiration stores the value for key with a specific expiration time,
// overriding the segment TTL for this entry.
func (s *segment[T]) SetWithExpiration(key string, v T, expiresAt time.Time) {
	s.setWithExpiration(key, v, expiresAt, true)
}

func (s *segment[T]) setWithExpiration(key string, v T, expiresAt time.Time, markDirty bool) {
	if key == "" {
		return
	}

	shard := s.shardFor(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if e, ok := shard.data[key]; ok {
		e.value = v
		e.expiresAt = expiresAt
		shard.lru.MoveToFront(e.elem)
		if markDirty {
			shard.dirty[key] = struct{}{}
		}
		return
	}

	shardLimit := s.currentShardLimit()
	if shardLimit > 0 && len(shard.data) >= shardLimit {
		s.evictLRU(shard)
	}

	elem := shard.lru.PushFront(key)
	shard.data[key] = &entry[T]{
		key:       key,
		value:     v,
		expiresAt: expiresAt,
		elem:      elem,
	}
	if markDirty {
		shard.dirty[key] = struct{}{}
	}
}

// Invalidate removes the entry for key if it exists.
func (s *segment[T]) Invalidate(key string) {
	if key == "" {
		return
	}
	shard := s.shardFor(key)
	shard.mu.Lock()
	if e, ok := shard.data[key]; ok {
		shard.removeEntry(e)
	}
	delete(shard.dirty, key)
	shard.mu.Unlock()
}

// Clear removes all entries from the segment.
func (s *segment[T]) Clear() {
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.Lock()
		clear(shard.data)
		clear(shard.dirty)
		shard.lru = list.New()
		shard.mu.Unlock()
	}
}

// CleanupExpired scans and removes all expired entries.
func (s *segment[T]) CleanupExpired(now time.Time) {
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.Lock()
		for k, e := range shard.data {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				shard.removeEntry(e)
				delete(shard.dirty, k)
			}
		}
		shard.mu.Unlock()
	}
}

// CleanupExpiredWithCallback removes expired entries and invokes onEvict for each removed entry.
// If onEvict is nil, behaves like CleanupExpired.
func (s *segment[T]) CleanupExpiredWithCallback(now time.Time, onEvict func(key string, value T)) {
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.Lock()
		if onEvict == nil {
			for k, e := range shard.data {
				if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
					shard.removeEntry(e)
					delete(shard.dirty, k)
				}
			}
			shard.mu.Unlock()
			continue
		}

		evicted := make([]*entry[T], 0)
		for k, e := range shard.data {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				shard.removeEntry(e)
				delete(shard.dirty, k)
				ev := *e
				evicted = append(evicted, &ev)
			}
		}
		shard.mu.Unlock()

		for _, e := range evicted {
			onEvict(e.key, e.value)
		}
	}
}

// Len returns the number of entries currently stored.
func (s *segment[T]) Len() int {
	total := 0
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.RLock()
		total += len(shard.data)
		shard.mu.RUnlock()
	}
	return total
}

// Keys returns a copy of current keys. Use with caution on large segments.
func (s *segment[T]) Keys() []string {
	keys := make([]string, 0)
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.RLock()
		keys = append(keys, slices.Collect(maps.Keys(shard.data))...)
		shard.mu.RUnlock()
	}
	return keys
}

// GetExpiration returns the expiration time for a key, if present.
// When the key is not present or has no expiration, returns zero time and false.
func (s *segment[T]) GetExpiration(key string) (time.Time, bool) {
	if key == "" {
		return time.Time{}, false
	}
	shard := s.shardFor(key)
	shard.mu.RLock()
	e, ok := shard.data[key]
	shard.mu.RUnlock()
	if !ok {
		return time.Time{}, false
	}
	return e.expiresAt, true
}

// SetTTL updates the TTL for subsequent insertions. Existing entries keep their current expiration.
func (s *segment[T]) SetTTL(ttl time.Duration) {
	s.ttlMu.Lock()
	s.ttl = ttl
	s.ttlMu.Unlock()
}

// SetLimit updates the capacity limit for the segment. If the new limit is lower than the current size,
// each shard evicts LRU entries to satisfy its local limit.
func (s *segment[T]) SetLimit(limit int) {
	shardLimit := shardScopedLimit(limit, len(s.shards))

	s.limitMu.Lock()
	s.limit = limit
	s.shardLimit = shardLimit
	s.limitMu.Unlock()

	if shardLimit <= 0 {
		return
	}

	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.Lock()
		for len(shard.data) > shardLimit {
			s.evictLRU(shard)
		}
		shard.mu.Unlock()
	}
}

// TakeDirtySnapshot drains the current dirty set and returns live entries for incremental persistence.
func (s *segment[T]) TakeDirtySnapshot(now time.Time) []segmentSnapshot[T] {
	snapshots := make([]segmentSnapshot[T], 0)
	for i := range s.shards {
		shard := &s.shards[i]
		shard.mu.Lock()
		for key := range shard.dirty {
			e, ok := shard.data[key]
			if !ok {
				delete(shard.dirty, key)
				continue
			}
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				shard.removeEntry(e)
				delete(shard.dirty, key)
				continue
			}
			snapshots = append(snapshots, segmentSnapshot[T]{
				Key:       key,
				Value:     e.value,
				ExpiresAt: e.expiresAt,
			})
			delete(shard.dirty, key)
		}
		shard.mu.Unlock()
	}
	return snapshots
}

// MarkDirty re-adds keys to the dirty set when an incremental persist attempt fails.
func (s *segment[T]) MarkDirty(keys []string) {
	for _, key := range keys {
		if key == "" {
			continue
		}
		shard := s.shardFor(key)
		shard.mu.Lock()
		if _, ok := shard.data[key]; ok {
			shard.dirty[key] = struct{}{}
		}
		shard.mu.Unlock()
	}
}

// SetCleanWithExpiration stores the value without adding it to the dirty set.
// Intended for warmup/reload paths that hydrate cache state from persistence.
func (s *segment[T]) SetCleanWithExpiration(key string, v T, expiresAt time.Time) {
	s.setWithExpiration(key, v, expiresAt, false)
}

func (s *segment[T]) currentTTL() time.Duration {
	s.ttlMu.RLock()
	ttl := s.ttl
	s.ttlMu.RUnlock()
	return ttl
}

func (s *segment[T]) currentShardLimit() int {
	s.limitMu.RLock()
	limit := s.shardLimit
	s.limitMu.RUnlock()
	return limit
}

func (s *segment[T]) shardFor(key string) *segmentShard[T] {
	return &s.shards[segmentShardIndex(key, len(s.shards))]
}

// evictLRU removes the least recently used entry from a shard. Caller must hold shard.mu.
func (s *segment[T]) evictLRU(shard *segmentShard[T]) {
	if shard == nil {
		return
	}
	back := shard.lru.Back()
	if back == nil {
		return
	}
	key := back.Value.(string)
	shard.lru.Remove(back)
	delete(shard.data, key)
	delete(shard.dirty, key)
	s.evictions.Add(1)
}

func (shard *segmentShard[T]) removeEntry(e *entry[T]) {
	if shard == nil || e == nil {
		return
	}
	shard.lru.Remove(e.elem)
	delete(shard.data, e.key)
	delete(shard.dirty, e.key)
}

func segmentShardIndex(key string, shardCount int) int {
	if shardCount <= 1 {
		return 0
	}
	var hash uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		hash ^= uint32(key[i])
		hash *= 16777619
	}
	return int(hash % uint32(shardCount))
}

func shardScopedLimit(limit, shardCount int) int {
	if limit <= 0 || shardCount <= 0 {
		return 0
	}
	return (limit + shardCount - 1) / shardCount
}

func configuredSegmentShardCount(limit int) int {
	if limit > 0 && limit <= 128 {
		return 1
	}
	return defaultSegmentShardCount
}

// segmentStats summarizes a segment's state and counters.
type segmentStats struct {
	Size        int           `json:"size"`
	Hits        uint64        `json:"hits"`
	Misses      uint64        `json:"misses"`
	Evictions   uint64        `json:"evictions"`
	TTL         time.Duration `json:"ttl"`
	Limit       int           `json:"limit"`
	Now         time.Time     `json:"now"`
	HasExpiry   bool          `json:"has_expiry"`
	HitRate     float64       `json:"hit_rate"`
	MissRate    float64       `json:"miss_rate"`
	EntriesSeen uint64        `json:"entries_seen"`
}

// Stats returns a snapshot of the segment's counters and configuration.
func (s *segment[T]) Stats() segmentStats {
	h := s.hits.Load()
	m := s.misses.Load()
	e := s.evictions.Load()
	size := s.Len()

	total := h + m
	var hitRate, missRate float64
	if total > 0 {
		hitRate = float64(h) / float64(total)
		missRate = float64(m) / float64(total)
	}

	s.ttlMu.RLock()
	ttl := s.ttl
	s.ttlMu.RUnlock()

	s.limitMu.RLock()
	limit := s.limit
	s.limitMu.RUnlock()

	return segmentStats{
		Size:        size,
		Hits:        h,
		Misses:      m,
		Evictions:   e,
		TTL:         ttl,
		Limit:       limit,
		Now:         time.Now(),
		HasExpiry:   ttl > 0,
		HitRate:     hitRate,
		MissRate:    missRate,
		EntriesSeen: total,
	}
}
