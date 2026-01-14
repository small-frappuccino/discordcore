package cache

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// warmupSession defines the subset of discordgo.Session used during warmup.
type warmupSession interface {
	StateGuilds() []*discordgo.Guild
	Guild(id string) (*discordgo.Guild, error)
	GuildRoles(id string) ([]*discordgo.Role, error)
	GuildChannels(id string) ([]*discordgo.Channel, error)
	GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error)
}

var newWarmupSession = func(s *discordgo.Session) warmupSession {
	return discordSessionAdapter{session: s}
}

type discordSessionAdapter struct {
	session *discordgo.Session
}

func (d discordSessionAdapter) StateGuilds() []*discordgo.Guild {
	if d.session == nil || d.session.State == nil {
		return nil
	}
	return d.session.State.Guilds
}

func (d discordSessionAdapter) Guild(id string) (*discordgo.Guild, error) {
	return d.session.Guild(id)
}

func (d discordSessionAdapter) GuildRoles(id string) ([]*discordgo.Role, error) {
	return d.session.GuildRoles(id)
}

func (d discordSessionAdapter) GuildChannels(id string) ([]*discordgo.Channel, error) {
	return d.session.GuildChannels(id)
}

func (d discordSessionAdapter) GuildMembers(guildID, after string, limit int, options ...discordgo.RequestOption) ([]*discordgo.Member, error) {
	return d.session.GuildMembers(guildID, after, limit, options...)
}

// WarmupConfig configures the intelligent warmup behavior
type WarmupConfig struct {
	// FetchMissingMembers fetches members from Discord if not in cache
	FetchMissingMembers bool
	// FetchMissingRoles fetches roles from Discord if not in cache
	FetchMissingRoles bool
	// FetchMissingGuilds fetches guilds from Discord if not in cache
	FetchMissingGuilds bool
	// FetchMissingChannels fetches channels from Discord if not in cache
	FetchMissingChannels bool
	// MaxMembersPerGuild limits how many members to fetch per guild (0 = all)
	MaxMembersPerGuild int
	// GuildIDs restricts warmup to specific guilds (nil = all guilds)
	GuildIDs []string
}

// DefaultWarmupConfig returns a sensible default warmup configuration
func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1000, // Limit to 1000 most recent members per guild
		GuildIDs:             nil,  // All guilds
	}
}

// IntelligentWarmup performs cache warmup by loading persisted cache and fetching missing data
func IntelligentWarmup(session *discordgo.Session, cache *UnifiedCache, store *storage.Store, config WarmupConfig) error {
	startTime := time.Now()
	log.ApplicationLogger().Info("?? Starting intelligent cache warmup...")

	ws := newWarmupSession(session)

	// Step 1: Load persisted cache entries from SQLite
	if err := cache.Warmup(); err != nil {
		log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup from persistent cache: %v", err))
	} else {
		members, _, _, _ := cache.MemberMetrics()
		guilds, _, _, _ := cache.GuildMetrics()
		roles, _, _, _ := cache.RolesMetrics()
		channels, _, _, _ := cache.ChannelMetrics()
		log.ApplicationLogger().Info(fmt.Sprintf("? Loaded from persistent cache: %d members, %d guilds, %d roles, %d channels",
			members, guilds, roles, channels))
	}

	// Step 2: Determine which guilds to warmup
	guildIDs := config.GuildIDs
	if len(guildIDs) == 0 {
		// Get all guilds the bot is in
		for _, guild := range ws.StateGuilds() {
			guildIDs = append(guildIDs, guild.ID)
		}
	}

	if len(guildIDs) == 0 {
		log.ApplicationLogger().Warn("No guilds found for warmup")
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf("?? Warming up %d guilds...", len(guildIDs)))

	// Step 3: Warmup each guild
	var totalMembers, totalRoles, totalChannels, totalGuilds int
	for _, guildID := range guildIDs {
		// Fetch missing guild data
		if config.FetchMissingGuilds {
			if err := warmupGuild(ws, cache, guildID); err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup guild %s: %v", guildID, err))
			} else {
				totalGuilds++
			}
		}

		// Fetch missing roles
		if config.FetchMissingRoles {
			rolesCount, err := warmupGuildRoles(ws, cache, store, guildID)
			if err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup roles for guild %s: %v", guildID, err))
			} else {
				totalRoles += rolesCount
			}
		}

		// Fetch missing channels
		if config.FetchMissingChannels {
			channelsCount, err := warmupGuildChannels(ws, cache, guildID)
			if err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup channels for guild %s: %v", guildID, err))
			} else {
				totalChannels += channelsCount
			}
		}

		// Fetch missing members (can be expensive, so we limit it)
		if config.FetchMissingMembers {
			membersCount, err := warmupGuildMembers(ws, cache, store, guildID, config.MaxMembersPerGuild)
			if err != nil {
				log.ApplicationLogger().Warn(fmt.Sprintf("Failed to warmup members for guild %s: %v", guildID, err))
			} else {
				totalMembers += membersCount
			}
		}
	}

	elapsed := time.Since(startTime)
	log.ApplicationLogger().Info(fmt.Sprintf("? Warmup completed in %v: %d guilds, %d members, %d roles, %d channels",
		elapsed, totalGuilds, totalMembers, totalRoles, totalChannels))

	return nil
}

// warmupGuild fetches guild data if not in cache
func warmupGuild(session warmupSession, cache *UnifiedCache, guildID string) error {
	// Check if already cached
	if _, ok := cache.GetGuild(guildID); ok {
		return nil // Already cached
	}

	// Fetch from Discord
	guild, err := session.Guild(guildID)
	if err != nil {
		return fmt.Errorf("fetch guild: %w", err)
	}

	// Cache it
	cache.SetGuild(guildID, guild)
	return nil
}

// warmupGuildRoles fetches roles if not in cache and stores in persistent storage
func warmupGuildRoles(session warmupSession, cache *UnifiedCache, store *storage.Store, guildID string) (int, error) {
	// Check if already cached
	if roles, ok := cache.GetRoles(guildID); ok && len(roles) > 0 {
		return 0, nil // Already cached
	}

	// Fetch from Discord
	roles, err := session.GuildRoles(guildID)
	if err != nil {
		return 0, fmt.Errorf("fetch roles: %w", err)
	}

	// Cache it
	cache.SetRoles(guildID, roles)

	// Store member roles mapping if available
	if store != nil {
		// This is handled by member warmup
	}

	return len(roles), nil
}

// warmupGuildChannels fetches channels if not in cache
func warmupGuildChannels(session warmupSession, cache *UnifiedCache, guildID string) (int, error) {
	// Fetch from Discord (discordgo doesn't provide a simple cache check)
	channels, err := session.GuildChannels(guildID)
	if err != nil {
		return 0, fmt.Errorf("fetch channels: %w", err)
	}

	// Cache each channel
	cachedCount := 0
	for _, channel := range channels {
		// Check if already cached
		if _, ok := cache.GetChannel(channel.ID); !ok {
			cache.SetChannel(channel.ID, channel)
			cachedCount++
		}
	}

	return cachedCount, nil
}

// warmupGuildMembers fetches members if missing from storage and caches them
func warmupGuildMembers(session warmupSession, cache *UnifiedCache, store *storage.Store, guildID string, maxMembers int) (int, error) {
	// Get existing members from storage
	storedMembers := make(map[string]time.Time)
	if store != nil {
		var err error
		storedMembers, err = store.GetAllMemberJoins(guildID)
		if err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to get stored members for guild %s: %v", guildID, err))
		}
	}

	// Fetch members from Discord
	// Use chunking for large guilds
	after := ""
	fetchedCount := 0
	cachedCount := 0
	limit := 1000 // Discord API limit

	for {
		if maxMembers > 0 && fetchedCount >= maxMembers {
			break
		}

		// Adjust limit for last batch
		currentLimit := limit
		if maxMembers > 0 && fetchedCount+limit > maxMembers {
			currentLimit = maxMembers - fetchedCount
		}

		members, err := session.GuildMembers(guildID, after, currentLimit)
		if err != nil {
			return cachedCount, fmt.Errorf("fetch members: %w", err)
		}

		if len(members) == 0 {
			break
		}

		// Process each member
		for _, member := range members {
			fetchedCount++

			// Check if member is already in storage
			_, existsInStorage := storedMembers[member.User.ID]

			// Cache the member
			if _, ok := cache.GetMember(guildID, member.User.ID); !ok {
				cache.SetMember(guildID, member.User.ID, member)
				cachedCount++
			}

			// Store member join time if not exists
			if store != nil && !existsInStorage {
				joinedAt := time.Now().UTC()
				if !member.JoinedAt.IsZero() {
					joinedAt = member.JoinedAt
				}
				if err := store.UpsertMemberJoin(guildID, member.User.ID, joinedAt); err != nil {
					log.ApplicationLogger().Warn(fmt.Sprintf("Failed to store member join: %v", err))
				}
			}

			// Store member roles if not exists
			if store != nil && len(member.Roles) > 0 {
				if err := store.UpsertMemberRoles(guildID, member.User.ID, member.Roles, time.Now().UTC()); err != nil {
					log.ApplicationLogger().Warn(fmt.Sprintf("Failed to store member roles: %v", err))
				}
			}

			// Touch existing members to keep them fresh
			if store != nil && existsInStorage {
				if err := store.TouchMemberJoin(guildID, member.User.ID); err != nil {
					log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member join: %v", err))
				}
				if len(member.Roles) > 0 {
					if err := store.TouchMemberRoles(guildID, member.User.ID); err != nil {
						log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member roles: %v", err))
					}
				}
			}
		}

		// Check if we got all members
		if len(members) < currentLimit {
			break
		}

		// Set after for next batch
		after = members[len(members)-1].User.ID
	}

	return cachedCount, nil
}

// RefreshMemberData refreshes member data for active members in a guild
func RefreshMemberData(session *discordgo.Session, cache *UnifiedCache, store *storage.Store, guildID string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf("?? Refreshing %d members in guild %s", len(userIDs), guildID))

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
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := store.CleanupAllObsoleteData(); err != nil {
					log.ErrorLoggerRaw().Error(fmt.Sprintf("Periodic cleanup failed: %v", err))
				}
			case <-stopChan:

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
		// Touch member join to keep it fresh
		if err := store.TouchMemberJoin(guildID, userID); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member join for %s: %v", userID, err))
		}

		// Touch member roles to keep them fresh
		if err := store.TouchMemberRoles(guildID, userID); err != nil {
			log.ApplicationLogger().Warn(fmt.Sprintf("Failed to touch member roles for %s: %v", userID, err))
		}
	}

	return nil
}
