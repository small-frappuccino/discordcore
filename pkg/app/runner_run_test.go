package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/util"
)

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

	util.SetAppName(appName)
	settingsPath := util.GetSettingsFilePath()
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("create settings directory: %v", err)
	}

	boolPtr := func(v bool) *bool { return &v }
	cfg := files.BotConfig{
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
	rawCfg, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("marshal settings config: %v", err)
	}
	if err := os.WriteFile(settingsPath, rawCfg, 0o644); err != nil {
		t.Fatalf("write settings config: %v", err)
	}

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
	origWaitForInterrupt := waitForInterrupt
	origShutdownDelay := shutdownDelay
	origSetupCommandHandler := setupCommandHandler
	origShutdownCommandHandler := shutdownCommandHandler
	t.Cleanup(func() {
		newDiscordSession = origNewDiscordSession
		waitForInterrupt = origWaitForInterrupt
		shutdownDelay = origShutdownDelay
		setupCommandHandler = origSetupCommandHandler
		shutdownCommandHandler = origShutdownCommandHandler
	})

	newDiscordSession = func(string) (*discordgo.Session, error) {
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
}
