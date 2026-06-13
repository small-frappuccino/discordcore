package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

// WarmupConfig configures the intelligent warmup behavior
type WarmupConfig struct {
	FetchMissingMembers  bool
	FetchMissingRoles    bool
	FetchMissingGuilds   bool
	FetchMissingChannels bool
	MaxMembersPerGuild   int
	GuildIDs             []string
}

func DefaultWarmupConfig() WarmupConfig {
	return WarmupConfig{
		FetchMissingMembers:  true,
		FetchMissingRoles:    true,
		FetchMissingGuilds:   true,
		FetchMissingChannels: true,
		MaxMembersPerGuild:   1000,
		GuildIDs:             nil,
	}
}

// IntelligentWarmupContext performs cache warmup with cooperative cancellation checks using Arikawa's Cabinet.
func IntelligentWarmupContext(ctx context.Context, st *state.State, store *storage.Store, config WarmupConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("IntelligentWarmupContext: %w", err)
	}

	startTime := time.Now()
	log.ApplicationLogger().Info("🚀 Starting cache warmup (Discord backfill via Arikawa)...")

	// Step 1: Determine which guilds to warmup
	guildIDs := config.GuildIDs
	if len(guildIDs) == 0 {
		// Get all guilds the bot is in
		guilds, err := st.Cabinet.Guilds()
		if err == nil {
			for _, guild := range guilds {
				guildIDs = append(guildIDs, guild.ID.String())
			}
		}
	}

	if len(guildIDs) == 0 {
		log.ApplicationLogger().Warn("No guilds found for warmup")
		return nil
	}

	log.ApplicationLogger().Info(fmt.Sprintf("🔄 Backfilling cache for %d guild(s) from Discord...", len(guildIDs)))

	var totalMembers, totalRoles, totalChannels, totalGuilds int
	for _, guildIDStr := range guildIDs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("IntelligentWarmupContext: %w", err)
		}

		guildID, err := discord.ParseSnowflake(guildIDStr)
		if err != nil {
			continue
		}
		gID := discord.GuildID(guildID)

		// Fetch missing guild data
		if config.FetchMissingGuilds {
			_, err := st.Cabinet.Guild(gID)
			if err != nil { // not found
				guild, err := st.Client.Guild(gID)
				if err == nil {
					st.Cabinet.GuildSet(guild, false)
					totalGuilds++
				}
			}
		}

		// Fetch missing roles
		if config.FetchMissingRoles {
			roles, err := st.Client.Roles(gID)
			if err == nil {
				for i := range roles {
					st.Cabinet.RoleSet(gID, &roles[i], false)
				}
				totalRoles += len(roles)
			}
		}

		// Fetch missing channels
		if config.FetchMissingChannels {
			channels, err := st.Client.Channels(gID)
			if err == nil {
				for i := range channels {
					st.Cabinet.ChannelSet(&channels[i], false)
				}
				totalChannels += len(channels)
			}
		}

		// Fetch missing members
		if config.FetchMissingMembers {
			limit := uint(1000)
			var after discord.UserID = 0
			fetchedCount := 0

			for {
				if err := ctx.Err(); err != nil {
					return fmt.Errorf("warmup members: %w", err)
				}
				if config.MaxMembersPerGuild > 0 && fetchedCount >= config.MaxMembersPerGuild {
					break
				}

				currentLimit := limit
				if config.MaxMembersPerGuild > 0 && fetchedCount+int(limit) > config.MaxMembersPerGuild {
					currentLimit = uint(config.MaxMembersPerGuild - fetchedCount)
				}

				members, err := st.Client.MembersAfter(gID, after, currentLimit)
				if err != nil || len(members) == 0 {
					break
				}

				for i := range members {
					if err := ctx.Err(); err != nil {
						return err
					}
					st.Cabinet.MemberSet(gID, &members[i], false)
					fetchedCount++

					if store != nil {
						joinedAt := time.Now().UTC()
						if members[i].Joined.IsValid() {
							joinedAt = members[i].Joined.Time()
						}
						_ = store.UpsertMemberJoin(guildIDStr, members[i].User.ID.String(), joinedAt)
						if len(members[i].RoleIDs) > 0 {
							var roleStrs []string
							for _, rid := range members[i].RoleIDs {
								roleStrs = append(roleStrs, rid.String())
							}
							_ = store.UpsertMemberRoles(guildIDStr, members[i].User.ID.String(), roleStrs, time.Now().UTC())
						}
					}
				}

				totalMembers += len(members)
				if uint(len(members)) < currentLimit {
					break
				}
				after = members[len(members)-1].User.ID
			}
		}
	}

	elapsed := time.Since(startTime)
	log.ApplicationLogger().Info(fmt.Sprintf("✅ Warmup completed in %v: %d guilds, %d members, %d roles, %d channels",
		elapsed, totalGuilds, totalMembers, totalRoles, totalChannels))

	return nil
}

// RefreshMemberData refreshes member data for active members in a guild
func RefreshMemberData(st *state.State, store *storage.Store, guildIDStr string, userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}

	gSnowflake, err := discord.ParseSnowflake(guildIDStr)
	if err != nil {
		return err
	}
	gID := discord.GuildID(gSnowflake)

	log.ApplicationLogger().Info(fmt.Sprintf("🔄 Refreshing %d members in guild %s", len(userIDs), guildIDStr))

	for _, userIDStr := range userIDs {
		uSnowflake, err := discord.ParseSnowflake(userIDStr)
		if err != nil {
			continue
		}
		uID := discord.UserID(uSnowflake)

		member, err := st.Client.Member(gID, uID)
		if err != nil {
			continue
		}

		st.Cabinet.MemberSet(gID, member, true)

		if store != nil {
			joinedAt := time.Now().UTC()
			if member.Joined.IsValid() {
				joinedAt = member.Joined.Time()
			}
			_ = store.UpsertMemberJoin(guildIDStr, userIDStr, joinedAt)

			if len(member.RoleIDs) > 0 {
				var roleStrs []string
				for _, rid := range member.RoleIDs {
					roleStrs = append(roleStrs, rid.String())
				}
				_ = store.UpsertMemberRoles(guildIDStr, userIDStr, roleStrs, time.Now().UTC())
			}
		}
	}

	return nil
}

// SchedulePeriodicCleanup starts a background goroutine that periodically cleans up obsolete data.
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
		_ = store.TouchMemberJoin(guildID, userID)
		_ = store.TouchMemberRoles(guildID, userID)
	}
	return nil
}
