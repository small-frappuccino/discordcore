package app

import (
	"context"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/monitoring"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordgo"
)

func TestInitializeBotRuntimeSkipsCommandHandlerWhenCommandsDisabled(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}
	session.Token = "" // clear it so we don't trigger the commands-clear cleanup logic

	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})

	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: new(bool(false)),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-1",
					}},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	origNewCommandHandlerForBot := newCommandHandlerForBot
	origSetupCommandHandler := setupCommandHandler
	defer func() {
		newCommandHandlerForBot = origNewCommandHandlerForBot
		setupCommandHandler = origSetupCommandHandler
	}()

	var created bool
	var setupCommandsCalls int

	newCommandHandlerForBot = func(session *discordgo.Session, configManager *files.ConfigManager, botInstanceID string) *commands.CommandHandler {
		created = true
		return commands.NewCommandHandlerForBot(session, configManager, botInstanceID)
	}
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		setupCommandsCalls++
		return nil
	}

	runtime := &botRuntime{
		instanceID:   "main",
		capabilities: botRuntimeCapabilities{qotdRuntime: true},
		session:      session,
	}
	err = initializeBotRuntime(context.Background(), runtime, botRuntimeOptions{
		runtimeCount:  1,
		configManager: cfgMgr,
	})
	if err != nil {
		t.Fatalf("initialize bot runtime: %v", err)
	}

	if setupCommandsCalls != 0 {
		t.Fatalf("expected slash command setup to be skipped, got %d calls", setupCommandsCalls)
	}
	if created {
		t.Fatal("expected command handler creation to be skipped when commands are disabled")
	}
	if runtime.commandHandler != nil {
		t.Fatal("expected no command handler when commands are disabled")
	}
}

func TestOpenBotRuntime(t *testing.T) {
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origOpenBotDiscordSession := openBotDiscordSession
	t.Cleanup(func() {
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		openBotDiscordSession = origOpenBotDiscordSession
	})

	session, _ := discordgo.New("Bot fake")
	session.State.User = &discordgo.User{Username: "test", Discriminator: "1234"}

	newDiscordSessionWithIntents = func(token string, i discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error {
		return nil
	}

	rt, err := openBotRuntime(resolvedBotInstance{ID: "test", Token: "tok"}, botRuntimeCapabilities{intents: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rt == nil || rt.session != session {
		t.Errorf("expected initialized runtime")
	}
}

func TestInitializeBotRuntime_FullCapabilities(t *testing.T) {
	origNewCommandHandlerForBot := newCommandHandlerForBot
	origSetupCommandHandler := setupCommandHandler
	t.Cleanup(func() {
		newCommandHandlerForBot = origNewCommandHandlerForBot
		setupCommandHandler = origSetupCommandHandler
	})

	newCommandHandlerForBot = func(session *discordgo.Session, configManager *files.ConfigManager, botInstanceID string) *commands.CommandHandler {
		return commands.NewCommandHandlerForBot(session, configManager, botInstanceID)
	}
	setupCommandHandler = func(ch *commands.CommandHandler) error { return nil }

	session, _ := discordgo.New("Bot fake")
	cfgMgr := files.NewConfigManagerWithStore(nil)
	cfg := files.BotConfig{
		Guilds: []files.GuildConfig{{GuildID: "g1", BotInstanceTokens: map[string]files.EncryptedString{"test": "a"}}},
	}
	cfgMgr.ApplyConfig(&cfg)

	rt := &botRuntime{
		instanceID: "test",
		capabilities: botRuntimeCapabilities{
			monitoring:  false,
			automod:     true,
			userPrune:   false,
			qotdRuntime: true,
			admin:       true,
			hasCommands: true,
		},
		session: session,
	}

	err := initializeBotRuntime(context.Background(), rt, botRuntimeOptions{
		configManager:        cfgMgr,
		qotdLifecycleService: &mockQotdLifecycleService{},
		store:                &storage.Store{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rt.serviceManager == nil {
		t.Fatal("expected serviceManager to be initialized")
	}

	all := rt.serviceManager.GetAllServices()
	if len(all) < 3 {
		t.Errorf("expected at least 3 services, got %d", len(all))
	}

	shutdownBotRuntime(rt, context.Background())
}

type mockQotdLifecycleService struct{}

func (m *mockQotdLifecycleService) InitializeGuilds(ctx context.Context, session *discordgo.Session, config *files.ConfigManager) error {
	return nil
}
func (m *mockQotdLifecycleService) Start() {}
func (m *mockQotdLifecycleService) Stop()  {}
func (m *mockQotdLifecycleService) EnforcePoliciesNow(ctx context.Context, session *discordgo.Session, config *files.ConfigManager, guildID string) error {
	return nil
}
func (m *mockQotdLifecycleService) GetRunningPolicyGoroutines() int { return 0 }
func (m *mockQotdLifecycleService) StartThreadArchivePolicy(ctx context.Context, session *discordgo.Session, config *files.ConfigManager) {
}
func (m *mockQotdLifecycleService) NextScheduledPublishTime(guildID string, now time.Time) (time.Time, bool) {
	return time.Time{}, false
}
func (m *mockQotdLifecycleService) PublishScheduledIfDue(ctx context.Context, guildID string) (bool, error) {
	return false, nil
}
func (m *mockQotdLifecycleService) ReconcileGuild(ctx context.Context, guildID string) error {
	return nil
}
func (m *mockQotdLifecycleService) ScheduleDailyAutomatedArchiveForGuild(guildID string) {}
func (m *mockQotdLifecycleService) CancelDailyAutomatedArchiveForGuild(guildID string)   {}

func TestScheduleRuntimeWarmup(t *testing.T) {
	origIntelligent := intelligentWarmupFn
	origUnifiedCache := monitoringUnifiedCacheFn
	origSchedule := scheduleStartupMemberWarmupFn
	t.Cleanup(func() {
		intelligentWarmupFn = origIntelligent
		monitoringUnifiedCacheFn = origUnifiedCache
		scheduleStartupMemberWarmupFn = origSchedule
	})

	done := make(chan struct{})
	var count int

	intelligentWarmupFn = func(ctx context.Context, s *discordgo.Session, c *cache.UnifiedCache, store *storage.Store, config cache.WarmupConfig) error {
		count++
		if count == 2 {
			close(done)
		}
		return nil
	}
	monitoringUnifiedCacheFn = func(runtime *botRuntime) *cache.UnifiedCache {
		return cache.NewUnifiedCache(cache.CacheConfig{})
	}
	scheduleStartupMemberWarmupFn = func(ms *monitoring.MonitoringService, config cache.WarmupConfig) bool {
		return false
	}

	rt := &botRuntime{
		instanceID:        "test",
		capabilities:      botRuntimeCapabilities{warmup: true},
		session:           &discordgo.Session{},
		monitoringService: &monitoring.MonitoringService{},
	}

	scheduleRuntimeWarmup(context.Background(), rt, nil, nil)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Errorf("expected warmup to be called twice")
	}
}
