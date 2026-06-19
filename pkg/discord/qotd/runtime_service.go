package qotd

import (
	"context"
	"errors"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/clock"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/log"
	"github.com/small-frappuccino/discordcore/pkg/service"
	"github.com/small-frappuccino/discordgo"
)

const (
	// runtimePublishInterval bounds how long the loop is allowed to sleep
	// between publish-cycle wakeups. The loop normally sleeps until the
	// nearest scheduled publish moment reported by the lifecycle service,
	// which gives sub-second precision at the slot boundary; this cap
	// guarantees we still re-evaluate at least once per minute so that
	// configuration changes (new guilds, edited schedules, suppressions
	// applied/cleared) and clock anomalies are picked up promptly even
	// while sleeping. Lowering this value increases responsiveness at the
	// cost of more wakeups; raising it is safe but trades discovery
	// latency for cheaper idle behavior.
	runtimePublishInterval = time.Minute

	runtimeReconcileInterval = 15 * time.Minute
	runtimeOperationTimeout  = 45 * time.Second

	// runtimePublishMinSleep keeps the timer from busy-spinning when the
	// computed next-publish moment is already in the past (e.g. a slot was
	// missed during a stop-the-world pause). PublishScheduledIfDue is
	// idempotent, so we just wake quickly, let it process, and re-arm.
	runtimePublishMinSleep = time.Millisecond
)

// GuildLifecycleService is the per-guild QOTD lifecycle work that RuntimeService
// drives on its publish and reconcile timers. NextScheduledPublishTime is only a
// wake-up hint; PublishScheduledIfDue remains the authoritative due check.
type GuildLifecycleService interface {
	PublishScheduledIfDue(ctx context.Context, guildID string) (bool, error)
	ReconcileGuild(ctx context.Context, guildID string) error
	// NextScheduledPublishTime returns the next eligible scheduled publish
	// moment for a guild based on its current configuration. It is consulted
	// only as a wake-up hint: the authoritative "is this slot due" decision
	// still lives inside PublishScheduledIfDue. Implementations must return
	// ok=false when the guild has no schedule, no enabled deck, no channel,
	// or any other reason it would not autopublish.
	NextScheduledPublishTime(guildID string, now time.Time) (time.Time, bool)
}

// RuntimeService runs the background publish and reconcile loops for QOTD,
// invoking a GuildLifecycleService on timers. It is safe to Start after a prior
// Stop: Start reinitializes stopCh and stopOnce so the loops resume rather than
// exiting immediately.
type RuntimeService struct {
	session          *discordgo.Session
	configManager    *files.ConfigManager
	lifecycleService GuildLifecycleService
	botInstanceID    string
	now              func() time.Time
	publishInterval  time.Duration
	reconcileEvery   time.Duration

	stopOnce sync.Once
	stopCh   chan struct{}
	wg       sync.WaitGroup

	mu        sync.RWMutex
	running   bool
	startTime time.Time

	dependencies []string
}

// NewRuntimeService news runtime service.
func NewRuntimeService(session *discordgo.Session, configManager *files.ConfigManager, lifecycleService GuildLifecycleService) *RuntimeService {
	return NewRuntimeServiceForBot(session, configManager, lifecycleService, "")
}

// NewRuntimeServiceForBot news runtime service for bot.
func NewRuntimeServiceForBot(
	session *discordgo.Session,
	configManager *files.ConfigManager,
	lifecycleService GuildLifecycleService,
	botInstanceID string,
) *RuntimeService {
	return &RuntimeService{
		session:          session,
		configManager:    configManager,
		lifecycleService: lifecycleService,
		botInstanceID:    files.NormalizeBotInstanceID(botInstanceID),
		now: func() time.Time {
			return time.Now().UTC()
		},
		publishInterval: runtimePublishInterval,
		reconcileEvery:  runtimeReconcileInterval,
		stopCh:          make(chan struct{}),
	}
}

// SetDependencies allows the orchestrator to inject dynamic dependencies.
func (s *RuntimeService) SetDependencies(deps []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dependencies = append([]string(nil), deps...)
}

// SetClock injects a custom clock into the service.
func (s *RuntimeService) SetClock(c clock.Clock) {
	if c == nil {
		c = clock.RealClock{}
	}
	s.now = func() time.Time {
		return c.Now().UTC()
	}
}

// Name returns the service name.
func (s *RuntimeService) Name() string { return "qotd" }

// Type returns the service type.
func (s *RuntimeService) Type() service.ServiceType { return service.TypeMonitoring }

// Priority returns the service priority.
func (s *RuntimeService) Priority() service.ServicePriority { return service.PriorityNormal }

// Dependencies returns the service dependencies.
func (s *RuntimeService) Dependencies() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string(nil), s.dependencies...)
}

// HealthCheck returns the current health status.
func (s *RuntimeService) HealthCheck(ctx context.Context) service.HealthStatus {
	return service.HealthStatus{
		Healthy:   true,
		Message:   "QOTD Runtime Service is active",
		LastCheck: time.Now(),
	}
}

// Stats returns runtime statistics.
func (s *RuntimeService) Stats() service.ServiceStats {
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
func (s *RuntimeService) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}
	select {
	case <-s.stopCh:
		s.stopCh = make(chan struct{})
		s.stopOnce = sync.Once{}
	default:
	}
	s.running = true
	s.startTime = time.Now()
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()
	return nil
}

// Stop stops.
func (s *RuntimeService) Stop(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stopCh) })

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
func (s *RuntimeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func calculateJitter(base time.Duration) time.Duration {
	jitterFraction := 0.1 + rand.Float64()*0.1
	jitterAmount := time.Duration(float64(base) * jitterFraction)
	return base + jitterAmount
}

func (s *RuntimeService) loop() {
	defer s.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.ApplicationLogger().Error("QOTD runtime loop panic caught", "panic", r, "stack", string(debug.Stack()))
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

	// Startup catch-up: handle any slot that became due while the bot was
	// down, and reconcile state once before entering the timer loop.
	s.runPublishCycle(s.clock())
	s.runReconcileCycle(s.clock())

	for {
		// Compute the wake-up delay each iteration so newly-added guilds,
		// edited schedules, applied/cleared suppressions, and just-published
		// slots (whose next moment is now tomorrow) are all reflected on the
		// very next sleep. The cap (publishInterval) is the worst-case
		// discovery latency for any of those changes.
		publishTimer := time.NewTimer(calculateJitter(s.nextPublishDelay(s.clock())))
		reconcileTimer := time.NewTimer(calculateJitter(s.reconcileEvery))

		select {
		case <-publishTimer.C:
			reconcileTimer.Stop()
			s.runPublishCycle(s.clock())
		case <-reconcileTimer.C:
			// Reconcile interrupts an in-flight publish sleep. Stopping
			// the timer is best-effort; if it already fired we ignore the
			// pending tick rather than running the publish cycle eagerly,
			// because reconcile already implies a fresh evaluation pass
			// that the next loop iteration will re-arm against.
			publishTimer.Stop()
			s.runReconcileCycle(s.clock())
		case <-ctx.Done():
			publishTimer.Stop()
			reconcileTimer.Stop()
			return
		}
	}
}

// nextPublishDelay computes how long the loop should sleep before the next
// publish-cycle wakeup. It returns the time-to-next-slot across all
// configured guilds, clamped into [runtimePublishMinSleep, publishInterval].
// When no guild has a schedulable next moment the delay is the full cap,
// preserving the legacy fixed-interval cadence as a fallback.
func (s *RuntimeService) nextPublishDelay(now time.Time) time.Duration {
	maxSleep := s.publishInterval
	if maxSleep <= 0 {
		maxSleep = runtimePublishInterval
	}
	next, ok := s.computeNextPublish(now)
	if !ok {
		return maxSleep
	}
	delay := next.Sub(now)
	if delay <= 0 {
		return runtimePublishMinSleep
	}
	if delay > maxSleep {
		return maxSleep
	}
	return delay
}

// computeNextPublish returns the earliest scheduled publish moment across all
// configured-and-enabled guilds on this runtime. Guilds that the lifecycle
// service reports as ineligible (no schedule, deck off, etc.) are skipped.
func (s *RuntimeService) computeNextPublish(now time.Time) (time.Time, bool) {
	if s == nil || s.lifecycleService == nil {
		return time.Time{}, false
	}
	var earliest time.Time
	for _, guildID := range s.configuredGuildIDs(true) {
		candidate, ok := s.lifecycleService.NextScheduledPublishTime(guildID, now)
		if !ok || candidate.IsZero() {
			continue
		}
		if earliest.IsZero() || candidate.Before(earliest) {
			earliest = candidate
		}
	}
	if earliest.IsZero() {
		return time.Time{}, false
	}
	return earliest, true
}

func (s *RuntimeService) runPublishCycle(now time.Time) {
	for _, guildID := range s.configuredGuildIDs(true) {
		select {
		case <-s.stopCh:
			return
		default:
		}
		ctx, cancel := s.operationContext()
		published, err := s.lifecycleService.PublishScheduledIfDue(ctx, guildID)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) && s.stopping() {
				continue
			}
			log.ApplicationLogger().Warn(
				"QOTD scheduled publish failed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
				"err", err,
			)
			continue
		}
		if published {
			log.ApplicationLogger().Info(
				"QOTD scheduled publish completed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
			)
		}
	}
}

func (s *RuntimeService) runReconcileCycle(now time.Time) {
	for _, guildID := range s.configuredGuildIDs(false) {
		select {
		case <-s.stopCh:
			return
		default:
		}
		ctx, cancel := s.operationContext()
		err := s.lifecycleService.ReconcileGuild(ctx, guildID)
		cancel()
		if err != nil {
			if errors.Is(err, context.Canceled) && s.stopping() {
				continue
			}
			log.ApplicationLogger().Warn(
				"QOTD reconcile failed",
				"guildID", guildID,
				"botInstanceID", s.botInstanceID,
				"at", now.UTC(),
				"err", err,
			)
		}
	}
}

func (s *RuntimeService) configuredGuildIDs(requireEnabled bool) []string {
	if s == nil || s.configManager == nil || s.lifecycleService == nil || s.session == nil {
		return nil
	}
	cfg := s.configManager.Config()
	if cfg == nil {
		return nil
	}

	guilds := cfg.GuildsForBotInstance(s.botInstanceID)
	if len(guilds) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(guilds))
	ids := make([]string, 0, len(guilds))
	for _, guild := range guilds {
		guildID := strings.TrimSpace(guild.GuildID)
		if guildID == "" {
			continue
		}

		route, _ := guild.ResolveFeatureBotInstanceID("qotd")
		if route != s.botInstanceID {
			continue
		}
		if requireEnabled {
			deck, ok := guild.QOTD.ActiveDeck()
			if !ok || !deck.Enabled || strings.TrimSpace(deck.ChannelID) == "" {
				continue
			}
		} else if guild.QOTD.IsZero() {
			continue
		}
		if _, ok := seen[guildID]; ok {
			continue
		}
		seen[guildID] = struct{}{}
		ids = append(ids, guildID)
	}
	return ids
}

func (s *RuntimeService) clock() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *RuntimeService) operationContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), runtimeOperationTimeout)
	if s == nil {
		return ctx, cancel
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		select {
		case <-s.stopCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func (s *RuntimeService) stopping() bool {
	if s == nil {
		return false
	}
	select {
	case <-s.stopCh:
		return true
	default:
		return false
	}
}
