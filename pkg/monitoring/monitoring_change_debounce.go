package monitoring

import (
	"sync"
	"time"
)

// changeDebouncer suppresses duplicate change notifications keyed by an
// arbitrary string within a caller-supplied window. Access is serialized by
// mu. The zero value is ready to use: the entry map is created lazily on first
// write.
type changeDebouncer struct {
	mu      sync.RWMutex
	entries map[string]time.Time
}

// recentlyChanged reports whether key was last recorded within window.
func (d *changeDebouncer) recentlyChanged(key string, window time.Duration) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	last, ok := d.entries[key]
	return ok && time.Since(last) < window
}

// record stamps key with the current time. Once the map grows past maxEntries
// it evicts entries older than maxAge to bound memory.
func (d *changeDebouncer) record(key string, maxEntries int, maxAge time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.entries == nil {
		d.entries = make(map[string]time.Time)
	}
	d.entries[key] = time.Now()
	if len(d.entries) > maxEntries {
		for k, ts := range d.entries {
			if time.Since(ts) > maxAge {
				delete(d.entries, k)
			}
		}
	}
}
