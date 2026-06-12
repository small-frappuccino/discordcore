package stats

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	svc "github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

const (
	defaultStatsInterval          = 30 * time.Minute
	defaultStatsReconcileInterval = 6 * time.Hour
	maxStatsReconcileInterval     = 24 * time.Hour
	statsSeedMetadataPrefix       = "stats_channels.seeded:"
	heartbeatInterval             = 5 * time.Minute
	monitoringDependencyTimeout   = 15 * time.Second
)

// StatsService manages the stats-channel state. guilds and lastRun are protected by mu.
// Construct with NewStatsService; the zero value has nil maps.
type StatsService struct {
	session              *discordgo.Session
	configManager        *files.ConfigManager
	store                *storage.Store
	logger               *slog.Logger
	botInstanceID        string
	defaultBotInstanceID string

	mu      sync.RWMutex
	guilds  map[string]*statsGuildState
	lastRun map[string]time.Time

	cancel        context.CancelFunc
	wg            sync.WaitGroup
	eventHandlers []func()
}

// NewStatsService news stats service.
func NewStatsService(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	logger *slog.Logger,
	botInstanceID string,
	defaultBotInstanceID string,
) *StatsService {
	return &StatsService{
		session:              session,
		configManager:        configManager,
		store:                store,
		logger:               logger,
		botInstanceID:        botInstanceID,
		defaultBotInstanceID: defaultBotInstanceID,
		guilds:               make(map[string]*statsGuildState),
		lastRun:              make(map[string]time.Time),
	}
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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancel != nil
}

// HealthCheck healths check.
func (s *StatsService) HealthCheck(ctx context.Context) svc.HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
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

	s.eventHandlers = append(s.eventHandlers,
		s.session.AddHandler(s.HandleStatsMemberAdd),
		s.session.AddHandler(s.HandleStatsMemberRemove),
		s.session.AddHandler(s.HandleStatsMemberUpdate),
	)
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
	s.mu.Lock()
	if s.cancel == nil {
		s.mu.Unlock()
		return nil
	}
	s.cancel()
	s.cancel = nil
	for _, h := range s.eventHandlers {
		if h != nil {
			h()
		}
	}
	s.eventHandlers = nil
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *StatsService) handlesGuild(guildID string) bool {
	if s == nil || s.configManager == nil {
		return false
	}
	cfg := s.configManager.GuildConfig(guildID)
	if cfg == nil {
		return false
	}
	if !cfg.BelongsToBotInstance(s.botInstanceID) {
		return false
	}
	resolvedID, _ := cfg.ResolveFeatureBotInstanceID("roles", s.defaultBotInstanceID)
	return resolvedID == s.botInstanceID
}

func (s *StatsService) scopedConfig() *files.BotConfig {
	if s == nil || s.configManager == nil {
		return nil
	}
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

// UpdateStatsChannels updates stats channels.
func (s *StatsService) UpdateStatsChannels(ctx context.Context) error {
	if s == nil || s.session == nil || s.configManager == nil {
		return nil
	}
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
		if !features.Services.Monitoring || !features.StatsChannels || !Enabled(gcfg.Stats) {
			continue
		}
		activeGuilds[gcfg.GuildID] = struct{}{}

		needsReconcile, prepErr := s.prepareStatsState(ctx, gcfg)
		if prepErr != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to prepare stats state",
				"operation", "monitoring.stats.prepare",
				"guildID", gcfg.GuildID,
				"err", prepErr,
			)
		}
		shouldPublish := s.shouldRunStatsUpdate(gcfg.GuildID, statsInterval(gcfg.Stats))
		if needsReconcile {
			if err := s.reconcileStatsForGuild(ctx, gcfg); err != nil {
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
		if err := s.publishStatsForGuild(ctx, gcfg); err != nil {
			log.ErrorLoggerRaw().Error(
				"Failed to update stats channels",
				"operation", "monitoring.stats.publish",
				"guildID", gcfg.GuildID,
				"err", err,
			)
		}
	}

	s.pruneStatsGuildState(activeGuilds)
	return nil
}

func Enabled(cfg files.StatsConfig) bool {
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

func (s *StatsService) shouldRunStatsUpdate(guildID string, interval time.Duration) bool {
	if guildID == "" {
		return false
	}
	if interval <= 0 {
		interval = defaultStatsInterval
	}
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastRun == nil {
		s.lastRun = make(map[string]time.Time)
	}
	last, ok := s.lastRun[guildID]
	if ok && now.Sub(last) < interval {
		return false
	}
	s.lastRun[guildID] = now
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
		userID, snapshot, ok := statsSnapshotFromMember(member, trackedRoles)
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

	log.ApplicationLogger().Info(
		"Reconciled stats counters",
		"operation", "monitoring.stats.reconcile",
		"guildID", gcfg.GuildID,
		"members", len(state.members),
		"trackedRoles", len(state.roleTotals),
	)
	return nil
}

func (s *StatsService) prepareStatsState(ctx context.Context, gcfg files.GuildConfig) (bool, error) {
	_, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)

	s.mu.Lock()
	state := s.ensureStatsGuildStateLocked(gcfg.GuildID)
	keysMatch := state.trackedRolesKey == trackedRolesKey
	var needsReconcile, skipRest bool
	if state.initialized && keysMatch && !state.dirty {
		lastReconciled := state.lastReconciled
		needsReconcile = time.Since(lastReconciled) >= statsReconcileInterval(gcfg.Stats)
		skipRest = true
	}
	s.mu.Unlock()

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
	if s == nil || s.store == nil {
		return false, nil
	}
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
	limit := statsStoreFreshnessLimit(gcfg.Stats)
	lastHeartbeat, ok, err := s.getHeartbeat(ctx)
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

func (s *StatsService) hasStatsSeed(ctx context.Context, guildID string) bool {
	if s == nil || s.store == nil {
		return false
	}
	_, ok, err := s.store.Metadata(ctx, statsSeedMetadataKey(guildID))
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

func (s *StatsService) markStatsSeeded(ctx context.Context, guildID string, at time.Time) {
	if s == nil || s.store == nil || strings.TrimSpace(guildID) == "" {
		return
	}
	if at.IsZero() {
		at = time.Now().UTC()
	}
	if err := s.store.SetMetadata(ctx, statsSeedMetadataKey(guildID), at); err != nil {
		log.ApplicationLogger().Warn(
			"Failed to persist stats seed metadata",
			"operation", "monitoring.stats.seed.write",
			"guildID", guildID,
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

func (s *StatsService) updateStatsChannelName(ctx context.Context, guildID string, cfg files.StatsChannelConfig, count int) error {
	channelID := strings.TrimSpace(cfg.ChannelID)
	if channelID == "" {
		return nil
	}

	published, hasPublished := s.statsPublishedChannel(guildID, channelID)
	label := strings.TrimSpace(cfg.Label)
	if label == "" {
		label = strings.TrimSpace(published.label)
	}

	channelName := ""
	if !hasPublished || label == "" {
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

	if _, err := runWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Channel, error) {
		return s.session.ChannelEdit(channelID, &discordgo.ChannelEdit{Name: newName})
	}); err != nil {
		return fmt.Errorf("channel edit: %w", err)
	}

	s.recordStatsPublishedChannel(guildID, channelID, statsPublishedChannel{
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

func (s *StatsService) resolveChannel(ctx context.Context, channelID string) (*discordgo.Channel, error) {
	if s.session == nil || channelID == "" {
		return nil, fmt.Errorf("session not available or channel id empty")
	}
	if s.session.State != nil {
		if ch, err := s.session.State.Channel(channelID); err == nil && ch != nil {
			return ch, nil
		}
	}
	return runWithTimeout(ctx, monitoringDependencyTimeout, func() (*discordgo.Channel, error) {
		return s.session.Channel(channelID)
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

// HandleStatsMemberAdd handles stats member add.
func (s *StatsService) HandleStatsMemberAdd(_ *discordgo.Session, m *discordgo.GuildMemberAdd) {
	if m == nil || m.Member == nil || m.Member.User == nil {
		return
	}
	if !s.handlesGuild(m.GuildID) {
		return
	}
	s.applyStatsMemberAdd(m.Member)
}

// HandleStatsMemberRemove handles stats member remove.
func (s *StatsService) HandleStatsMemberRemove(_ *discordgo.Session, m *discordgo.GuildMemberRemove) {
	if m == nil || m.User == nil {
		return
	}
	if !s.handlesGuild(m.GuildID) {
		return
	}
	s.applyStatsMemberRemove(m.GuildID, m.User.ID)
}

// HandleStatsMemberUpdate handles stats member update.
func (s *StatsService) HandleStatsMemberUpdate(_ *discordgo.Session, m *discordgo.GuildMemberUpdate) {
	if m == nil || m.User == nil {
		return
	}
	if !s.handlesGuild(m.GuildID) {
		return
	}
	s.ApplyStatsMemberUpdate(m.GuildID, m.User.ID, m.User.Bot, m.Roles)
}

func (s *StatsService) applyStatsMemberAdd(member *discordgo.Member) {
	if member == nil || member.User == nil {
		return
	}
	guildID := strings.TrimSpace(member.GuildID)
	userID := strings.TrimSpace(member.User.ID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := s.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	s.persistStatsMemberActive(guildID, userID, member.JoinedAt, member.User.Bot, member.Roles)
	snapshot := statsMemberSnapshot{
		isBot:        member.User.Bot,
		trackedRoles: filterTrackedRoles(member.Roles, trackedRoles),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyAdd(userID, snapshot) {
		state.dirty = true
	}
}

// ApplyStatsMemberUpdate applys stats member update.
func (s *StatsService) ApplyStatsMemberUpdate(guildID, userID string, isBot bool, roles []string) {
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	_, trackedRoles, trackedRolesKey, enabled := s.statsGuildConfig(guildID)
	if !enabled {
		return
	}
	s.persistStatsMemberActive(guildID, userID, time.Time{}, isBot, roles)
	snapshot := statsMemberSnapshot{
		isBot:        isBot,
		trackedRoles: filterTrackedRoles(roles, trackedRoles),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyUpdate(userID, snapshot) {
		state.dirty = true
	}
}

func (s *StatsService) applyStatsMemberRemove(guildID, userID string) {
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

	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.ensureStatsGuildStateLocked(guildID)
	if !state.initialized || state.trackedRolesKey != trackedRolesKey {
		state.dirty = true
		return
	}
	if !state.applyRemove(userID) {
		state.dirty = true
	}
}

func (s *StatsService) persistStatsMemberActive(guildID, userID string, joinedAt time.Time, isBot bool, roles []string) {
	if s == nil || s.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := runErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		if err := s.store.UpsertMemberPresenceContext(runCtx, storage.MemberPresenceInput{GuildID: guildID, UserID: userID, JoinedAt: joinedAt, SeenAt: time.Now().UTC(), IsBot: isBot}); err != nil {
			return fmt.Errorf("upsert member presence: %w", err)
		}
		if err := s.store.UpsertMemberRoles(guildID, userID, roles, time.Now().UTC()); err != nil {
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

func (s *StatsService) persistStatsMemberLeft(guildID, userID string) {
	if s == nil || s.store == nil {
		return
	}
	guildID = strings.TrimSpace(guildID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return
	}

	err := runErrWithTimeoutContext(context.Background(), monitoringPersistenceTimeout, func(runCtx context.Context) error {
		return s.store.MarkMemberLeftContext(runCtx, guildID, userID, time.Now().UTC())
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
		if !features.Services.Monitoring || !features.StatsChannels || !Enabled(gcfg.Stats) {
			return gcfg, nil, "", false
		}
		trackedRoles, trackedRolesKey := statsTrackedRoles(gcfg.Stats.Channels)
		return gcfg, trackedRoles, trackedRolesKey, true
	}
	return files.GuildConfig{}, nil, "", false
}

func (s *StatsService) ensureStatsGuildStateLocked(guildID string) *statsGuildState {
	if s.guilds == nil {
		s.guilds = make(map[string]*statsGuildState)
	}
	state := s.guilds[guildID]
	if state != nil {
		return state
	}
	state = newStatsGuildState("", nil)
	s.guilds[guildID] = state
	return state
}

func (s *StatsService) replaceStatsGuildState(guildID string, state *statsGuildState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.guilds == nil {
		s.guilds = make(map[string]*statsGuildState)
	}
	s.guilds[guildID] = state
}

func (s *StatsService) statsPublishedChannels(guildID string) map[string]statsPublishedChannel {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.guilds == nil {
		return nil
	}
	state := s.guilds[guildID]
	if state == nil {
		return nil
	}
	return cloneStatsPublishedChannels(state.published)
}

func (s *StatsService) statsPublishedChannel(guildID, channelID string) (statsPublishedChannel, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.guilds == nil {
		return statsPublishedChannel{}, false
	}
	state := s.guilds[guildID]
	if state == nil || state.published == nil {
		return statsPublishedChannel{}, false
	}
	published, ok := state.published[channelID]
	return published, ok
}

func (s *StatsService) recordStatsPublishedChannel(guildID, channelID string, published statsPublishedChannel) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureStatsGuildStateLocked(guildID)
	if state.published == nil {
		state.published = make(map[string]statsPublishedChannel)
	}
	state.published[channelID] = published
}

func (s *StatsService) statsSnapshot(guildID string) (statsGuildSnapshot, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.guilds == nil {
		return statsGuildSnapshot{}, false
	}
	state := s.guilds[guildID]
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

func (s *StatsService) pruneStatsGuildState(activeGuilds map[string]struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for guildID := range s.lastRun {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(s.lastRun, guildID)
	}
	for guildID := range s.guilds {
		if _, ok := activeGuilds[guildID]; ok {
			continue
		}
		delete(s.guilds, guildID)
	}
}

const monitoringPersistenceTimeout = 10 * time.Second

func runWithTimeout[T any](ctx context.Context, timeout time.Duration, fn func() (T, error)) (T, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)
	resCh := make(chan T, 1)
	go func() {
		res, err := fn()
		if err != nil {
			errCh <- err
		} else {
			resCh <- res
		}
	}()
	select {
	case <-ctx.Done():
		var zero T
		return zero, ctx.Err()
	case err := <-errCh:
		var zero T
		return zero, err
	case res := <-resCh:
		return res, nil
	}
}

func runErrWithTimeoutContext(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- fn(ctx)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func (s *StatsService) streamGuildMembers(ctx context.Context, guildID string) iter.Seq2[*discordgo.Member, error] {
	return func(yield func(*discordgo.Member, error) bool) {
		if s == nil || s.session == nil {
			yield(nil, fmt.Errorf("discord session is unavailable"))
			return
		}
		if ctx == nil {
			ctx = context.Background()
		}
		pageSize := 1000

		after := ""
		for {
			if err := ctx.Err(); err != nil {
				yield(nil, fmt.Errorf("streamGuildMembers: %w", err))
				return
			}
			members, err := runWithTimeout(ctx, monitoringDependencyTimeout, func() ([]*discordgo.Member, error) {
				return s.session.GuildMembers(guildID, after, pageSize)
			})
			if err != nil {
				yield(nil, fmt.Errorf("streamGuildMembers: %w", err))
				return
			}
			if len(members) == 0 {
				return
			}
			for _, m := range members {
				if !yield(m, nil) {
					return
				}
			}
			if len(members) < pageSize {
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

func (s *StatsService) getHeartbeat(ctx context.Context) (time.Time, bool, error) {
	if s == nil || s.store == nil {
		return time.Time{}, false, nil
	}
	// We read the heartbeat from the monitoring service since stats used to be part of it,
	// and monitoring is what asserts the cache is warm.
	return s.store.HeartbeatForBot(ctx, s.botInstanceID)
}
