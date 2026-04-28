//go:build integration

package qotd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

const integrationDeckChannelID = "123456789012345678"

type fakePublisher struct {
	publishedParams []discordqotd.PublishOfficialPostParams
	threadStates    map[string]discordqotd.ThreadState
}

func (p *fakePublisher) PublishOfficialPost(_ context.Context, _ *discordgo.Session, params discordqotd.PublishOfficialPostParams) (*discordqotd.PublishedOfficialPost, error) {
	p.publishedParams = append(p.publishedParams, params)
	publishedAt := time.Now().UTC()
	return &discordqotd.PublishedOfficialPost{
		QuestionListThreadID:       "questions-list-thread",
		QuestionListEntryMessageID: fmt.Sprintf("list-entry-%d", params.OfficialPostID),
		ThreadID:                   fmt.Sprintf("thread-%d", params.OfficialPostID),
		StarterMessageID:           fmt.Sprintf("message-%d", params.OfficialPostID),
		AnswerChannelID:            fmt.Sprintf("thread-%d", params.OfficialPostID),
		PublishedAt:                publishedAt,
		PostURL:                    discordqotd.BuildMessageJumpURL(params.GuildID, params.ChannelID, fmt.Sprintf("message-%d", params.OfficialPostID)),
	}, nil
}

func (p *fakePublisher) SetThreadState(_ context.Context, _ *discordgo.Session, threadID string, state discordqotd.ThreadState) error {
	if p.threadStates == nil {
		p.threadStates = make(map[string]discordqotd.ThreadState)
	}
	p.threadStates[threadID] = state
	return nil
}

func (p *fakePublisher) FetchThreadMessages(_ context.Context, _ *discordgo.Session, _ string) ([]discordqotd.ArchivedMessage, error) {
	return nil, nil
}

func (p *fakePublisher) FetchChannelMessages(_ context.Context, _ *discordgo.Session, _, _ string, _ int) ([]discordqotd.ArchivedMessage, error) {
	return nil, nil
}

func newIntegrationQOTDCommandTestRouter(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
) (*core.CommandRouter, *files.ConfigManager, *applicationqotd.Service, *storage.Store) {
	return newIntegrationQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, nil)
}

func newIntegrationQOTDCommandTestRouterWithPublisher(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
	publisher applicationqotd.Publisher,
) (*core.CommandRouter, *files.ConfigManager, *applicationqotd.Service, *storage.Store) {
	t.Helper()

	baseDSN, err := testdb.BaseDatabaseURLFromEnv()
	if err != nil {
		if testdb.IsDatabaseURLNotConfigured(err) {
			t.Skipf("skipping postgres integration test: %v", err)
		}
		t.Fatalf("resolve test database dsn: %v", err)
	}
	db, cleanup, err := testdb.OpenIsolatedDatabase(context.Background(), baseDSN)
	if err != nil {
		t.Fatalf("open isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if err := cleanup(); err != nil {
			t.Fatalf("cleanup isolated test database: %v", err)
		}
	})

	store := storage.NewStore(db)
	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	service := applicationqotd.NewService(cm, store, publisher)
	router := core.NewCommandRouter(session, cm)
	NewCommands(service).RegisterCommands(router)
	return router, cm, service, store
}

func qotdStringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func qotdIntOpt(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionInteger,
		Value: float64(value),
	}
}

func mustConfigureQOTDDecks(t *testing.T, cm *files.ConfigManager, guildID string, cfg files.QOTDConfig) {
	t.Helper()
	_, err := cm.UpdateConfig(func(botCfg *files.BotConfig) error {
		for idx := range botCfg.Guilds {
			if botCfg.Guilds[idx].GuildID != guildID {
				continue
			}
			botCfg.Guilds[idx].QOTD = cfg
			return nil
		}
		return fmt.Errorf("guild config not found: %s", guildID)
	})
	if err != nil {
		t.Fatalf("update qotd config: %v", err)
	}
}

func mustCreateQuestion(
	t *testing.T,
	service *applicationqotd.Service,
	guildID string,
	actorID string,
	deckID string,
	body string,
	status applicationqotd.QuestionStatus,
) {
	t.Helper()
	if _, err := service.CreateQuestion(context.Background(), guildID, actorID, applicationqotd.QuestionMutation{
		DeckID: deckID,
		Body:   body,
		Status: status,
	}); err != nil {
		t.Fatalf("create question: %v", err)
	}
}