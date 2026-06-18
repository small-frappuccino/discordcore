package app

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/persistence"
	"github.com/small-frappuccino/discordgo"
	"log/slog"
)

func TestRun_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := Run("testapp")
	if err == nil {
		t.Fatal("expected Run to fail without DISCORDCORE_DATABASE_URL")
	}
}

func TestRunWithOptions_MissingDatabaseURL(t *testing.T) {
	os.Unsetenv("DISCORDCORE_DATABASE_URL")
	err := RunWithOptions("testapp", RunOptions{})
	if err == nil {
		t.Fatal("expected RunWithOptions to fail without DISCORDCORE_DATABASE_URL")
	}
}

func TestSetupStorage(t *testing.T) {
	dbb := resolvedDatabaseBootstrap{}
	_, _, err := setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bad config")
	}

	dbb.Config.Driver = "postgres"
	dbb.Config.DatabaseURL = "postgres://username:password@127.0.0.1:5433/bogus?sslmode=disable"
	_, _, err = setupStorage(dbb)
	if err == nil {
		t.Fatal("expected setupStorage to fail with bogus URL")
	}
}

func TestRunner_ShutdownStartupServices(t *testing.T) {
	shutdownStartupServices(nil, nil, "ok")
}

func TestRunner_RollbackStoreClose(t *testing.T) {
	rollbackStoreClose(true, nil)
}

func TestRunner_ResolveRuntimeCapabilities(t *testing.T) {
	cfg := &files.BotConfig{}

	instances := []resolvedBotInstance{{ID: "bot1"}}
	caps := resolveRuntimeCapabilities(cfg, instances, RunProfileDiscordMain)
	if caps["bot1"].qotdRuntime {
		t.Fatal("expected qotdRuntime to be false by default")
	}

	cfg.Guilds = []files.GuildConfig{
		{
			BotInstanceTokens: map[string]files.EncryptedString{
				"bot1": "token",
			},
			FeatureRouting: map[string]string{
				"qotd": "bot1",
			},
			QOTD: files.QOTDConfig{
				Decks: []files.QOTDDeckConfig{
					{
						ID:      "deck1",
						Enabled: true,
					},
				},
				ActiveDeckID: "123",
			},
		},
	}
	caps = resolveRuntimeCapabilities(cfg, instances, RunProfileDiscordMain)
	if !caps["bot1"].qotdRuntime {
		t.Fatalf("expected qotdRuntime to be true: %+v", caps["bot1"])
	}
}

func TestRunner_ApplyConfiguredTheme(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	applyConfiguredTheme(cm)
}

func TestRunner_ScheduleDBCleanup(t *testing.T) {
	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	scheduleDBCleanup(nil, cm)
}

func TestFormatStartupMessage(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		appName     string
		appVersion  string
		coreVersion string
		want        string
	}{
		{
			name:        "no app version includes discordcore",
			appName:     "discordmain",
			appVersion:  "",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain (discordcore v0.146.0)...",
		},
		{
			name:        "different versions include both",
			appName:     "discordmain",
			appVersion:  "v0.114.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain v0.114.0 (discordcore v0.146.0)...",
		},
		{
			name:        "same versions omit discordcore suffix",
			appName:     "discordmain",
			appVersion:  "v0.146.0",
			coreVersion: "v0.146.0",
			want:        "🚀 Starting discordmain v0.146.0...",
		},
		{
			name:        "trims spaces",
			appName:     " discordmain ",
			appVersion:  " v0.146.0 ",
			coreVersion: " v0.146.0 ",
			want:        "🚀 Starting discordmain v0.146.0...",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := formatStartupMessage(tc.appName, tc.appVersion, tc.coreVersion)
			if got != tc.want {
				t.Fatalf("formatStartupMessage() mismatch\nwant: %q\ngot:  %q", tc.want, got)
			}
		})
	}
}

// Mocks openRunnerConfigStore and setRunnerDatabaseBootstrapEnv which were present in runner_run_test.go
func openRunnerConfigStore(t *testing.T) files.DatabaseRuntimeConfig {
	return files.DatabaseRuntimeConfig{
		Driver:      "postgres",
		DatabaseURL: "postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable",
	}
}

func setRunnerDatabaseBootstrapEnv(t *testing.T, dbCfg files.DatabaseRuntimeConfig) {
	t.Setenv(databaseDriverEnv, dbCfg.Driver)
	t.Setenv(databaseURLEnv, dbCfg.DatabaseURL)
}

func seedRunnerConfig(t *testing.T, dbCfg files.DatabaseRuntimeConfig, cfg files.BotConfig) {
	dbc := persistence.Config{
		Driver:      dbCfg.Driver,
		DatabaseURL: dbCfg.DatabaseURL,
	}
	db, err := persistence.Open(context.Background(), dbc)
	if err != nil {
		t.Fatalf("failed to open database for seeding: %v", err)
	}
	defer db.Close()
	store := files.NewPostgresConfigStore(db, files.DefaultPostgresConfigStoreKey, slog.Default())
	if err := store.Save(&cfg); err != nil {
		t.Fatalf("failed to save test config to postgres: %v", err)
	}
}

func TestRun_CascadingRollbackFailures(t *testing.T) {
	const appName = "discordmain-cascading-rollback-test"

	appDataDir := t.TempDir()
	t.Setenv("APPDATA", appDataDir)

	dbCfg := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(bool(false)),
				Automod:    new(bool(false)),
				Commands:   new(bool(true)),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: new(bool(false)),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"generic": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, dbCfg, cfg)

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "testuser",
		Discriminator: "0001",
		Bot:           true,
	}
	session.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origCloseStore := closeStore
	origCloseDiscordSession := closeDiscordSession

	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		shutdownDelay = origShutdownDelay
		setupCommandHandler = origSetupCommandHandler
		closeStore = origCloseStore
		closeDiscordSession = origCloseDiscordSession
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	origOpenBotDiscordSession := openBotDiscordSession
	t.Cleanup(func() {
		openBotDiscordSession = origOpenBotDiscordSession
	})
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error { return nil }
	shutdownDelay = func(time.Duration) {}

	sabotageErr := errors.New("store close failure")
	storeCloseErr := sabotageErr
	discordCloseErr := errors.New("discord close failure")

	closeStore = func(c interface{ Close() error }) error {
		return storeCloseErr
	}
	closeDiscordSession = func(c interface{ Close() error }) error {
		return discordCloseErr
	}

	shutdownCh := make(chan struct{})
	testShutdownCh = shutdownCh
	t.Cleanup(func() { testShutdownCh = nil })

	go func() {
		time.Sleep(2 * time.Second)
		close(shutdownCh)
	}()

	err = Run(appName)

	if err == nil {
		t.Fatalf("expected Run to return cascading errors on shutdown")
	}

	errStr := err.Error()
	if !strings.Contains(errStr, sabotageErr.Error()) {
		t.Errorf("expected final error to contain the original boot failure %q, got: %v", sabotageErr.Error(), err)
	}
	if !strings.Contains(errStr, storeCloseErr.Error()) {
		t.Errorf("expected final error to aggregate store close failure %q, got: %v", storeCloseErr.Error(), err)
	}
	if !strings.Contains(errStr, discordCloseErr.Error()) {
		t.Errorf("expected final error to aggregate discord close failure %q, got: %v", discordCloseErr.Error(), err)
	}
}

func TestRun_ResourceCleanupOnBootFailure(t *testing.T) {
	const appName = "discordmain-resource-cleanup-test"

	appDataDir := t.TempDir()
	t.Setenv("APPDATA", appDataDir)

	dbCfg := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(bool(false)),
				Automod:    new(bool(false)),
				Commands:   new(bool(true)),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: new(bool(false)),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"generic": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, dbCfg, cfg)

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "testuser",
		Discriminator: "0001",
		Bot:           true,
	}
	session.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origCloseStore := closeStore
	origCloseDiscordSession := closeDiscordSession

	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		shutdownDelay = origShutdownDelay
		setupCommandHandler = origSetupCommandHandler
		closeStore = origCloseStore
		closeDiscordSession = origCloseDiscordSession
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	origOpenBotDiscordSession := openBotDiscordSession
	t.Cleanup(func() {
		openBotDiscordSession = origOpenBotDiscordSession
	})
	openBotDiscordSession = func(ctx context.Context, s *discordgo.Session) error { return nil }
	shutdownDelay = func(time.Duration) {}

	var storeCloseCalls int32
	var discordCloseCalls int32
	closeStore = func(c interface{ Close() error }) error {
		atomic.AddInt32(&storeCloseCalls, 1)
		return nil
	}
	closeDiscordSession = func(c interface{ Close() error }) error {
		atomic.AddInt32(&discordCloseCalls, 1)
		return nil
	}

	shutdownCh := make(chan struct{})
	testShutdownCh = shutdownCh
	t.Cleanup(func() { testShutdownCh = nil })

	go func() {
		time.Sleep(2 * time.Second)
		close(shutdownCh)
	}()

	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	err = Run(appName)

	if err != nil {
		t.Fatalf("expected clean shutdown, got: %v", err)
	}

	if got := atomic.LoadInt32(&storeCloseCalls); got != 1 {
		t.Errorf("expected 1 store close call on rollback, got %d", got)
	}

	if got := atomic.LoadInt32(&discordCloseCalls); got != 1 {
		t.Errorf("expected 1 discord session close call on rollback, got %d", got)
	}

	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	if goroutinesAfter > goroutinesBefore+2 {
		t.Errorf("goroutine leak detected: before %d, after %d", goroutinesBefore, goroutinesAfter)
	}
}
