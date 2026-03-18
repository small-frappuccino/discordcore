package app

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

func runtimeDatabaseConfigForRunnerTests(t *testing.T) files.DatabaseRuntimeConfig {
	t.Helper()

	dsn, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}

	return files.DatabaseRuntimeConfig{
		Driver:        "postgres",
		DatabaseURL:   dsn,
		MaxOpenConns:  5,
		MaxIdleConns:  5,
		PingTimeoutMS: 5000,
	}
}

func setRunnerDatabaseBootstrapEnv(t *testing.T, cfg files.DatabaseRuntimeConfig) {
	t.Helper()

	t.Setenv(databaseDriverEnv, cfg.Driver)
	t.Setenv(databaseURLEnv, cfg.DatabaseURL)
	t.Setenv(databaseMaxOpenConnsEnv, strconv.Itoa(cfg.MaxOpenConns))
	t.Setenv(databaseMaxIdleConnsEnv, strconv.Itoa(cfg.MaxIdleConns))
	t.Setenv(databaseConnMaxLifetimeSecsEnv, strconv.Itoa(cfg.ConnMaxLifetimeSecs))
	t.Setenv(databaseConnMaxIdleTimeSecsEnv, strconv.Itoa(cfg.ConnMaxIdleTimeSecs))
	t.Setenv(databasePingTimeoutMSEnv, strconv.Itoa(cfg.PingTimeoutMS))
}

func writeRunnerLegacyConfig(t *testing.T, appName string, cfg files.BotConfig) string {
	t.Helper()

	util.SetAppName(appName)
	settingsPath := util.GetSettingsFilePath()
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create settings directory: %v", err)
	}
	if err := files.SaveSettingsFileWithPath(settingsPath, &cfg); err != nil {
		t.Fatalf("write settings config: %v", err)
	}
	return settingsPath
}

func TestRun_GracefulShutdownInvokesCommandHandlerShutdown(t *testing.T) {
	const (
		appName  = "alicebot-run-test"
		tokenEnv = "ALICE_TEST_TOKEN"
	)

	appDataDir, err := os.MkdirTemp("", "alicebot-run-test-*")
	if err != nil {
		t.Fatalf("create APPDATA temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataDir)
	})
	t.Setenv("APPDATA", appDataDir)
	t.Setenv(tokenEnv, "test-token")

	boolPtr := func(v bool) *bool { return &v }
	dbCfg := runtimeDatabaseConfigForRunnerTests(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    boolPtr(false),
				Automod:       boolPtr(false),
				Commands:      boolPtr(true),
				AdminCommands: boolPtr(false),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: boolPtr(false),
			},
		},
		Guilds: []files.GuildConfig{},
	}
	settingsPath := writeRunnerLegacyConfig(t, appName, cfg)

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "alice-test",
		Discriminator: "0001",
		Bot:           true,
	}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		waitForInterrupt = origWaitForInterrupt
		shutdownDelay = origShutdownDelay
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	waitForInterrupt = func() {}
	shutdownDelay = func(time.Duration) {}

	var setupCalls int32
	var shutdownCalls int32
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		atomic.AddInt32(&setupCalls, 1)
		return nil
	}
	shutdownCommandHandler = func(ch *commands.CommandHandler) error {
		atomic.AddInt32(&shutdownCalls, 1)
		return nil
	}

	if err := Run(appName, tokenEnv); err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if got := atomic.LoadInt32(&setupCalls); got != 1 {
		t.Fatalf("expected one setup command call, got %d", got)
	}
	if got := atomic.LoadInt32(&shutdownCalls); got != 1 {
		t.Fatalf("expected one shutdown command call, got %d", got)
	}
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected migrated settings file to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Dir(settingsPath)); !os.IsNotExist(err) {
		t.Fatalf("expected migrated settings directory to be removed, stat err=%v", err)
	}
}

func TestRun_ShutdownAggregatesStoreAndSessionCloseErrors(t *testing.T) {
	const (
		appName  = "alicebot-run-shutdown-error-test"
		tokenEnv = "ALICE_TEST_TOKEN"
	)

	appDataDir, err := os.MkdirTemp("", "alicebot-run-shutdown-error-test-*")
	if err != nil {
		t.Fatalf("create APPDATA temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataDir)
	})
	t.Setenv("APPDATA", appDataDir)
	t.Setenv(tokenEnv, "test-token")

	boolPtr := func(v bool) *bool { return &v }
	dbCfg := runtimeDatabaseConfigForRunnerTests(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    boolPtr(false),
				Automod:       boolPtr(false),
				Commands:      boolPtr(false),
				AdminCommands: boolPtr(false),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: boolPtr(false),
			},
		},
		Guilds: []files.GuildConfig{},
	}
	_ = writeRunnerLegacyConfig(t, appName, cfg)

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "alice-test",
		Discriminator: "0001",
		Bot:           true,
	}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	origCloseStore := closeStore
	origCloseDiscordSession := closeDiscordSession
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		waitForInterrupt = origWaitForInterrupt
		shutdownDelay = origShutdownDelay
		closeStore = origCloseStore
		closeDiscordSession = origCloseDiscordSession
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	waitForInterrupt = func() {}
	shutdownDelay = func(time.Duration) {}

	storeCloseErr := errors.New("store close failure")
	discordCloseErr := errors.New("discord close failure")

	var storeCloseCalls int32
	var discordCloseCalls int32
	closeStore = func(interface{ Close() error }) error {
		atomic.AddInt32(&storeCloseCalls, 1)
		return storeCloseErr
	}
	closeDiscordSession = func(interface{ Close() error }) error {
		atomic.AddInt32(&discordCloseCalls, 1)
		return discordCloseErr
	}

	err = Run(appName, tokenEnv)
	if err == nil {
		t.Fatalf("expected shutdown error, got nil")
	}
	if !errors.Is(err, storeCloseErr) {
		t.Fatalf("expected shutdown error to wrap store close failure, got: %v", err)
	}
	if !errors.Is(err, discordCloseErr) {
		t.Fatalf("expected shutdown error to wrap discord close failure, got: %v", err)
	}
	if got := atomic.LoadInt32(&storeCloseCalls); got != 1 {
		t.Fatalf("expected one store close call, got %d", got)
	}
	if got := atomic.LoadInt32(&discordCloseCalls); got != 1 {
		t.Fatalf("expected one discord close call, got %d", got)
	}
}

func TestRun_ControlServerBindFailureIsNonFatal(t *testing.T) {
	const (
		appName  = "alicebot-run-bind-warning-test"
		tokenEnv = "ALICE_TEST_TOKEN"
	)

	appDataDir, err := os.MkdirTemp("", "alicebot-run-bind-warning-test-*")
	if err != nil {
		t.Fatalf("create APPDATA temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataDir)
	})
	t.Setenv("APPDATA", appDataDir)
	t.Setenv(tokenEnv, "test-token")

	boolPtr := func(v bool) *bool { return &v }
	dbCfg := runtimeDatabaseConfigForRunnerTests(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    boolPtr(false),
				Automod:       boolPtr(false),
				Commands:      boolPtr(false),
				AdminCommands: boolPtr(false),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: boolPtr(false),
			},
		},
		Guilds: []files.GuildConfig{},
	}
	_ = writeRunnerLegacyConfig(t, appName, cfg)

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create fake discord session: %v", err)
	}
	session.State.User = &discordgo.User{
		ID:            "bot-id",
		Username:      "alice-test",
		Discriminator: "0001",
		Bot:           true,
	}

	occupiedListener, err := net.Listen("tcp", defaultControlAddr)
	if err != nil {
		t.Fatalf("listen on fixed control address: %v", err)
	}
	t.Cleanup(func() {
		_ = occupiedListener.Close()
	})

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		waitForInterrupt = origWaitForInterrupt
		shutdownDelay = origShutdownDelay
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
		return session, nil
	}
	newDiscordSessionWithIntents = func(string, discordgo.Intent) (*discordgo.Session, error) {
		return session, nil
	}
	waitForInterrupt = func() {}
	shutdownDelay = func(time.Duration) {}

	if err := Run(appName, tokenEnv); err != nil {
		t.Fatalf("run returned error despite control bind conflict: %v", err)
	}
}
