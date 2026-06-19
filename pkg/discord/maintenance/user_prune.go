package maintenance

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/logging"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/theme"
	"github.com/small-frappuccino/discordgo"
)

const (
	pruneDayOfMonth                  = 28
	pruneInactiveDays         uint32 = 30
	pruneCheckInterval               = time.Hour
	userPruneLastRunKeyPrefix        = "user_prune_last_run:"
)

type pruneResponse struct {
	Pruned *uint32 `json:"pruned"`
}

// UserPruneService runs the scheduled background job that prunes members
// inactive beyond the configured threshold. Start launches the loop once; Stop
// closes the stop channel and waits for it to drain. Unlike qotd.RuntimeService,
// Start does not reinitialize the stop channel, so the service is single-use and
// must not be restarted after Stop.
type UserPruneService struct {
	session       *discordgo.Session
	configManager *files.ConfigManager
	botID         string
	store         *storage.Store

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup

	mu        sync.RWMutex
	running   bool
	startTime time.Time

	dependencies []string
}

// NewUserPruneService news user prune service for bot.
func NewUserPruneService(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	store *storage.Store,
	botInstanceID string,
) *UserPruneService {
	return &UserPruneService{
		session:       session,
		configManager: configManager,
		botID:         files.NormalizeBotInstanceID(botInstanceID),
		store:         store,
		stopCh:        make(chan struct{}),
	}
}

// SetDependencies allows the orchestrator to inject dynamic dependencies.
func (s *UserPruneService) SetDependencies(deps []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dependencies = append([]string(nil), deps...)
}

// Name returns the service name.
func (s *UserPruneService) Name() string { return "user-prune" }

// Type returns the service type.
func (s *UserPruneService) Type() service.ServiceType { return service.TypeMonitoring }

// Priority returns the service priority.
func (s *UserPruneService) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns the service dependencies.
func (s *UserPruneService) Dependencies() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.dependencies...)
}

// HealthCheck returns the current health status.
func (s *UserPruneService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "User Prune Service is active",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics.
func (s *UserPruneService) Stats() service.ServiceStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var uptime time.Duration
	if s.running {
		uptime = time.Since(s.startTime)
	}

	return service.ServiceStats{
		StartTime: s.startTime,
		Uptime:    uptime,
		Metrics: []service.ServiceMetric{
			{Label: "Status", Value: "Running"},
		},
	}
}

// Start starts.
func (s *UserPruneService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()
	return nil
}

// Stop stops.
func (s *UserPruneService) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stopCh) })

	// Wait for loop to finish or context to expire
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}

	s.mu.Lock()
	s.running = false
	s.mu.Unlock()
	return nil
}

// IsRunning is running.
func (s *UserPruneService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func calculateJitter(base time.Duration) time.Duration {
	jitterFraction := 0.1 + rand.Float64()*0.1
	jitterAmount := time.Duration(float64(base) * jitterFraction)
	return base + jitterAmount
}

func (s *UserPruneService) loop() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.ApplicationLogger().Error("UserPrune maintenance loop panic caught", "panic", r, "stack", string(debug.Stack()))
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	s.runIfDue(ctx, time.Now().UTC())

	for {
		timer := time.NewTimer(calculateJitter(pruneCheckInterval))
		select {
		case <-timer.C:
			s.runIfDue(ctx, time.Now().UTC())
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

func (s *UserPruneService) runIfDue(ctx context.Context, now time.Time) {
	if s.configManager == nil {
		return
	}
	cfg := s.configManager.Config()
	if cfg == nil {
		return
	}
	guilds := cfg.GuildsForBotInstance(s.botID)
	if len(guilds) == 0 {
		return
	}
	if s.session == nil || s.store == nil {
		return
	}
	if !isPruneExecutionDay(now) {
		return
	}
	runCtx := ctx

	botID := s.currentBotID()

	for _, gcfg := range guilds {

		if !gcfg.UserPrune.Enabled {
			continue
		}
		resolvedID, _ := gcfg.ResolveFeatureBotInstanceID("moderation")
		if resolvedID != s.botID {
			continue
		}
		if s.didRunGuildPruneThisMonth(runCtx, gcfg.GuildID, now) {
			continue
		}

		pruned, estimated, err := s.executeGuildPrune(gcfg.GuildID)
		if err != nil {
			log.ApplicationLogger().Warn(
				"User prune: failed to execute Discord prune",
				"guildID", gcfg.GuildID,
				"day", pruneDayOfMonth,
				"days", pruneInactiveDays,
				"err", err,
			)
			continue
		}

		if err := s.markGuildPruneRun(runCtx, gcfg.GuildID, now); err != nil {
			log.ApplicationLogger().Warn(
				"User prune: failed to persist run marker",
				"guildID", gcfg.GuildID,
				"err", err,
			)
		}

		log.ApplicationLogger().Info(
			"User prune completed via native Discord prune",
			"guildID", gcfg.GuildID,
			"day", pruneDayOfMonth,
			"days", pruneInactiveDays,
			"pruned", pruned,
			"estimated", estimated,
		)
		s.sendRunEmbed(gcfg.GuildID, botID, estimated, pruned)
	}
}

func (s *UserPruneService) currentBotID() string {
	if s.session == nil || s.session.State == nil || s.session.State.User == nil {
		return ""
	}
	return strings.TrimSpace(s.session.State.User.ID)
}

func (s *UserPruneService) executeGuildPrune(guildID string) (uint32, uint32, error) {
	estimated, err := s.guildPruneCount(guildID, pruneInactiveDays)
	if err != nil {
		log.ApplicationLogger().Warn(
			"User prune: failed to fetch prune estimate",
			"guildID", guildID,
			"day", pruneDayOfMonth,
			"days", pruneInactiveDays,
			"err", err,
		)
	}

	reason := truncateAuditReason("automatic monthly Discord prune via discordmain (day 28, 30 days inactive)")
	pruned, pruneErr := s.guildPrune(guildID, pruneInactiveDays, reason)
	if pruneErr != nil {
		return 0, estimated, pruneErr
	}
	return pruned, estimated, nil
}

func (s *UserPruneService) guildPruneCount(guildID string, days uint32) (uint32, error) {
	if s.session == nil || guildID == "" {
		return 0, fmt.Errorf("session or guildID is empty")
	}
	if days < 1 {
		return 0, fmt.Errorf("invalid prune days: %d", days)
	}

	endpoint := discordgo.EndpointGuildPrune(guildID)
	uri := fmt.Sprintf("%s?days=%d", endpoint, days)
	body, err := s.session.RequestWithBucketID("GET", uri, nil, endpoint)
	if err != nil {
		return 0, fmt.Errorf("request prune count: %w", err)
	}

	var resp pruneResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("decode prune count response: %w", err)
	}
	if resp.Pruned == nil {
		return 0, nil
	}
	return *resp.Pruned, nil
}

func (s *UserPruneService) guildPrune(guildID string, days uint32, reason string) (uint32, error) {
	if s.session == nil || guildID == "" {
		return 0, fmt.Errorf("session or guildID is empty")
	}
	if days < 1 {
		return 0, fmt.Errorf("invalid prune days: %d", days)
	}

	endpoint := discordgo.EndpointGuildPrune(guildID)
	payload := map[string]any{
		"days":                days,
		"compute_prune_count": true,
	}
	body, err := s.session.RequestWithBucketID(
		"POST",
		endpoint,
		payload,
		endpoint,
		discordgo.WithAuditLogReason(reason),
	)
	if err != nil {
		return 0, fmt.Errorf("request guild prune: %w", err)
	}

	var resp pruneResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, fmt.Errorf("decode guild prune response: %w", err)
	}
	if resp.Pruned == nil {
		return 0, nil
	}
	return *resp.Pruned, nil
}

func (s *UserPruneService) didRunGuildPruneThisMonth(ctx context.Context, guildID string, now time.Time) bool {
	guildID = strings.TrimSpace(guildID)
	if s.store == nil || guildID == "" {
		return false
	}
	ts, ok, err := s.store.Metadata(ctx, userPruneLastRunKey(guildID))
	if err != nil {
		log.ApplicationLogger().Warn("User prune: failed to read last run metadata", "guildID", guildID, "err", err)
		return false
	}
	if !ok {
		return false
	}
	return sameYearMonth(ts.UTC(), now.UTC())
}

func (s *UserPruneService) markGuildPruneRun(ctx context.Context, guildID string, when time.Time) error {
	guildID = strings.TrimSpace(guildID)
	if s.store == nil || guildID == "" {
		return fmt.Errorf("store or guildID is empty")
	}
	if when.IsZero() {
		when = time.Now().UTC()
	}
	if err := s.store.SetMetadata(ctx, userPruneLastRunKey(guildID), when.UTC()); err != nil {
		return fmt.Errorf("set last run metadata: %w", err)
	}
	return nil
}

func userPruneLastRunKey(guildID string) string {
	return userPruneLastRunKeyPrefix + strings.TrimSpace(guildID)
}

func isPruneExecutionDay(now time.Time) bool {
	return now.UTC().Day() == pruneDayOfMonth
}

func sameYearMonth(a, b time.Time) bool {
	a = a.UTC()
	b = b.UTC()
	return a.Year() == b.Year() && a.Month() == b.Month()
}

func (s *UserPruneService) nextGuildCaseNumber(guildID string) (int64, bool) {
	if s.store == nil || strings.TrimSpace(guildID) == "" {
		return 0, false
	}
	n, err := s.store.NextModerationCaseNumber(guildID)
	if err != nil {
		log.ApplicationLogger().Warn("User prune: failed to allocate moderation case number", "guildID", guildID, "err", err)
		return 0, false
	}
	return n, true
}

func (s *UserPruneService) sendRunEmbed(guildID, botID string, estimated, pruned uint32) {
	if s.session == nil || s.configManager == nil || strings.TrimSpace(guildID) == "" {
		return
	}
	if botID == "" {
		botID = s.currentBotID()
	}
	emit := logging.CheckFeatureEnabled(s.configManager, logging.LogEventModerationCase, guildID)
	if !emit.Enabled || strings.TrimSpace(emit.ChannelID) == "" {
		return
	}
	channelID := emit.ChannelID

	casePart := "?"
	if caseNum, hasCase := s.nextGuildCaseNumber(guildID); hasCase && caseNum > 0 {
		casePart = fmt.Sprintf("%d", caseNum)
	}

	actorValue := "Unknown"
	if botID != "" {
		actorValue = fmt.Sprintf("<@%s> (`%s`)", botID, botID)
	}

	eventAt := time.Now()
	descriptionLines := []string{
		fmt.Sprintf("**Pruned:** %d", pruned),
		"**Window:** 30 days",
		fmt.Sprintf("**Responsible moderator:** %s", actorValue),
	}
	if estimated > 0 {
		descriptionLines = append(descriptionLines, fmt.Sprintf("**Estimated:** %d", estimated))
	}
	descriptionLines = append(descriptionLines, "**Reason:** Automatic Discord native prune (day 28 of each month).")
	descriptionLines = append(descriptionLines, fmt.Sprintf("ID: `%s` • <t:%d:F>", guildID, eventAt.Unix()))

	embed := &discordgo.MessageEmbed{
		Title:       "prune | case " + casePart,
		Description: strings.Join(descriptionLines, "\n"),
		Color:       theme.AutomodAction(),
		Timestamp:   eventAt.Format(time.RFC3339),
	}

	if _, err := s.session.ChannelMessageSendEmbed(channelID, embed); err != nil {
		log.ErrorLoggerRaw().Error("Failed to send user prune moderation log", "guildID", guildID, "channelID", channelID, "err", err)
	}
}

func truncateAuditReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return reason
	}
	const maxLen = 512
	if len(reason) <= maxLen {
		return reason
	}
	return reason[:maxLen]
}
