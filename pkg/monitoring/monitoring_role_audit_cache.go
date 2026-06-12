package monitoring

import (
	"runtime"
	"sync"
	"time"
	"weak"

	"github.com/small-frappuccino/discordgo"
)

type cachedRoleUpdateAudit struct {
	fetchedAt time.Time
	entries   []*discordgo.AuditLogEntry
}

// roleUpdateAuditStore is a short-lived, self-evicting cache of per-guild
// member-role-update audit-log entries together with a per-(guild,user)
// debounce of audit refreshes. All access is serialized by mu. The zero value
// is ready to use: the maps are created lazily by ensureLocked.
type roleUpdateAuditStore struct {
	mu       sync.Mutex
	cache    map[string]weak.Pointer[cachedRoleUpdateAudit]
	debounce map[string]time.Time
}

func (s *roleUpdateAuditStore) ensureLocked() {
	if s.cache == nil {
		s.cache = make(map[string]weak.Pointer[cachedRoleUpdateAudit])
	}
	if s.debounce == nil {
		s.debounce = make(map[string]time.Time)
	}
}

// cachedEntries returns a copy of the entries cached for guildID when present
// and younger than monitoringRoleAuditCacheTTL relative to now. The boolean
// reports a cache hit.
func (s *roleUpdateAuditStore) cachedEntries(guildID string, now time.Time) ([]*discordgo.AuditLogEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked()
	if weakPtr, ok := s.cache[guildID]; ok {
		if entry := weakPtr.Value(); entry != nil {
			if now.Sub(entry.fetchedAt) < monitoringRoleAuditCacheTTL {
				return append([]*discordgo.AuditLogEntry(nil), entry.entries...), true
			}
		}
	}
	return nil, false
}

// storeEntries records entries for guildID stamped at now, copying the slice.
// Map eviction is tied to the lifecycle of the pointer utilizing runtime.AddCleanup.
func (s *roleUpdateAuditStore) storeEntries(guildID string, now time.Time, entries []*discordgo.AuditLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked()

	entry := &cachedRoleUpdateAudit{
		fetchedAt: now,
		entries:   append([]*discordgo.AuditLogEntry(nil), entries...),
	}

	s.cache[guildID] = weak.Make(entry)

	runtime.AddCleanup(entry, func(k string) {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.cache, k)
	}, guildID)
}

// shouldDebounce reports whether a refresh for (guildID,userID) occurred within
// monitoringRoleAuditDebounceTTL relative to now. When it returns false it
// stamps the pair so subsequent calls inside the window are debounced, evicting
// stale debounce keys once the map grows past 200 entries.
func (s *roleUpdateAuditStore) shouldDebounce(guildID, userID string, now time.Time) bool {
	key := guildID + ":" + userID
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureLocked()
	if last, ok := s.debounce[key]; ok && now.Sub(last) < monitoringRoleAuditDebounceTTL {
		return true
	}
	s.debounce[key] = now
	if len(s.debounce) > 200 {
		for debounceKey, last := range s.debounce {
			if now.Sub(last) > 5*time.Minute {
				delete(s.debounce, debounceKey)
			}
		}
	}
	return false
}

// sizes returns the current cache and debounce map lengths for status display.
func (s *roleUpdateAuditStore) sizes() (cacheSize, debounceSize int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.cache), len(s.debounce)
}
