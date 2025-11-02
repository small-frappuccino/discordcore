package cache

// Generic LRU+TTL segment for reducing repetition across cache types (members/guilds/roles/channels).
//
// This segment maintains a map for O(1) lookup and a container/list-based LRU for eviction order.
// Entries can have a TTL defined at segment-level (uniform for all entries). TTL <= 0 disables
// expiration. Eviction occurs on Set when a limit is configured and the size threshold is met.
// Get will return false if the key does not exist or has expired (expired entries are removed).
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

// segment is a generic LRU+TTL container.
type segment[T any] struct {
	mu    sync.RWMutex
	data  map[string]*entry[T]
	lru   *list.List
	ttl   time.Duration
	limit int

	// Metrics
	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64
}

type entry[T any] struct {
	key       string
	value     T
	expiresAt time.Time // zero means no expiration
	elem      *list.Element
}

// newSegment creates a new segment with the given TTL and capacity limit.
// ttl <= 0 disables expiration. limit <= 0 means unbounded size (no LRU evictions).
func newSegment[T any](ttl time.Duration, limit int) *segment[T] {
	return &segment[T]{
		data:  make(map[string]*entry[T]),
		lru:   list.New(),
		ttl:   ttl,
		limit: limit,
	}
}

// Get returns the value for key if present and not expired.
// If the entry exists and is valid, it is promoted to the front of the LRU.
// If missing or expired, returns the zero value for T and false.
func (s *segment[T]) Get(key string) (T, bool) {
	var zero T
	if key == "" {
		s.misses.Add(1)
		return zero, false
	}

	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		s.misses.Add(1)
		return zero, false
	}
	// Expired?
	if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
		s.lru.Remove(e.elem)
		delete(s.data, key)
		s.misses.Add(1)
		return zero, false
	}

	// Promote
	s.lru.MoveToFront(e.elem)
	s.hits.Add(1)
	return e.value, true
}

// Set stores the value for key, updating TTL and promoting to the front of the LRU.
// If limit > 0 and size >= limit, evicts one LRU entry before inserting.
func (s *segment[T]) Set(key string, v T) {
	if key == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update existing
	if e, ok := s.data[key]; ok {
		e.value = v
		if s.ttl > 0 {
			e.expiresAt = time.Now().Add(s.ttl)
		} else {
			e.expiresAt = time.Time{}
		}
		s.lru.MoveToFront(e.elem)
		return
	}

	// Evict if full
	if s.limit > 0 && len(s.data) >= s.limit {
		s.evictLRU()
	}

	// Insert new
	expiresAt := time.Time{}
	if s.ttl > 0 {
		expiresAt = time.Now().Add(s.ttl)
	}
	elem := s.lru.PushFront(key)
	s.data[key] = &entry[T]{
		key:       key,
		value:     v,
		expiresAt: expiresAt,
		elem:      elem,
	}
}

// SetWithExpiration stores the value for key with a specific expiration time,
// overriding the segment TTL for this entry.
func (s *segment[T]) SetWithExpiration(key string, v T, expiresAt time.Time) {
	if key == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update existing
	if e, ok := s.data[key]; ok {
		e.value = v
		e.expiresAt = expiresAt
		s.lru.MoveToFront(e.elem)
		return
	}

	// Evict if full
	if s.limit > 0 && len(s.data) >= s.limit {
		s.evictLRU()
	}

	// Insert new
	elem := s.lru.PushFront(key)
	s.data[key] = &entry[T]{
		key:       key,
		value:     v,
		expiresAt: expiresAt,
		elem:      elem,
	}
}

// Invalidate removes the entry for key if it exists.
func (s *segment[T]) Invalidate(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	if e, ok := s.data[key]; ok {
		s.lru.Remove(e.elem)
		delete(s.data, key)
	}
	s.mu.Unlock()
}

// Clear removes all entries from the segment.
func (s *segment[T]) Clear() {
	s.mu.Lock()
	clear(s.data)
	s.lru = list.New()
	s.mu.Unlock()
}

// CleanupExpired scans and removes all expired entries.
// Callers should pass time.Now() for typical usage; passing a custom time eases testing.
func (s *segment[T]) CleanupExpired(now time.Time) {
	s.mu.Lock()
	for k, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			s.lru.Remove(e.elem)
			delete(s.data, k)
		}
	}
	s.mu.Unlock()
}

// CleanupExpiredWithCallback removes expired entries and invokes onEvict for each removed entry.
// If onEvict is nil, behaves like CleanupExpired.
func (s *segment[T]) CleanupExpiredWithCallback(now time.Time, onEvict func(key string, value T)) {
	s.mu.Lock()
	if onEvict == nil {
		for k, e := range s.data {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				s.lru.Remove(e.elem)
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
		return
	}
	// Collect evicted to call callback without holding lock too long
	evicted := make([]*entry[T], 0)
	for k, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			s.lru.Remove(e.elem)
			delete(s.data, k)
			// keep a copy to call outside lock
			ev := *e
			evicted = append(evicted, &ev)
		}
	}
	s.mu.Unlock()
	for _, e := range evicted {
		onEvict(e.key, e.value)
	}
}

// Len returns the number of entries currently stored (including non-expired only as expired are removed on access/cleanup).
func (s *segment[T]) Len() int {
	s.mu.RLock()
	n := len(s.data)
	s.mu.RUnlock()
	return n
}

// Keys returns a copy of current keys. Use with caution on large segments.
func (s *segment[T]) Keys() []string {
	s.mu.RLock()
	keys := slices.Collect(maps.Keys(s.data))
	s.mu.RUnlock()
	return keys
}

// GetExpiration returns the expiration time for a key, if present.
// When the key is not present or has no expiration, returns zero time and false.
func (s *segment[T]) GetExpiration(key string) (time.Time, bool) {
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return time.Time{}, false
	}
	return e.expiresAt, true
}

// SetTTL updates the TTL for subsequent insertions. Existing entries keep their current expiration.
func (s *segment[T]) SetTTL(ttl time.Duration) {
	s.mu.Lock()
	s.ttl = ttl
	s.mu.Unlock()
}

// SetLimit updates the capacity limit for the segment. If the new limit is lower than the current size,
// the segment will evict LRU entries to satisfy the limit.
func (s *segment[T]) SetLimit(limit int) {
	s.mu.Lock()
	s.limit = limit
	// Enforce limit if needed
	if s.limit > 0 {
		for len(s.data) > s.limit {
			s.evictLRU()
		}
	}
	s.mu.Unlock()
}

// evictLRU removes the least recently used entry. Caller must hold s.mu (write lock).
func (s *segment[T]) evictLRU() {
	back := s.lru.Back()
	if back == nil {
		return
	}
	key := back.Value.(string)
	s.lru.Remove(back)
	delete(s.data, key)
	s.evictions.Add(1)
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
// EntriesSeen = Hits + Misses; HitRate/MissRate computed when EntriesSeen > 0.
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

	s.mu.RLock()
	ttl := s.ttl
	limit := s.limit
	s.mu.RUnlock()

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
