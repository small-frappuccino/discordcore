package app

import (
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordgo"
)

func TestResolveBotRuntimeCapabilitiesUsesScopedGuildsAndMinimalIntents(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    new(bool(false)),
				Automod:       new(bool(false)),
				Commands:      new(bool(false)),
				AdminCommands: new(bool(false)),
			},
			Logging: files.FeatureLoggingToggles{
				AvatarLogging:  new(bool(false)),
				RoleUpdate:     new(bool(false)),
				MemberJoin:     new(bool(false)),
				MemberLeave:    new(bool(false)),
				MessageProcess: new(bool(false)),
				MessageEdit:    new(bool(false)),
				MessageDelete:  new(bool(false)),
				ReactionMetric: new(bool(false)),
				AutomodAction:  new(bool(false)),
			},
			PresenceWatch: files.FeaturePresenceWatchToggles{
				Bot:  new(bool(false)),
				User: new(bool(false)),
			},
			Safety: files.FeatureSafetyToggles{
				BotRolePermMirror: new(bool(false)),
			},
			Backfill: files.FeatureBackfillToggles{
				Enabled: new(bool(false)),
			},
			StatsChannels:  new(bool(false)),
			AutoRoleAssign: new(bool(false)),
			UserPrune:      new(bool(false)),
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:           "main-guild",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: new(bool(true)),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-main",
					}},
				},
			},
			{
				GuildID:           "companion-guild",
				BotInstanceTokens: map[string]files.EncryptedString{"companion": "a"},
				FeatureRouting: map[string]string{
					"qotd":           "companion",
					"roles":          "companion",
					"moderation":     "companion",
					"commands":       "companion",
					"admin_commands": "companion",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring:    new(bool(true)),
						Commands:      new(bool(true)),
						AdminCommands: new(bool(true)),
					},
					Logging: files.FeatureLoggingToggles{
						ReactionMetric: new(bool(true)),
					},
					UserPrune: new(bool(true)),
				},
				UserPrune: files.UserPruneConfig{Enabled: true},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-companion",
					}},
				},
			},
		},
	}

	capabilities := resolveBotRuntimeCapabilities(cfg, "companion")
	if !capabilities.monitoring {
		t.Fatal("expected monitoring capability for companion runtime")
	}
	if !capabilities.HasCommands() {
		t.Fatal("expected commands capability for companion runtime")
	}
	if !capabilities.admin {
		t.Fatal("expected admin commands capability for companion runtime")
	}
	if !capabilities.userPrune {
		t.Fatal("expected user prune capability for companion runtime")
	}
	if !capabilities.qotdRuntime {
		t.Fatal("expected qotd runtime capability for companion runtime")
	}

	required := discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessageReactions
	if capabilities.intents&required != required {
		t.Fatalf("expected intents mask to include %d, got %d", required, capabilities.intents)
	}
	if capabilities.intents&discordgo.IntentsGuildMessages != 0 {
		t.Fatalf("did not expect guild messages intent, got %d", capabilities.intents)
	}
	if capabilities.intents&discordgo.IntentMessageContent != 0 {
		t.Fatalf("did not expect message content intent, got %d", capabilities.intents)
	}
	if capabilities.intents&discordgo.IntentsGuildPresences != 0 {
		t.Fatalf("did not expect guild presences intent, got %d", capabilities.intents)
	}
}

func TestResolveBotRuntimeCapabilitiesWithoutGuildBindingsIsIdle(t *testing.T) {
	t.Parallel()

	capabilities := resolveBotRuntimeCapabilities(&files.BotConfig{}, "companion")
	if capabilities.monitoring || capabilities.HasCommands() || capabilities.admin || capabilities.automod || capabilities.userPrune || capabilities.qotdRuntime {
		t.Fatalf("expected idle capabilities for unbound bot, got %+v", capabilities)
	}
	if capabilities.intents != discordgo.IntentsGuilds {
		t.Fatalf("expected guilds-only intent for unbound bot, got %d", capabilities.intents)
	}
}

func TestResolveBotRuntimeCapabilitiesAggregatesAllGuildsForSameBotInstance(t *testing.T) {
	t.Parallel()

	cfg := &files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: new(bool(false)),
				Commands:   new(bool(false)),
			},
			Logging: files.FeatureLoggingToggles{
				ReactionMetric: new(bool(false)),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:           "g1",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: new(bool(true)),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-1",
					}},
				},
			},
			{
				GuildID:           "g2",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
				FeatureRouting: map[string]string{
					"qotd":       "main",
					"roles":      "main",
					"moderation": "main",
				},
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: new(bool(true)),
					},
					Logging: files.FeatureLoggingToggles{
						ReactionMetric: new(bool(true)),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						ChannelID: "question-2",
					}},
				},
			},
		},
	}

	capabilities := resolveBotRuntimeCapabilities(cfg, "main")
	if !capabilities.HasCommands() {
		t.Fatal("expected commands capability to include any guild assigned to main")
	}
	if !capabilities.monitoring {
		t.Fatal("expected monitoring capability to include any guild assigned to main")
	}
	if !capabilities.qotdRuntime {
		t.Fatal("expected qotd runtime capability to include any configured guild assigned to main")
	}
	if capabilities.intents&discordgo.IntentsGuildMessageReactions == 0 {
		t.Fatalf("expected reaction intents from guild aggregation, got %d", capabilities.intents)
	}
}
