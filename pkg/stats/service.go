package stats

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
)

const (
	defaultStatsInterval          = 30 * time.Minute
	defaultStatsReconcileInterval = 6 * time.Hour
	maxStatsReconcileInterval     = 24 * time.Hour
	statsSeedMetadataPrefix       = "stats_channels.seeded:"
	heartbeatInterval             = 5 * time.Minute
	monitoringDependencyTimeout   = 15 * time.Second
)

// StateStore abstracts the storage operations required by the StatsService.
type StateStore interface {
	GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[storage.GuildMemberCurrentState, error]
	Metadata(ctx context.Context, key string) (time.Time, bool, error)
	SetMetadata(ctx context.Context, key string, at time.Time) error
	UpsertMemberPresenceContext(ctx context.Context, input storage.MemberPresenceInput) error
	UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error
	MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error
	HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error)
}

// StatsService manages the stats-channel state. guilds and lastRun are protected by mu.
// Construct with NewStatsService; the zero value has nil maps.
type StatsService struct {
	gateway       Gateway
	configManager *files.ConfigManager
	store         StateStore
	logger        *slog.Logger
	botInstanceID string

	cancelMu sync.Mutex
	guilds   sync.Map
	lastRun  sync.Map

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewStatsService news stats service.
func NewStatsService(
	gateway Gateway,
	configManager *files.ConfigManager,
	store StateStore,
	logger *slog.Logger,
	botInstanceID string,
) *StatsService {
	return &StatsService{
		gateway:       gateway,
		configManager: configManager,
		store:         store,
		logger:        logger,
		botInstanceID: botInstanceID,
	}
}

func (s *StatsService) log(guildID string) *slog.Logger {
	if guildID == "" {
		return s.logger
	}
	return s.logger.With(slog.String("guild_id", guildID))
}

// Name names.
func (s *StatsService) Name() string {
	return "stats"
}

// Type types.
func (s *StatsService) Type() svc.ServiceType {
	return svc.TypeMonitoring
}

// Priority prioritys.
func (s *StatsService) Priority() svc.ServicePriority {
	return svc.PriorityLow
}

// Dependencies dependencies.
func (s *StatsService) Dependencies() []string {
	return nil
}

// IsRunning is running.
func (s *StatsService) IsRunning() bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return s.cancel != nil
}

// HealthCheck healths check.
func (s *StatsService) HealthCheck(ctx context.Context) svc.HealthStatus {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	return svc.HealthStatus{
		Healthy:   s.cancel != nil,
		Message:   "Stats service health check",
		LastCheck: time.Now(),
	}
}

// Stats stats.
func (s *StatsService) Stats() svc.ServiceStats {
	return svc.ServiceStats{}
}

// Start starts.
func (s *StatsService) Start(ctx context.Context) error {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	if s.cancel != nil {
		return nil // already running
	}

	runCtx, cancel := context.WithCancel(context.Background())
	if ctx != nil {
		runCtx, cancel = context.WithCancel(context.WithoutCancel(ctx))
	}
	s.cancel = cancel
	s.wg.Add(1)

	go s.runCron(runCtx)

	return nil
}

func (s *StatsService) runCron(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// initial run
	_ = s.UpdateStatsChannels(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.UpdateStatsChannels(ctx)
		}
	}
}

// Stop stops.
func (s *StatsService) Stop(ctx context.Context) error {
	s.cancelMu.Lock()
	if s.cancel == nil {
		s.cancelMu.Unlock()
		return nil
	}
	s.cancel()
	s.cancel = nil
	s.cancelMu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *StatsService) handlesGuild(guildID string) bool {
	cfg := s.configManager.GuildConfig(guildID)
	if cfg == nil {
		return false
	}
	if !cfg.BelongsToBotInstance(s.botInstanceID) {
		return false
	}
	resolvedID, _ := cfg.ResolveFeatureBotInstanceID("stats")
	return resolvedID == s.botInstanceID
}

func (s *StatsService) scopedConfig() *files.BotConfig {
	cfg := s.configManager.Config()
	if cfg == nil {
		return nil
	}
	scopedGuilds := cfg.GuildsForBotInstance(s.botInstanceID)
	if len(scopedGuilds) == len(cfg.Guilds) {
		return cfg
	}
	scoped := *cfg
	scoped.Guilds = scopedGuilds
	return &scoped
}

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
	mu              sync.Mutex
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

func (state *statsGuildState) applyDelta(userID string, snapshot statsMemberSnapshot, isAdd, isRemove bool) bool {
	if state == nil || strings.TrimSpace(userID) == "" {
		return false
	}
	if state.members == nil {
		state.members = make(map[string]statsMemberSnapshot)
	}
	prev, exists := state.members[userID]
	if isAdd && exists {
		return false
	}
	if isRemove && !exists {
		return false
	}
	if exists {
		state.addContribution(prev, -1)
		delete(state.members, userID)
	}
	if !isRemove {
		state.members[userID] = snapshot
		state.addContribution(snapshot, 1)
	}
	return true
}

func (state *statsGuildState) applyAdd(userID string, snapshot statsMemberSnapshot) bool {
	return state.applyDelta(userID, snapshot, true, false)
}

func (state *statsGuildState) applyRemove(userID string) bool {
	return state.applyDelta(userID, statsMemberSnapshot{}, false, true)
}

func (state *statsGuildState) applyUpdate(userID string, snapshot statsMemberSnapshot) bool {
	return state.applyDelta(userID, snapshot, false, false)
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

func (s *StatsService) UpdateStatsChannels(ctx context.Context) error {
	cfg := s.scopedConfig()
	if cfg == nil || len(cfg.Guilds) == 0 {
		s.pruneStatsGuildState(nil)
		return nil
	}

	activeGuilds := make(map[string]struct{}, len(cfg.Guilds))
	for _, gcfg := range cfg.Guilds {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.updateStatsChannels: %w", err)
		}
		features := cfg.ResolveFeatures(gcfg.GuildID)
		if !features.Services.Monitoring || !Enabled(gcfg.Stats) {
			continue
		}
		if !s.handlesGuild(gcfg.GuildID) {
			continue
		}
		activeGuilds[gcfg.GuildID] = struct{}{}

		needsReconcile, prepErr := s.prepareStatsState(ctx, gcfg)
		if prepErr != nil {
			s.log(gcfg.GuildID).Error(
				"Failed to prepare stats state",
				"operation", "monitoring.stats.prepare",
				"err", prepErr,
			)
		}
		shouldPublish := s.shouldRunStatsUpdate(gcfg.GuildID, statsInterval())
		if needsReconcile {
			if err := s.reconcileStatsForGuild(ctx, gcfg); err != nil {
				s.log(gcfg.GuildID).Error(
					"Failed to reconcile stats channels",
					"operation", "monitoring.stats.reconcile",
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
		if err := s.publishStatsForGuild(ctx, gcfg); err != nil {
			s.log(gcfg.GuildID).Error(
				"Failed to update stats channels",
				"operation", "monitoring.stats.publish",
				"err", err,
			)
		}
	}

	s.pruneStatsGuildState(activeGuilds)
	return nil
}

func Enabled(cfg files.StatsConfig) bool {
	return len(cfg.Channels) > 0
}

func statsInterval() time.Duration {
	return 5 * time.Minute
}

func statsReconcileInterval() time.Duration {
	interval := statsInterval() * 12
	if interval < defaultStatsReconcileInterval {
		return defaultStatsReconcileInterval
	}
	if interval > maxStatsReconcileInterval {
		return maxStatsReconcileInterval
	}
	return interval
}

// ForceGuildUpdate clears the last run timestamp for the guild,
// ensuring the next update runs immediately.
func (s *StatsService) ForceGuildUpdate(guildID string) {
	if strings.TrimSpace(guildID) == "" {
		return
	}
	s.lastRun.Delete(guildID)
}

func (s *StatsService) shouldRunStatsUpdate(guildID string, interval time.Duration) bool {
	if guildID == "" {
		return false
	}
	if interval <= 0 {
		interval = defaultStatsInterval
	}
	now := time.Now()

	var last time.Time
	if v, ok := s.lastRun.Load(guildID); ok {
		last = v.(time.Time)
	}
	if !last.IsZero() && now.Sub(last) < interval {
		return false
	}
	s.lastRun.Store(guildID, now)
	return true
}

func (s *StatsService) reconcileStatsForGuild(ctx context.Context, gcfg files.GuildConfig) error {
	if gcfg.GuildID == "" {
		return fmt.Errorf("guild id is empty")
	}
	if len(gcfg.Stats.Channels) == 0 {
		return nil
	}

	trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
	state := newStatsGuildState(trackedRolesKey, s.statsPublishedChannels(gcfg.GuildID))

	for member, err := range s.streamGuildMembers(ctx, gcfg.GuildID) {
		if err != nil {
			return fmt.Errorf("fetch guild members: %w", err)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.reconcileStatsForGuild: %w", err)
		}
		userID, snapshot, ok := statsSnapshotFromGatewayMember(member, trackedRoles)
		if !ok {
			continue
		}
		_ = state.applyAdd(userID, snapshot)
	}

	state.initialized = true
	state.dirty = false
	state.lastReconciled = time.Now().UTC()
	s.replaceStatsGuildState(gcfg.GuildID, state)
	s.markStatsSeeded(ctx, gcfg.GuildID, state.lastReconciled)

	s.log(gcfg.GuildID).Info(
		"Reconciled stats counters",
		"operation", "monitoring.stats.reconcile",
		"members", len(state.members),
		"trackedRoles", len(state.roleTotals),
	)
	return nil
}

func (s *StatsService) prepareStatsState(ctx context.Context, gcfg files.GuildConfig) (bool, error) {
	_, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)

	state := s.getOrInitStatsGuildState(gcfg.GuildID)
	state.mu.Lock()
	keysMatch := state.trackedRolesKey == trackedRolesKey
	var needsReconcile, skipRest bool
	if state.initialized && keysMatch && !state.dirty {
		lastReconciled := state.lastReconciled
		needsReconcile = time.Since(lastReconciled) >= statsReconcileInterval()
		skipRest = true
	}
	state.mu.Unlock()

	if skipRest {
		return needsReconcile, nil
	}

	hydrated, err := s.hydrateStatsForGuildFromStore(ctx, gcfg)
	if err != nil {
		return true, fmt.Errorf("MonitoringService.prepareStatsState: %w", err)
	}
	if !hydrated {
		return true, nil
	}
	return s.statsStoreStateStale(ctx, gcfg), nil
}

func (s *StatsService) hydrateStatsForGuildFromStore(ctx context.Context, gcfg files.GuildConfig) (bool, error) {
	if gcfg.GuildID == "" {
		return false, nil
	}
	if !s.hasStatsSeed(ctx, gcfg.GuildID) {
		return false, nil
	}

	trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
	requiresBotClass := statsRequiresBotClassification(gcfg.Stats.Channels)

	state := newStatsGuildState(trackedRolesKey, s.statsPublishedChannels(gcfg.GuildID))
	for member, err := range s.store.GetActiveGuildMemberStatesContext(ctx, gcfg.GuildID) {
		if err != nil {
			return false, fmt.Errorf("load active member state: %w", err)
		}
		if requiresBotClass && !member.HasBot {
			return false, nil
		}
		userID, snapshot, ok := statsSnapshotFromStoredState(member, trackedRoles)
		if !ok {
			continue
		}
		_ = state.applyAdd(userID, snapshot)
	}

	state.initialized = true
	state.dirty = false
	state.lastReconciled = time.Now().UTC()
	s.replaceStatsGuildState(gcfg.GuildID, state)
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

func (s *StatsService) statsStoreStateStale(ctx context.Context, gcfg files.GuildConfig) bool {
	limit := statsStoreFreshnessLimit()
	lastHeartbeat, ok, err := s.getHeartbeat(ctx)
	if err != nil || !ok {
		return true
	}
	return time.Since(lastHeartbeat) > limit
}

func statsStoreFreshnessLimit() time.Duration {
	limit := statsInterval()
	minimum := 2 * heartbeatInterval
	if limit < minimum {
		return minimum
	}
	return limit
}

func statsSeedMetadataKey(guildID string) string {
	return statsSeedMetadataPrefix + strings.TrimSpace(guildID)
}

func (s *StatsService) hasStatsSeed(ctx context.Context, guildID string) bool {
	_, ok, err := s.store.Metadata(ctx, statsSeedMetadataKey(guildID))
	if err != nil {
		s.log(guildID).Warn(
			"Failed to read stats seed metadata",
			"operation", "monitoring.stats.seed.read",
			"err", err,
		)
		return false
	}
	return ok
}

func (s *StatsService) markStatsSeeded(ctx context.Context, guildID string, at time.Time) {
	if strings.TrimSpace(guildID) == "" {
		return
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if err := s.store.SetMetadata(ctx, statsSeedMetadataKey(guildID), at); err != nil {
		s.log(guildID).Warn(
			"Failed to persist stats seed metadata",
			"operation", "monitoring.stats.seed.write",
			"err", err,
		)
	}
}

func (s *StatsService) publishStatsForGuild(ctx context.Context, gcfg files.GuildConfig) error {
	snapshot, ok := s.statsSnapshot(gcfg.GuildID)
	if !ok {
		return fmt.Errorf("stats state unavailable")
	}

	for _, sc := range gcfg.Stats.Channels {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("MonitoringService.publishStatsForGuild: %w", err)
		}
		count := statsCountForChannel(snapshot, sc)
		if err := s.updateStatsChannelName(ctx, gcfg.GuildID, sc, count); err != nil {
			s.log(gcfg.GuildID).Error(
				"Failed to update stats channel name",
				"operation", "monitoring.stats.publish_channel",
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

func statsSnapshotFromGatewayMember(member MemberSnapshot, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
	userID := strings.TrimSpace(member.UserID)
	if userID == "" {
		return "", statsMemberSnapshot{}, false
	}
	return userID, statsMemberSnapshot{
		isBot:        member.IsBot,
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
		trackedRoles: filterTrackedRoles(slices.Values(member.Roles), trackedRoles),
	}, true
}

func filterTrackedRoles(roles iter.Seq[string], trackedRoles map[string]struct{}) []string {
	if len(trackedRoles) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	var filtered []string
	for roleID := range roles {
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

func (s *StatsService) updateStatsChannelName(ctx context.Context, guildID string, cfg files.StatsChannelConfig, count int) error {
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return nil
	}

	published, hasPublished := s.statsPublishedChannel(guildID, channelID)
	label := cfg.Label
	if strings.TrimSpace(label) == "" {
		label = published.label
	}

	channelName := ""
	if !hasPublished || strings.TrimSpace(label) == "" {
		channel, err := s.resolveChannel(ctx, channelID)
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
		if strings.TrimSpace(label) == "" {
			label = channel.Name
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
		s.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
			count: count,
			name:  newName,
			label: label,
		})
		return nil
	}
	if hasPublished && published.name == newName {
		s.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
			count: count,
			name:  newName,
			label: label,
		})
		return nil
	}

	metaKey := "stats_channel:" + channelID
	lastUpdate, exists, err := s.store.Metadata(ctx, metaKey)
	if err == nil && exists && time.Since(lastUpdate) < 6*time.Minute {
		// Discord rate limit for channel name is 2 edits per 10 mins.
		// Enforce a strict 6 min interval to avoid 429 when multiple bots/nodes try to update.
		return nil
	}

	if err := s.gateway.UpdateChannelName(ctx, channelID, newName); err != nil {
		return fmt.Errorf("channel edit: %w", err)
	}

	_ = s.store.SetMetadata(ctx, metaKey, time.Now().UTC())

	s.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
		count: count,
		name:  newName,
		label: label,
	})
	s.log(guildID).Info(
		"Updated stats channel name",
		"operation", "monitoring.stats.publish_channel",
		"channelID", channelID,
		"count", count,
		"name", newName,
	)
	return nil
}

func (s *StatsService) resolveChannel(ctx context.Context, channelID string) (*Channel, error) {
	if s.gateway == nil || channelID == "" {
		return nil, fmt.Errorf("gateway not available or channel id empty")
	}
	return s.gateway.GetChannel(ctx, channelID)
}

func renderStatsChannelName(label, template string, count int) string {
	if template == "" {
		return fmt.Sprintf("%s%d", label, count)
	}
	out := strings.ReplaceAll(template, "{count}", fmt.Sprintf("%d", count))
	out = strings.ReplaceAll(out, "{label}", label)
	return out
}

// ApplyMemberAdd is called by the adapter when a member joins.
func (s *StatsService) ApplyMemberAdd(guildID string, userID string, joinedAt time.Time, isBot bool, roles iter.Seq[string]) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := s.statsGuildConfig(guildID)
	if !enabled {
		return
	}

	snapshot := statsMemberSnapshot{
		isBot:        isBot,
		trackedRoles: filterTrackedRoles(roles, trackedRoles),
	}

	state := s.getOrInitStatsGuildState(guildID)
	state.mu.Lock()
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
	} else if !state.applyAdd(userID, snapshot) {
		state.dirty = true
	}
	state.mu.Unlock()

	var rolesSlice []string
	if roles != nil {
		for r := range roles {
			rolesSlice = append(rolesSlice, r)
		}
	}
	s.persistStatsMemberActive(guildID, userID, joinedAt, isBot, rolesSlice)
}

// ApplyStatsMemberUpdate applys stats member update.
func (s *StatsService) ApplyStatsMemberUpdate(guildID, userID string, isBot bool, roles iter.Seq[string]) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := s.statsGuildConfig(guildID)
	if !enabled {
		return
	}

	snapshot := statsMemberSnapshot{
		isBot:        isBot,
		trackedRoles: filterTrackedRoles(roles, trackedRoles),
	}

	state := s.getOrInitStatsGuildState(guildID)
	state.mu.Lock()
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
	} else if !state.applyUpdate(userID, snapshot) {
		state.dirty = true
	}
	state.mu.Unlock()

	var rolesSlice []string
	if roles != nil {
		for r := range roles {
			rolesSlice = append(rolesSlice, r)
		}
	}
	s.persistStatsMemberActive(guildID, userID, time.Time{}, isBot, rolesSlice)
}

// ApplyMemberRemove is called by the adapter when a member leaves.
func (s *StatsService) ApplyMemberRemove(guildID, userID string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, _, trackedRolesKey, enabled := s.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	s.persistStatsMemberLeft(guildID, userID)

	state := s.getOrInitStatsGuildState(guildID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyRemove(userID) {
		state.dirty = true
	}
}

func (s *StatsService) persistStatsMemberActive(guildID, userID string, joinedAt time.Time, isBot bool, roles []string) {
	if s.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	runCtx, cancel := context.WithTimeout(context.Background(), monitoringPersistenceTimeout)
	defer cancel()

	if err := s.store.UpsertMemberPresenceContext(runCtx, storage.MemberPresenceInput{GuildID: guildID, UserID: userID, JoinedAt: joinedAt, SeenAt: time.Now().UTC(), IsBot: isBot}); err != nil {
		s.log(guildID).Warn(
			"Failed to persist stats member state",
			"operation", "monitoring.stats.persist_member_active",
			"userID", userID,
			"err", err,
		)
		return
	}
	if err := s.store.UpsertMemberRoles(guildID, userID, roles, time.Now().UTC()); err != nil {
		s.log(guildID).Warn(
			"Failed to persist stats member roles",
			"operation", "monitoring.stats.persist_member_active",
			"userID", userID,
			"err", err,
		)
		return
	}
}

func (s *StatsService) persistStatsMemberLeft(guildID, userID string) {
	if s.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	runCtx, cancel := context.WithTimeout(context.Background(), monitoringPersistenceTimeout)
	defer cancel()

	if err := s.store.MarkMemberLeftContext(runCtx, guildID, userID, time.Now().UTC()); err != nil {
		s.log(guildID).Warn(
			"Failed to persist stats member leave",
			"operation", "monitoring.stats.persist_member_left",
			"userID", userID,
			"err", err,
		)
	}
}

func (s *StatsService) statsGuildConfig(guildID string) (files.GuildConfig, map[string]struct{}, string, bool) {
	cfg := s.scopedConfig()
	if cfg == nil {
		return files.GuildConfig{}, nil, "", false
	}
	for _, gcfg := range cfg.Guilds {
		if gcfg.GuildID != guildID {
			continue
		}
		features := cfg.ResolveFeatures(guildID)
		if !features.Services.Monitoring || !Enabled(gcfg.Stats) {
			return gcfg, nil, "", false
		}
		trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
		return gcfg, trackedRoles, trackedRolesKey, true
	}
	return files.GuildConfig{}, nil, "", false
}

func (s *StatsService) getOrInitStatsGuildState(guildID string) *statsGuildState {
	v, ok := s.guilds.Load(guildID)
	if ok {
		return v.(*statsGuildState)
	}
	newState := newStatsGuildState("", nil)
	v, _ = s.guilds.LoadOrStore(guildID, newState)
	return v.(*statsGuildState)
}

func (s *StatsService) replaceStatsGuildState(guildID string, state *statsGuildState) {
	s.guilds.Store(guildID, state)
}

func (s *StatsService) statsPublishedChannels(guildID string) map[string]statsPublishedChannel {
	v, ok := s.guilds.Load(guildID)
	if !ok {
		return nil
	}
	state := v.(*statsGuildState)
	state.mu.Lock()
	defer state.mu.Unlock()

	res := make(map[string]statsPublishedChannel, len(state.published))
	for k, v := range state.published {
		res[k] = v
	}
	return res
}

func (s *StatsService) statsPublishedChannel(guildID, channelID string) (statsPublishedChannel, bool) {
	v, ok := s.guilds.Load(guildID)
	if !ok {
		return statsPublishedChannel{}, false
	}
	state := v.(*statsGuildState)
	state.mu.Lock()
	defer state.mu.Unlock()

	if state.published == nil {
		return statsPublishedChannel{}, false
	}
	published, ok := state.published[channelID]
	return published, ok
}

func (s *StatsService) recordStatsPublishedChannel(guildID, channelID string, published statsPublishedChannel) {
	state := s.getOrInitStatsGuildState(guildID)
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.published == nil {
		state.published = make(map[string]statsPublishedChannel)
	}
	state.published[channelID] = published
}

func (s *StatsService) statsSnapshot(guildID string) (statsGuildSnapshot, bool) {
	v, ok := s.guilds.Load(guildID)
	if !ok {
		return statsGuildSnapshot{}, false
	}
	state := v.(*statsGuildState)

	state.mu.Lock()
	defer state.mu.Unlock()

	if !state.initialized {
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

func (s *StatsService) pruneStatsGuildState(activeGuilds map[string]struct{}) {
	s.lastRun.Range(func(key, value any) bool {
		guildID := key.(string)
		if _, ok := activeGuilds[guildID]; !ok {
			s.lastRun.Delete(guildID)
		}
		return true
	})
	s.guilds.Range(func(key, value any) bool {
		guildID := key.(string)
		if _, ok := activeGuilds[guildID]; !ok {
			s.guilds.Delete(guildID)
		}
		return true
	})
}

const monitoringPersistenceTimeout = 10 * time.Second

func (s *StatsService) streamGuildMembers(ctx context.Context, guildID string) iter.Seq2[MemberSnapshot, error] {
	return func(yield func(MemberSnapshot, error) bool) {
		if ctx == nil {
			ctx = context.Background()
		}
		for m, err := range s.gateway.StreamGuildMembers(ctx, guildID) {
			if !yield(m, err) {
				return
			}
		}
	}
}

func (s *StatsService) getHeartbeat(ctx context.Context) (time.Time, bool, error) {
	// We read the heartbeat from the monitoring service since stats used to be part of it,
	// and monitoring is what asserts the cache is warm.
	return s.store.HeartbeatForBot(ctx, s.botInstanceID)
}
