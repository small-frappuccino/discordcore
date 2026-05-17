package moderation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

type moderationInteractionRecorder struct {
	mu        sync.Mutex
	responses []discordgo.InteractionResponse
}

func (r *moderationInteractionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *moderationInteractionRecorder) lastResponse(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one interaction response")
	}
	return r.responses[len(r.responses)-1]
}

type moderationCommandTestHarness struct {
	session *discordgo.Session
	rec     *moderationInteractionRecorder
	router  *core.CommandRouter
	cm      *files.ConfigManager
	guildID string
	ownerID string
}

func newModerationCommandTestHarness(t *testing.T, guildID, ownerID string) *moderationCommandTestHarness {
	t.Helper()

	session, rec := newModerationCommandTestSession(t)
	router, cm := newModerationCommandTestRouter(t, session, guildID, ownerID)
	return &moderationCommandTestHarness{
		session: session,
		rec:     rec,
		router:  router,
		cm:      cm,
		guildID: guildID,
		ownerID: ownerID,
	}
}

func (h *moderationCommandTestHarness) runSlash(
	t *testing.T,
	commandName string,
	options ...*discordgo.ApplicationCommandInteractionDataOption,
) discordgo.InteractionResponse {
	t.Helper()

	h.router.HandleInteraction(h.session, newModerationSlashInteraction(h.guildID, h.ownerID, commandName, options))
	return h.rec.lastResponse(t)
}

func newModerationCommandTestSession(t *testing.T) (*discordgo.Session, *moderationInteractionRecorder) {
	t.Helper()

	rec := &moderationInteractionRecorder{}
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
	return session, rec
}

func newModerationCommandTestRouter(t *testing.T, session *discordgo.Session, guildID, ownerID string) (*core.CommandRouter, *files.ConfigManager) {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	RegisterModerationCommands(router)
	return router, cm
}

func newModerationSlashInteraction(
	guildID, userID, commandName string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + commandName,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name:    commandName,
				Options: options,
			},
		},
	}
}

func moderationStringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func moderationUserOpt(name, userID string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionUser,
		Value: userID,
	}
}

func assertModerationPublicResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral != 0 {
		t.Fatalf("expected public response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func assertModerationEphemeralResponse(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func assertModerationPublicContains(t *testing.T, resp discordgo.InteractionResponse, want string) {
	t.Helper()
	assertModerationPublicResponse(t, resp)
	if !strings.Contains(resp.Data.Content, want) {
		t.Fatalf("expected public response to contain %q, got %q", want, resp.Data.Content)
	}
}

func assertModerationEphemeralContains(t *testing.T, resp discordgo.InteractionResponse, want string) {
	t.Helper()
	assertModerationEphemeralResponse(t, resp)
	if !strings.Contains(resp.Data.Content, want) {
		t.Fatalf("expected ephemeral response to contain %q, got %q", want, resp.Data.Content)
	}
}
