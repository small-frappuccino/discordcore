package monitoring

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/task"
	"github.com/small-frappuccino/discordgo"
)

const (
	// TaskTypeMonitorBackfillEntryExitDay names the task that scans a single UTC day
	// of an entry/exit channel for join/leave events. Payload must be
	// [BackfillEntryExitDayPayload]; dispatchers and the handler share that type so
	// the type-assertion is a single point of contract.
	TaskTypeMonitorBackfillEntryExitDay = "monitor.backfill_entry_exit_day"

	// TaskTypeMonitorBackfillEntryExitRange names the task that scans an arbitrary
	// UTC time range of an entry/exit channel. Payload must be
	// [BackfillEntryExitRangePayload].
	TaskTypeMonitorBackfillEntryExitRange = "monitor.backfill_entry_exit_range"
)

// BackfillEntryExitDayPayload carries the channel and target UTC day for a
// [TaskTypeMonitorBackfillEntryExitDay] dispatch. Day uses the YYYY-MM-DD form.
type BackfillEntryExitDayPayload struct {
	ChannelID string
	Day       string
}

// BackfillEntryExitRangePayload carries the channel and the inclusive UTC range
// for a [TaskTypeMonitorBackfillEntryExitRange] dispatch. Start and End are
// RFC3339 timestamps; End must be strictly after Start.
type BackfillEntryExitRangePayload struct {
	ChannelID string
	Start     string
	End       string
}

// registerBackfillHandlers wires the entry/exit backfill task handlers and, once at
// startup, dispatches any backfill work implied by the current runtime config.
func (ms *MonitoringService) registerBackfillHandlers(serviceCtx context.Context, workload monitoringWorkloadState) {
	ms.router.RegisterHandler(TaskTypeMonitorBackfillEntryExitDay, func(_ context.Context, payload any) error {
		return ms.handleBackfillEntryExitDay(serviceCtx, payload)
	})
	ms.router.RegisterHandler(TaskTypeMonitorBackfillEntryExitRange, func(_ context.Context, payload any) error {
		return ms.handleBackfillEntryExitRange(serviceCtx, payload)
	})

	ms.dispatchStartupBackfills(serviceCtx)
}

// handleBackfillEntryExitDay scans a single UTC day. An empty day defaults to today.
func (ms *MonitoringService) handleBackfillEntryExitDay(serviceCtx context.Context, payload any) error {
	p, ok := payload.(BackfillEntryExitDayPayload)
	if !ok {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Invalid payload type for "+TaskTypeMonitorBackfillEntryExitDay, slog.String("type", fmt.Sprintf("%T", payload)))
		return nil
	}
	channelID := strings.TrimSpace(p.ChannelID)
	if channelID == "" {
		return nil
	}
	day := strings.TrimSpace(p.Day)
	if day == "" {
		day = time.Now().UTC().Format("2006-01-02")
	}
	start, err := time.Parse("2006-01-02", day)
	if err != nil {
		return nil
	}
	return ms.runEntryExitBackfill(serviceCtx, channelID, start, start.Add(24*time.Hour), "day")
}

// handleBackfillEntryExitRange scans an explicit RFC3339 [start, end) window; it is
// used for downtime recovery and historical scans.
func (ms *MonitoringService) handleBackfillEntryExitRange(serviceCtx context.Context, payload any) error {
	p, ok := payload.(BackfillEntryExitRangePayload)
	if !ok {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Invalid payload type for "+TaskTypeMonitorBackfillEntryExitRange, slog.String("type", fmt.Sprintf("%T", payload)))
		return nil
	}

	channelID := strings.TrimSpace(p.ChannelID)
	startRaw := strings.TrimSpace(p.Start)
	endRaw := strings.TrimSpace(p.End)
	if channelID == "" || startRaw == "" || endRaw == "" {
		ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Missing required fields for backfill range", slog.String("channelID", channelID), slog.String("start", startRaw), slog.String("end", endRaw))
		return nil
	}

	start, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to parse start date for backfill range", slog.Any("err", err), slog.String("start", startRaw))
		return nil
	}
	end, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to parse end date for backfill range", slog.Any("err", err), slog.String("end", endRaw))
		return nil
	}
	start = start.UTC()
	end = end.UTC()
	if !end.After(start) {
		ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "End date must be after start date for backfill range", slog.Any("start", start), slog.Any("end", end))
		return nil
	}
	return ms.runEntryExitBackfill(serviceCtx, channelID, start, end, "range")
}

// runEntryExitBackfill pages channelID newest-to-oldest and persists the member
// join/leave counters derived from log messages whose timestamp falls in [start, end).
// mode distinguishes the originating handler ("day" or "range") in operational logs.
func (ms *MonitoringService) runEntryExitBackfill(serviceCtx context.Context, channelID string, start, end time.Time, mode string) error {
	guildID := ms.resolveGuildIDForChannel(channelID)
	if guildID == "" {
		ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Could not resolve guild ID for channel during backfill", slog.String("mode", mode), slog.String("channelID", channelID))
		return nil
	}

	ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "📥 Starting entry/exit backfill", slog.String("mode", mode), slog.String("channelID", channelID), slog.String("guildID", guildID), slog.String("start", start.Format(time.RFC3339)), slog.String("end", end.Format(time.RFC3339)))

	botID := ""
	if ms.session != nil && ms.session.State != nil && ms.session.State.User != nil {
		botID = ms.session.State.User.ID
	}

	var before string
	processedCount := 0
	eventsFound := 0
	startTime := time.Now()

	for {
		if err := serviceCtx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.runEntryExitBackfill: %w", err)
		}
		msgs, err := RunWithTimeout(serviceCtx, DependencyTimeout, func() ([]*discordgo.Message, error) {
			return ms.session.ChannelMessages(channelID, 100, before, "", "")
		})
		if err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to fetch channel messages for backfill", slog.String("mode", mode), slog.String("channelID", channelID), slog.Any("err", err))
			break
		}
		if len(msgs) == 0 {
			break
		}

		page, err := ms.applyBackfillPage(serviceCtx, backfillScope{GuildID: guildID, ChannelID: channelID, BotID: botID, Mode: mode}, msgs, start, end)
		if err != nil {
			return err
		}
		processedCount += page.processed
		eventsFound += page.eventsFound

		if processedCount%500 == 0 || processedCount < 500 && processedCount%100 == 0 {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "⏳ Backfill in progress...", slog.String("mode", mode), slog.String("channelID", channelID), slog.Int("processed", processedCount), slog.Int("events_found", eventsFound))
		}

		before = msgs[len(msgs)-1].ID
		if page.reachedStart {
			break
		}
	}

	ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "✅ Backfill completed", slog.String("mode", mode), slog.String("channelID", channelID), slog.Int("processed", processedCount), slog.Int("events_found", eventsFound), slog.Any("duration", time.Since(startTime).Round(time.Millisecond)))
	return nil
}

// backfillPageResult summarizes one processed page of channel messages: the messages
// counted toward progress, the recognized join/leave events persisted, and whether a
// message older than the window start was seen — the signal for the caller to stop paging.
type backfillPageResult struct {
	processed    int
	eventsFound  int
	reachedStart bool
}

// backfillScope identifies the channel/bot context shared across a backfill
// pass: the guild and channel scanned, the bot ID whose log messages are
// parsed, and the mode label used in progress logs.
type backfillScope struct {
	GuildID   string
	ChannelID string
	BotID     string
	Mode      string
}

// applyBackfillPage processes a single page of messages ordered newest-to-oldest,
// persisting the member join/leave events whose timestamp falls in [start, end). It
// returns as soon as it encounters a message older than start, with reachedStart set,
// so the caller can stop paging without a separate loop-control flag. A canceled
// serviceCtx aborts mid-page and surfaces the wrapped context error.
func (ms *MonitoringService) applyBackfillPage(serviceCtx context.Context, scope backfillScope, msgs []*discordgo.Message, start, end time.Time) (backfillPageResult, error) {
	var res backfillPageResult
	for _, m := range msgs {
		if err := serviceCtx.Err(); err != nil {
			return res, fmt.Errorf("MonitoringService.applyBackfillPage: %w", err)
		}
		t := m.Timestamp.UTC()
		if t.Before(start) {
			res.reachedStart = true
			return res, nil
		}
		if !t.Before(end) {
			res.processed++
			continue
		}
		if ms.persistBackfillMessage(serviceCtx, scope, m, t) {
			res.eventsFound++
		}
		res.processed++
	}
	return res, nil
}

// persistBackfillMessage parses a single backfilled log message and, when it encodes
// a member join or leave, updates the corresponding counters and the per-channel
// progress marker. It reports whether the message yielded a recognized event.
func (ms *MonitoringService) persistBackfillMessage(serviceCtx context.Context, scope backfillScope, m *discordgo.Message, t time.Time) bool {
	if ms.store == nil {
		return false
	}
	var rc files.RuntimeConfig
	if cfg := ms.configManager.Config(); cfg != nil {
		rc = cfg.ResolveRuntimeConfig(scope.GuildID)
	}
	evt, userID, ok := parseEntryExitBackfillMessage(m, scope.BotID, rc)
	if !ok {
		return false
	}

	switch evt {
	case "join":
		if err := ms.store.UpsertMemberJoin(scope.GuildID, userID, t); err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Backfill: failed to persist member join", slog.String("mode", scope.Mode), slog.String("guildID", scope.GuildID), slog.String("channelID", scope.ChannelID), slog.String("userID", userID), slog.Any("at", t), slog.Any("err", err))
		}
		if err := ms.store.IncrementDailyMemberJoin(scope.GuildID, userID, t); err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Backfill: failed to increment daily member join", slog.String("mode", scope.Mode), slog.String("guildID", scope.GuildID), slog.String("channelID", scope.ChannelID), slog.String("userID", userID), slog.Any("at", t), slog.Any("err", err))
		}
	case "leave":
		// If the member is no longer in the guild, count the leave for the day.
		if !ms.memberStillInGuild(serviceCtx, scope.GuildID, userID) {
			if err := ms.store.IncrementDailyMemberLeave(scope.GuildID, userID, t); err != nil {
				ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Backfill: failed to increment daily member leave", slog.String("mode", scope.Mode), slog.String("guildID", scope.GuildID), slog.String("channelID", scope.ChannelID), slog.String("userID", userID), slog.Any("at", t), slog.Any("err", err))
			}
		}
	}

	if err := ms.store.SetMetadata(serviceCtx, "backfill_progress:"+scope.ChannelID, t); err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelWarn, "Backfill: failed to persist progress metadata", slog.String("mode", scope.Mode), slog.String("guildID", scope.GuildID), slog.String("channelID", scope.ChannelID), slog.Any("at", t), slog.Any("err", err))
	}
	return true
}

// memberStillInGuild reports whether userID is currently a member of guildID,
// treating any lookup failure as "not present".
func (ms *MonitoringService) memberStillInGuild(serviceCtx context.Context, guildID, userID string) bool {
	if ms.session == nil {
		return false
	}
	mem, err := RunWithTimeout(serviceCtx, DependencyTimeout, func() (*discordgo.Member, error) {
		return ms.session.GuildMember(guildID, userID)
	})
	return err == nil && mem != nil
}

// resolveGuildIDForChannel resolves the guild that owns channelID, preferring the
// session state cache and falling back to a REST lookup. It returns "" when the
// channel cannot be resolved.
func (ms *MonitoringService) resolveGuildIDForChannel(channelID string) string {
	if ms.session == nil {
		return ""
	}
	if ms.session.State != nil {
		if ch, _ := ms.session.State.Channel(channelID); ch != nil && ch.GuildID != "" {
			return ch.GuildID
		}
	}
	if ch, err := ms.session.Channel(channelID); err == nil && ch != nil {
		return ch.GuildID
	}
	return ""
}

// backfillTarget is a channel the startup scan may dispatch backfill work for,
// paired with its resolved runtime config and feature-gate state.
type backfillTarget struct {
	ChannelID      string
	RC             files.RuntimeConfig
	FeatureEnabled bool
}

// dispatchStartupBackfills inspects the runtime config once at startup and dispatches
// the appropriate backfill task per configured channel: a day scan when a start day is
// set, an initial range scan on first run, or a downtime-recovery range scan otherwise.
func (ms *MonitoringService) dispatchStartupBackfills(serviceCtx context.Context) {
	cfg := ms.scopedConfig()
	if cfg == nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "Backfill skip: config manager or config is nil")
		return
	}

	targets := ms.collectBackfillTargets(cfg)
	if len(targets) == 0 {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "No target channels for backfill check")
		return
	}

	lastEvent, hasLastEvent, err := ms.getLastEvent(serviceCtx)
	if err != nil {
		lastEvent = time.Time{}
		hasLastEvent = false
		ms.logger.LogAttrs(context.Background(), slog.LevelError,
			"Failed to read last event for backfill recovery; downtime recovery disabled for this startup",
			slog.String("operation", "monitoring.start.backfill.get_last_event"),
			slog.Any("err", err),
		)
	}
	now := time.Now().UTC()

	for _, target := range targets {
		ms.dispatchBackfillForTarget(serviceCtx, target, lastEvent, hasLastEvent, now)
	}
}

// collectBackfillTargets gathers the global backfill channel (if configured) and every
// guild-scoped backfill channel into a single target list.
func (ms *MonitoringService) collectBackfillTargets(cfg *files.BotConfig) []backfillTarget {
	globalRC := cfg.RuntimeConfig
	targets := make([]backfillTarget, 0)

	if globalRC.BackfillChannelID != "" {
		targets = append(targets, backfillTarget{
			ChannelID:      strings.TrimSpace(globalRC.BackfillChannelID),
			RC:             globalRC,
			FeatureEnabled: true,
		})
	}

	for _, g := range cfg.Guilds {
		cid := g.Channels.BackfillChannelID()
		if cid == "" {
			continue
		}
		targets = append(targets, backfillTarget{
			ChannelID:      cid,
			RC:             cfg.ResolveRuntimeConfig(g.GuildID),
			FeatureEnabled: true,
		})
	}

	return targets
}

// dispatchBackfillForTarget decides and dispatches the backfill work for a single
// target channel based on its config, persisted progress, and observed downtime.
func (ms *MonitoringService) dispatchBackfillForTarget(serviceCtx context.Context, target backfillTarget, lastEvent time.Time, hasLastEvent bool, now time.Time) {
	cid := target.ChannelID
	rc := target.RC

	if !target.FeatureEnabled {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "Backfill disabled by features.backfill.enabled", slog.String("channelID", cid))
		return
	}

	day := strings.TrimSpace(rc.BackfillStartDay)
	if day != "" {
		err := ms.dispatchBackfillTask(serviceCtx, task.Task{
			Type:    TaskTypeMonitorBackfillEntryExitDay,
			Payload: BackfillEntryExitDayPayload{ChannelID: cid, Day: day},
			Options: task.TaskOptions{GroupKey: "backfill:" + cid},
		})
		if err != nil {
			ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to dispatch entry/exit backfill task (day)", slog.String("channelID", cid), slog.String("day", day), slog.Any("err", err))
		} else {
			ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "▶️ Dispatched entry/exit backfill task (day)", slog.String("channelID", cid), slog.String("day", day))
		}
		return
	}

	initialDate := strings.TrimSpace(rc.BackfillInitialDate)
	if initialDate == "" {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "Backfill skip for channel: no day set and initial_date is empty", slog.String("channelID", cid))
		return
	}

	_, hasProgress, err := ms.store.Metadata(serviceCtx, "backfill_progress:"+cid)
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError,
			"Failed to read backfill progress; skipping backfill dispatch for channel",
			slog.String("operation", "monitoring.start.backfill.get_progress"),
			slog.String("channelID", cid),
			slog.Any("err", err),
		)
		return
	}

	if !hasProgress {
		ms.dispatchInitialBackfill(serviceCtx, cid, initialDate, now)
		return
	}

	if !hasLastEvent {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "No last event recorded, skipping downtime recovery", slog.String("channelID", cid))
		return
	}

	downtime := now.Sub(lastEvent)
	if downtime <= downtimeThreshold {
		ms.logger.LogAttrs(context.Background(), slog.LevelDebug, "Downtime below threshold, skipping recovery", slog.String("channelID", cid), slog.Any("downtime", downtime))
		return
	}

	start := lastEvent.UTC().Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	err = ms.dispatchBackfillTask(serviceCtx, task.Task{
		Type:    TaskTypeMonitorBackfillEntryExitRange,
		Payload: BackfillEntryExitRangePayload{ChannelID: cid, Start: start, End: end},
		Options: task.TaskOptions{GroupKey: "backfill:" + cid},
	})
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to dispatch entry/exit backfill recovery (range)", slog.String("channelID", cid), slog.String("start", start), slog.String("end", end), slog.Any("err", err))
	} else {
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "▶️ Dispatched entry/exit backfill recovery (range)", slog.String("channelID", cid), slog.String("start", start), slog.String("end", end))
	}
}

// dispatchInitialBackfill dispatches the first-run range scan from initialDate to now.
func (ms *MonitoringService) dispatchInitialBackfill(serviceCtx context.Context, cid, initialDate string, now time.Time) {
	parsedDate, err := time.Parse("2006-01-02", initialDate)
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to parse backfill_initial_date", slog.String("date", initialDate), slog.Any("err", err))
		return
	}
	start := parsedDate.Format(time.RFC3339)
	end := now.Format(time.RFC3339)
	err = ms.dispatchBackfillTask(serviceCtx, task.Task{
		Type:    TaskTypeMonitorBackfillEntryExitRange,
		Payload: BackfillEntryExitRangePayload{ChannelID: cid, Start: start, End: end},
		Options: task.TaskOptions{GroupKey: "backfill:" + cid},
	})
	if err != nil {
		ms.logger.LogAttrs(context.Background(), slog.LevelError, "Failed to dispatch initial entry/exit backfill (range)", slog.String("channelID", cid), slog.String("start", start), slog.String("end", end), slog.Any("err", err))
	} else {
		ms.logger.LogAttrs(context.Background(), slog.LevelInfo, "▶️ Dispatched initial entry/exit backfill (range)", slog.String("channelID", cid), slog.String("start", start))
	}
}

// dispatchBackfillTask dispatches t under a bounded startup timeout.
func (ms *MonitoringService) dispatchBackfillTask(serviceCtx context.Context, t task.Task) error {
	dispatchCtx, cancel := context.WithTimeout(serviceCtx, monitoringStartupDispatchLimit)
	defer cancel()
	return ms.router.Dispatch(dispatchCtx, t)
}
