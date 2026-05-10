package qotd

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type fakeGuildLifecycleService struct {
	mu             sync.Mutex
	publishCalls   []string
	reconcileCalls []string
	publishCh      chan string
	reconcileCh    chan string
	// nextPublish overrides NextScheduledPublishTime per guild. When a guild
	// is absent we report ok=false, which makes the runtime fall back to the
	// publishInterval cap — the legacy fixed-interval cadence preserved for
	// tests that don't care about scheduling precision.
	nextPublish map[string]time.Time
}

func (f *fakeGuildLifecycleService) PublishScheduledIfDue(_ context.Context, guildID string, _ *discordgo.Session) (bool, error) {
	f.mu.Lock()
	f.publishCalls = append(f.publishCalls, guildID)
	f.mu.Unlock()
	if f.publishCh != nil {
		select {
		case f.publishCh <- guildID:
		default:
		}
	}
	return false, nil
}

func (f *fakeGuildLifecycleService) ReconcileGuild(_ context.Context, guildID string, _ *discordgo.Session) error {
	f.mu.Lock()
	f.reconcileCalls = append(f.reconcileCalls, guildID)
	f.mu.Unlock()
	if f.reconcileCh != nil {
		select {
		case f.reconcileCh <- guildID:
		default:
		}
	}
	return nil
}

func (f *fakeGuildLifecycleService) NextScheduledPublishTime(guildID string, _ time.Time) (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nextPublish == nil {
		return time.Time{}, false
	}
	t, ok := f.nextPublish[guildID]
	if !ok || t.IsZero() {
		return time.Time{}, false
	}
	return t, true
}

func (f *fakeGuildLifecycleService) snapshotPublishCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.publishCalls))
	copy(out, f.publishCalls)
	return out
}

func (f *fakeGuildLifecycleService) snapshotReconcileCalls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.reconcileCalls))
	copy(out, f.reconcileCalls)
	return out
}

type blockingContextLifecycleService struct {
	started chan struct{}
	done    chan struct{}
	errCh   chan error
	once    sync.Once
}

func (f *blockingContextLifecycleService) PublishScheduledIfDue(ctx context.Context, _ string, _ *discordgo.Session) (bool, error) {
	f.once.Do(func() {
		if f.started != nil {
			close(f.started)
		}
	})
	<-ctx.Done()
	if f.errCh != nil {
		f.errCh <- ctx.Err()
	}
	if f.done != nil {
		close(f.done)
	}
	return false, ctx.Err()
}

func (f *blockingContextLifecycleService) ReconcileGuild(context.Context, string, *discordgo.Session) error {
	return nil
}

func (f *blockingContextLifecycleService) NextScheduledPublishTime(string, time.Time) (time.Time, bool) {
	return time.Time{}, false
}

func TestRuntimeServiceLoopRunsPublishCycleOnStartAndInterval(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-enabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-disabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{
		publishCh:   make(chan string, 8),
		reconcileCh: make(chan string, 8),
	}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = 10 * time.Millisecond
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.Start()
	t.Cleanup(service.Stop)
	if !service.IsRunning() {
		t.Fatal("expected runtime service to report running after Start()")
	}

	for idx := 0; idx < 2; idx++ {
		select {
		case guildID := <-fake.publishCh:
			if guildID != "g-enabled" {
				t.Fatalf("unexpected publish cycle guild %q", guildID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for publish cycle %d", idx+1)
		}
	}

	service.Stop()

	if service.IsRunning() {
		t.Fatal("expected runtime service to stop running after Stop()")
	}
	if len(fake.publishCalls) < 2 {
		t.Fatalf("expected startup and interval publish cycles, got %v", fake.publishCalls)
	}
	for _, guildID := range fake.publishCalls {
		if guildID != "g-enabled" {
			t.Fatalf("expected runtime loop to publish only for enabled guilds, got %v", fake.publishCalls)
		}
	}
	if len(fake.reconcileCalls) == 0 || fake.reconcileCalls[0] != "g-enabled" {
		t.Fatalf("expected startup reconcile cycle to include enabled guild first, got %v", fake.reconcileCalls)
	}
	if len(fake.reconcileCalls) < 2 || fake.reconcileCalls[1] != "g-disabled" {
		t.Fatalf("expected startup reconcile cycle to include configured disabled guild too, got %v", fake.reconcileCalls)
	}
}

func TestRuntimeServiceCyclesUseScopedGuilds(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-enabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-enabled-missing-channel",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:      files.LegacyQOTDDefaultDeckID,
					Name:    files.LegacyQOTDDefaultDeckName,
					Enabled: true,
				}},
			},
		},
		{
			GuildID:       "g-configured-disabled",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
		{
			GuildID:       "g-other-runtime",
			BotInstanceID: "other",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-other",
				}},
			},
		},
		{
			GuildID:       "g-empty",
			BotInstanceID: "main",
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.runPublishCycle(service.clock())
	service.runReconcileCycle(service.clock())

	if len(fake.publishCalls) != 1 || fake.publishCalls[0] != "g-enabled" {
		t.Fatalf("expected publish cycle to include only enabled guilds with channel targets on the runtime, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 3 {
		t.Fatalf("expected reconcile cycle to include configured guilds on the runtime, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-enabled" || fake.reconcileCalls[1] != "g-enabled-missing-channel" || fake.reconcileCalls[2] != "g-configured-disabled" {
		t.Fatalf("unexpected reconcile target order: %v", fake.reconcileCalls)
	}
}

func TestRuntimeServiceCyclesUseQOTDDomainScopedGuilds(t *testing.T) {
	t.Parallel()

	configManager := files.NewMemoryConfigManager()
	for _, guild := range []files.GuildConfig{
		{
			GuildID:       "g-qotd-enabled",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-enabled",
				}},
			},
		},
		{
			GuildID:       "g-qotd-configured-disabled",
			BotInstanceID: "main",
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					ChannelID: "question-disabled",
				}},
			},
		},
		{
			GuildID:       "g-default-main",
			BotInstanceID: "main",
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-alice",
				}},
			},
		},
	} {
		if err := configManager.AddGuildConfig(guild); err != nil {
			t.Fatalf("AddGuildConfig(%s) failed: %v", guild.GuildID, err)
		}
	}

	fake := &fakeGuildLifecycleService{}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "companion", "main")
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.runPublishCycle(service.clock())
	service.runReconcileCycle(service.clock())

	if len(fake.publishCalls) != 1 || fake.publishCalls[0] != "g-qotd-enabled" {
		t.Fatalf("expected publish cycle to include only enabled qotd-domain guilds for companion, got %v", fake.publishCalls)
	}
	if len(fake.reconcileCalls) != 2 {
		t.Fatalf("expected reconcile cycle to include configured qotd-domain guilds for companion, got %v", fake.reconcileCalls)
	}
	if fake.reconcileCalls[0] != "g-qotd-enabled" || fake.reconcileCalls[1] != "g-qotd-configured-disabled" {
		t.Fatalf("unexpected reconcile target order: %v", fake.reconcileCalls)
	}
}

func TestRuntimeServiceRestartResumesIntervalCycles(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
		GuildID:       "g-enabled",
		BotInstanceID: "main",
		QOTD: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "question-enabled",
			}},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	fake := &fakeGuildLifecycleService{publishCh: make(chan string, 16)}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = time.Hour
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.Start()
	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected first start publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first start publish cycle")
	}
	service.Stop()

	service.publishInterval = 10 * time.Millisecond
	service.Start()
	t.Cleanup(service.Stop)

	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected restart startup publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restart startup publish cycle")
	}

	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected restart interval publish guild %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for restart interval publish cycle")
	}
}

// TestRuntimeServiceMultipleRestartsResumeIntervalCycles guards the
// stopCh/stopOnce reset path under more than one restart. The 2-cycle test
// above only proves the FIRST restart works; if a regression in Start()'s
// reset logic only manifests on the second-or-later restart (e.g. failing to
// reseat stopOnce after a previous reset), this test catches it.
func TestRuntimeServiceMultipleRestartsResumeIntervalCycles(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
		GuildID:       "g-enabled",
		BotInstanceID: "main",
		QOTD: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "question-enabled",
			}},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	fake := &fakeGuildLifecycleService{publishCh: make(chan string, 32)}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = time.Hour
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	expectStartupPublish := func(t *testing.T, label string) {
		t.Helper()
		select {
		case guildID := <-fake.publishCh:
			if guildID != "g-enabled" {
				t.Fatalf("%s: unexpected publish guild %q", label, guildID)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("%s: timed out waiting for startup publish cycle", label)
		}
	}

	for cycle := 1; cycle <= 3; cycle++ {
		service.Start()
		if !service.IsRunning() {
			t.Fatalf("cycle %d: expected runtime service to report running after Start()", cycle)
		}
		expectStartupPublish(t, fmt.Sprintf("cycle %d", cycle))
		service.Stop()
		if service.IsRunning() {
			t.Fatalf("cycle %d: expected runtime service to stop running after Stop()", cycle)
		}
	}

	// After the third Stop, draining any leftover publishes triggered by
	// the startup cycle should yield none beyond what we already consumed.
	// More importantly, a final restart must still spin up a fresh interval
	// loop — proving the reset path remains effective without bound.
	service.publishInterval = 10 * time.Millisecond
	service.Start()
	t.Cleanup(service.Stop)
	expectStartupPublish(t, "final restart startup")
	select {
	case guildID := <-fake.publishCh:
		if guildID != "g-enabled" {
			t.Fatalf("unexpected interval publish guild after multiple restarts: %q", guildID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for interval publish after multiple restarts")
	}
}

func TestRuntimeServiceStopCancelsInflightPublish(t *testing.T) {
	configManager := files.NewMemoryConfigManager()
	if err := configManager.AddGuildConfig(files.GuildConfig{
		GuildID:       "g-enabled",
		BotInstanceID: "main",
		QOTD: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "question-enabled",
			}},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig() failed: %v", err)
	}

	fake := &blockingContextLifecycleService{
		started: make(chan struct{}),
		done:    make(chan struct{}),
		errCh:   make(chan error, 1),
	}
	service := NewRuntimeServiceForBot(&discordgo.Session{}, configManager, fake, "main", "main")
	service.publishInterval = time.Hour
	service.reconcileEvery = time.Hour

	service.Start()
	select {
	case <-fake.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for publish operation to start")
	}

	stopped := make(chan struct{})
	go func() {
		service.Stop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Stop() to cancel in-flight publish")
	}

	select {
	case <-fake.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for publish operation to observe cancellation")
	}

	select {
	case err := <-fake.errCh:
		if err == nil {
			t.Fatal("expected in-flight publish to receive context cancellation")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for cancellation error")
	}
}

// nextPublishDelay is a unit-level guard for the timer-arm math: the production
// loop dynamically sleeps based on this calculation, so a regression here
// could either cause a busy-loop (returning 0) or miss slots (returning a
// duration past the configured cap). Each branch is explicitly named.
func TestNextPublishDelayClampsToConfiguredBounds(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	const cap = 30 * time.Second

	cases := []struct {
		name     string
		next     time.Time
		hasNext  bool
		expected time.Duration
	}{
		{
			name:     "no schedule falls back to cap",
			hasNext:  false,
			expected: cap,
		},
		{
			name:     "near-future slot returns exact remainder",
			next:     now.Add(2 * time.Second),
			hasNext:  true,
			expected: 2 * time.Second,
		},
		{
			name:     "far-future slot is clamped to cap",
			next:     now.Add(6 * time.Hour),
			hasNext:  true,
			expected: cap,
		},
		{
			name:     "past-due slot returns the minimum sleep",
			next:     now.Add(-5 * time.Minute),
			hasNext:  true,
			expected: runtimePublishMinSleep,
		},
		{
			name:     "exact-now slot returns the minimum sleep",
			next:     now,
			hasNext:  true,
			expected: runtimePublishMinSleep,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cm := files.NewMemoryConfigManager()
			if err := cm.AddGuildConfig(files.GuildConfig{
				GuildID:       "g",
				BotInstanceID: "main",
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "c",
					}},
				},
			}); err != nil {
				t.Fatalf("AddGuildConfig: %v", err)
			}

			fake := &fakeGuildLifecycleService{}
			if tc.hasNext {
				fake.nextPublish = map[string]time.Time{"g": tc.next}
			}

			service := NewRuntimeServiceForBot(&discordgo.Session{}, cm, fake, "main", "main")
			service.publishInterval = cap
			service.now = func() time.Time { return now }

			got := service.nextPublishDelay(now)
			if got != tc.expected {
				t.Fatalf("nextPublishDelay = %v, want %v", got, tc.expected)
			}
		})
	}
}

// Confirms that when a guild reports a near-future scheduled publish moment,
// the loop wakes up at that moment instead of polling at the
// publishInterval cap. This is the core precision claim of the timer-driven
// loop and is the regression most likely to surface in production (publishes
// firing late by up to one cap interval).
func TestRuntimeServiceLoopWakesAtScheduledMoment(t *testing.T) {
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID:       "g-enabled",
		BotInstanceID: "main",
		QOTD: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "c",
			}},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	startWall := time.Date(2026, 4, 3, 12, 0, 0, 0, time.UTC)
	// Slot is well below the configured cap so the precision path is the
	// one being exercised. If the loop ever falls back to the cap (a
	// regression), the test would still pass via the cap branch — to
	// catch that case, the cap is set far higher than the slot delay.
	scheduled := startWall.Add(40 * time.Millisecond)

	fake := &fakeGuildLifecycleService{
		publishCh:   make(chan string, 8),
		nextPublish: map[string]time.Time{"g-enabled": scheduled},
	}

	service := NewRuntimeServiceForBot(&discordgo.Session{}, cm, fake, "main", "main")
	service.publishInterval = 5 * time.Second
	service.reconcileEvery = time.Hour

	// Drive the fake clock from real wall time minus the offset so the
	// computed delay maps to real elapsed wall time the timer will observe.
	clockOffset := time.Since(startWall)
	service.now = func() time.Time { return time.Now().UTC().Add(-clockOffset) }

	service.Start()
	t.Cleanup(service.Stop)

	// Drain the startup publish that runs before the timer loop.
	select {
	case <-fake.publishCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for startup publish")
	}

	// Second publish should fire close to the scheduled moment, not after
	// the 5-second cap.
	select {
	case <-fake.publishCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for scheduled publish wakeup")
	}

	// Sanity check: at least two publish calls observed (startup + slot).
	if got := len(fake.snapshotPublishCalls()); got < 2 {
		t.Fatalf("expected at least 2 publish calls, got %d", got)
	}
}

// When no guild reports a next publish moment (e.g. nothing configured with a
// schedule), the loop must keep firing at the publishInterval cap so that
// configuration changes are still discovered. Regression guard for a future
// "if no schedule, sleep forever" mistake.
func TestRuntimeServiceLoopFallsBackToCapWithoutSchedule(t *testing.T) {
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{
		GuildID:       "g-enabled",
		BotInstanceID: "main",
		QOTD: files.QOTDConfig{
			ActiveDeckID: files.LegacyQOTDDefaultDeckID,
			Decks: []files.QOTDDeckConfig{{
				ID:        files.LegacyQOTDDefaultDeckID,
				Name:      files.LegacyQOTDDefaultDeckName,
				Enabled:   true,
				ChannelID: "c",
			}},
		},
	}); err != nil {
		t.Fatalf("AddGuildConfig: %v", err)
	}

	fake := &fakeGuildLifecycleService{
		publishCh: make(chan string, 8),
		// nextPublish nil -> NextScheduledPublishTime returns false, forcing
		// the cap-based fallback.
	}

	service := NewRuntimeServiceForBot(&discordgo.Session{}, cm, fake, "main", "main")
	service.publishInterval = 10 * time.Millisecond
	service.reconcileEvery = time.Hour
	service.now = func() time.Time {
		return time.Date(2026, 4, 3, 13, 0, 0, 0, time.UTC)
	}

	service.Start()
	t.Cleanup(service.Stop)

	for idx := 0; idx < 3; idx++ {
		select {
		case <-fake.publishCh:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for cap-fallback publish %d", idx)
		}
	}
}
