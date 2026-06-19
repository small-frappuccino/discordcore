package cache

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

// WarmupSession holds function closures to interact with Discord API, enabling easy mocking without interface bloat.
type WarmupSession struct {
	StateGuilds         func() []*discordgo.Guild
	RequestGuildMembers func(guildID, query string, limit int, nonce string, presences bool) error
}

var NewWarmupSession = func(s *discordgo.Session) WarmupSession {
	return WarmupSession{
		StateGuilds: func() []*discordgo.Guild {
			if s == nil || s.State == nil {
				return nil
			}
			return s.State.Guilds
		},
		RequestGuildMembers: s.RequestGuildMembers,
	}
}

// WarmupConfig configures the intelligent warmup behavior
type WarmupConfig struct {
	// FetchMissingMembers fetches members from Discord if not in cache (via Gateway Opcode 8)
	FetchMissingMembers bool
	// MaxMembersPerGuild limits how many members to fetch per guild (0 = all)
	MaxMembersPerGuild int
	// GuildIDs restricts warmup to specific guilds (nil = all guilds)
	GuildIDs []string
}

// DefaultWarmupConfig returns a sensible default warmup configuration
func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{
		FetchMissingMembers: true,
		MaxMembersPerGuild:  1000, // Limit to 1000 most recent members per guild
		GuildIDs:            nil,  // All guilds
	}
}

// IntelligentWarmup performs cache warmup by loading persisted cache and fetching missing data
func IntelligentWarmup(session *discordgo.Session, cache *UnifiedCache, store *storage.Store, config WarmupConfig) error {
	return IntelligentWarmupContext(context.Background(), session, cache, store, config)
}

// IntelligentWarmupContext performs cache warmup with cooperative cancellation checks.
// Guilds, Roles, and Channels are hydrated implicitly via GUILD_CREATE events.
// Members are backfilled via Gateway Opcode 8 requests if FetchMissingMembers is true.
func IntelligentWarmupContext(ctx context.Context, session *discordgo.Session, cache *UnifiedCache, store *storage.Store, config WarmupConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("IntelligentWarmupContext: %w", err)
	}

	startTime := time.Now()
	log.ApplicationLogger().Info("🚀 Starting cache warmup (persistent preload)...")

	ws := NewWarmupSession(session)

	// Step 1: Load persisted cache entries from the persistent store
	if err := cache.Warmup(); err != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup from persistent cache: %v", err))
	} else {
		members := cache.MemberCount()
		guilds := cache.GuildCount()
		roles := cache.RolesCount()
		channels := cache.ChannelCount()
		preloadMsg := fmt.Sprintf("💾 Restored from persistent cache: %d members, %d guilds, %d roles, %d channels",
			members, guilds, roles, channels)
		if members == 0 && guilds == 0 && roles == 0 && channels == 0 {
			preloadMsg += " (normal on first run or after expiration; Discord backfill comes next)"
		}
		log.ApplicationLogger().Info(preloadMsg)
	}

	// Step 2: Dispatch Gateway Opcode 8 if member hydration is requested
	if !config.FetchMissingMembers {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("IntelligentWarmupContext: %w", err)
	}
	guildIDs := config.GuildIDs
	if len(guildIDs) == 0 {
		// Get all guilds the bot is in
		for _, guild := range ws.StateGuilds() {
			guildIDs = append(guildIDs, guild.ID)
		}
	}

	if len(guildIDs) == 0 {
		log.ApplicationLogger().Warn("No guilds found for member stream warmup")
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf("🔄 Dispatching Opcode 8 Gateway requests for %d guild(s)...", len(guildIDs)))

	dispatchedGuilds := 0
	for _, guildID := range guildIDs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("IntelligentWarmupContext: %w", err)
		}

		// Dispatch asynchronous Opcode 8 Request Guild Members payload
		// This does not block or execute HTTP requests.
		if ws.RequestGuildMembers != nil {
			if err := ws.RequestGuildMembers(guildID, "", config.MaxMembersPerGuild, "", true); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to request members stream for guild %s: %v", guildID, err))
			} else {
				dispatchedGuilds++
			}
		}
	}

	elapsed := time.Since(startTime)
	log.ApplicationLogger().Info(fmt.Sprintf("✅ Warmup Opcode 8 dispatch completed in %v: %d guilds",
		elapsed, dispatchedGuilds))

	return nil
}

// RefreshMemberData refreshes member data for active members in a guild
func RefreshMemberData(session *discordgo.Session, cache *UnifiedCache, store *storage.Store, guildID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf("🔄 Refreshing %d members in guild %s", len(userIDs), guildID))

	for _, userID := range userIDs {
		member, err := session.GuildMember(guildID, userID)
		if err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to refresh member %s: %v", userID, err))
			continue
		}

		// Update cache
		cache.SetMember(guildID, userID, member)

		// Update storage
		if store != nil {
			joinedAt := time.Now().UTC()
			if !member.JoinedAt.IsZero() {
				joinedAt = member.JoinedAt
			}
			if err := store.UpsertMemberJoin(guildID, userID, joinedAt); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to update member join: %v", err))
			}

			if len(member.Roles) > 0 {
				if err := store.UpsertMemberRoles(guildID, userID, member.Roles, time.Now().UTC()); err != nil {
					log.ApplicationLogger().Warn(fmt.Sprintf("Failed to update member roles: %v", err))
				}
			}
		}
	}

	return nil
}

// SchedulePeriodicCleanup starts a background goroutine that periodically cleans up obsolete data.
// Pass interval <= 0 to disable cleanup (returns nil).
func SchedulePeriodicCleanup(store *storage.Store, interval time.Duration) chan struct{} {
	if interval <= 0 {
		return nil
	}

	stopChan := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.ErrorLoggerRaw().Error("Warmup cleanup loop panic caught", "panic", r, "stack", string(debug.Stack()))
			}
		}()

		// Initial warm-up dispatch
		if err := store.CleanupAllObsoleteData(); err != nil {
			log.ErrorLoggerRaw().Error(fmt.Sprintf("Initial cleanup failed: %v", err))
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			select {
			case <-stopChan:
				cancel()
			case <-ctx.Done():
			}
		}()

		for {
			timer := time.NewTimer(calculateJitter(interval))
			select {
			case <-timer.C:
				if err := store.CleanupAllObsoleteData(); err != nil {
					log.ErrorLoggerRaw().Error(fmt.Sprintf("Periodic cleanup failed: %v", err))
				}
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	}()

	return stopChan
}

// KeepMemberDataFresh updates timestamps for active members to prevent cleanup
func KeepMemberDataFresh(store *storage.Store, guildID string, userIDs []string) error {
	if store == nil || len(userIDs) == 0 {
		return nil
	}

	for _, userID := range userIDs {
		if err := store.TouchMemberJoin(guildID, userID); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member join freshness for %s: %v", userID, err))
		}
		if err := store.TouchMemberRoles(guildID, userID); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member roles for %s: %v", userID, err))
		}
	}

	return nil
}
