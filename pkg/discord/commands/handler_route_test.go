package commands

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestCommandHandlerRoutesFeaturesToCorrectBotInstance(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a", "custom": "s"},
				FeatureRouting: map[string]string{
					"roles":      "custom",
					"moderation": "custom",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(true),
					},
				},
			},
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	mainHandler := NewCommandHandlerForBot(nil, cfgMgr, files.TokenHash("a"))
	customHandler := NewCommandHandlerForBot(nil, cfgMgr, files.TokenHash("s"))

	tests := []struct {
		name       string
		path       string
		wantMain   bool
		wantCustom bool
	}{
		{"Roles command goes to custom", "rolepanel", false, true},
		{"Moderation command goes to custom", "ban", false, true},
		{"Base command goes to all bots", "config", true, true},
		{"Unrouted QOTD command goes to no one", "qotd", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mainHandles := mainHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Path: tt.path})
			if mainHandles != tt.wantMain {
				t.Errorf("main handles %s: got %v, want %v", tt.path, mainHandles, tt.wantMain)
			}

			customHandles := customHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Path: tt.path})
			if customHandles != tt.wantCustom {
				t.Errorf("custom handles %s: got %v, want %v", tt.path, customHandles, tt.wantCustom)
			}
		})
	}
}
