package app

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRun_MidBootSabotageTriggersTeardown(t *testing.T) {
	const appName = "discordmain-rollback-test"

	appDataDir, err := os.MkdirTemp("", "discordmain-rollback-test-*")
	if err != nil {
		t.Fatalf("create APPDATA temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataDir)
	})
	t.Setenv("APPDATA", appDataDir)

	dbCfg, configStore := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    Ptr(false),
				Automod:       Ptr(false),
				Commands:      Ptr(true),
				AdminCommands: Ptr(false),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: Ptr(false),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"main": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, configStore, cfg)

	// Mock Discord Session
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
	session.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origCloseStore := closeStore
	origCloseDiscordSession := closeDiscordSession

	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		waitForInterrupt = origWaitForInterrupt
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
	origOpenDiscordSession := openDiscordSession
	t.Cleanup(func() {
		openDiscordSession = origOpenDiscordSession
	})
	openDiscordSession = func(s interface{ Open() error }) error { return nil }
	waitForInterrupt = func() { time.Sleep(100 * time.Millisecond) }
	shutdownDelay = func(time.Duration) {}

	// We sabotage the boot by forcing the command handler setup to fail.
	// This occurs mid-boot inside startBotInstanceBackground -> initializeBotRuntime.
	sabotageErr := errors.New("simulated boot failure")
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		return sabotageErr
	}

	var storeCloseCalls int32
	var discordCloseCalls int32
	closeStore = func(c interface{ Close() error }) error {
		atomic.AddInt32(&storeCloseCalls, 1)
		return c.Close()
	}
	closeDiscordSession = func(c interface{ Close() error }) error {
		atomic.AddInt32(&discordCloseCalls, 1)
		return nil
	}

	time.Sleep(100 * time.Millisecond)
	goroutinesBefore := runtime.NumGoroutine()

	err = Run(appName)

	if err == nil {
		t.Fatalf("expected Run to fail due to mid-boot sabotage")
	}

	if !strings.Contains(err.Error(), sabotageErr.Error()) {
		t.Fatalf("expected error to contain %q, got: %v", sabotageErr.Error(), err)
	}

	if got := atomic.LoadInt32(&storeCloseCalls); got != 1 {
		t.Errorf("expected 1 store close call on rollback, got %d", got)
	}

	if got := atomic.LoadInt32(&discordCloseCalls); got != 1 {
		t.Errorf("expected 1 discord session close call on rollback, got %d", got)
	}

	// Wait for goroutines to settle
	time.Sleep(200 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()

	// Check for major goroutine leaks (allowing small buffer for background go testing mechanisms)
	if goroutinesAfter > goroutinesBefore+2 {
		t.Errorf("goroutine leak detected: before %d, after %d", goroutinesBefore, goroutinesAfter)
	}
}

func TestRun_CascadingRollbackFailures(t *testing.T) {
	const appName = "discordmain-cascading-rollback-test"

	appDataDir, err := os.MkdirTemp("", "discordmain-cascading-rollback-test-*")
	if err != nil {
		t.Fatalf("create APPDATA temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(appDataDir)
	})
	t.Setenv("APPDATA", appDataDir)

	dbCfg, configStore := openRunnerConfigStore(t)
	setRunnerDatabaseBootstrapEnv(t, dbCfg)
	cfg := files.BotConfig{
		RuntimeConfig: files.RuntimeConfig{
			Database: dbCfg,
		},
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    Ptr(false),
				Automod:       Ptr(false),
				Commands:      Ptr(true),
				AdminCommands: Ptr(false),
			},
			Maintenance: files.FeatureMaintenanceToggles{
				DBCleanup: Ptr(false),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID: "guild-1",
			BotInstanceTokens: map[string]files.EncryptedString{
				"main": files.EncryptedString("test-token"),
			},
		}},
	}
	seedRunnerConfig(t, configStore, cfg)

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
	session.State.Guilds = []*discordgo.Guild{{ID: "guild-1"}}

	origNewDiscordSession := newDiscordSession
	origNewDiscordSessionWithIntents := newDiscordSessionWithIntents
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origCloseStore := closeStore
	origCloseDiscordSession := closeDiscordSession

	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		newDiscordSessionWithIntents = origNewDiscordSessionWithIntents
		waitForInterrupt = origWaitForInterrupt
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
	origOpenDiscordSession := openDiscordSession
	t.Cleanup(func() {
		openDiscordSession = origOpenDiscordSession
	})
	openDiscordSession = func(s interface{ Open() error }) error { return nil }
	waitForInterrupt = func() { time.Sleep(100 * time.Millisecond) }
	shutdownDelay = func(time.Duration) {}

	// Cause mid-boot sabotage
	sabotageErr := errors.New("simulated boot failure")
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		return sabotageErr
	}

	// Cascading failures on teardown hooks
	storeCloseErr := errors.New("store close failure")
	discordCloseErr := errors.New("discord close failure")

	closeStore = func(c interface{ Close() error }) error {
		return storeCloseErr
	}
	closeDiscordSession = func(c interface{ Close() error }) error {
		return discordCloseErr
	}

	err = Run(appName)

	if err == nil {
		t.Fatalf("expected Run to fail due to mid-boot sabotage")
	}

	// Ensure the original error is preserved and the cascading errors are joined gracefully
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
