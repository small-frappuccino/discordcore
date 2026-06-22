package app

import (
	"context"
	"testing"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/gateway"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
)

func TestResolveBotRuntimeCapabilities_GuildAggregation(t *testing.T) {
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
				FeatureRouting: map[string]string{
					"qotd": "main",
				},
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
						ChannelID: "q1",
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
			},
		},
	}

	instances := []resolvedBotInstance{{ID: "main", Token: "abc"}}
	capsMap := resolveRuntimeCapabilities(cfg, instances, RunProfileDiscordMain)
	capabilities := capsMap["main"]

	if !capabilities.HasCommands() {
		t.Fatal("expected commands capability")
	}
	if !capabilities.monitoring {
		t.Fatal("expected monitoring capability")
	}
	if !capabilities.qotdRuntime {
		t.Fatal("expected qotd runtime capability")
	}
	if int(capabilities.intents)&int(gateway.IntentGuildMessageReactions) == 0 {
		t.Fatal("expected reaction intents")
	}
}

func TestBotRuntime_InitializationRouting(t *testing.T) {
	origFetchBotArikawaMe := fetchBotArikawaMe
	t.Cleanup(func() {
		fetchBotArikawaMe = origFetchBotArikawaMe
	})
	fetchBotArikawaMe = func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	origOpenBotArikawaState := openBotArikawaState
	t.Cleanup(func() {
		openBotArikawaState = origOpenBotArikawaState
	})
	openBotArikawaState = func(ctx context.Context, s *state.State) error { return nil }
	origNewCommandHandlerForBot := newCommandHandlerForBot
	origSetupCommandHandler := setupCommandHandler
	t.Cleanup(func() {
		newCommandHandlerForBot = origNewCommandHandlerForBot
		setupCommandHandler = origSetupCommandHandler
	})
	newCommandHandlerForBot = func(deps CommandHandlerDeps) (*CommandHandler, error) {
		return &CommandHandler{botInstanceID: deps.BotInstanceID, session: deps.Session}, nil
	}
	setupCommandHandler = func(ch *CommandHandler) error { return nil }

	tests := []struct {
		name                 string
		cfg                  *files.BotConfig
		expectedServices     []string
		unexpectedServices   []string
		expectedCommandsSkip bool
	}{
		{
			name: "Exhaustive Mocking: All Features Enabled",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID:           "g1",
						BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
						FeatureRouting: map[string]string{
							"moderation": "main",
							"logging":    "main",
							"roles":      "main",
							"stats":      "main",
							"qotd":       "main",
						},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands:   new(bool(true)),
								Monitoring: new(bool(true)), // Moderation implicitly needs commands
							},
							Logging: files.FeatureLoggingToggles{
								AvatarLogging:  new(bool(true)),
								RoleUpdate:     new(bool(true)),
								MemberJoin:     new(bool(true)),
								MemberLeave:    new(bool(true)),
								MessageProcess: new(bool(true)),
								MessageEdit:    new(bool(true)),
								MessageDelete:  new(bool(true)),
							},
						},
						Channels: files.ChannelsConfig{
							AutomodAction: "channel1",
							MemberJoin:    "channel2",
							MemberLeave:   "channel2",
							MessageEdit:   "channel3",
							MessageDelete: "channel3",
						},
						QOTD: files.QOTDConfig{
							ActiveDeckID: "deck1",
							Decks: []files.QOTDDeckConfig{
								{ID: "deck1", Enabled: true, ChannelID: "c"},
							},
						},
					},
				},
			},
			expectedServices: []string{
				"discord_automod_adapter",
				"messages",
				"member_events_main",
				"stats",
				"qotd",
				"command-handler",
			},
			unexpectedServices:   nil,
			expectedCommandsSkip: false,
		},
		{
			name: "Routing Disabled Features Yields Idle Core",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID:           "g1",
						BotInstanceTokens: map[string]files.EncryptedString{"main": "a"},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands: new(bool(false)),
							},
							Logging: files.FeatureLoggingToggles{
								AvatarLogging: new(bool(false)),
								MessageEdit:   new(bool(false)),
							},
						},
					},
				},
			},
			expectedServices:     []string{},
			unexpectedServices:   []string{"command-handler", "discord_automod_adapter", "messages", "member_events_main"},
			expectedCommandsSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgMgr := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
			cfgMgr.ApplyConfig(tt.cfg)

			instances := []resolvedBotInstance{{ID: "main", Token: "abc"}}
			capsMap := resolveRuntimeCapabilities(tt.cfg, instances, RunProfileDiscordMain)
			caps := capsMap["main"]

			rt := &botRuntime{
				instanceID:    "main",
				capabilities:  caps,
				legacySession: session.NewEmptySessionForCompat("Bot fake"),
				arikawaState:  state.New("Bot fake"),
			}

			err := initializeBotRuntime(context.Background(), rt, botRuntimeOptions{
				runtimeCount:       1,
				configManager:      cfgMgr,
				store:              &postgres.Store{},
				qotdCommandService: &qotd.Service{},
			})
			if err != nil {
				t.Fatalf("unexpected init error: %v", err)
			}

			if rt.serviceManager == nil {
				t.Fatal("expected serviceManager to be initialized")
			}

			services := rt.serviceManager.GetAllServices()
			for _, expected := range tt.expectedServices {
				if _, ok := services[expected]; !ok {
					t.Errorf("expected service %q to be registered, but it was missing", expected)
				}
			}

			for _, unexpected := range tt.unexpectedServices {
				if _, ok := services[unexpected]; ok {
					t.Errorf("expected service %q to NOT be registered, but it was found", unexpected)
				}
			}

			if tt.expectedCommandsSkip {
				if rt.commandHandler != nil {
					t.Errorf("expected command handler to be skipped/nil")
				}
			} else {
				if rt.commandHandler == nil {
					t.Errorf("expected command handler to be populated")
				}
			}

			shutdownBotRuntime(rt, context.Background())
		})
	}
}
