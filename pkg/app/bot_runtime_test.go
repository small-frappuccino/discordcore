package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"github.com/small-frappuccino/discordcore/pkg/discord/session"
	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage/postgres"
	"github.com/small-frappuccino/discordgo"
	"golang.org/x/sync/errgroup"
)

func TestBotRuntime_InitializationRouting(t *testing.T) {
	t.Parallel()

	fetchBotArikawaMeHook := func(s *state.State) (*discord.User, error) {
		return &discord.User{ID: 123, Username: "test"}, nil
	}
	openBotArikawaStateHook := func(ctx context.Context, s *state.State) error { return nil }
	newCommandHandlerForBotHook := func(deps CommandHandlerDeps) (*CommandHandler, error) {
		return &CommandHandler{botInstanceID: deps.BotInstanceID, session: deps.Session}, nil
	}
	setupCommandHandlerHook := func(ch *CommandHandler) error { return nil }
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

			caps := resolveBotRuntimeCapabilities(tt.cfg, "main")

			rt := &botRuntime{
				instanceID:    "main",
				capabilities:  caps,
				legacySession: session.NewEmptySessionForCompat("Bot fake"),
				arikawaState:  state.New("Bot fake"),
			}

			err := initializeBotRuntime(context.Background(), rt, botRuntimeOptions{
				runtimeCount:            1,
				configManager:           cfgMgr,
				store:                   &postgres.Store{},
				qotdCommandService:      &qotd.Service{},
				fetchBotArikawaMe:       fetchBotArikawaMeHook,
				openBotArikawaState:     openBotArikawaStateHook,
				newCommandHandlerForBot: newCommandHandlerForBotHook,
				setupCommandHandler:     setupCommandHandlerHook,
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

func TestBotRuntime_CapabilityBitmaskDerivation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		botInstanceID      string
		cfg                *files.BotConfig
		expectedIntents    discordgo.Intent
		expectedCommands   bool
		expectedMonitoring bool
	}{
		{
			name:          "Commands and Moderation Escalation",
			botInstanceID: "main",
			cfg: &files.BotConfig{
				Guilds: []files.GuildConfig{
					{
						GuildID: "g1",
						BotInstanceTokens: map[string]files.EncryptedString{
							"main": "mock_token",
						},
						Features: files.FeatureToggles{
							Services: files.FeatureServiceToggles{
								Commands:   new(bool(true)),
								Monitoring: new(bool(false)),
							},
						},
						FeatureRouting: map[string]string{
							"moderation": "main",
						},
						Channels: files.ChannelsConfig{
							AutomodAction: "channel1",
						},
					},
				},
			},
			// Expects Guilds (base) + AutoModerationExecution + Messages (from Automod)
			expectedIntents:    discordgo.IntentsGuilds | discordgo.IntentAutoModerationExecution,
			expectedCommands:   true,
			expectedMonitoring: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			caps := resolveBotRuntimeCapabilities(tt.cfg, tt.botInstanceID)

			if (caps.intents & tt.expectedIntents) != tt.expectedIntents {
				t.Errorf("intent bitmask failure: expected %d to contain %d", caps.intents, tt.expectedIntents)
			}
			if caps.hasCommands != tt.expectedCommands {
				t.Errorf("command state failure: expected %t, got %t", tt.expectedCommands, caps.hasCommands)
			}
			if caps.monitoring != tt.expectedMonitoring {
				t.Errorf("monitoring state failure: expected %t, got %t", tt.expectedMonitoring, caps.monitoring)
			}
		})
	}
}

func TestBotRuntimeResolver_ConcurrentMemoryRotation(t *testing.T) {
	t.Parallel()

	resolver := newBotRuntimeResolver(nil, map[string]*botRuntime{
		"test": {instanceID: "test"},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	eg, egCtx := errgroup.WithContext(ctx)

	// Writer Routine
	eg.Go(func() error {
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-egCtx.Done():
				return nil
			case <-ticker.C:
				resolver.addRuntime("test", &botRuntime{instanceID: "test"})
			}
		}
	})

	// Reader Routines
	for i := 0; i < 50; i++ {
		eg.Go(func() error {
			for {
				select {
				case <-egCtx.Done():
					return nil
				default:
					var rt *botRuntime
					for id, runtime := range resolver.getRuntimes() {
						if id == "test" {
							rt = runtime
						}
					}

					// Structural validation, not just a panic check
					if rt == nil || rt.instanceID != "test" {
						return fmt.Errorf("memory layout corrupted during atomic rotation")
					}

					// Micro-yield to prevent writer starvation
					time.Sleep(time.Microsecond)
				}
			}
		})
	}

	if err := eg.Wait(); err != nil && err != context.DeadlineExceeded {
		t.Fatalf("atomic synchronization boundary failed: %v", err)
	}
}

func TestBotRuntimeResolver_WaitBarrierOrchestration(t *testing.T) {
	t.Parallel()

	resolver := newBotRuntimeResolver(nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := resolver.waitForReady(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}

	resolver = newBotRuntimeResolver(nil, nil)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel2()
		if err := resolver.waitForReady(ctx2); err != nil {
			t.Errorf("expected nil error, got: %v", err)
		}
	}()

	resolver.markReady()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for ready signal propagation")
	}
}
