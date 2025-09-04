package cache

import (
	"fmt"
	"time"
)

// CacheStats provides statistics about cache usage
type CacheStats struct {
	TotalEntries  int            `json:"total_entries"`
	MemoryUsage   int64          `json:"memory_usage_bytes"`
	HitRate       float64        `json:"hit_rate"`
	MissRate      float64        `json:"miss_rate"`
	LastCleanup   time.Time      `json:"last_cleanup"`
	TTLEnabled    bool           `json:"ttl_enabled"`
	PerGuildStats map[string]int `json:"per_guild_stats,omitempty"`
	CustomMetrics map[string]any `json:"custom_metrics,omitempty"`
}

// CacheManager provides a unified interface for cache management
type CacheManager interface {
	// Get retrieves a value by key
	Get(key string) (any, bool)

	// Set stores a value with optional TTL
	Set(key string, value any, ttl time.Duration) error

	// Delete removes a key from the cache
	Delete(key string) error

	// Has checks if a key exists
	Has(key string) bool

	// Stats returns cache statistics
	Stats() CacheStats

	// Cleanup removes expired or old entries
	Cleanup() error

	// Clear removes all entries
	Clear() error

	// Size returns the number of entries
	Size() int

	// Keys returns all keys (use with caution on large caches)
	Keys() []string
}

// TTLCache extends CacheManager with TTL-specific methods
type TTLCache interface {
	CacheManager

	// SetTTL updates the TTL for an existing key
	SetTTL(key string, ttl time.Duration) error

	// GetTTL returns the remaining TTL for a key
	GetTTL(key string) (time.Duration, bool)

	// GetExpiration returns the expiration time for a key
	GetExpiration(key string) (time.Time, bool)
}

// PersistentCache extends CacheManager with persistence methods
type PersistentCache interface {
	CacheManager

	// Save persists the cache to storage
	Save() error

	// Load loads the cache from storage
	Load() error

	// SaveThrottled saves with throttling to avoid excessive I/O
	SaveThrottled(interval time.Duration) error
}

// GuildCache extends CacheManager with guild-specific operations
type GuildCache interface {
	CacheManager

	// GetGuildKeys returns keys for a specific guild
	GetGuildKeys(guildID string) []string

	// ClearGuild removes all entries for a specific guild
	ClearGuild(guildID string) error

	// GuildStats returns statistics for a specific guild
	GuildStats(guildID string) CacheStats
}

// CacheType represents different cache implementations
type CacheType string

const (
	CacheTypeMemory     CacheType = "memory"
	CacheTypePersistent CacheType = "persistent"
	CacheTypeRedis      CacheType = "redis"
	CacheTypeHybrid     CacheType = "hybrid"
)

// CacheConfig holds configuration for cache creation
type CacheConfig struct {
	Type            CacheType     `json:"type"`
	MaxSize         int           `json:"max_size"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	CleanupInterval time.Duration `json:"cleanup_interval"`
	PersistPath     string        `json:"persist_path,omitempty"`
	GuildAware      bool          `json:"guild_aware"`
}

// CacheError represents cache-related errors
type CacheError struct {
	Operation string
	Key       string
	Cause     error
}

func (e CacheError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("cache %s failed for key '%s': %v", e.Operation, e.Key, e.Cause)
	}
	return fmt.Sprintf("cache %s failed: %v", e.Operation, e.Cause)
}

func (e CacheError) Unwrap() error {
	return e.Cause
}

// NewCacheError creates a new cache error
func NewCacheError(operation, key string, cause error) CacheError {
	return CacheError{
		Operation: operation,
		Key:       key,
		Cause:     cause,
	}
}
