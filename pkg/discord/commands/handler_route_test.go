package commands

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestCommandHandlerRoutesFeaturesToCorrectBotInstance(t *testing.T) {
	boolPtr := func(v bool) *bool { return &v }
	cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	if _, err := cfgMgr.UpdateConfig(context.Background(), func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
				FeatureRouting: map[string]string{
					"roles":      "generic",
					"moderation": "generic",
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

	genericHandler := NewCommandHandlerForBot(nil, cfgMgr, "generic")

	tests := []struct {
		name        string
		path        string
		wantHandles bool
	}{
		{"Roles command goes to generic", "rolepanel", true},
		{"Moderation command goes to generic", "ban", true},
		{"Base command goes to generic", "config", true},
		{"Unrouted QOTD command goes to no one", "qotd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handles := genericHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Path: tt.path})
			if handles != tt.wantHandles {
				t.Errorf("generic handles %s: got %v, want %v", tt.path, handles, tt.wantHandles)
			}
		})
	}
}
