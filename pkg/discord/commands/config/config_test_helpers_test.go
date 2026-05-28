package config

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type configCommandTestHarness struct {
	session *discordgo.Session
	rec     *interactionRecorder
	router  *core.CommandRouter
	cm      *files.ConfigManager
	guildID string
	ownerID string
}

func newConfigCommandTestHarness(t *testing.T, guildID, ownerID string) *configCommandTestHarness {
	t.Helper()

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouterWithClock(t, session, guildID, ownerID, nil)
	return &configCommandTestHarness{
		session: session,
		rec:     rec,
		router:  router,
		cm:      cm,
		guildID: guildID,
		ownerID: ownerID,
	}
}

func newConfigCommandTestHarnessWithClock(t *testing.T, guildID, ownerID string, now func() time.Time) *configCommandTestHarness {
	t.Helper()

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouterWithClock(t, session, guildID, ownerID, now)
	return &configCommandTestHarness{
		session: session,
		rec:     rec,
		router:  router,
		cm:      cm,
		guildID: guildID,
		ownerID: ownerID,
	}
}

func (h *configCommandTestHarness) runSlash(
	t *testing.T,
	subCommand string,
	options ...*discordgo.ApplicationCommandInteractionDataOption,
) discordgo.InteractionResponse {
	t.Helper()

	h.router.HandleInteraction(h.session, newConfigSlashInteraction(h.guildID, h.ownerID, subCommand, options))
	return h.rec.lastResponse(t)
}

func mustUpdateConfig(t *testing.T, cm *files.ConfigManager, fn func(*files.BotConfig)) {
	t.Helper()

	_, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		if fn != nil {
			fn(cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("update config: %v", err)
	}
}

func mustSetGuildQOTDConfig(t *testing.T, cm *files.ConfigManager, guildID string, cfg files.QOTDConfig) {
	t.Helper()

	mustUpdateConfig(t, cm, func(config *files.BotConfig) {
		for idx := range config.Guilds {
			if config.Guilds[idx].GuildID != guildID {
				continue
			}
			config.Guilds[idx].QOTD = cfg
		}
	})
}

func buildTestQOTDConfig(enabled bool, channelID string, schedule files.QOTDPublishScheduleConfig) files.QOTDConfig {
	return files.QOTDConfig{
		ActiveDeckID: files.LegacyQOTDDefaultDeckID,
		Schedule:     schedule,
		Decks: []files.QOTDDeckConfig{{
			ID:        files.LegacyQOTDDefaultDeckID,
			Name:      files.LegacyQOTDDefaultDeckName,
			Enabled:   enabled,
			ChannelID: channelID,
		}},
	}
}

func testCommandSchedule() files.QOTDPublishScheduleConfig {
	hourUTC := 12
	minuteUTC := 43
	return files.QOTDPublishScheduleConfig{
		HourUTC:   &hourUTC,
		MinuteUTC: &minuteUTC,
	}
}

func assertQOTDSuppressionDate(t *testing.T, cm *files.ConfigManager, guildID, want string) {
	t.Helper()

	qotdConfig, err := cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	got := qotdConfig.SuppressScheduledPublishDatesUTC
	if want == "" {
		if len(got) != 0 {
			t.Fatalf("unexpected qotd suppression dates: got %+v want empty", got)
		}
		return
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("unexpected qotd suppression dates: got %+v want [%q]", got, want)
	}
}

func boolOpt(name string, value bool) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: value,
	}
}

func channelOpt(name, channelID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionChannel,
		Value: channelID,
	}
}

func intOpt(name string, value int64) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionInteger,
		Value: float64(value),
	}
}

func assertPublicContains(t *testing.T, resp discordgo.InteractionResponse, want string) {
	t.Helper()

	assertPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, want) {
		t.Fatalf("expected public response to contain %q, got %q", want, resp.Data.Content)
	}
}

func assertPublicResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()

	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func assertEphemeralResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()

	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func assertEphemeralContains(t *testing.T, resp discordgo.InteractionResponse, want string) {
	t.Helper()

	assertEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, want) {
		t.Fatalf("expected ephemeral response to contain %q, got %q", want, resp.Data.Content)
	}
}

func assertActiveQOTDDeckState(
	t *testing.T,
	cm *files.ConfigManager,
	guildID string,
	wantChannel string,
	wantEnabled bool,
	wantSchedule files.QOTDPublishScheduleConfig,
) {
	t.Helper()

	qotdConfig, err := cm.QOTDConfig(guildID)
	if err != nil {
		t.Fatalf("QOTDConfig() failed: %v", err)
	}
	deck, ok := qotdConfig.ActiveDeck()
	if !ok {
		t.Fatalf("expected active deck after update: %+v", qotdConfig)
	}
	if deck.ChannelID != wantChannel || deck.Enabled != wantEnabled {
		t.Fatalf("unexpected active deck state: got channel=%q enabled=%v want channel=%q enabled=%v", deck.ChannelID, deck.Enabled, wantChannel, wantEnabled)
	}
	if !testSchedulesEqual(qotdConfig.Schedule, wantSchedule) {
		t.Fatalf("unexpected qotd schedule: got %+v want %+v", qotdConfig.Schedule, wantSchedule)
	}
}

func testSchedulesEqual(left, right files.QOTDPublishScheduleConfig) bool {
	if (left.HourUTC == nil) != (right.HourUTC == nil) {
		return false
	}
	if left.HourUTC != nil && *left.HourUTC != *right.HourUTC {
		return false
	}
	if (left.MinuteUTC == nil) != (right.MinuteUTC == nil) {
		return false
	}
	if left.MinuteUTC != nil && *left.MinuteUTC != *right.MinuteUTC {
		return false
	}
	return true
}

type interactionRecorder struct {
	mu                 sync.Mutex
	responses          []discordgo.InteractionResponse
	webhookPatchCalls  int
	webhookLookupCalls int
	messageLookupCalls int
	lastWebhookReqPath string
	lastMessageReqPath string
	lastWebhookGetPath string
}

func (r *interactionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *interactionRecorder) recordWebhookPatch(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookPatchCalls++
	r.lastWebhookReqPath = path
}

func (r *interactionRecorder) recordWebhookLookup(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookLookupCalls++
	r.lastWebhookGetPath = path
}

func (r *interactionRecorder) recordMessageLookup(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.messageLookupCalls++
	r.lastMessageReqPath = path
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

func (r *interactionRecorder) webhookPatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookPatchCalls
}

func (r *interactionRecorder) webhookPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastWebhookReqPath
}

func (r *interactionRecorder) webhookLookupCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookLookupCalls
}

func (r *interactionRecorder) messageLookupCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.messageLookupCalls
}

type webhookEndpointBehavior struct {
	patchStatus         int
	webhookLookupStatus int
	messageLookupStatus int
}

func (b webhookEndpointBehavior) normalized() webhookEndpointBehavior {
	out := b
	if out.patchStatus == 0 {
		out.patchStatus = http.StatusOK
	}
	if out.webhookLookupStatus == 0 {
		out.webhookLookupStatus = http.StatusOK
	}
	if out.messageLookupStatus == 0 {
		out.messageLookupStatus = http.StatusOK
	}
	return out
}

func newConfigCommandTestSession(t *testing.T) (*discordgo.Session, *interactionRecorder) {
	return newConfigCommandTestSessionWithWebhookBehavior(t, webhookEndpointBehavior{})
}

func newConfigCommandTestSessionWithWebhookPatchStatus(t *testing.T, webhookPatchStatus int) (*discordgo.Session, *interactionRecorder) {
	return newConfigCommandTestSessionWithWebhookBehavior(t, webhookEndpointBehavior{
		patchStatus: webhookPatchStatus,
	})
}

func newConfigCommandTestSessionWithWebhookValidationStatus(
	t *testing.T,
	webhookLookupStatus, messageLookupStatus int,
) (*discordgo.Session, *interactionRecorder) {
	return newConfigCommandTestSessionWithWebhookBehavior(t, webhookEndpointBehavior{
		webhookLookupStatus: webhookLookupStatus,
		messageLookupStatus: messageLookupStatus,
	})
}

func newConfigCommandTestSessionWithWebhookBehavior(
	t *testing.T,
	behavior webhookEndpointBehavior,
) (*discordgo.Session, *interactionRecorder) {
	t.Helper()

	behavior = behavior.normalized()
	rec := &interactionRecorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case strings.Contains(req.URL.Path, "/callback"):
			var resp discordgo.InteractionResponse
			_ = json.NewDecoder(req.Body).Decode(&resp)
			rec.addResponse(resp)
			w.WriteHeader(http.StatusOK)
			return

		case strings.Contains(req.URL.Path, "/webhooks/") && req.Method == http.MethodPatch:
			rec.recordWebhookPatch(req.URL.Path)
			if behavior.patchStatus != http.StatusOK {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(behavior.patchStatus)
				_, _ = w.Write([]byte(`{"message":"forced patch failure"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"message-edited"}`))
			return

		case strings.Contains(req.URL.Path, "/webhooks/") &&
			req.Method == http.MethodGet &&
			strings.Contains(req.URL.Path, "/messages/"):
			rec.recordMessageLookup(req.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			if behavior.messageLookupStatus != http.StatusOK {
				w.WriteHeader(behavior.messageLookupStatus)
				_, _ = w.Write([]byte(`{"message":"forced message lookup failure"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"message-target","channel_id":"1","content":""}`))
			return

		case strings.Contains(req.URL.Path, "/webhooks/") && req.Method == http.MethodGet:
			rec.recordWebhookLookup(req.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			if behavior.webhookLookupStatus != http.StatusOK {
				w.WriteHeader(behavior.webhookLookupStatus)
				_, _ = w.Write([]byte(`{"message":"forced webhook lookup failure"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"123","type":1,"name":"test","token":"test-token","channel_id":"1","guild_id":"1"}`))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
	})

	session, err := discordgo.New("Bot test-token")
	if err != nil {
		t.Fatalf("failed to create discord session: %v", err)
	}
	if session.State == nil {
		t.Fatalf("expected session state to be initialized")
	}

	return session, rec
}

func newConfigCommandTestRouter(t *testing.T, session *discordgo.Session, guildID, ownerID string) (*core.CommandRouter, *files.ConfigManager) {
	return newConfigCommandTestRouterWithClock(t, session, guildID, ownerID, nil)
}

func newConfigCommandTestRouterWithClock(t *testing.T, session *discordgo.Session, guildID, ownerID string, now func() time.Time) (*core.CommandRouter, *files.ConfigManager) {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}

	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewConfigCommandsWithClock(cm, now).RegisterCommands(router)
	return router, cm
}

func newConfigSlashInteraction(guildID, userID, subCommand string, options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "config",
				Options: []*discordgo.ApplicationCommandInteractionDataOption{
					{
						Name:    subCommand,
						Type:    discordgo.ApplicationCommandOptionSubCommand,
						Options: options,
					},
				},
			},
		},
	}
}
