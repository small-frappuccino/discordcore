package system

import "time"

type CacheEntryRecord struct {
	CacheType string
	Key       string
	GuildID   string
	Data      string
	ExpiresAt time.Time
}

type CacheEntry struct {
	Key       string
	Data      string
	ExpiresAt time.Time
}

type PersistentCacheStats struct {
	Total  int            `json:"total"`
	ByType map[string]int `json:"by_type,omitempty"`
}
