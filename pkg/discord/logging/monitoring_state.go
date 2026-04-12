package logging

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
)

func (ms *MonitoringService) markEvent(ctx context.Context) {
	if ms.activity == nil {
		return
	}
	ms.activity.MarkEvent(ctx, "monitoring_service")
}

func (ms *MonitoringService) startHeartbeat(ctx context.Context) {
	if ms.activity == nil {
		return
	}
	ms.activity.StartHeartbeat(ctx, heartbeatTickInterval)
}

func (ms *MonitoringService) stopHeartbeat(ctx context.Context) error {
	if ms.activity == nil {
		return nil
	}
	return ms.activity.StopHeartbeat(ctx)
}

func (ms *MonitoringService) rolesCacheCleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ms.cleanupRolesCache()
		case <-ctx.Done():
			return
		case <-ms.rolesCacheCleanup:
			return
		}
	}
}

func (ms *MonitoringService) cleanupRolesCache() {
	now := time.Now()
	var toDelete []string

	ms.rolesCacheMu.RLock()
	for key, entry := range ms.rolesCache {
		if now.After(entry.expiresAt) {
			toDelete = append(toDelete, key)
		}
	}
	ms.rolesCacheMu.RUnlock()

	if len(toDelete) > 0 {
		ms.rolesCacheMu.Lock()
		for _, key := range toDelete {
			delete(ms.rolesCache, key)
		}
		ms.rolesCacheMu.Unlock()
		log.ApplicationLogger().Info("Cleaned up expired roles cache entries", "count", len(toDelete))
	}
}

func (ms *MonitoringService) cacheRolesSet(guildID, userID string, roles []string) {
	key := guildID + ":" + userID
	if len(roles) == 0 {
		ms.rolesCacheMu.Lock()
		delete(ms.rolesCache, key)
		ms.rolesCacheMu.Unlock()
		return
	}
	ttl := ms.rolesTTL
	if ms.configManager != nil {
		if gcfg := ms.configManager.GuildConfig(guildID); gcfg != nil {
			if d := gcfg.RolesCacheTTLDuration(); d > 0 {
				ttl = d
			}
		}
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	ms.rolesCacheMu.Lock()
	ms.rolesCache[key] = cachedRoles{
		roles:     append([]string(nil), roles...),
		expiresAt: time.Now().Add(ttl),
	}
	ms.rolesCacheMu.Unlock()
}

func (ms *MonitoringService) cacheRolesGet(guildID, userID string) ([]string, bool) {
	key := guildID + ":" + userID
	ms.rolesCacheMu.RLock()
	entry, ok := ms.rolesCache[key]
	ms.rolesCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			ms.rolesCacheMu.Lock()
			delete(ms.rolesCache, key)
			ms.rolesCacheMu.Unlock()
		}
		return nil, false
	}
	out := append([]string(nil), entry.roles...)
	return out, true
}

func (ms *MonitoringService) GetCacheStats() map[string]interface{} {
	ms.rolesCacheMu.RLock()
	size := len(ms.rolesCache)
	ms.rolesCacheMu.RUnlock()
	ms.roleUpdateAuditMu.Lock()
	roleAuditCacheSize := len(ms.roleUpdateAuditCache)
	roleAuditDebounceSize := len(ms.roleUpdateAuditDebounce)
	ms.roleUpdateAuditMu.Unlock()
	ttl := ms.rolesTTL
	isRunning := ms.IsRunning()

	stats := map[string]interface{}{
		"isRunning":                   isRunning,
		"rolesCacheSize":              size,
		"rolesCacheTTLSeconds":        int(ttl.Seconds()),
		"roleUpdateAuditCacheSize":    roleAuditCacheSize,
		"roleUpdateAuditDebounceSize": roleAuditDebounceSize,
		"apiAuditLogCalls":            atomic.LoadUint64(&ms.apiAuditLogCalls),
		"apiGuildMemberCalls":         atomic.LoadUint64(&ms.apiGuildMemberCalls),
		"apiMessagesSent":             atomic.LoadUint64(&ms.apiMessagesSent),
		"cacheStateMemberHits":        atomic.LoadUint64(&ms.cacheStateMemberHits),
		"cacheRolesMemoryHits":        atomic.LoadUint64(&ms.cacheRolesMemoryHits),
		"cacheRolesStoreHits":         atomic.LoadUint64(&ms.cacheRolesStoreHits),
		"cacheRoleAuditHits":          atomic.LoadUint64(&ms.cacheRoleAuditHits),
	}

	if ms.unifiedCache != nil {
		ucStats := ms.unifiedCache.GetStats()
		stats["unifiedCache"] = ucStats

		var memberEntries, guildEntries, rolesEntries, channelEntries int
		var memberHits, memberMisses, guildHits, guildMisses, rolesHits, rolesMisses, channelHits, channelMisses uint64

		if ucStats.CustomMetrics != nil {
			if v, ok := ucStats.CustomMetrics["memberEntries"]; ok {
				switch t := v.(type) {
				case int:
					memberEntries = t
				case int64:
					memberEntries = int(t)
				case float64:
					memberEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["guildEntries"]; ok {
				switch t := v.(type) {
				case int:
					guildEntries = t
				case int64:
					guildEntries = int(t)
				case float64:
					guildEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesEntries"]; ok {
				switch t := v.(type) {
				case int:
					rolesEntries = t
				case int64:
					rolesEntries = int(t)
				case float64:
					rolesEntries = int(t)
				}
			}
			if v, ok := ucStats.CustomMetrics["channelEntries"]; ok {
				switch t := v.(type) {
				case int:
					channelEntries = t
				case int64:
					channelEntries = int(t)
				case float64:
					channelEntries = int(t)
				}
			}

			if v, ok := ucStats.CustomMetrics["memberHits"]; ok {
				switch t := v.(type) {
				case uint64:
					memberHits = t
				case int:
					if t >= 0 {
						memberHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						memberHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						memberHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["memberMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					memberMisses = t
				case int:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						memberMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["guildHits"]; ok {
				switch t := v.(type) {
				case uint64:
					guildHits = t
				case int:
					if t >= 0 {
						guildHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						guildHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						guildHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["guildMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					guildMisses = t
				case int:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						guildMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesHits"]; ok {
				switch t := v.(type) {
				case uint64:
					rolesHits = t
				case int:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						rolesHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["rolesMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					rolesMisses = t
				case int:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						rolesMisses = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["channelHits"]; ok {
				switch t := v.(type) {
				case uint64:
					channelHits = t
				case int:
					if t >= 0 {
						channelHits = uint64(t)
					}
				case int64:
					if t >= 0 {
						channelHits = uint64(t)
					}
				case float64:
					if t >= 0 {
						channelHits = uint64(t)
					}
				}
			}
			if v, ok := ucStats.CustomMetrics["channelMisses"]; ok {
				switch t := v.(type) {
				case uint64:
					channelMisses = t
				case int:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				case int64:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				case float64:
					if t >= 0 {
						channelMisses = uint64(t)
					}
				}
			}
		}

		stats["unifiedCacheSpecific"] = map[string]interface{}{
			"memberEntries":  memberEntries,
			"guildEntries":   guildEntries,
			"rolesEntries":   rolesEntries,
			"channelEntries": channelEntries,
			"memberHits":     memberHits,
			"memberMisses":   memberMisses,
			"guildHits":      guildHits,
			"guildMisses":    guildMisses,
			"rolesHits":      rolesHits,
			"rolesMisses":    rolesMisses,
			"channelHits":    channelHits,
			"channelMisses":  channelMisses,
		}
	}

	return stats
}

func (ms *MonitoringService) handleStartupDowntimeAndMaybeRefresh(ctx context.Context) error {
	if ms.store == nil {
		return nil
	}
	type heartbeatState struct {
		at time.Time
		ok bool
	}
	hb, err := monitoringRunWithTimeout(ctx, monitoringPersistenceTimeout, func() (heartbeatState, error) {
		at, ok, err := ms.getHeartbeat()
		return heartbeatState{at: at, ok: ok}, err
	})
	lastHB := hb.at
	okHB := hb.ok
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to read last heartbeat; skipping downtime check", "err", err)
	} else {
		if !okHB || time.Since(lastHB) > downtimeThreshold {
			downtimeDuration := "unknown"
			if okHB {
				downtimeDuration = time.Since(lastHB).Round(time.Second).String()
			}
			log.ApplicationLogger().Info("⏱️ Detected downtime; performing silent avatar refresh before enabling notifications", "downtime", downtimeDuration, "threshold", downtimeThreshold.String())
			cfg := ms.scopedConfig()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.ApplicationLogger().Info("No configured guilds for startup silent refresh")
				return nil
			}
			startTime := time.Now()
			guildIDs := make([]string, 0, len(cfg.Guilds))
			for _, gcfg := range cfg.Guilds {
				if gid := strings.TrimSpace(gcfg.GuildID); gid != "" {
					guildIDs = append(guildIDs, gid)
				}
			}
			if err := runGuildTasksWithLimit(ctx, guildIDs, monitoringMaxConcurrentGuildScan, func(runCtx context.Context, guildID string) error {
				return ms.initializeGuildCacheContext(runCtx, guildID)
			}); err != nil {
				return err
			}
			log.ApplicationLogger().Info("✅ Silent avatar refresh completed", "duration", time.Since(startTime).Round(time.Millisecond))
			return nil
		}
	}
	log.ApplicationLogger().Info("No significant downtime detected; skipping heavy avatar refresh")
	return nil
}

type guildMemberPageFetcher func(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error)

func paginateGuildMembersContext(
	ctx context.Context,
	guildID string,
	pageSize int,
	fetch guildMemberPageFetcher,
	handle func([]*discordgo.Member) error,
) (int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if pageSize <= 0 {
		pageSize = monitoringGuildMembersPageSize
	}
	if fetch == nil {
		return 0, fmt.Errorf("guild member fetcher is nil")
	}

	total := 0
	after := ""
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		members, err := fetch(ctx, guildID, after, pageSize)
		if err != nil {
			return total, err
		}
		if len(members) == 0 {
			return total, nil
		}
		if handle != nil {
			if err := handle(members); err != nil {
				return total, err
			}
		}
		total += len(members)
		if len(members) < pageSize {
			return total, nil
		}
		last := members[len(members)-1]
		if last == nil || last.User == nil || strings.TrimSpace(last.User.ID) == "" {
			return total, fmt.Errorf("paginate guild members: invalid page tail for guild %s", guildID)
		}
		after = last.User.ID
	}
}

func (ms *MonitoringService) fetchGuildMemberPageContext(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error) {
	if ms == nil || ms.session == nil {
		return nil, fmt.Errorf("discord session is unavailable")
	}
	if limit <= 0 {
		limit = monitoringGuildMembersPageSize
	}
	return monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() ([]*discordgo.Member, error) {
		return ms.session.GuildMembers(guildID, after, limit)
	})
}

func (ms *MonitoringService) forEachGuildMemberPageContext(ctx context.Context, guildID string, handle func([]*discordgo.Member) error) (int, error) {
	total, err := paginateGuildMembersContext(ctx, guildID, monitoringGuildMembersPageSize, ms.fetchGuildMemberPageContext, handle)
	if err != nil {
		log.ErrorLoggerRaw().Error("Failed to paginate guild members", "guildID", guildID, "fetched_so_far", total, "err", err)
		return total, err
	}
	log.ApplicationLogger().Info("Pagination completed successfully", "guildID", guildID, "total_members_fetched", total)
	return total, nil
}

// fetchAllGuildMembers paginates through all guild members until exhaustion and materializes them in memory.
func (ms *MonitoringService) fetchAllGuildMembers(guildID string) ([]*discordgo.Member, error) {
	return ms.fetchAllGuildMembersContext(context.Background(), guildID)
}

func (ms *MonitoringService) fetchAllGuildMembersContext(ctx context.Context, guildID string) ([]*discordgo.Member, error) {
	all := make([]*discordgo.Member, 0)
	_, err := ms.forEachGuildMemberPageContext(ctx, guildID, func(members []*discordgo.Member) error {
		all = append(all, members...)
		return nil
	})
	if err != nil {
		return all, err
	}
	return all, nil
}

func (ms *MonitoringService) performPeriodicCheck(ctx context.Context) error {
	log.ApplicationLogger().Info("Running periodic avatar check...")
	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		log.ApplicationLogger().Info("No configured guilds for periodic check")
		return nil
	}
	for _, gcfg := range cfg.Guilds {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := ms.forEachGuildMemberPageContext(ctx, gcfg.GuildID, func(members []*discordgo.Member) error {
			joinSnapshots := make([]storage.GuildMemberSnapshot, 0, len(members))
			for _, member := range members {
				if err := ctx.Err(); err != nil {
					return err
				}
				if member == nil || member.User == nil {
					continue
				}
				if ms.store != nil && !member.JoinedAt.IsZero() {
					joinSnapshots = append(joinSnapshots, storage.GuildMemberSnapshot{
						UserID:   member.User.ID,
						JoinedAt: member.JoinedAt,
						IsBot:    member.User.Bot,
						HasBot:   true,
					})
				}
			}
			if ms.store != nil && len(joinSnapshots) > 0 {
				if err := ms.store.UpsertGuildMemberSnapshotsContext(ctx, gcfg.GuildID, joinSnapshots, time.Now().UTC()); err != nil {
					log.ApplicationLogger().Warn(
						"Periodic check: failed to backfill member join page",
						"operation", "monitoring.periodic_check.persist_joins_page",
						"guildID", gcfg.GuildID,
						"members", len(joinSnapshots),
						"err", err,
					)
				}
			}

			for _, member := range members {
				if err := ctx.Err(); err != nil {
					return err
				}
				if member == nil || member.User == nil {
					continue
				}

				avatarHash := member.User.Avatar
				if avatarHash == "" {
					avatarHash = "default"
				}
				ms.checkAvatarChange(gcfg.GuildID, member.User.ID, avatarHash, member.User.Username)
			}
			return nil
		})
		if err != nil {
			log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", gcfg.GuildID, "err", err)
			continue
		}
	}
	return nil
}

func runGuildTasksWithLimit(ctx context.Context, guildIDs []string, limit int, fn func(context.Context, string) error) error {
	if fn == nil || len(guildIDs) == 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if limit <= 0 {
		limit = 1
	}

	sem := make(chan struct{}, limit)
	errCh := make(chan error, len(guildIDs))
	var wg sync.WaitGroup

	for _, guildID := range guildIDs {
		guildID = strings.TrimSpace(guildID)
		if guildID == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			break
		}
		sem <- struct{}{}
		wg.Add(1)
		go func(gid string) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := fn(ctx, gid); err != nil {
				errCh <- err
			}
		}(guildID)
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return ctx.Err()
}

// MemberEvents exposes the member event sub-service.
func (ms *MonitoringService) MemberEvents() *MemberEventService {
	return ms.memberEventService
}

// MessageEvents exposes the message event sub-service.
func (ms *MonitoringService) MessageEvents() *MessageEventService {
	return ms.messageEventService
}

// Notifier exposes the notification sender used by monitoring.
func (ms *MonitoringService) Notifier() *NotificationSender {
	return ms.notifier
}

// CacheManager exposes the avatar cache manager used by monitoring.
func (ms *MonitoringService) Store() *storage.Store {
	return ms.store
}

// GetUnifiedCache exposes the unified cache for use by other components
func (ms *MonitoringService) GetUnifiedCache() *cache.UnifiedCache {
	return ms.unifiedCache
}

func (ms *MonitoringService) TaskRouter() *task.TaskRouter {
	return ms.router
}
