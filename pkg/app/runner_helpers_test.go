package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestRunner_ShutdownStartupServices(t *testing.T) {
	// Should handle nil safely
	shutdownStartupServices(nil, nil, "ok")
}

func TestRunner_RollbackStoreClose(t *testing.T) {
	// Should handle nil safely
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
