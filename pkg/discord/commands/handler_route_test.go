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
				BotInstanceTokens: map[string]files.EncryptedString{"alice": "a", "sandrone": "s"},
				FeatureRouting: map[string]string{
					"roles":      "sandrone",
					"moderation": "sandrone",
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

	aliceHandler := NewCommandHandlerForBot(nil, cfgMgr, "alice", "alice")
	sandroneHandler := NewCommandHandlerForBot(nil, cfgMgr, "sandrone", "alice")

	tests := []struct {
		name         string
		path         string
		wantAlice    bool
		wantSandrone bool
	}{
		{"Roles command goes to Sandrone", "rolepanel", false, true},
		{"Moderation command goes to Sandrone", "ban", false, true},
		{"Unrouted command goes to default (Alice)", "config", true, false},
		{"Unrouted QOTD command goes to default (Alice)", "qotd", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aliceHandles := aliceHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Path: tt.path})
			if aliceHandles != tt.wantAlice {
				t.Errorf("Alice handles %s: got %v, want %v", tt.path, aliceHandles, tt.wantAlice)
			}

			sandroneHandles := sandroneHandler.handlesGuildRoute("guild-1", core.InteractionRouteKey{Path: tt.path})
			if sandroneHandles != tt.wantSandrone {
				t.Errorf("Sandrone handles %s: got %v, want %v", tt.path, sandroneHandles, tt.wantSandrone)
			}
		})
	}
}
