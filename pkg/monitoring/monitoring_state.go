package monitoring

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/log"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"

	"github.com/small-frappuccino/discordcore/pkg/notifications"
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

// metricsRows returns the monitoring-local display rows surfaced via
// Service.Stats().Metrics (and therefore /admin status). Cache observability
// lives on the typed cache.CacheMetricsSnapshot exposed at /v1/health/cache;
// API/cache counters live on MetricsSnapshot exposed at /v1/health/monitoring.
// What remains here is monitoring-local bookkeeping that no other endpoint
// covers (roles cache size, audit dedup state, run flag), plus a one-line
// mirror of the headline counters so /admin status stays useful inside
// Discord without curling the control plane.
//
// Rows are appended in display order; callers MUST iterate in slice order
// (the consumer contract on ServiceMetric).
func (ms *MonitoringService) metricsRows() []svc.ServiceMetric {
	size := ms.rolesCacheService.CacheRolesSize()
	roleAuditCacheSize, roleAuditDebounceSize := ms.rolesCacheService.AuditSizes()

	rows := []svc.ServiceMetric{
		{Label: "Running", Value: formatBoolYesNo(ms.IsRunning())},
		{Label: "Roles cache size", Value: strconv.Itoa(size)},
		{Label: "Roles cache default TTL", Value: (5 * time.Minute).String()},
		{Label: "Role update audit cache size", Value: strconv.Itoa(roleAuditCacheSize)},
		{Label: "Role update audit debounce size", Value: strconv.Itoa(roleAuditDebounceSize)},
	}

	if provider, ok := ms.observability().(SnapshotProvider); ok {
		snap := provider.Snapshot()
		rows = append(rows,
			svc.ServiceMetric{Label: "API audit log calls", Value: strconv.FormatInt(snap.API.AuditLogCallsTotal, 10)},
			svc.ServiceMetric{Label: "API guild member calls", Value: strconv.FormatInt(snap.API.GuildMemberCallsTotal, 10)},
			svc.ServiceMetric{Label: "API messages sent", Value: strconv.FormatInt(snap.API.MessagesSentTotal, 10)},
			svc.ServiceMetric{Label: "State member cache hits", Value: strconv.FormatInt(snap.Cache.StateMemberHitsTotal, 10)},
			svc.ServiceMetric{Label: "Roles cache memory hits", Value: strconv.FormatInt(snap.Cache.RolesMemoryHitsTotal, 10)},
			svc.ServiceMetric{Label: "Roles cache store hits", Value: strconv.FormatInt(snap.Cache.RolesStoreHitsTotal, 10)},
			svc.ServiceMetric{Label: "Roles audit cache hits", Value: strconv.FormatInt(snap.Cache.RolesAuditHitsTotal, 10)},
		)
	}

	if ms.unifiedCache != nil {
		// Snapshot is intentionally called without a store: persisted cache
		// totals belong on /v1/health/cache, not on the monitoring rows. The
		// in-memory segment summaries are what makes /admin status useful,
		// and they require no DB call.
		uc := ms.unifiedCache.Snapshot(context.Background(), nil)
		rows = append(rows,
			svc.ServiceMetric{Label: "Cache members", Value: formatSegmentSummary(uc.Members)},
			svc.ServiceMetric{Label: "Cache guilds", Value: formatSegmentSummary(uc.Guilds)},
			svc.ServiceMetric{Label: "Cache roles", Value: formatSegmentSummary(uc.Roles)},
			svc.ServiceMetric{Label: "Cache channels", Value: formatSegmentSummary(uc.Channels)},
		)
	}

	return rows
}

// formatBoolYesNo renders a boolean as a human label for the /admin status
// display rows. The display-side helpers live next to metricsRows because
// they are coupled to the row vocabulary, not generally reusable.
func formatBoolYesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

// formatSegmentSummary renders a UnifiedCache SegmentSnapshot into a
// single-line "<entries> entries · <hits> hits · <hit_rate>% hit rate" form
// for /admin status. The Discord client only sees a short string per row, so
// embedding rate and totals in one line keeps the surface compact.
func formatSegmentSummary(segment cache.SegmentSnapshot) string {
	if segment.Hits+segment.Misses == 0 {
		return fmt.Sprintf("%d entries", segment.Entries)
	}
	return fmt.Sprintf(
		"%d entries · %d hits · %.1f%% hit rate",
		segment.Entries,
		segment.Hits,
		segment.HitRate*100,
	)
}

func (ms *MonitoringService) handleStartupDowntimeAndMaybeRefresh(ctx context.Context) error {
	if ms.store == nil {
		return nil
	}
	type heartbeatState struct {
		at time.Time
		ok bool
	}
	hb, err := RunWithTimeout(ctx, monitoringPersistenceTimeout, func() (heartbeatState, error) {
		at, ok, err := ms.getHeartbeat(ctx)
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
			log.ApplicationLogger().Info("⏱️ Detected downtime; relying on background cache warmup and Gateway events for hydration", "downtime", downtimeDuration, "threshold", downtimeThreshold.String())
			cfg := ms.scopedConfig()
			if cfg == nil || len(cfg.Guilds) == 0 {
				log.ApplicationLogger().Info("No configured guilds for startup silent refresh")
				return nil
			}
			// Background cache warmup worker and live Gateway events handle hydration dynamically.
			// Removed heavy inline pagination to eliminate startup blocking.
			return nil
		}
	}
	log.ApplicationLogger().Info("No significant downtime detected; skipping heavy avatar refresh")
	return nil
}

type guildMemberPageFetcher func(ctx context.Context, guildID, after string, limit int) ([]*discordgo.Member, error)

// StreamGuildMembersContext returns an iterator yielding individual guild members.
// The backing slice of []*discordgo.Member is allocated freshly per page by the discord client,
// and this wrapper yields them incrementally to eliminate inner loop nesting for consumers.
func (ms *MonitoringService) StreamGuildMembersContext(ctx context.Context, guildID string) iter.Seq2[*discordgo.Member, error] {
	return func(yield func(*discordgo.Member, error) bool) {
		if ms == nil || ms.session == nil {
			yield(nil, fmt.Errorf("discord session is unavailable"))
			return
		}
		if ctx == nil {
			ctx = context.Background()
		}
		pageSize := monitoringGuildMembersPageSize

		after := ""
		total := 0
		for {
			if err := ctx.Err(); err != nil {
				yield(nil, fmt.Errorf("StreamGuildMembersContext: %w", err))
				return
			}
			members, err := RunWithTimeout(ctx, DependencyTimeout, func() ([]*discordgo.Member, error) {
				return ms.session.GuildMembers(guildID, after, pageSize)
			})
			if err != nil {
				yield(nil, fmt.Errorf("StreamGuildMembersContext: %w", err))
				return
			}
			if len(members) == 0 {
				log.ApplicationLogger().Info("Pagination completed successfully", "guildID", guildID, "total_members_fetched", total)
				return
			}
			total += len(members)
			for _, m := range members {
				if !yield(m, nil) {
					return
				}
			}
			if len(members) < pageSize {
				log.ApplicationLogger().Info("Pagination completed successfully", "guildID", guildID, "total_members_fetched", total)
				return
			}
			last := members[len(members)-1]
			if last == nil || last.User == nil || strings.TrimSpace(last.User.ID) == "" {
				yield(nil, fmt.Errorf("stream guild members: invalid page tail for guild %s", guildID))
				return
			}
			after = last.User.ID
		}
	}
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
			return fmt.Errorf("MonitoringService.performPeriodicCheck: %w", err)
		}
		var joinSnapshots []storage.GuildMemberSnapshot
		flush := func() {
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
			joinSnapshots = joinSnapshots[:0]
		}

		for member, err := range ms.StreamGuildMembersContext(ctx, gcfg.GuildID) {
			if err != nil {
				log.ErrorLoggerRaw().Error("Error getting members for guild", "guildID", gcfg.GuildID, "err", err)
				continue
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

			avatarHash := member.User.Avatar
			if avatarHash == "" {
				avatarHash = "default"
			}
			ms.checkAvatarChange(gcfg.GuildID, member.User.ID, avatarHash, member.User.Username)

			if len(joinSnapshots) >= monitoringGuildMembersPageSize {
				flush()
			}
		}
		flush()
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

// Notifier exposes the notification sender used by monitoring.
func (ms *MonitoringService) Notifier() *notifications.NotificationSender {
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

// TaskRouter tasks router.
func (ms *MonitoringService) TaskRouter() *task.TaskRouter {
	return ms.router
}
