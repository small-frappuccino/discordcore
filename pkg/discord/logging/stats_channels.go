package logging

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	defaultStatsInterval          = 30 * time.Minute
	defaultStatsReconcileInterval = 6 * time.Hour
	maxStatsReconcileInterval     = 24 * time.Hour
	statsSeedMetadataPrefix       = "stats_channels.seeded:"
)

type statsCounterBucket struct {
	all    int
	humans int
	bots   int
}

func (b *statsCounterBucket) addMember(isBot bool, delta int) {
	b.all += delta
	if isBot {
		b.bots += delta
	} else {
		b.humans += delta
	}
	if b.all < 0 {
		b.all = 0
	}
	if b.humans < 0 {
		b.humans = 0
	}
	if b.bots < 0 {
		b.bots = 0
	}
}

func (b statsCounterBucket) total(memberType string) int {
	switch normalizeMemberType(memberType) {
	case "bots":
		return b.bots
	case "humans":
		return b.humans
	default:
		return b.all
	}
}

type statsMemberSnapshot struct {
	isBot        bool
	trackedRoles []string
}

type statsPublishedChannel struct {
	count int
	name  string
	label string
}

type statsGuildState struct {
	initialized     bool
	dirty           bool
	trackedRolesKey string
	lastReconciled  time.Time
	members         map[string]statsMemberSnapshot
	totals          statsCounterBucket
	roleTotals      map[string]statsCounterBucket
	published       map[string]statsPublishedChannel
}

type statsGuildSnapshot struct {
	totals     statsCounterBucket
	roleTotals map[string]statsCounterBucket
}

func newStatsGuildState(trackedRolesKey string, published map[string]statsPublishedChannel) *statsGuildState {
	return &statsGuildState{
		trackedRolesKey: trackedRolesKey,
		members:         make(map[string]statsMemberSnapshot),
		roleTotals:      make(map[string]statsCounterBucket),
		published:       cloneStatsPublishedChannels(published),
	}
}

func cloneStatsPublishedChannels(in map[string]statsPublishedChannel) map[string]statsPublishedChannel {
	if len(in) == 0 {
		return make(map[string]statsPublishedChannel)
	}
	out := make(map[string]statsPublishedChannel, len(in))
	for channelID, published := range in {
		out[channelID] = published
	}
	return out
}

func (state *statsGuildState) applyAdd(userID string, snapshot statsMemberSnapshot) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	if state.members == nil {
		state.members = make(map[string]statsMemberSnapshot)
	}
	if _, exists := state.members[userID]; exists {
		return false
	}
	state.members[userID] = snapshot
	state.addContribution(snapshot, 1)
	return true
}

func (state *statsGuildState) applyRemove(userID string) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	prev, ok := state.members[userID]
	if !ok {
		return false
	}
	delete(state.members, userID)
	state.addContribution(prev, -1)
	return true
}

func (state *statsGuildState) applyUpdate(userID string, snapshot statsMemberSnapshot) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	prev, ok := state.members[userID]
	if !ok {
		return false
	}
	state.addContribution(prev, -1)
	state.members[userID] = snapshot
	state.addContribution(snapshot, 1)
	return true
}

func (state *statsGuildState) addContribution(snapshot statsMemberSnapshot, delta int) {
	state.totals.addMember(snapshot.isBot, delta)
	if state.roleTotals == nil {
		state.roleTotals = make(map[string]statsCounterBucket)
	}
	for _, roleID := range snapshot.trackedRoles {
		bucket := state.roleTotals[roleID]
		bucket.addMember(snapshot.isBot, delta)
		if bucket.all == 0 && bucket.humans == 0 && bucket.bots == 0 {
			delete(state.roleTotals, roleID)
			continue
		}
		state.roleTotals[roleID] = bucket
	}
}

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

func statsEnabled(cfg files.StatsConfig) bool {
	if !cfg.Enabled {
		return false
	}
	return len(cfg.Channels) > 0
}

func statsInterval(cfg files.StatsConfig) time.Duration {
	if cfg.UpdateIntervalMins <= 0 {
		return defaultStatsInterval
	}
	return time.Duration(cfg.UpdateIntervalMins) * time.Minute
}

func statsReconcileInterval(cfg files.StatsConfig) time.Duration {
	interval := statsInterval(cfg) * 12
	if interval < defaultStatsReconcileInterval {
		return defaultStatsReconcileInterval
	}
	if interval > maxStatsReconcileInterval {
		return maxStatsReconcileInterval
	}
	return interval
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
	return ms.statsStoreStateStale(ctx, gcfg), nil
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

func statsRequiresBotClassification(channels []files.StatsChannelConfig) bool {
	for _, channel := range channels {
		switch normalizeMemberType(channel.MemberType) {
		case "bots", "humans":
			return true
		}
	}
	return false
}

func (ms *MonitoringService) statsStoreStateStale(ctx context.Context, gcfg files.GuildConfig) bool {
	limit := statsStoreFreshnessLimit(gcfg.Stats)
	lastHeartbeat, ok, err := ms.getHeartbeat(ctx)
	if err != nil || !ok {
		return true
	}
	return time.Since(lastHeartbeat) > limit
}

func statsStoreFreshnessLimit(cfg files.StatsConfig) time.Duration {
	limit := statsInterval(cfg)
	minimum := 2 * heartbeatInterval
	if limit < minimum {
		return minimum
	}
	return limit
}

func statsSeedMetadataKey(guildID string) string {
	return statsSeedMetadataPrefix + strings.TrimSpace(guildID)
}

func (ms *MonitoringService) hasStatsSeed(ctx context.Context, guildID string) bool {
	if ms == nil || ms.store == nil {
		return false
	}
	_, ok, err := ms.store.Metadata(ctx, statsSeedMetadataKey(guildID))
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
	if err := ms.store.SetMetadata(ctx, statsSeedMetadataKey(guildID), at); err != nil {
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

func statsTrackedRoles(channels []files.StatsChannelConfig) (map[string]struct{}, string) {
	roleIDs := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		roleID := strings.TrimSpace(channel.RoleID)
		if roleID == "" {
			continue
		}
		if _, ok := seen[roleID]; ok {
			continue
		}
		seen[roleID] = struct{}{}
		roleIDs = append(roleIDs, roleID)
	}
	sort.Strings(roleIDs)

	tracked := make(map[string]struct{}, len(roleIDs))
	for _, roleID := range roleIDs {
		tracked[roleID] = struct{}{}
	}
	return tracked, strings.Join(roleIDs, ",")
}

func statsSnapshotFromMember(member *discordgo.Member, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
	if member == nil || member.User == nil {
		return "", statsMemberSnapshot{}, false
	}
	userID := strings.TrimSpace(member.User.ID)
	if userID == "" {
		return "", statsMemberSnapshot{}, false
	}
	return userID, statsMemberSnapshot{
		isBot:        member.User.Bot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}, true
}

func statsSnapshotFromStoredState(member storage.GuildMemberCurrentState, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
	userID := strings.TrimSpace(member.UserID)
	if userID == "" || !member.Active {
		return "", statsMemberSnapshot{}, false
	}
	return userID, statsMemberSnapshot{
		isBot:        member.IsBot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}, true
}

func filterTrackedRoles(roles []string, trackedRoles map[string]struct{}) []string {
	if len(roles) == 0 || len(trackedRoles) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roles))
	filtered := make([]string, 0, len(roles))
	for _, roleID := range roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, ok := trackedRoles[roleID]; !ok {
			continue
		}
		if _, duplicate := seen[roleID]; duplicate {
			continue
		}
		seen[roleID] = struct{}{}
		filtered = append(filtered, roleID)
	}
	sort.Strings(filtered)
	return filtered
}

func statsCountForChannel(snapshot statsGuildSnapshot, cfg files.StatsChannelConfig) int {
	roleID := strings.TrimSpace(cfg.RoleID)
	if roleID == "" {
		return snapshot.totals.total(cfg.MemberType)
	}
	return snapshot.roleTotals[roleID].total(cfg.MemberType)
}

func memberTypeMatches(memberType string, isBot bool) bool {
	switch normalizeMemberType(memberType) {
	case "bots":
		return isBot
	case "humans":
		return !isBot
	default:
		return true
	}
}

func normalizeMemberType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "bots", "bot":
		return "bots"
	case "humans", "human":
		return "humans"
	default:
		return "all"
	}
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

func (ms *MonitoringService) resolveChannel(ctx context.Context, channelID string) (*discordgo.Channel, error) {
	if ms.session == nil || channelID == "" {
		return nil, fmt.Errorf("session not available or channel id empty")
	}
	if ms.session.State != nil {
		if ch, err := ms.session.State.Channel(channelID); err == nil && ch != nil {
			return ch, nil
		}
	}
	return monitoringRunWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Channel, error) {
		return ms.session.Channel(channelID)
	})
}

func renderStatsChannelName(label, template string, count int) string {
	label = strings.TrimSpace(label)
	tmpl := strings.TrimSpace(template)
	if tmpl == "" {
		if label == "" {
			return fmt.Sprintf("☆  ☆ : %d", count)
		}
		return fmt.Sprintf("☆ %s ☆ : %d", strings.ToLower(label), count)
	}
	out := strings.ReplaceAll(tmpl, "{count}", fmt.Sprintf("%d", count))
	out = strings.ReplaceAll(out, "{label}", label)
	return strings.TrimSpace(out)
}

func (ms *MonitoringService) handleStatsMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m == nil || m.Member == nil || m.Member.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	ms.applyStatsMemberAdd(m.Member)
}

func (ms *MonitoringService) handleStatsMemberRemove(s *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil {
		return
	}
	if !ms.handlesGuild(m.GuildID) {
		return
	}
	ms.applyStatsMemberRemove(m.GuildID, m.User.ID)
}

func (ms *MonitoringService) applyStatsMemberAdd(member *discordgo.Member) {
	if member == nil || member.User == nil {
		return
	}
	guildID := strings.TrimSpace(member.GuildID)
	userID := strings.TrimSpace(member.User.ID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberActive(guildID, userID, member.JoinedAt, member.User.Bot, member.Roles)
	snapshot := statsMemberSnapshot{
		isBot:        member.User.Bot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyAdd(userID, snapshot) {
		state.dirty = true
	}
}

func (ms *MonitoringService) applyStatsMemberUpdate(guildID, userID string, isBot bool, roles []string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberActive(guildID, userID, time.Time{}, isBot, roles)
	snapshot := statsMemberSnapshot{
		isBot:        isBot,
		trackedRoles: filterTrackedRoles(roles, trackedRoles),
	}

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyUpdate(userID, snapshot) {
		state.dirty = true
	}
}

func (ms *MonitoringService) applyStatsMemberRemove(guildID, userID string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, _, trackedRolesKey, enabled := ms.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	ms.persistStatsMemberLeft(guildID, userID)

	ms.statsMu.Lock()
	defer ms.statsMu.Unlock()

	state := ms.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyRemove(userID) {
		state.dirty = true
	}
}

func (ms *MonitoringService) persistStatsMemberActive(guildID, userID string, joinedAt time.Time, isBot bool, roles []string) {
	if ms == nil || ms.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := monitoringRunErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		if err := ms.store.UpsertMemberPresenceContext(runCtx, guildID, userID, joinedAt, time.Now().UTC(), isBot); err != nil {
			return fmt.Errorf("upsert member presence: %w", err)
		}
		if err := ms.store.UpsertMemberRoles(guildID, userID, roles, time.Now().UTC()); err != nil {
			return fmt.Errorf("upsert member roles: %w", err)
		}
		return nil
	})
	if err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats member state",
			"operation", "monitoring.stats.persist_member_active",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
}

func (ms *MonitoringService) persistStatsMemberLeft(guildID, userID string) {
	if ms == nil || ms.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := monitoringRunErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		return ms.store.MarkMemberLeftContext(runCtx, guildID, userID, time.Now().UTC())
	})
	if err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats member leave",
			"operation", "monitoring.stats.persist_member_left",
			"guildID", guildID,
			"userID", userID,
			"err", err,
		)
	}
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
