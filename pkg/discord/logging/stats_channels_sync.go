package logging

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
)

func (ms *MonitoringService) updateStatsChannels(ctx context.Context) error {
	if ms == nil || ms.session == nil || ms.configManager == nil {
		return nil
	}
	cfg := ms.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		ms.pruneStatsGuildState(nil)
		return nil
	}

	activeGuilds := make(map[string]struct{}, len(cfg.Guilds))
	for _, gcfg := range cfg.Guilds {
		if err := ctx.Err(); err != nil {
			return err
		}
		features := cfg.ResolveFeatures(gcfg.GuildID)
		if !features.Services.Monitoring || !features.StatsChannels || !statsEnabled(gcfg.Stats) {
			continue
		}
		activeGuilds[gcfg.GuildID] = struct{}{}

		needsReconcile, prepErr := ms.prepareStatsState(ctx, gcfg)
		if prepErr != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to prepare stats state",
				"operation", "monitoring.stats.prepare",
				"guildID", gcfg.GuildID,
				"err", prepErr,
			)
		}
		shouldPublish := ms.shouldRunStatsUpdate(gcfg.GuildID, statsInterval(gcfg.Stats))
		if needsReconcile {
			if err := ms.reconcileStatsForGuild(ctx, gcfg); err != nil {
				log.ErrorLoggerRaw().Error(
					"Failed to reconcile stats channels",
					"operation", "monitoring.stats.reconcile",
					"guildID", gcfg.GuildID,
					"err", err,
				)
				if !shouldPublish {
					continue
				}
			}
		}
		if !shouldPublish {
			continue
		}
		if err := ms.publishStatsForGuild(ctx, gcfg); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to update stats channels",
				"operation", "monitoring.stats.publish",
				"guildID", gcfg.GuildID,
				"err", err,
			)
		}
	}

	ms.pruneStatsGuildState(activeGuilds)
	return nil
}

func (ms *MonitoringService) shouldRunStatsUpdate(guildID string, interval time.Duration) bool {
	if guildID == "" {
		return false
	}
	if interval <= 0 {
		interval = defaultStatsInterval
	}
	now := time.Now()
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	if ms.statsLastRun == nil {
		ms.statsLastRun = make(map[string]time.Time)
	}
	last, ok := ms.statsLastRun[guildID]
	if ok && now.Sub(last) < interval {
		return false
	}
	ms.statsLastRun[guildID] = now
	return true
}

func (ms *MonitoringService) reconcileStatsForGuild(ctx context.Context, gcfg files.GuildConfig) error {
	if gcfg.GuildID == "" {
		return fmt.Errorf("guild id is empty")
	}
	if len(gcfg.Stats.Channels) == 0 {
		return nil
	}

	trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
	state := newStatsGuildState(trackedRolesKey, ms.statsPublishedChannels(gcfg.GuildID))
	if _, err := ms.forEachGuildMemberPageContext(ctx, gcfg.GuildID, func(members []*discordgo.Member) error {
		for _, member := range members {
			if err := ctx.Err(); err != nil {
				return err
			}
			userID, snapshot, ok := statsSnapshotFromMember(member, trackedRoles)
			if !ok {
				continue
			}
			_ = state.applyAdd(userID, snapshot)
		}
		return nil
	}); err != nil {
		return fmt.Errorf("fetch guild members: %w", err)
	}

	state.initialized = true
	state.dirty = false
	state.lastReconciled = time.Now().UTC()
	ms.replaceStatsGuildState(gcfg.GuildID, state)
	ms.markStatsSeeded(ctx, gcfg.GuildID, state.lastReconciled)

	log.ApplicationLogger().Info(
		"Reconciled stats counters",
		"operation", "monitoring.stats.reconcile",
		"guildID", gcfg.GuildID,
		"members", len(state.members),
		"trackedRoles", len(state.roleTotals),
	)
	return nil
}

func (ms *MonitoringService) prepareStatsState(ctx context.Context, gcfg files.GuildConfig) (bool, error) {
	_, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)

	ms.statsMu.Lock()
	state := ms.ensureStatsGuildStateLocked(gcfg.GuildID)
	keysMatch := state.trackedRolesKey == trackedRolesKey
	if state.initialized && keysMatch && !state.dirty {
		lastReconciled := state.lastReconciled
		ms.statsMu.Unlock()
		return time.Since(lastReconciled) >= statsReconcileInterval(gcfg.Stats), nil
	}
	ms.statsMu.Unlock()

	hydrated, err := ms.hydrateStatsForGuildFromStore(ctx, gcfg)
	if err != nil {
		return true, err
	}
	if !hydrated {
		return true, nil
	}
	return ms.statsStoreStateStale(gcfg), nil
}

func (ms *MonitoringService) hydrateStatsForGuildFromStore(ctx context.Context, gcfg files.GuildConfig) (bool, error) {
	if ms == nil || ms.store == nil {
		return false, nil
	}
	if gcfg.GuildID == "" {
		return false, nil
	}
	if !ms.hasStatsSeed(ctx, gcfg.GuildID) {
		return false, nil
	}

	storedMembers, err := ms.store.GetActiveGuildMemberStatesContext(ctx, gcfg.GuildID)
	if err != nil {
		return false, fmt.Errorf("load active member state: %w", err)
	}

	trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
	if statsRequiresBotClassification(gcfg.Stats.Channels) {
		for _, member := range storedMembers {
			if !member.HasBot {
				return false, nil
			}
		}
	}

	state := newStatsGuildState(trackedRolesKey, ms.statsPublishedChannels(gcfg.GuildID))
	for _, member := range storedMembers {
		userID, snapshot, ok := statsSnapshotFromStoredState(member, trackedRoles)
		if !ok {
			continue
		}
		_ = state.applyAdd(userID, snapshot)
	}
	state.initialized = true
	state.dirty = false
	state.lastReconciled = time.Now().UTC()
	ms.replaceStatsGuildState(gcfg.GuildID, state)
	return true, nil
}

func (ms *MonitoringService) statsStoreStateStale(gcfg files.GuildConfig) bool {
	limit := statsStoreFreshnessLimit(gcfg.Stats)
	lastHeartbeat, ok, err := ms.getHeartbeat()
	if err != nil || !ok {
		return true
	}
	return time.Since(lastHeartbeat) > limit
}

func (ms *MonitoringService) hasStatsSeed(ctx context.Context, guildID string) bool {
	if ms == nil || ms.store == nil {
		return false
	}
	_, ok, err := ms.store.GetMetadataContext(ctx, statsSeedMetadataKey(guildID))
	if err != nil {
		log.ApplicationLogger().Warn(
			"Failed to read stats seed metadata",
			"operation", "monitoring.stats.seed.read",
			"guildID", guildID,
			"err", err,
		)
		return false
	}
	return ok
}

func (ms *MonitoringService) markStatsSeeded(ctx context.Context, guildID string, at time.Time) {
	if ms == nil || ms.store == nil || strings.TrimSpace(guildID) == "" {
		return
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if err := ms.store.SetMetadataContext(ctx, statsSeedMetadataKey(guildID), at); err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats seed metadata",
			"operation", "monitoring.stats.seed.write",
			"guildID", guildID,
			"err", err,
		)
	}
}

func (ms *MonitoringService) publishStatsForGuild(ctx context.Context, gcfg files.GuildConfig) error {
	snapshot, ok := ms.statsSnapshot(gcfg.GuildID)
	if !ok {
		return fmt.Errorf("stats state unavailable")
	}

	for _, sc := range gcfg.Stats.Channels {
		if err := ctx.Err(); err != nil {
			return err
		}
		count := statsCountForChannel(snapshot, sc)
		if err := ms.updateStatsChannelName(ctx, gcfg.GuildID, sc, count); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to update stats channel name",
				"operation", "monitoring.stats.publish_channel",
				"guildID", gcfg.GuildID,
				"channelID", strings.TrimSpace(sc.ChannelID),
				"err", err,
			)
		}
	}
	return nil
}

func (ms *MonitoringService) updateStatsChannelName(ctx context.Context, guildID string, cfg files.StatsChannelConfig, count int) error {
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return nil
	}

	published, hasPublished := ms.statsPublishedChannel(guildID, channelID)
	label := strings.TrimSpace(cfg.Label)
	if label == "" {
		label = strings.TrimSpace(published.label)
	}

	channelName := ""
	if !hasPublished || label == "" {
		channel, err := ms.resolveChannel(ctx, channelID)
		if err != nil {
			return fmt.Errorf("resolve channel: %w", err)
		}
		if channel == nil {
			return fmt.Errorf("channel not found")
		}
		if guildID != "" && channel.GuildID != "" && channel.GuildID != guildID {
			return fmt.Errorf("channel guild mismatch: %s", channel.GuildID)
		}
		channelName = channel.Name
		if label == "" {
			label = strings.TrimSpace(channel.Name)
		}
	}

	newName := renderStatsChannelName(label, cfg.NameTemplate, count)
	if newName == "" {
		return nil
	}
	if len(newName) > 100 {
		newName = newName[:100]
	}
	if hasPublished && published.name == newName && published.count == count {
		return nil
	}
	if channelName != "" && channelName == newName {
		ms.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
			count: count,
			name:  newName,
			label: label,
		})
		return nil
	}
	if hasPublished && published.name == newName {
		ms.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
			count: count,
			name:  newName,
			label: label,
		})
		return nil
	}

	if _, err := monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Channel, error) {
		return ms.session.ChannelEdit(channelID, &discordgo.ChannelEdit{Name: newName})
	}); err != nil {
		return fmt.Errorf("channel edit: %w", err)
	}

	ms.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
		count: count,
		name:  newName,
		label: label,
	})
	log.ApplicationLogger().Info(
		"Updated stats channel name",
		"operation", "monitoring.stats.publish_channel",
		"guildID", guildID,
		"channelID", channelID,
		"count", count,
		"name", newName,
	)
	return nil
}

func (ms *MonitoringService) statsGuildConfig(guildID string) (files.GuildConfig, map[string]struct{}, string, bool) {
	cfg := ms.scopedConfig()
	if cfg == nil {
		return files.GuildConfig{}, nil, "", false
	}
	for _, gcfg := range cfg.Guilds {
		if gcfg.GuildID != guildID {
			continue
		}
		features := cfg.ResolveFeatures(guildID)
		if !features.Services.Monitoring || !features.StatsChannels || !statsEnabled(gcfg.Stats) {
			return gcfg, nil, "", false
		}
		trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
		return gcfg, trackedRoles, trackedRolesKey, true
	}
	return files.GuildConfig{}, nil, "", false
}

func (ms *MonitoringService) ensureStatsGuildStateLocked(guildID string) *statsGuildState {
	if ms.statsGuilds == nil {
		ms.statsGuilds = make(map[string]*statsGuildState)
	}
	state := ms.statsGuilds[guildID]
	if state != nil {
		return state
	}
	state = newStatsGuildState("", nil)
	ms.statsGuilds[guildID] = state
	return state
}

func (ms *MonitoringService) replaceStatsGuildState(guildID string, state *statsGuildState) {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	if ms.statsGuilds == nil {
		ms.statsGuilds = make(map[string]*statsGuildState)
	}
	ms.statsGuilds[guildID] = state
}

func (ms *MonitoringService) statsPublishedChannels(guildID string) map[string]statsPublishedChannel {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	if ms.statsGuilds == nil {
		return nil
	}
	state := ms.statsGuilds[guildID]
	if state == nil {
		return nil
	}
	return cloneStatsPublishedChannels(state.published)
}

func (ms *MonitoringService) statsPublishedChannel(guildID, channelID string) (statsPublishedChannel, bool) {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	if ms.statsGuilds == nil {
		return statsPublishedChannel{}, false
	}
	state := ms.statsGuilds[guildID]
	if state == nil || state.published == nil {
		return statsPublishedChannel{}, false
	}
	published, ok := state.published[channelID]
	return published, ok
}

func (ms *MonitoringService) recordStatsPublishedChannel(guildID, channelID string, published statsPublishedChannel) {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()
	state := ms.ensureStatsGuildStateLocked(guildID)
	if state.published == nil {
		state.published = make(map[string]statsPublishedChannel)
	}
	state.published[channelID] = published
}

func (ms *MonitoringService) statsSnapshot(guildID string) (statsGuildSnapshot, bool) {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	if ms.statsGuilds == nil {
		return statsGuildSnapshot{}, false
	}
	state := ms.statsGuilds[guildID]
	if state == nil || !state.initialized {
		return statsGuildSnapshot{}, false
	}

	roleTotals := make(map[string]statsCounterBucket, len(state.roleTotals))
	for roleID, bucket := range state.roleTotals {
		roleTotals[roleID] = bucket
	}
	return statsGuildSnapshot{
		totals:     state.totals,
		roleTotals: roleTotals,
	}, true
}

func (ms *MonitoringService) pruneStatsGuildState(activeGuilds map[string]struct{}) {
	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	for guildID := range ms.statsLastRun {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(ms.statsLastRun, guildID)
	}
	for guildID := range ms.statsGuilds {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(ms.statsGuilds, guildID)
	}
}
