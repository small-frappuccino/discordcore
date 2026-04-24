package partner

import (
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
)

type partnerInteractionRecorder struct {
	mu                 sync.Mutex
	responses          []discordgo.InteractionResponse
	webhookPatchCalls  int
	channelPatchCalls  int
	lastWebhookReqPath string
	lastChannelReqPath string
}

func (r *partnerInteractionRecorder) addResponse(resp discordgo.InteractionResponse) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.responses = append(r.responses, resp)
}

func (r *partnerInteractionRecorder) lastResponse(t *testing.T) discordgo.InteractionResponse {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.responses) == 0 {
		t.Fatal("expected at least one response")
	}
	return r.responses[len(r.responses)-1]
}

func (r *partnerInteractionRecorder) recordWebhookPatch(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.webhookPatchCalls++
	r.lastWebhookReqPath = path
}

func (r *partnerInteractionRecorder) recordChannelPatch(path string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channelPatchCalls++
	r.lastChannelReqPath = path
}

func (r *partnerInteractionRecorder) webhookPatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.webhookPatchCalls
}

func (r *partnerInteractionRecorder) channelPatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.channelPatchCalls
}

func (r *partnerInteractionRecorder) webhookPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastWebhookReqPath
}

func (r *partnerInteractionRecorder) channelPath() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastChannelReqPath
}

type partnerPatchBehavior struct {
	webhookPatchStatus int
	channelPatchStatus int
}

func (b partnerPatchBehavior) normalized() partnerPatchBehavior {
	out := b
	if out.webhookPatchStatus == 0 {
		out.webhookPatchStatus = http.StatusOK
	}
	if out.channelPatchStatus == 0 {
		out.channelPatchStatus = http.StatusOK
	}
	return out
}

func newPartnerCommandTestSession(t *testing.T) (*discordgo.Session, *partnerInteractionRecorder) {
	return newPartnerCommandTestSessionWithBehavior(t, partnerPatchBehavior{})
}

func newPartnerCommandTestSessionWithBehavior(
	t *testing.T,
	behavior partnerPatchBehavior,
) (*discordgo.Session, *partnerInteractionRecorder) {
	t.Helper()

	behavior = behavior.normalized()
	rec := &partnerInteractionRecorder{}
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
			w.Header().Set("Content-Type", "application/json")
			if behavior.webhookPatchStatus != http.StatusOK {
				w.WriteHeader(behavior.webhookPatchStatus)
				_, _ = w.Write([]byte(`{"message":"forced webhook patch failure"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"webhook-edited"}`))
			return

		case strings.Contains(req.URL.Path, "/channels/") && req.Method == http.MethodPatch:
			rec.recordChannelPatch(req.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			if behavior.channelPatchStatus != http.StatusOK {
				w.WriteHeader(behavior.channelPatchStatus)
				_, _ = w.Write([]byte(`{"message":"forced channel patch failure"}`))
				return
			}
			_, _ = w.Write([]byte(`{"id":"channel-edited"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"ok"}`))
	}))
	t.Cleanup(server.Close)

	oldAPI := discordgo.EndpointAPI
	oldWebhooks := discordgo.EndpointWebhooks
	oldChannels := discordgo.EndpointChannels
	discordgo.EndpointAPI = server.URL + "/"
	discordgo.EndpointWebhooks = server.URL + "/webhooks/"
	discordgo.EndpointChannels = server.URL + "/channels/"
	t.Cleanup(func() {
		discordgo.EndpointAPI = oldAPI
		discordgo.EndpointWebhooks = oldWebhooks
		discordgo.EndpointChannels = oldChannels
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

func newPartnerCommandTestRouter(
	t *testing.T,
	session *discordgo.Session,
	guildID string,
	ownerID string,
) (*core.CommandRouter, *files.ConfigManager) {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}
	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewPartnerCommands(cm).RegisterCommands(router)
	return router, cm
}

func newPartnerSlashInteraction(
	guildID string,
	userID string,
	subCommand string,
	options []*discordgo.ApplicationCommandInteractionDataOption,
) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{
		Interaction: &discordgo.Interaction{
			ID:      "interaction-" + subCommand,
			AppID:   "app",
			Token:   "token",
			Type:    discordgo.InteractionApplicationCommand,
			GuildID: guildID,
			Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
			Data: discordgo.ApplicationCommandInteractionData{
				Name: "partner",
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

func partnerStringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func partnerEphemeralError(resp discordgo.InteractionResponse) error {
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		return fmt.Errorf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
	return nil
}

func TestPartnerCommandsCRUDInteractions(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newPartnerCommandTestSession(t)
	router, cm := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	// ADD
	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionFandom, "Genshin Impact"),
		partnerStringOpt(optionName, "Citlali Mains"),
		partnerStringOpt(optionLink, "discord.gg/citlali"),
	}))
	addResp := rec.lastResponse(t)
	if err := partnerEphemeralError(addResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(addResp.Data.Content, "Partner added") {
		t.Fatalf("unexpected add response: %q", addResp.Data.Content)
	}

	created, err := cm.Partner(guildID, "Citlali Mains")
	if err != nil {
		t.Fatalf("expected created partner: %v", err)
	}
	if created.Link != "https://discord.gg/citlali" {
		t.Fatalf("unexpected canonical link after add: %+v", created)
	}

	// READ
	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "read", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionName, "Citlali Mains"),
	}))
	readResp := rec.lastResponse(t)
	if err := partnerEphemeralError(readResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readResp.Data.Content, "Partner details") {
		t.Fatalf("unexpected read response: %q", readResp.Data.Content)
	}
	if !strings.Contains(readResp.Data.Content, "https://discord.gg/citlali") {
		t.Fatalf("unexpected read link output: %q", readResp.Data.Content)
	}

	// UPDATE
	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "update", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionCurrentName, "Citlali Mains"),
		partnerStringOpt(optionFandom, "Genshin Impact"),
		partnerStringOpt(optionName, "Citlali Hub"),
		partnerStringOpt(optionLink, "https://discord.gg/citlalihub"),
	}))
	updateResp := rec.lastResponse(t)
	if err := partnerEphemeralError(updateResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updateResp.Data.Content, "Partner updated") {
		t.Fatalf("unexpected update response: %q", updateResp.Data.Content)
	}
	updated, err := cm.Partner(guildID, "Citlali Hub")
	if err != nil {
		t.Fatalf("expected updated partner: %v", err)
	}
	if updated.Link != "https://discord.gg/citlalihub" {
		t.Fatalf("unexpected updated link: %+v", updated)
	}

	// LIST
	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "list", nil))
	listResp := rec.lastResponse(t)
	if err := partnerEphemeralError(listResp); err != nil {
		t.Fatal(err)
	}
	if len(listResp.Data.Embeds) == 0 {
		t.Fatalf("expected list embed response, got none: %+v", listResp.Data)
	}
	if !strings.Contains(listResp.Data.Embeds[0].Description, "Citlali Hub") {
		t.Fatalf("unexpected list response embed: %+v", listResp.Data.Embeds[0])
	}

	// DELETE
	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "delete", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionName, "Citlali Hub"),
	}))
	deleteResp := rec.lastResponse(t)
	if err := partnerEphemeralError(deleteResp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deleteResp.Data.Content, "deleted") {
		t.Fatalf("unexpected delete response: %q", deleteResp.Data.Content)
	}
	if _, err := cm.Partner(guildID, "Citlali Hub"); err == nil {
		t.Fatalf("expected deleted partner to be missing")
	}
}

func TestPartnerCommandsDuplicateValidation(t *testing.T) {
	const (
		guildID = "guild-2"
		ownerID = "owner-2"
	)

	session, rec := newPartnerCommandTestSession(t)
	router, _ := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionFandom, "ZZZ"),
		partnerStringOpt(optionName, "Jane Mains"),
		partnerStringOpt(optionLink, "https://discord.gg/jane"),
	}))
	firstAdd := rec.lastResponse(t)
	if err := partnerEphemeralError(firstAdd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(firstAdd.Data.Content, "Partner added") {
		t.Fatalf("unexpected first add response: %q", firstAdd.Data.Content)
	}

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "add", []*discordgo.ApplicationCommandInteractionDataOption{
		partnerStringOpt(optionFandom, "ZZZ"),
		partnerStringOpt(optionName, "jane mains"),
		partnerStringOpt(optionLink, "https://discord.gg/jane2"),
	}))
	secondAdd := rec.lastResponse(t)
	if err := partnerEphemeralError(secondAdd); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(secondAdd.Data.Content), "already exists") {
		t.Fatalf("expected duplicate error response, got: %q", secondAdd.Data.Content)
	}
}

func TestPartnerSyncCommandWebhookTargetSuccess(t *testing.T) {
	const (
		guildID = "guild-sync-webhook-ok"
		ownerID = "owner-sync-webhook-ok"
	)

	session, rec := newPartnerCommandTestSessionWithBehavior(t, partnerPatchBehavior{
		webhookPatchStatus: http.StatusOK,
	})
	router, cm := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.SetPartnerBoardTarget(guildID, files.EmbedUpdateTargetConfig{
		Type:       files.EmbedUpdateTargetTypeWebhookMessage,
		MessageID:  "123456789012345678",
		WebhookURL: "https://discord.com/api/webhooks/123/token-abc",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := cm.CreatePartner(guildID, files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Columbina Mains",
		Link:   "discord.gg/columbina",
	}); err != nil {
		t.Fatalf("create partner seed: %v", err)
	}

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "sync", nil))
	resp := rec.lastResponse(t)
	if err := partnerEphemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(resp.Data.Content), "synced successfully") {
		t.Fatalf("unexpected sync response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch, got %d", rec.webhookPatchCount())
	}
	if !strings.Contains(rec.webhookPath(), "/webhooks/123/token-abc/messages/123456789012345678") {
		t.Fatalf("unexpected webhook patch path: %q", rec.webhookPath())
	}
	if rec.channelPatchCount() != 0 {
		t.Fatalf("expected zero channel patch calls, got %d", rec.channelPatchCount())
	}
}

func TestPartnerSyncCommandWebhookTargetFailure(t *testing.T) {
	const (
		guildID = "guild-sync-webhook-fail"
		ownerID = "owner-sync-webhook-fail"
	)

	session, rec := newPartnerCommandTestSessionWithBehavior(t, partnerPatchBehavior{
		webhookPatchStatus: http.StatusInternalServerError,
	})
	router, cm := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.SetPartnerBoardTarget(guildID, files.EmbedUpdateTargetConfig{
		Type:       files.EmbedUpdateTargetTypeWebhookMessage,
		MessageID:  "223456789012345678",
		WebhookURL: "https://discord.com/api/webhooks/223/token-fail",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := cm.CreatePartner(guildID, files.PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "discord.gg/jane",
	}); err != nil {
		t.Fatalf("create partner seed: %v", err)
	}

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "sync", nil))
	resp := rec.lastResponse(t)
	if err := partnerEphemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(resp.Data.Content), "failed to sync") {
		t.Fatalf("unexpected sync failure response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch attempt, got %d", rec.webhookPatchCount())
	}
}

func TestPartnerSyncCommandChannelTargetSuccess(t *testing.T) {
	const (
		guildID = "guild-sync-channel-ok"
		ownerID = "owner-sync-channel-ok"
	)

	session, rec := newPartnerCommandTestSessionWithBehavior(t, partnerPatchBehavior{
		channelPatchStatus: http.StatusOK,
	})
	router, cm := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.SetPartnerBoardTarget(guildID, files.EmbedUpdateTargetConfig{
		Type:      files.EmbedUpdateTargetTypeChannelMessage,
		MessageID: "323456789012345678",
		ChannelID: "423456789012345678",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := cm.CreatePartner(guildID, files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	}); err != nil {
		t.Fatalf("create partner seed: %v", err)
	}

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "sync", nil))
	resp := rec.lastResponse(t)
	if err := partnerEphemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(resp.Data.Content), "synced successfully") {
		t.Fatalf("unexpected sync response: %q", resp.Data.Content)
	}
	if rec.channelPatchCount() != 1 {
		t.Fatalf("expected one channel patch, got %d", rec.channelPatchCount())
	}
	if !strings.Contains(rec.channelPath(), "/channels/423456789012345678/messages/323456789012345678") {
		t.Fatalf("unexpected channel patch path: %q", rec.channelPath())
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected zero webhook patch calls, got %d", rec.webhookPatchCount())
	}
}

func TestPartnerSyncCommandChannelTargetFailure(t *testing.T) {
	const (
		guildID = "guild-sync-channel-fail"
		ownerID = "owner-sync-channel-fail"
	)

	session, rec := newPartnerCommandTestSessionWithBehavior(t, partnerPatchBehavior{
		channelPatchStatus: http.StatusInternalServerError,
	})
	router, cm := newPartnerCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.SetPartnerBoardTarget(guildID, files.EmbedUpdateTargetConfig{
		Type:      files.EmbedUpdateTargetTypeChannelMessage,
		MessageID: "523456789012345678",
		ChannelID: "623456789012345678",
	}); err != nil {
		t.Fatalf("set target: %v", err)
	}
	if err := cm.CreatePartner(guildID, files.PartnerEntryConfig{
		Fandom: "ZZZ",
		Name:   "Jane Mains",
		Link:   "discord.gg/jane",
	}); err != nil {
		t.Fatalf("create partner seed: %v", err)
	}

	router.HandleInteraction(session, newPartnerSlashInteraction(guildID, ownerID, "sync", nil))
	resp := rec.lastResponse(t)
	if err := partnerEphemeralError(resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToLower(resp.Data.Content), "failed to sync") {
		t.Fatalf("unexpected sync failure response: %q", resp.Data.Content)
	}
	if rec.channelPatchCount() != 1 {
		t.Fatalf("expected one channel patch attempt, got %d", rec.channelPatchCount())
	}
}
