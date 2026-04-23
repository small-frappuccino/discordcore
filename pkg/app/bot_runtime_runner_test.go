package app

import (
	"context"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type noopAppAnswerSubmissionService struct{}

func (noopAppAnswerSubmissionService) SubmitAnswer(context.Context, *discordgo.Session, discordqotd.SubmitAnswerParams) (*discordqotd.SubmitAnswerResult, error) {
	return nil, nil
}

func TestInitializeBotRuntimeRegistersQOTDInteractionsWithoutCommands(t *testing.T) {
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
	origSetupQOTDInteractionHandler := setupQOTDInteractionHandler
	defer func() {
		newCommandHandlerForBot = origNewCommandHandlerForBot
		setupCommandHandler = origSetupCommandHandler
		setupQOTDInteractionHandler = origSetupQOTDInteractionHandler
	}()

	var created *commands.CommandHandler
	var setupCommandsCalls int
	var setupQOTDCalls int

	newCommandHandlerForBot = func(session *discordgo.Session, configManager *files.ConfigManager, botInstanceID string, defaultBotInstanceID string) *commands.CommandHandler {
		created = commands.NewCommandHandlerForBot(session, configManager, botInstanceID, defaultBotInstanceID)
		return created
	}
	setupCommandHandler = func(ch *commands.CommandHandler) error {
		setupCommandsCalls++
		return nil
	}
	setupQOTDInteractionHandler = func(ch *commands.CommandHandler) error {
		setupQOTDCalls++
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
		qotdReplyService:     noopAppAnswerSubmissionService{},
	})
	if err != nil {
		t.Fatalf("initialize bot runtime: %v", err)
	}

	if setupCommandsCalls != 0 {
		t.Fatalf("expected slash command setup to be skipped, got %d calls", setupCommandsCalls)
	}
	if setupQOTDCalls != 1 {
		t.Fatalf("expected qotd interaction setup once, got %d calls", setupQOTDCalls)
	}
	if runtime.commandHandler == nil || runtime.commandHandler != created {
		t.Fatal("expected qotd-only runtime to retain the created command handler for shutdown")
	}
}
