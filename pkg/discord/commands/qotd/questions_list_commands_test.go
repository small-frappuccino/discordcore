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
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	discordqotd "github.com/small-frappuccino/discordcore/pkg/discord/qotd"
	"github.com/small-frappuccino/discordcore/pkg/files"
	applicationqotd "github.com/small-frappuccino/discordcore/pkg/qotd"
	"github.com/small-frappuccino/discordcore/pkg/storage"
	"github.com/small-frappuccino/discordcore/pkg/testdb"
)

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

func TestQuestionsListIdleTimeoutResetsOnActivity(t *testing.T) {
	fired := make(chan struct{}, 2)
	command := &questionsListCommand{
		idleTimeout: 80 * time.Millisecond,
		editComponents: func(_ *discordgo.Session, channelID, messageID string, components []discordgo.MessageComponent) error {
			if channelID != "channel-1" || messageID != "message-1" {
				t.Fatalf("unexpected message target: channel=%q message=%q", channelID, messageID)
			}
			if len(components) != 0 {
				t.Fatalf("expected controls to be cleared, got %+v", components)
			}
			fired <- struct{}{}
			return nil
		},
	}

	command.armQuestionsListIdleTimeout(&discordgo.Session{}, "channel-1", "message-1")
	time.Sleep(40 * time.Millisecond)
	command.armQuestionsListIdleTimeout(&discordgo.Session{}, "channel-1", "message-1")

	select {
	case <-fired:
		t.Fatal("expected renewed activity to keep controls visible before the new timeout expires")
	case <-time.After(55 * time.Millisecond):
	}

	select {
	case <-fired:
	case <-time.After(400 * time.Millisecond):
		t.Fatal("expected idle timeout to hide controls after inactivity")
	}

	select {
	case <-fired:
		t.Fatal("expected controls to be hidden only once for the same message")
	case <-time.After(40 * time.Millisecond):
	}
}

func newQOTDCommandTestRouter(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
) (*core.CommandRouter, *files.ConfigManager, *applicationqotd.Service, *storage.Store) {
	return newQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, nil)
}

func newQOTDCommandTestRouterWithPublisher(
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

func newQOTDSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-questions-" + subCommand,
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
						Name:    subCommand,
						Type:    discordgo.ApplicationCommandOptionSubCommand,
						Options: options,
					}},
				}},
			},
		},
	}
}

func newQOTDRootSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-qotd-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: groupName,
				Options: []*discordgo.ApplicationCommandInteractionDataOption{{
					Name:    subCommand,
					Type:    discordgo.ApplicationCommandOptionSubCommand,
					Options: options,
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

func requireEphemeralResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func requirePublicResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
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

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdStringOpt(questionsDeckOptionName, "Spicy"),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
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
	if !strings.Contains(embed.Description, "ID:") {
		t.Fatalf("expected question ID in embed description, got %q", embed.Description)
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
	row, ok := resp.Data.Components[0].(discordgo.ActionsRow)
	if !ok || len(row.Components) < 2 {
		t.Fatalf("expected action row buttons, got %+v", resp.Data.Components)
	}
	prevButton, ok := row.Components[1].(discordgo.Button)
	if !ok || prevButton.Style != discordgo.PrimaryButton {
		t.Fatalf("expected previous button to use primary style, got %+v", row.Components[1])
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

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	firstResp := rec.lastResponse(t)
	requirePublicResponse(t, firstResp)
	if !strings.Contains(firstResp.Data.Embeds[0].Description, "Question number 01") {
		t.Fatalf("expected first page to contain first question, got %q", firstResp.Data.Embeds[0].Description)
	}
	if !strings.Contains(firstResp.Data.Embeds[0].Description, "ID:") {
		t.Fatalf("expected first page to include question IDs, got %q", firstResp.Data.Embeds[0].Description)
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

func TestQuestionsAddCommandCreatesQuestionWithVisibleID(t *testing.T) {
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

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsAddSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdStringOpt(questionsBodyOptionName, "What is your favorite snack?"),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Added QOTD question ID") {
		t.Fatalf("expected add confirmation with ID, got %q", resp.Data.Content)
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after add: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question after add, got %d", len(questions))
	}
	if questions[0].Body != "What is your favorite snack?" {
		t.Fatalf("unexpected added question: %+v", questions[0])
	}
	if questions[0].DisplayID != 1 {
		t.Fatalf("expected added question to receive visible ID 1, got %+v", questions[0])
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, fmt.Sprintf("ID:%d", questions[0].DisplayID)) {
		t.Fatalf("expected list embed to expose created question ID, got %q", listResp.Data.Embeds[0].Description)
	}
}

func TestQuestionsRemoveCommandDeletesByID(t *testing.T) {
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
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Question to remove", applicationqotd.QuestionStatusReady)

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions before remove: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question before remove, got %d", len(questions))
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsRemoveSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, questions[0].DisplayID),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, fmt.Sprintf("Removed QOTD question ID %d", questions[0].DisplayID)) {
		t.Fatalf("expected remove confirmation with ID, got %q", resp.Data.Content)
	}

	remaining, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after remove: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("expected question removal to persist, got %+v", remaining)
	}
}

func TestQuestionsNextCommandSetsSelectedQuestionAsNextReady(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "channel-123",
		}},
	})

	created := make([]*storage.QOTDQuestionRecord, 0, 6)
	for idx := 1; idx <= 6; idx++ {
		question, err := service.CreateQuestion(context.Background(), guildID, ownerID, applicationqotd.QuestionMutation{
			DeckID: files.LegacyQOTDDefaultDeckID,
			Body:   fmt.Sprintf("Question %02d", idx),
			Status: applicationqotd.QuestionStatusReady,
		})
		if err != nil {
			t.Fatalf("CreateQuestion(%d) failed: %v", idx, err)
		}
		created = append(created, question)
	}

	for idx := 0; idx < 4; idx++ {
		if _, err := service.PublishNow(context.Background(), guildID, session); err != nil {
			t.Fatalf("PublishNow(%d) failed: %v", idx, err)
		}
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions before next command: %v", err)
	}
	selected := created[5]
	if questions[5].ID != selected.ID || questions[5].DisplayID != 6 {
		t.Fatalf("expected selected question to begin at visible ID 6, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsNextSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, 6),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "QOTD question ID 6 is now the next ready question") {
		t.Fatalf("expected next-question confirmation, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "ID 5") {
		t.Fatalf("expected next-question response to mention the new visible ID, got %q", resp.Data.Content)
	}

	updated, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after next command: %v", err)
	}
	if updated[4].ID != selected.ID || updated[4].DisplayID != 5 {
		t.Fatalf("expected selected question to become the next ready slot, got %+v", updated)
	}
	if updated[5].ID != created[4].ID || updated[5].DisplayID != 6 {
		t.Fatalf("expected the previous next question to shift back by one slot, got %+v", updated)
	}
}

func TestQuestionsNextCommandShowsSpecificErrorForUsedQuestion(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "channel-123",
		}},
	})

	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Already used", applicationqotd.QuestionStatusReady)
	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("ListQuestions() failed: %v", err)
	}
	if len(questions) != 1 {
		t.Fatalf("expected one question before publish, got %+v", questions)
	}
	created := questions[0]
	if _, err := service.PublishNow(context.Background(), guildID, session); err != nil {
		t.Fatalf("PublishNow() failed: %v", err)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsNextSubCommand, []*discordgo.ApplicationCommandInteractionDataOption{
		qotdIntOpt(questionsIDOptionName, created.DisplayID),
	}))

	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, fmt.Sprintf("QOTD question ID %d is already scheduled or used and cannot be set as next.", created.DisplayID)) {
		t.Fatalf("expected specific immutable-question error, got %q", resp.Data.Content)
	}
	if strings.Contains(resp.Data.Content, "An error occurred while executing the command") {
		t.Fatalf("expected command-specific error response, got generic fallback %q", resp.Data.Content)
	}
}

func TestQuestionsResetCommandResetsUsedQuestionsToReady(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "channel-1",
		}},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Reset me", applicationqotd.QuestionStatusReady)

	if _, err := service.PublishNow(context.Background(), guildID, session); err != nil {
		t.Fatalf("publish for reset setup: %v", err)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsResetSubCommand, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Reset 1 QOTD question states") {
		t.Fatalf("expected reset confirmation, got %q", resp.Data.Content)
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after reset: %v", err)
	}
	if len(questions) != 1 || questions[0].Status != string(applicationqotd.QuestionStatusReady) || questions[0].UsedAt != nil {
		t.Fatalf("expected question to be reset to ready, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, "✅") {
		t.Fatalf("expected reset question to show check icon, got %q", listResp.Data.Embeds[0].Description)
	}
}

func TestQOTDPublishCommandPublishesManually(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	fake := &fakePublisher{}
	session, rec := newQOTDCommandTestSession(t)
	router, cm, service, _ := newQOTDCommandTestRouterWithPublisher(t, session, guildID, ownerID, fake)
	mustConfigureQOTDDecks(t, cm, guildID, files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   true,
			ChannelID: "channel-123",
		}},
	})
	mustCreateQuestion(t, service, guildID, ownerID, files.LegacyQOTDDefaultDeckID, "Publish me", applicationqotd.QuestionStatusReady)

	router.HandleInteraction(session, newQOTDRootSlashInteraction(guildID, ownerID, publishSubCommandName, nil))
	resp := rec.lastResponse(t)
	requirePublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, "Published QOTD question ID 1 manually.") {
		t.Fatalf("expected publish confirmation, got %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "https://discord.com/channels/") {
		t.Fatalf("expected publish response to include jump url, got %q", resp.Data.Content)
	}
	if len(fake.publishedParams) != 1 {
		t.Fatalf("expected fake publisher to be invoked once, got %d", len(fake.publishedParams))
	}
	if fake.publishedParams[0].ThreadName != "Question of the Day" {
		t.Fatalf("expected manual publish to use the fixed thread title, got %+v", fake.publishedParams[0])
	}

	questions, err := service.ListQuestions(context.Background(), guildID, files.LegacyQOTDDefaultDeckID)
	if err != nil {
		t.Fatalf("list questions after manual publish: %v", err)
	}
	if len(questions) != 1 || questions[0].Status != string(applicationqotd.QuestionStatusUsed) || questions[0].UsedAt == nil {
		t.Fatalf("expected published question to be marked used, got %+v", questions)
	}

	router.HandleInteraction(session, newQOTDSlashInteraction(guildID, ownerID, questionsListSubCommand, nil))
	listResp := rec.lastResponse(t)
	requirePublicResponse(t, listResp)
	if !strings.Contains(listResp.Data.Embeds[0].Description, "🚫") {
		t.Fatalf("expected used question to show prohibited icon, got %q", listResp.Data.Embeds[0].Description)
	}
}
