package app

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestInitializeBotRuntimeSkipsCommandHandlerWhenCommandsDisabled(t *testing.T) {
	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("create discord session: %v", err)
	}

	cfgMgr := files.NewMemoryConfigManager()
	boolPtr := func(v bool) *bool { return &v }
	if _, err := cfgMgr.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = []files.GuildConfig{
			{
				GuildID:       "guild-1",
				BotInstanceID: "alice",
				Features: files.FeatureToggles{
					Services: files.FeatureServiceToggles{
						Commands: boolPtr(false),
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
		}
		return nil
	}); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	origNewCommandHandlerForBot := newCommandHandlerForBot
	origSetupCommandHandler := setupCommandHandler
	defer func() {
		newCommandHandlerForBot = origNewCommandHandlerForBot
		setupCommandHandler = origSetupCommandHandler
	}()

	var created bool
	var setupCommandsCalls int

	newCommandHandlerForBot = func(session *discordgo.Session, configManager *files.ConfigManager, botInstanceID string, defaultBotInstanceID string) *commands.CommandHandler {
		created = true
		return nil
	}
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		setupCommandsCalls++
		return nil
	}

	runtime := &botRuntime{
		instanceID:   "alice",
		capabilities: botRuntimeCapabilities{qotd: true},
		session:      session,
	}
	err = initializeBotRuntime(runtime, botRuntimeOptions{
		defaultBotInstanceID: "alice",
		runtimeCount:         1,
		configManager:        cfgMgr,
	})
	if err != nil {
		t.Fatalf("initialize bot runtime: %v", err)
	}

	if setupCommandsCalls != 0 {
		t.Fatalf("expected slash command setup to be skipped, got %d calls", setupCommandsCalls)
	}
	if created {
		t.Fatal("expected command handler creation to be skipped when commands are disabled")
	}
	if runtime.commandHandler != nil {
		t.Fatal("expected no command handler when commands are disabled")
	}
}
