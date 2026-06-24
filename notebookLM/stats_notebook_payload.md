# Domain Architecture: stats

## Layout Topology
```text
stats/
├── discord_port.go
├── service.go
├── service_apply_test.go
├── service_preemption_test.go
├── service_pure_test.go
├── service_reconcile_test.go
├── service_routing_test.go
└── service_test.go
```

## Source Stream Aggregation

// === FILE: pkg/stats/discord_port.go ===
```go
package stats

import (
	"context"
	"iter"
)

// Gateway defines the contract for Discord API and Gateway interactions
// required by the stats domain.
type Gateway interface {
	// UpdateChannelName updates the name of a voice channel used for stats.
	UpdateChannelName(ctx context.Context, channelID, newName string) error

	// GetChannel retrieves information about a specific channel.
	GetChannel(ctx context.Context, channelID string) (*Channel, error)

	// StreamGuildMembers returns an iterator over all members in a guild.
	StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[MemberSnapshot, error]
}

// Channel represents a Discord channel's basic metadata needed for stats.
type Channel struct {
	ID      string
	Name    string
	GuildID string
}

// MemberSnapshot represents the state of a member needed for stats reconciliation.
type MemberSnapshot struct {
	UserID string
	IsBot  bool
	Roles  iter.Seq[string]
}

```

// === FILE: pkg/stats/service.go ===
```go
package stats

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"runtime/debug"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
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
	GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error]
	Metadata(ctx context.Context, key string) (time.Time, bool, error)
	SetMetadata(ctx context.Context, key string, at time.Time) error
	UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error
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
	logger := s.logger
	if logger == nil {
		logger = slog.Default()
	}
	if guildID == "" {
		return logger
	}
	return logger.With(slog.String("guild_id", guildID))
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
	defer func() {
		if r := recover(); r != nil {
			logger := s.logger
			if logger == nil {
				logger = slog.Default()
			}
			logger.Error("StatsService runCron panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	// initial run
	s.UpdateStatsChannels(ctx)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.UpdateStatsChannels(ctx)
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
		state.applyAdd(userID, snapshot)
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
		state.applyAdd(userID, snapshot)
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

func statsSnapshotFromStoredState(member members.CurrentState, trackedRoles map[string]struct{}) (string, statsMemberSnapshot, bool) {
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
	var filtered []string
	for roleID := range roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		if _, ok := trackedRoles[roleID]; !ok {
			continue
		}
		duplicate := false
		for _, f := range filtered {
			if f == roleID {
				duplicate = true
				break
			}
		}
		if duplicate {
			continue
		}
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

	s.store.SetMetadata(ctx, metaKey, time.Now().UTC())

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

	if err := s.store.UpsertMemberPresenceContext(runCtx, members.PresenceInput{GuildID: guildID, UserID: userID, JoinedAt: joinedAt, SeenAt: time.Now().UTC(), IsBot: isBot}); err != nil {
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

```

// === FILE: pkg/stats/service_apply_test.go ===
```go
package stats

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestApplyMemberAdd(t *testing.T) {
	t.Parallel()
	// test store

	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{},
				Stats: files.StatsConfig{
					Channels: []files.StatsChannelConfig{
						{ChannelID: "c1"},
					},
				},
			},
		}
		return nil
	})

	// Test early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyMemberAdd("guild-stats-main", "user1", time.Now(), false, func(yield func(string) bool) {
		yield("role1")
		yield("role2")
	})

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyMemberAdd("guild-stats-main", "user1", time.Now(), false, func(yield func(string) bool) {
		yield("role1")
		yield("role2")
	})
}

func TestApplyMemberRemove(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "guild-stats-main", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}},
		}
		return nil
	})

	// Testing early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyMemberRemove("guild-stats-main", "user1")

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyMemberRemove("guild-stats-main", "user1")
}

func TestApplyStatsMemberUpdate(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "guild-stats-main", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}, Features: files.FeatureToggles{}, Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{{ChannelID: "c1"}}}},
		}
		return nil
	})

	// Testing early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.ApplyStatsMemberUpdate("guild-stats-main", "user1", true, func(yield func(string) bool) {
		yield("role1")
	})

	store := newMockStateStore()

	svc := NewStatsService(nil, cm, store, slog.Default(), "generic")
	svc.ApplyStatsMemberUpdate("guild-stats-main", "user1", true, func(yield func(string) bool) {
		yield("role1")
	})
}

```

// === FILE: pkg/stats/service_preemption_test.go ===
```go
package stats

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"golang.org/x/sync/errgroup"
)

// blockingStore is a mock that blocks indefinitely until context is canceled.
type blockingStore struct {
	StateStore
	entered chan struct{}
}

func (b *blockingStore) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	close(b.entered)
	<-ctx.Done()
	return time.Time{}, false, ctx.Err()
}

func (b *blockingStore) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	<-ctx.Done()
	return func(yield func(members.CurrentState, error) bool) {}
}

func (b *blockingStore) SetMetadata(ctx context.Context, key string, at time.Time) error {
	return nil
}

func (b *blockingStore) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	return nil
}

func (b *blockingStore) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	return nil
}

func (b *blockingStore) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	return nil
}

func TestStatsService_DatabasePreemption(t *testing.T) {
	t.Parallel()

	store := &blockingStore{
		entered: make(chan struct{}),
	}
	configManager := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)

	monitoringEnabled := true
	guildCfg := files.GuildConfig{
		GuildID: "test-guild",
		BotInstanceTokens: map[string]files.EncryptedString{
			"test-bot": "fake-token",
		},
		FeatureRouting: map[string]string{
			"stats": "test-bot",
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: &monitoringEnabled,
			},
		},
		Stats: files.StatsConfig{
			Channels: []files.StatsChannelConfig{
				{
					ChannelID: "stats-channel",
					RoleID:    "stats-role",
				},
			},
		},
	}
	if err := configManager.AddGuildConfig(guildCfg); err != nil {
		t.Fatalf("Failed to add guild config: %v", err)
	}

	gateway := &mockGateway{}
	s := NewStatsService(gateway, configManager, store, nil, "test-bot")

	ctx, cancel := context.WithCancel(context.Background())

	// Start the service
	if err := s.Start(ctx); err != nil {
		t.Fatalf("Failed to start service: %v", err)
	}

	// Wait until it hits the blocking store
	<-store.entered

	// Preempt the execution via context cancellation
	cancel()

	eg, egCtx := errgroup.WithContext(context.Background())
	done := make(chan struct{})
	eg.Go(func() error {
		select {
		case <-egCtx.Done():
			return egCtx.Err()
		default:
		}
		s.Stop(context.Background())
		close(done)
		return nil
	})

	select {
	case <-done:
		// Success! The database mock cleanly yielded control to ctx.Done()
	case <-time.After(2 * time.Second):
		t.Fatal("Service failed to preempt database operation on context cancellation")
	}

	if err := eg.Wait(); err != nil {
		t.Fatalf("unexpected errgroup wait error: %v", err)
	}
}

```

// === FILE: pkg/stats/service_pure_test.go ===
```go
package stats

import (
	"context"
	"iter"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
)

type mockStateStore struct {
	members      map[string]map[string]members.CurrentState
	metadata     map[string]time.Time
	botHeartbeat map[string]time.Time
}

func newMockStateStore() *mockStateStore {
	return &mockStateStore{
		members: map[string]map[string]members.CurrentState{
			"guild-stats-main": {
				"user1": {
					UserID:   "user1",
					HasBot:   true,
					IsBot:    false,
					Roles:    []string{"role1"},
					JoinedAt: time.Now().UTC(),
				},
			},
		},
		metadata:     map[string]time.Time{"stats_channels.seeded:guild-stats-main": time.Now().UTC()},
		botHeartbeat: map[string]time.Time{"generic": time.Now().UTC()},
	}
}

func (m *mockStateStore) GetActiveGuildMemberStatesContext(ctx context.Context, guildID string) iter.Seq2[members.CurrentState, error] {
	return func(yield func(members.CurrentState, error) bool) {
		for _, v := range m.members[guildID] {
			if !yield(v, nil) {
				return
			}
		}
	}
}

func (m *mockStateStore) Metadata(ctx context.Context, key string) (time.Time, bool, error) {
	t, ok := m.metadata[key]
	return t, ok, nil
}

func (m *mockStateStore) SetMetadata(ctx context.Context, key string, at time.Time) error {
	m.metadata[key] = at
	return nil
}

func (m *mockStateStore) UpsertMemberPresenceContext(ctx context.Context, input members.PresenceInput) error {
	if m.members[input.GuildID] == nil {
		m.members[input.GuildID] = make(map[string]members.CurrentState)
	}
	v := m.members[input.GuildID][input.UserID]
	v.UserID = input.UserID
	v.IsBot = input.IsBot
	m.members[input.GuildID][input.UserID] = v
	return nil
}

func (m *mockStateStore) UpsertMemberRoles(guildID, userID string, roles []string, at time.Time) error {
	if m.members[guildID] == nil {
		m.members[guildID] = make(map[string]members.CurrentState)
	}
	v := m.members[guildID][userID]
	v.UserID = userID
	v.Roles = roles
	m.members[guildID][userID] = v
	return nil
}

func (m *mockStateStore) MarkMemberLeftContext(ctx context.Context, guildID, userID string, at time.Time) error {
	if m.members[guildID] == nil {
		m.members[guildID] = make(map[string]members.CurrentState)
	}
	v := m.members[guildID][userID]
	v.UserID = userID
	v.LeftAt = at
	m.members[guildID][userID] = v
	return nil
}

func (m *mockStateStore) HeartbeatForBot(ctx context.Context, botInstanceID string) (time.Time, bool, error) {
	t, ok := m.botHeartbeat[botInstanceID]
	return t, ok, nil
}

func TestHandlesGuild(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"}, FeatureRouting: map[string]string{"stats": "generic"}},
			{GuildID: "g2", BotInstanceTokens: map[string]files.EncryptedString{"other": "token"}, FeatureRouting: map[string]string{"stats": "other"}},
		}
		return nil
	})

	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	if !svc.handlesGuild("g1") {
		t.Errorf("expected to handle g1")
	}
	if svc.handlesGuild("g2") {
		t.Errorf("expected not to handle g2")
	}
	if svc.handlesGuild("g3") {
		t.Errorf("expected not to handle g3")
	}
}

func TestStatsServiceMethods(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	if svc.Name() != "stats" {
		t.Errorf("unexpected name")
	}
	// svc.Type() returns svc.ServiceType, which is an integer. Let's just call it.
	svc.Type()
	svc.Priority()
	svc.Dependencies()
	svc.Stats()

	ctx := context.Background()
	svc.HealthCheck(ctx)
}

func TestShouldRunStatsUpdate(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	interval := time.Minute
	// first run
	if !svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected true on first run")
	}
	// second run immediately after
	if svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected false on second run")
	}

	// Force update
	svc.ForceGuildUpdate("g1")
	if !svc.shouldRunStatsUpdate("g1", interval) {
		t.Errorf("expected true after force")
	}
}

func TestStatsTrackedRoles(t *testing.T) {
	t.Parallel()
	channels := []files.StatsChannelConfig{
		{ChannelID: "1", RoleID: "r1"},
		{ChannelID: "2"},
		{ChannelID: "3", RoleID: "r2"},
		{ChannelID: "4", RoleID: "r1"},
	}
	trackedRoles, key := statsTrackedRoles(channels)
	if len(trackedRoles) != 2 {
		t.Errorf("expected 2 tracked roles, got %d", len(trackedRoles))
	}
	_, hasR1 := trackedRoles["r1"]
	_, hasR2 := trackedRoles["r2"]
	if !hasR1 || !hasR2 {
		t.Errorf("missing tracked roles")
	}
	// "r1,r2" or "r2,r1"
	if key != "r1,r2" && key != "r2,r1" {
		t.Errorf("unexpected key: %s", key)
	}
}

func TestStatsRequiresBotClassification(t *testing.T) {
	t.Parallel()
	if statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "all"}}) {
		t.Errorf("expected false")
	}
	if !statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "humans"}}) {
		t.Errorf("expected true")
	}
	if !statsRequiresBotClassification([]files.StatsChannelConfig{{MemberType: "bots"}}) {
		t.Errorf("expected true")
	}
}

func TestFilterTrackedRoles(t *testing.T) {
	t.Parallel()
	trackedRoles := map[string]struct{}{
		"r1": {},
		"r3": {},
	}
	roles := []string{"r1", "r2", "r3", "r4"}
	filtered := filterTrackedRoles(slices.Values(roles), trackedRoles)
	if len(filtered) != 2 {
		t.Errorf("expected 2 filtered roles, got %d", len(filtered))
	}
	if filtered[0] != "r1" && filtered[1] != "r3" {
		t.Errorf("unexpected filtered roles")
	}
}

func TestStatsCountForChannel(t *testing.T) {
	t.Parallel()
	state := newStatsGuildState("r1", nil)
	state.applyAdd("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("user2", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("bot1", statsMemberSnapshot{isBot: true, trackedRoles: []string{}})

	snapshot := statsGuildSnapshot{
		totals:     state.totals,
		roleTotals: state.roleTotals,
	}

	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all"}); count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "humans"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "bots"}); count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r1"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r2"}); count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

func TestStatsGuildStateMethods(t *testing.T) {
	t.Parallel()
	state := newStatsGuildState("r1", map[string]statsPublishedChannel{
		"c1": {count: 10, name: "test", label: "test"},
	})

	state.applyAdd("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("user2", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}})
	state.applyAdd("bot1", statsMemberSnapshot{isBot: true, trackedRoles: []string{}})

	state.applyUpdate("user1", statsMemberSnapshot{isBot: false, trackedRoles: []string{}})
	state.applyRemove("user2")

	// user1: lost r1
	// user2: removed completely
	// bot1: unchanged

	snapshot := statsGuildSnapshot{
		totals:     state.totals,
		roleTotals: state.roleTotals,
	}

	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all"}); count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
	if count := statsCountForChannel(snapshot, files.StatsChannelConfig{MemberType: "all", RoleID: "r1"}); count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	state.applyDelta("user3", statsMemberSnapshot{isBot: false, trackedRoles: []string{"r1"}}, true, false) // isAdd=true, isRemove=false
	if state.totals.humans != 2 {
		t.Errorf("expected 2 humans, got %d", state.totals.humans)
	}
}

func TestStatsSnapshotHelpers(t *testing.T) {
	t.Parallel()
	gwMem := MemberSnapshot{
		UserID: "u1",
		IsBot:  true,
		Roles: func(yield func(string) bool) {
			yield("r1")
			yield("r2")
		},
	}
	trackedRoles := map[string]struct{}{"r1": {}}

	_, snap, active := statsSnapshotFromGatewayMember(gwMem, trackedRoles)
	if !active {
		t.Errorf("expected active")
	}
	if !snap.isBot {
		t.Errorf("expected isBot to be true")
	}
	if len(snap.trackedRoles) != 1 || snap.trackedRoles[0] != "r1" {
		t.Errorf("unexpected tracked roles: %v", snap.trackedRoles)
	}

	storedState := members.CurrentState{
		UserID: "u2",
		IsBot:  false,
		Roles:  []string{"r1", "r2"},
		Active: true,
	}
	_, snap2, active2 := statsSnapshotFromStoredState(storedState, trackedRoles)
	if !active2 {
		t.Errorf("expected active")
	}
	if snap2.isBot {
		t.Errorf("expected isBot to be false")
	}
	if len(snap2.trackedRoles) != 1 || snap2.trackedRoles[0] != "r1" {
		t.Errorf("unexpected tracked roles: %v", snap2.trackedRoles)
	}
}

func TestStatsIntervalHelpers(t *testing.T) {
	t.Parallel()
	// interval logic testing
	if statsInterval() != 5*time.Minute {
		t.Errorf("expected 5m default")
	}

	if statsReconcileInterval() != 6*time.Hour {
		t.Errorf("expected 6 hour reconcile interval")
	}
	statsStoreFreshnessLimit() // Just execute for coverage
	if statsSeedMetadataKey("g1") == "" {
		t.Errorf("unexpected seed key")
	}
}

func TestStatsStateAndStoreHelpers(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	ctx := context.Background()

	// These depend on DB, so skip them when we don't have a DB mock.
	// We will only call publishStatsForGuild and streamGuildMembers since they have nil checks.

	// publishStatsForGuild should exit quickly if store/gw is nil
	err := svc.publishStatsForGuild(ctx, files.GuildConfig{GuildID: "g1"})
	if err == nil {
		t.Errorf("expected err for nil store/gw")
	}

	// streamGuildMembers should just return nil err since gw is nil
	svc.streamGuildMembers(ctx, "g1")
}

func TestStatsGuildStateMemoryHelpers(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	svc := NewStatsService(nil, cm, nil, slog.Default(), "generic")

	// test replace and prune
	state := newStatsGuildState("r1", nil)
	state.initialized = true
	svc.replaceStatsGuildState("g1", state)

	channels := svc.statsPublishedChannels("g1")
	if channels == nil {
		t.Errorf("expected empty map, got nil")
	}

	ch, ok := svc.statsPublishedChannel("g1", "c1")
	if ok || ch.name != "" {
		t.Errorf("expected empty channel")
	}

	svc.recordStatsPublishedChannel("g1", "c1", statsPublishedChannel{count: 10, name: "test", label: "label"})
	ch2, ok2 := svc.statsPublishedChannel("g1", "c1")
	if !ok2 || ch2.count != 10 || ch2.name != "test" || ch2.label != "label" {
		t.Errorf("unexpected channel properties")
	}

	snap, okSnap := svc.statsSnapshot("g1")
	if !okSnap || snap.totals.all != 0 {
		t.Errorf("expected 0 totals")
	}
}

func TestStatsReconcileInterval(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "g1",
				FeatureRouting:    map[string]string{"stats": "generic"},
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "token"},
				Stats:             files.StatsConfig{},
			},
		}
		return nil
	})

	NewStatsService(nil, cm, newMockStateStore(), slog.Default(), "generic")

	if statsReconcileInterval() != defaultStatsReconcileInterval {
		t.Errorf("expected default")
	}
}

```

// === FILE: pkg/stats/service_reconcile_test.go ===
```go
package stats

import (
	"context"
	"iter"
	"log/slog"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockGateway struct {
	members []MemberSnapshot
	channel *Channel
}

func (m *mockGateway) UpdateChannelName(ctx context.Context, channelID, newName string) error {
	return nil
}

func (m *mockGateway) GetChannel(ctx context.Context, channelID string) (*Channel, error) {
	if m.channel != nil {
		return m.channel, nil
	}
	return &Channel{ID: channelID, Name: "test", GuildID: "guild-stats-main"}, nil
}

func (m *mockGateway) StreamGuildMembers(ctx context.Context, guildID string) iter.Seq2[MemberSnapshot, error] {
	return func(yield func(MemberSnapshot, error) bool) {
		for _, mem := range m.members {
			if !yield(mem, nil) {
				return
			}
		}
	}
}

func TestReconcileGuild(t *testing.T) {
	t.Parallel()

	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
		}
		return nil
	})

	gateway := &mockGateway{
		members: []MemberSnapshot{
			{
				UserID: "u1",
				IsBot:  false,
				Roles: func(yield func(string) bool) {
					yield("role1")
				},
			},
			{
				UserID: "u2",
				IsBot:  true,
				Roles: func(yield func(string) bool) {
					yield("role2")
				},
			},
		},
	}

	// Test early return with nil store
	svcNil := NewStatsService(gateway, cm, nil, slog.Default(), "generic")
	svcNil.reconcileStatsForGuild(context.Background(), files.GuildConfig{GuildID: "guild-stats-main"})

	store := newMockStateStore()
	svc := NewStatsService(gateway, cm, store, slog.Default(), "generic")
	ctx := context.Background()

	gcfg, _, _, ok := svc.statsGuildConfig("guild-stats-main")
	if !ok {
		t.Fatalf("statsGuildConfig returned ok=false")
	}
	if gcfg.GuildID == "" {
		t.Fatalf("gcfg is empty")
	}
	err := svc.reconcileStatsForGuild(ctx, gcfg)
	if err != nil {
		t.Fatalf("reconcileStatsForGuild failed: %v", err)
	}
}

func TestReconcileAllGuilds(t *testing.T) {
	t.Parallel()

	cm := newTestConfigManager(t)
	cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
		}
		return nil
	})

	gateway := &mockGateway{}

	store := newMockStateStore()
	svc := NewStatsService(gateway, cm, store, slog.Default(), "generic")
	ctx := context.Background()

	svc.UpdateStatsChannels(ctx)
}

func TestStatsServiceLifecycle(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)

	// Test early return with nil store
	svcNil := NewStatsService(nil, cm, nil, slog.Default(), "generic")
	svcNil.reconcileStatsForGuild(context.Background(), files.GuildConfig{GuildID: "guild-stats-main"})

	svc := NewStatsService(nil, cm, newMockStateStore(), slog.Default(), "generic")

	if err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !svc.IsRunning() {
		t.Errorf("expected IsRunning to be true")
	}

	if err := svc.Stop(context.Background()); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if svc.IsRunning() {
		t.Errorf("expected IsRunning to be false")
	}
}

```

// === FILE: pkg/stats/service_routing_test.go ===
```go
package stats

import (
	"context"
	"log/slog"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

type mockConfigStore struct {
	cfg *files.BotConfig
}

func (m *mockConfigStore) Load() (*files.BotConfig, error) {
	if m.cfg == nil {
		return &files.BotConfig{}, nil
	}
	return m.cfg, nil
}

func (m *mockConfigStore) Save(cfg *files.BotConfig) error {
	m.cfg = cfg
	return nil
}

func (m *mockConfigStore) Transaction(fn func(cfg *files.BotConfig) error) (bool, error) {
	if m.cfg == nil {
		m.cfg = &files.BotConfig{}
	}
	if err := fn(m.cfg); err != nil {
		return false, err
	}
	return true, nil
}

func (m *mockConfigStore) Describe() string {
	return "mock"
}

func (m *mockConfigStore) Exists() (bool, error) {
	return m.cfg != nil, nil
}

func newTestConfigManager(t *testing.T) *files.ConfigManager {
	t.Helper()
	cm := files.NewConfigManagerWithStore(&mockConfigStore{}, nil)
	cfg, _, err := cm.LoadConfigFromStore()
	if err != nil {
		t.Fatalf("failed to load config manager: %v", err)
	}
	cm.ApplyConfig(cfg)
	return cm
}

func TestStatsServiceHandlesGuild(t *testing.T) {
	t.Parallel()
	cm := newTestConfigManager(t)

	_, err := cm.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID: "guild-stats-main",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c1"},
				},
				},
			},
			{
				GuildID: "guild-stats-custom",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				FeatureRouting: map[string]string{
					"stats": "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c2"},
				},
				},
			},
			{
				GuildID: "guild-stats-default",
				BotInstanceTokens: map[string]files.EncryptedString{
					"generic": "token",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: testBoolPtr(true),
					},
				},
				Stats: files.StatsConfig{Channels: []files.StatsChannelConfig{
					{ChannelID: "c3"},
				},
				},
			},
		}
		return nil
	})

	if err != nil {
		t.Fatalf("update config failed: %v", err)
	}

	logger := slog.Default()
	genericSvc := NewStatsService(nil, cm, nil, logger, "generic")
	defaultSvc := NewStatsService(nil, cm, nil, logger, "")

	if !genericSvc.handlesGuild("guild-stats-main") {
		t.Errorf("expected generic service to handle guild-stats-main")
	}

	if !genericSvc.handlesGuild("guild-stats-custom") {
		t.Errorf("expected generic service to handle guild-stats-custom")
	}

	if genericSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected generic service to NOT handle guild-stats-default (unrouted)")
	}
	if defaultSvc.handlesGuild("guild-stats-default") {
		t.Errorf("expected default service to NOT handle guild-stats-default (unrouted sentinel)")
	}
}

func testBoolPtr(v bool) *bool { return &v }

```

// === FILE: pkg/stats/service_test.go ===
```go
package stats

import "testing"

func TestNormalizeMemberType(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":       "all",
		"ALL":    "all",
		"bots":   "bots",
		"Bot":    "bots",
		"humans": "humans",
		"HUMAN":  "humans",
		"weird":  "all",
	}

	for raw, want := range cases {
		if got := normalizeMemberType(raw); got != want {
			t.Fatalf("normalizeMemberType(%q) = %q; want %q", raw, got, want)
		}
	}
}

func TestMemberTypeMatches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw   string
		isBot bool
		want  bool
	}{
		{"", false, true},
		{"", true, true},
		{"bots", true, true},
		{"bots", false, false},
		{"humans", false, true},
		{"humans", true, false},
	}

	for _, tt := range tests {
		if got := memberTypeMatches(tt.raw, tt.isBot); got != tt.want {
			t.Fatalf("memberTypeMatches(%q, %v) = %v; want %v", tt.raw, tt.isBot, got, tt.want)
		}
	}
}

func TestRenderStatsChannelName(t *testing.T) {
	t.Parallel()
	got := renderStatsChannelName("Total Proxies: ", "", 42)
	if got != "Total Proxies: 42" {
		t.Fatalf("default template: got %q", got)
	}

	got = renderStatsChannelName("", "", 7)
	if got != "7" {
		t.Fatalf("default template without label: got %q", got)
	}

	got = renderStatsChannelName("Bunny", "{label} | {count}", 3)
	if got != "Bunny | 3" {
		t.Fatalf("custom template: got %q", got)
	}

	got = renderStatsChannelName("☆ custom proxies ☆ : ", "", 10)
	if got != "☆ custom proxies ☆ : 10" {
		t.Fatalf("pre-formatted label: got %q", got)
	}
}

```

