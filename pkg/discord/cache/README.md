# Unified Cache System

## Overview

The Unified Cache System provides a high-performance, in-memory caching layer for frequently accessed Discord API data. It significantly reduces API calls by caching Guild Members, Guilds, Roles, and Channels with configurable TTL (Time-To-Live) values.

## Architecture

### Three-Tier Caching Strategy

The system implements a three-tier fallback strategy for optimal performance:

```
1. Unified Cache (in-memory, TTL-based)
   ↓ miss
2. Discord.js State Cache (session.State)
   ↓ miss
3. Discord REST API (with cache backfill)
```

This approach minimizes API rate limit consumption while maintaining data freshness.

## Components

### 1. `UnifiedCache`

The core cache manager that stores Discord entities in memory with automatic TTL-based expiration.

**Cached Entities:**
- **Members** (`guildID:userID` → `*discordgo.Member`)
- **Guilds** (`guildID` → `*discordgo.Guild`)
- **Roles** (`guildID` → `[]*discordgo.Role`)
- **Channels** (`channelID` → `*discordgo.Channel`)

**Features:**
- Thread-safe operations using `sync.RWMutex`
- Automatic expiration with background cleanup
- Per-entity-type TTL configuration
- Comprehensive metrics tracking (hits/misses)
- Automatic invalidation on Discord events

### 2. `CachedSession`

A wrapper around `discordgo.Session` that transparently integrates caching into API calls.

**Benefits:**
- Drop-in replacement for session methods
- Automatic cache population on API calls
- Event-driven cache invalidation
- No manual cache management required

## Configuration

### Default TTL Values

```go
MemberTTL:       5 minutes   // Balance freshness vs. API calls
GuildTTL:        15 minutes  // Guilds change infrequently
RolesTTL:        10 minutes  // Roles updated occasionally
ChannelTTL:      15 minutes  // Channels mostly static
CleanupInterval: 2 minutes   // Background cleanup frequency
```

### Custom Configuration

```go
config := cache.CacheConfig{
    MemberTTL:       3 * time.Minute,
    GuildTTL:        20 * time.Minute,
    RolesTTL:        8 * time.Minute,
    ChannelTTL:      20 * time.Minute,
    CleanupInterval: 1 * time.Minute,
}

unifiedCache := cache.NewUnifiedCache(config)
```

## Usage

### Basic Usage

```go
// Initialize cache with defaults
cache := cache.NewUnifiedCache(cache.DefaultCacheConfig())

// Get a guild member (cache → state → API)
member, err := monitoringService.getGuildMember(guildID, userID)
if err != nil {
    // Handle error
}

// Get a guild (cache → state → API)
guild, err := monitoringService.getGuild(guildID)
if err != nil {
    // Handle error
}
```

### Integration with Components

The unified cache is automatically integrated into:

1. **MonitoringService** - All member/guild lookups use cache
2. **PermissionChecker** - Permission checks leverage cached data
3. **UserWatcher** - Username resolution uses cache
4. **Member Events** - Join/leave events benefit from cache

### Manual Cache Operations

```go
// Get member from cache (no fallback)
member, ok := cache.GetMember(guildID, userID)
if !ok {
    // Cache miss - fetch from API and populate
}

// Set member in cache
cache.SetMember(guildID, userID, member)

// Invalidate specific entry
cache.InvalidateMember(guildID, userID)

// Clear entire cache
cache.Clear()
```

## Metrics

### Available Metrics

The cache tracks comprehensive metrics for performance monitoring:

```go
stats := cache.GetStats()

// Per-entity metrics
stats.MemberEntries   // Current member cache size
stats.MemberHits      // Cache hits for members
stats.MemberMisses    // Cache misses for members

stats.GuildEntries    // Current guild cache size
stats.GuildHits       // Cache hits for guilds
stats.GuildMisses     // Cache misses for guilds

stats.RolesEntries    // Current roles cache size
stats.RolesHits       // Cache hits for roles
stats.RolesMisses     // Cache misses for roles

stats.ChannelEntries  // Current channel cache size
stats.ChannelHits     // Cache hits for channels
stats.ChannelMisses   // Cache misses for channels
```

### Viewing Metrics

Use the `/admin metrics` command to view real-time cache statistics:

```
Cache Statistics:
  Member Cache: 1,234 entries (5,678 hits, 234 misses)
  Guild Cache: 12 entries (890 hits, 45 misses)
  Roles Cache: 12 entries (456 hits, 78 misses)
  Channel Cache: 89 entries (234 hits, 12 misses)
```

## Cache Invalidation

### Automatic Invalidation

The cache automatically invalidates entries when Discord events occur:

| Event | Invalidation Action |
|-------|-------------------|
| `GuildMemberUpdate` | Invalidate specific member |
| `GuildMemberRemove` | Invalidate specific member |
| `GuildUpdate` | Invalidate guild |
| `GuildRoleCreate` | Invalidate guild roles |
| `GuildRoleUpdate` | Invalidate guild roles |
| `GuildRoleDelete` | Invalidate guild roles |
| `ChannelUpdate` | Invalidate channel |
| `ChannelDelete` | Invalidate channel |

### Manual Invalidation

```go
// Invalidate a member when data changes
cache.InvalidateMember(guildID, userID)

// Invalidate a guild when settings change
cache.InvalidateGuild(guildID)

// Invalidate roles after role updates
cache.InvalidateRoles(guildID)

// Invalidate a channel after updates
cache.InvalidateChannel(channelID)
```

## Performance Impact

### Before Unified Cache

```
Scenario: Role update notification (100 events/minute)
- GuildMember API calls: ~200/min (2 per event)
- Guild API calls: ~100/min (1 per event)
- Total API calls: ~300/min
- Rate limit risk: HIGH
```

### After Unified Cache

```
Scenario: Role update notification (100 events/minute)
- GuildMember API calls: ~5/min (cache hit rate ~95%)
- Guild API calls: ~1/min (cache hit rate ~99%)
- Total API calls: ~6/min
- Rate limit risk: LOW
- Performance improvement: ~50x reduction in API calls
```

## Best Practices

### 1. Always Use Helper Methods

```go
// ✅ GOOD - Uses cache
member, err := monitoringService.getGuildMember(guildID, userID)

// ❌ BAD - Bypasses cache
member, err := session.GuildMember(guildID, userID)
```

### 2. Tune TTL for Your Use Case

```go
// High-frequency updates? Use shorter TTL
config.MemberTTL = 2 * time.Minute

// Mostly static data? Use longer TTL
config.GuildTTL = 30 * time.Minute
```

### 3. Monitor Cache Hit Rates

Aim for:
- **Members**: 80-95% hit rate
- **Guilds**: 95-99% hit rate
- **Roles**: 85-95% hit rate
- **Channels**: 90-98% hit rate

If hit rates are low, consider increasing TTL values.

### 4. Invalidate When Necessary

Always invalidate cache when you modify data:

```go
// After updating member roles
cache.InvalidateMember(guildID, userID)

// After modifying guild settings
cache.InvalidateGuild(guildID)
```

## Troubleshooting

### High Cache Miss Rate

**Symptoms:** High API call count, low hit rate in metrics

**Solutions:**
1. Increase TTL values
2. Check if cache is being invalidated too frequently
3. Verify cache is properly initialized
4. Check cleanup interval (too aggressive?)

### Stale Data Issues

**Symptoms:** Users see outdated information

**Solutions:**
1. Decrease TTL values for affected entity type
2. Add manual invalidation on data changes
3. Verify event handlers are properly registered

### Memory Usage Concerns

**Symptoms:** High memory consumption

**Solutions:**
1. Decrease TTL values to allow faster expiration
2. Reduce cleanup interval for more aggressive cleanup
3. Monitor `*Entries` metrics to track cache size
4. Consider implementing max size limits (future enhancement)

## Future Enhancements

Potential improvements for the cache system:

1. **LRU Eviction Policy** - Limit memory by evicting least recently used entries
2. **Persistent Cache** - Optional SQLite backing for cache survivability across restarts
3. **Warm-up Strategy** - Pre-populate cache on startup with frequently accessed data
4. **Per-Guild TTL** - Allow different TTL values per guild based on activity
5. **Cache Compression** - Reduce memory footprint for large member lists
6. **Distributed Cache** - Share cache across multiple bot instances (Redis/Memcached)

## API Reference

### UnifiedCache Methods

```go
// Member operations
GetMember(guildID, userID string) (*discordgo.Member, bool)
SetMember(guildID, userID string, member *discordgo.Member)
InvalidateMember(guildID, userID string)

// Guild operations
GetGuild(guildID string) (*discordgo.Guild, bool)
SetGuild(guildID string, guild *discordgo.Guild)
InvalidateGuild(guildID string)

// Roles operations
GetRoles(guildID string) ([]*discordgo.Role, bool)
SetRoles(guildID string, roles []*discordgo.Role)
InvalidateRoles(guildID string)

// Channel operations
GetChannel(channelID string) (*discordgo.Channel, bool)
SetChannel(channelID string, channel *discordgo.Channel)
InvalidateChannel(channelID string)

// Management
GetStats() CacheStats
Clear()
Stop()
```

### CachedSession Methods

```go
// Cached API calls (transparent caching)
GuildMember(guildID, userID string) (*discordgo.Member, error)
Guild(guildID string) (*discordgo.Guild, error)
GuildRoles(guildID string) ([]*discordgo.Role, error)
Channel(channelID string) (*discordgo.Channel, error)

// Access underlying components
Session() *discordgo.Session
Cache() *UnifiedCache
```

## License

This cache system is part of the DiscordCore project and follows the same license.

## Contributing

When modifying the cache system:

1. Maintain thread-safety (use proper locking)
2. Update metrics when adding new cache types
3. Add event handlers for automatic invalidation
4. Document TTL recommendations
5. Add tests for new functionality

---

**Last Updated:** 2024-01
**Version:** 1.0.0