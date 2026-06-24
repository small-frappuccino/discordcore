package app

import (
	"context"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/runtimeapply"
	"github.com/small-frappuccino/discordgo"
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
					"commands":   "generic",
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

	session, _ := discordgo.New("Bot test-token")
	genericHandler, err := NewCommandHandlerForBot(CommandHandlerDeps{
		Session:        session,
		ConfigManager:  cfgMgr,
		BotInstanceID:  "generic",
		RuntimeApplier: runtimeapply.New(nil, nil),
	})
	if err != nil {
		t.Fatalf("setup handler: %v", err)
	}

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
			handles := genericHandler.handlesGuildRoute("guild-1", commands.InteractionRouteKey{Path: tt.path})
			if handles != tt.wantHandles {
				t.Errorf("generic handles %s: got %v, want %v", tt.path, handles, tt.wantHandles)
			}
		})
	}
}
