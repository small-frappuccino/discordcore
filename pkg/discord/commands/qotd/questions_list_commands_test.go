package qotd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

type interactionRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
}

func (r *interactionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *interactionRecorder) lastResponse(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

func newQOTDCommandTestSession(t *testing.T) (*discordgo.Session, *interactionRecorder) {
	t.Helper()

	rec := &interactionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.Contains(req.URL.Path, "/callback") {
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	discordgo.EndpointAPI = server.URL + "/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create discord session: %v", err)
	}
	if session.State == nil {
		t.Fatal("expected session state to be initialized")
	}
	return session, rec
}

func newQOTDCommandTestRouter(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
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

	service := applicationqotd.NewService(cm, store, nil)
	router := core.NewCommandRouter(session, cm)
	NewCommands(service).RegisterCommands(router)
	return router, cm, service, store
}

func newQOTDSlashInteraction(
	guildID string,
	userID string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-questions-list",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: groupName,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name: questionsGroupName,
					Type: discordgo.ApplicationCommandOptionSubCommandGroup,
					Options: []*discordgo.ApplicationCommandInteractionDataOption{{
						Name:    questionsListSubCommand,
						Type:    discordgo.ApplicationCommandOptionSubCommand,
						Options: options,
					}},
				}},
			},
		},
	}
}

func newQOTDComponentInteraction(guildID, userID, customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-questions-list-component",
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionMessageComponent,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.MessageComponentInteractionData{
				CustomID: customID,
			},
		},
	}
}

func qotdStringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
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

func requireEphemeralResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func TestQuestionsListCommandUsesRequestedDeck(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{
			{ID: files.LegacyQOTDDefaultDeckID, Name: files.LegacyQOTDDefaultDeckName},
			{ID: "spicy", Name: "Spicy"},
		},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Default deck question", applicationqotd.QuestionStatusReady)
	mustCreateQuestion(t, service, guildID, ownerID, "spicy", "Spicy deck question", applicationqotd.QuestionStatusDraft)

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdStringOpt(questionsDeckOptionName, "Spicy"),
	}))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if len(resp.Data.Embeds) != 1 {
		t.Fatalf("expected one embed, got %+v", resp.Data.Embeds)
	}
	embed := resp.Data.Embeds[0]
	if embed.Title != "☆ questions list! ☆" {
		t.Fatalf("unexpected embed title: %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "Spicy deck question") {
		t.Fatalf("expected selected deck question in description, got %q", embed.Description)
	}
	if strings.Contains(embed.Description, "Default deck question") {
		t.Fatalf("expected response to exclude active deck question, got %q", embed.Description)
	}
	if embed.Footer == nil || !strings.Contains(embed.Footer.Text, "Spicy") {
		t.Fatalf("expected spicy deck footer, got %+v", embed.Footer)
	}
	if len(resp.Data.Components) != 1 {
		t.Fatalf("expected one component row, got %+v", resp.Data.Components)
	}
}

func TestQuestionsListCommandPaginatesWithButtons(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})
	for idx := 1; idx <= 12; idx++ {
		mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, fmt.Sprintf("Question number %02d", idx), applicationqotd.QuestionStatusReady)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, nil))
	firstResp := rec.lastResponse(t)
	requireEphemeralResponse(t, firstResp)
	if !strings.Contains(firstResp.Data.Embeds[0].Description, "Question number 01") {
		t.Fatalf("expected first page to contain first question, got %q", firstResp.Data.Embeds[0].Description)
	}
	if strings.Contains(firstResp.Data.Embeds[0].Description, "Question number 11") {
		t.Fatalf("expected first page to exclude second page content, got %q", firstResp.Data.Embeds[0].Description)
	}

	nextCustomID := encodeQuestionsListState(questionsListRouteNext, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   0,
	})
	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, ownerID, nextCustomID))

	secondResp := rec.lastResponse(t)
	if secondResp.Type != discordgo.InteractionResponseUpdateMessage {
		t.Fatalf("expected update message response, got %+v", secondResp.Type)
	}
	if !strings.Contains(secondResp.Data.Embeds[0].Description, "Question number 11") {
		t.Fatalf("expected second page to contain later questions, got %q", secondResp.Data.Embeds[0].Description)
	}
	if strings.Contains(secondResp.Data.Embeds[0].Description, "Question number 01") {
		t.Fatalf("expected second page to exclude first page content, got %q", secondResp.Data.Embeds[0].Description)
	}
	if secondResp.Data.Embeds[0].Footer == nil || !strings.Contains(secondResp.Data.Embeds[0].Footer.Text, "Page 2/2") {
		t.Fatalf("expected second page footer, got %+v", secondResp.Data.Embeds[0].Footer)
	}
}

func TestQuestionsListComponentRejectsDifferentUser(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
		otherID = "other-user"
	)

	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouter(t, session, guildID, ownerID)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:   files.LegacyQOTDDefaultDeckID,
			Name: files.LegacyQOTDDefaultDeckName,
		}},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question number 01", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDComponentInteraction(guildID, otherID, encodeQuestionsListState(questionsListRouteNext, questionsListState{
		UserID: ownerID,
		DeckID: files.LegacyQOTDDefaultDeckID,
		Page:   0,
	})))

	resp := rec.lastResponse(t)
	requireEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, questionsListDeniedText) {
		t.Fatalf("expected denied interaction message, got %q", resp.Data.Content)
	}
}