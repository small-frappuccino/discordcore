package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestResolveBotRuntimeCapabilitiesResolvesGranularFeatures(t *testing.T) {
	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"generic": "a"},
				FeatureRouting: map[string]string{
					"roles": "generic",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: new(bool(true)),
					},
				},
			},
		},
	}

	customCaps := resolveBotRuntimeCapabilities(cfg, "generic")
	if !customCaps.HasCommands() {
		t.Errorf("Expected custom to have commands capability due to roles feature routing")
	}
	if customCaps.intents&discordgo.IntentsGuildMembers == 0 {
		t.Errorf("Expected custom to have IntentsGuildMembers due to roles feature routing")
	}

	mainCaps := resolveBotRuntimeCapabilities(cfg, "generic")
	if !mainCaps.HasCommands() {
		t.Errorf("Expected main to have commands capability due to default fallback for commands feature")
	}
}
