package app

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveBotRuntimeCapabilitiesResolvesGranularFeatures(t *testing.T) {
	cfg := &files.BotConfig{
		Guilds: []files.GuildConfig{
			{
				GuildID:           "guild-1",
				BotInstanceTokens: map[string]files.EncryptedString{"alice": "a", "sandrone": "s"},
				FeatureRouting: map[string]string{
					"roles": "sandrone",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: new(bool(true)),
					},
				},
			},
		},
	}

	sandroneCaps := resolveBotRuntimeCapabilities(cfg, "sandrone")
	if !sandroneCaps.HasCommands() {
		t.Errorf("Expected Sandrone to have commands capability due to roles feature routing")
	}
	if sandroneCaps.intents&discordgo.IntentsGuildMembers == 0 {
		t.Errorf("Expected Sandrone to have IntentsGuildMembers due to roles feature routing")
	}

	aliceCaps := resolveBotRuntimeCapabilities(cfg, "alice")
	if !aliceCaps.HasCommands() {
		t.Errorf("Expected Alice to have commands capability due to default fallback for commands feature")
	}
}
