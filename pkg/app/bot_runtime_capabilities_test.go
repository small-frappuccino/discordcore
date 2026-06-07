package app

import (
	"fmt"
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
				GuildID:           "main-guild",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
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
						ChannelID: "question-main",
					}},
				},
			},
			{
				GuildID:           "companion-guild",
				BotInstanceTokens: map[string]files.EncryptedString{"companion": "a"},
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

	capabilities := resolveBotRuntimeCapabilities(cfg, "companion", "main")
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

	capabilities := resolveBotRuntimeCapabilities(&files.BotConfig{}, "companion", "main")
	if capabilities.monitoring || capabilities.HasCommands() || capabilities.admin || capabilities.automod || capabilities.userPrune || capabilities.qotdRuntime {
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
				GuildID:           "g1",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
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
				GuildID:           "g2",
				BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
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

	capabilities := resolveBotRuntimeCapabilities(cfg, "main", "main")
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

func TestQOTDCapabilityPolicy_Modify_DeepCopyAndIsolation(t *testing.T) {
	t.Parallel()

	// 1. Arrange: Create a capability struct with many privileged intents and flags
	original := botRuntimeCapabilities{
		monitoring:  true,
		admin:       true,
		automod:     true,
		userPrune:   true,
		qotdRuntime: true,
		warmup:      true,
		intents:     discordgo.IntentsGuilds | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages,
		hasCommands: true,
	}

	// 2. Act: Apply the QOTD policy modifier
	var policy CapabilityModifier = QOTDCapabilityPolicy{}
	masked := policy.Modify(original)

	// 3. Assert: Verify the memory addresses are completely different (Deep Copy contract)
	addrOriginal := fmt.Sprintf("%p", &original)
	addrMasked := fmt.Sprintf("%p", &masked)
	if addrOriginal == addrMasked {
		t.Fatalf("CapabilityModifier returned the same struct reference (%s). OCP requires deep copy to prevent side effects", addrOriginal)
	}

	// 4. Assert: Validate the mask logic stripped privileged access
	if masked.monitoring || masked.admin || masked.automod || masked.userPrune || masked.warmup {
		t.Fatalf("QOTD policy failed to strip privileged flags: %+v", masked)
	}

	if !masked.qotdRuntime || !masked.hasCommands {
		t.Fatalf("QOTD policy incorrectly stripped essential QOTD flags: %+v", masked)
	}

	if masked.intents != discordgo.IntentsGuilds {
		t.Fatalf("QOTD policy should only allow IntentsGuilds, got %d", masked.intents)
	}
}

