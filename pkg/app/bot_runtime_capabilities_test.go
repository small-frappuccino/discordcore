package app

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestResolveBotRuntimeCapabilitiesUsesScopedGuildsAndMinimalIntents(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }
	cfg := &files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    boolPtr(false),
				Automod:       boolPtr(false),
				Commands:      boolPtr(false),
				AdminCommands: boolPtr(false),
			},
			Logging: files.FeatureLoggingToggles{
				AvatarLogging:  boolPtr(false),
				RoleUpdate:     boolPtr(false),
				MemberJoin:     boolPtr(false),
				MemberLeave:    boolPtr(false),
				MessageProcess: boolPtr(false),
				MessageEdit:    boolPtr(false),
				MessageDelete:  boolPtr(false),
				ReactionMetric: boolPtr(false),
				AutomodAction:  boolPtr(false),
			},
			PresenceWatch: files.FeaturePresenceWatchToggles{
				Bot:  boolPtr(false),
				User: boolPtr(false),
			},
			Safety: files.FeatureSafetyToggles{
				BotRolePermMirror: boolPtr(false),
			},
			Backfill: files.FeatureBackfillToggles{
				Enabled: boolPtr(false),
			},
			StatsChannels:  boolPtr(false),
			AutoRoleAssign: boolPtr(false),
			UserPrune:      boolPtr(false),
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:       "alice-guild",
				BotInstanceID: "alice",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(true),
					},
				},
				QOTD: files.QOTDConfig{
					ActiveDeckID: files.LegacyQOTDDefaultDeckID,
					Decks: []files.QOTDDeckConfig{{
						ID:        files.LegacyQOTDDefaultDeckID,
						Name:      files.LegacyQOTDDefaultDeckName,
						Enabled:   true,
						ChannelID: "question-alice",
					}},
				},
			},
			{
				GuildID:       "companion-guild",
				BotInstanceID: "companion",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring:    boolPtr(true),
						Commands:      boolPtr(true),
						AdminCommands: boolPtr(true),
					},
					Logging: files.FeatureLoggingToggles{
						ReactionMetric: boolPtr(true),
					},
					UserPrune: boolPtr(true),
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

	capabilities := resolveBotRuntimeCapabilities(cfg, "companion", "alice")
	if !capabilities.monitoring {
		t.Fatal("expected monitoring capability for companion runtime")
	}
	if !capabilities.commands {
		t.Fatal("expected commands capability for companion runtime")
	}
	if !capabilities.admin {
		t.Fatal("expected admin commands capability for companion runtime")
	}
	if !capabilities.userPrune {
		t.Fatal("expected user prune capability for companion runtime")
	}
	if !capabilities.qotd {
		t.Fatal("expected qotd capability for companion runtime")
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

	capabilities := resolveBotRuntimeCapabilities(&files.BotConfig{}, "companion", "alice")
	if capabilities.monitoring || capabilities.commands || capabilities.admin || capabilities.automod || capabilities.userPrune || capabilities.qotd {
		t.Fatalf("expected idle capabilities for unbound bot, got %+v", capabilities)
	}
	if capabilities.intents != discordgo.IntentsGuilds {
		t.Fatalf("expected guilds-only intent for unbound bot, got %d", capabilities.intents)
	}
}

func TestResolveBotRuntimeCapabilitiesAggregatesAllGuildsForSameBotInstance(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }
	cfg := &files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring: boolPtr(false),
				Commands:   boolPtr(false),
			},
			Logging: files.FeatureLoggingToggles{
				ReactionMetric: boolPtr(false),
			},
		},
		Guilds: []files.GuildConfig{
			{
				GuildID:       "g1",
				BotInstanceID: "alice",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(true),
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
				GuildID:       "g2",
				BotInstanceID: "alice",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Monitoring: boolPtr(true),
					},
					Logging: files.FeatureLoggingToggles{
						ReactionMetric: boolPtr(true),
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

	capabilities := resolveBotRuntimeCapabilities(cfg, "alice", "alice")
	if !capabilities.commands {
		t.Fatal("expected commands capability to include any guild assigned to alice")
	}
	if !capabilities.monitoring {
		t.Fatal("expected monitoring capability to include any guild assigned to alice")
	}
	if !capabilities.qotd {
		t.Fatal("expected qotd capability to include any configured guild assigned to alice")
	}
	if capabilities.intents&discordgo.IntentsGuildMessageReactions == 0 {
		t.Fatalf("expected reaction intents from guild aggregation, got %d", capabilities.intents)
	}
}

func TestResolveBotRuntimeCapabilitiesUsesQOTDDomainBindings(t *testing.T) {
	t.Parallel()

	boolPtr := func(v bool) *bool { return &v }
	cfg := &files.BotConfig{
		Features: files.FeatureToggles{
			Services: files.FeatureServiceToggles{
				Monitoring:    boolPtr(false),
				Automod:       boolPtr(false),
				Commands:      boolPtr(false),
				AdminCommands: boolPtr(false),
			},
			Logging: files.FeatureLoggingToggles{
				AvatarLogging:  boolPtr(false),
				RoleUpdate:     boolPtr(false),
				MemberJoin:     boolPtr(false),
				MemberLeave:    boolPtr(false),
				MessageProcess: boolPtr(false),
				MessageEdit:    boolPtr(false),
				MessageDelete:  boolPtr(false),
				ReactionMetric: boolPtr(false),
				AutomodAction:  boolPtr(false),
			},
		},
		Guilds: []files.GuildConfig{{
			GuildID:       "g1",
			BotInstanceID: "alice",
			Features: files.FeatureToggles{
				Services: files.FeatureServiceToggles{
					Commands: boolPtr(true),
				},
			},
			DomainBotInstanceIDs: map[string]string{
				files.BotDomainQOTD: "companion",
			},
			QOTD: files.QOTDConfig{
				ActiveDeckID: files.LegacyQOTDDefaultDeckID,
				Decks: []files.QOTDDeckConfig{{
					ID:        files.LegacyQOTDDefaultDeckID,
					Name:      files.LegacyQOTDDefaultDeckName,
					Enabled:   true,
					ChannelID: "question-companion",
				}},
			},
		}},
	}

	aliceCapabilities := resolveBotRuntimeCapabilities(cfg, "alice", "alice")
	if !aliceCapabilities.commands {
		t.Fatal("expected alice runtime to keep command capability from guild-wide binding")
	}
	if aliceCapabilities.qotd {
		t.Fatalf("expected alice runtime to lose qotd capability when qotd domain is overridden, got %+v", aliceCapabilities)
	}

	companionCapabilities := resolveBotRuntimeCapabilities(cfg, "companion", "alice")
	if !companionCapabilities.qotd {
		t.Fatal("expected companion runtime to gain qotd capability from domain override")
	}
	if !companionCapabilities.commands {
		t.Fatalf("expected companion runtime to start command handling for qotd-only catalog, got %+v", companionCapabilities)
	}
	if companionCapabilities.commandsDefaultDomain || companionCapabilities.admin || companionCapabilities.monitoring || companionCapabilities.userPrune {
		t.Fatalf("expected companion runtime to gain only qotd command catalog capability from domain override, got %+v", companionCapabilities)
	}
	if companionCapabilities.intents != discordgo.IntentsGuilds {
		t.Fatalf("expected qotd-only runtime to keep minimal intents, got %d", companionCapabilities.intents)
	}
}
