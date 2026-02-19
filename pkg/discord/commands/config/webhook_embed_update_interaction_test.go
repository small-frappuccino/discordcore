package config

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/core"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

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
	t.Helper()

	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: guildID}); err != nil {
		t.Fatalf("failed to add guild config: %v", err)
	}

	if err := session.State.GuildAdd(&discordgo.Guild{ID: guildID, OwnerID: ownerID}); err != nil {
		t.Fatalf("failed to add guild to state: %v", err)
	}

	router := core.NewCommandRouter(session, cm)
	NewConfigCommands(cm).RegisterCommands(router)
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

func stringOpt(name, value string) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
}

func boolOpt(name string, value bool) *discordgo.ApplicationCommandInteractionDataOption {
	return &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: value,
	}
}

func assertEphemeral(t *testing.T, resp discordgo.InteractionResponse) {
	t.Helper()
	if resp.Data.Flags&discordgo.MessageFlagsEphemeral == 0 {
		t.Fatalf("expected ephemeral response, got flags=%v content=%q", resp.Data.Flags, resp.Data.Content)
	}
}

func TestWebhookEmbedCommandsCRUDInteractions(t *testing.T) {
	const (
		guildID = "guild-1"
		ownerID = "owner-1"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	// CREATE (scope omitted -> defaults to guild)
	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_create", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m1"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/123/token-a"),
		stringOpt(optionEmbedJSON, `{"title":"first"}`),
	}))
	createResp := rec.lastResponse(t)
	assertEphemeral(t, createResp)
	if !strings.Contains(createResp.Data.Content, "Created webhook embed update") {
		t.Fatalf("unexpected create response: %q", createResp.Data.Content)
	}

	got, err := cm.GetWebhookEmbedUpdate(guildID, "m1")
	if err != nil {
		t.Fatalf("expected created entry in guild scope: %v", err)
	}
	if got.MessageID != "m1" {
		t.Fatalf("unexpected message_id after create: %+v", got)
	}

	// READ
	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_read", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m1"),
	}))
	readResp := rec.lastResponse(t)
	assertEphemeral(t, readResp)
	if !strings.Contains(readResp.Data.Content, "Scope: `guild:"+guildID+"`") {
		t.Fatalf("unexpected read scope output: %q", readResp.Data.Content)
	}
	if !strings.Contains(readResp.Data.Content, "Message ID: `m1`") {
		t.Fatalf("unexpected read message output: %q", readResp.Data.Content)
	}

	// UPDATE
	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_update", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m1"),
		stringOpt(optionNewMessage, "m2"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/123/token-b"),
		stringOpt(optionEmbedJSON, `{"title":"second"}`),
	}))
	updateResp := rec.lastResponse(t)
	assertEphemeral(t, updateResp)
	if !strings.Contains(updateResp.Data.Content, "Updated webhook embed entry") {
		t.Fatalf("unexpected update response: %q", updateResp.Data.Content)
	}

	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m1"); !errorsIsNotFound(err) {
		t.Fatalf("expected old message_id m1 to be replaced, got err=%v", err)
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m2"); err != nil {
		t.Fatalf("expected new message_id m2 to exist: %v", err)
	}

	// LIST
	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_list", nil))
	listResp := rec.lastResponse(t)
	assertEphemeral(t, listResp)
	if !strings.Contains(listResp.Data.Content, "message_id=`m2`") {
		t.Fatalf("unexpected list response: %q", listResp.Data.Content)
	}

	// DELETE
	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_delete", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m2"),
	}))
	deleteResp := rec.lastResponse(t)
	assertEphemeral(t, deleteResp)
	if !strings.Contains(deleteResp.Data.Content, "Deleted webhook embed update") {
		t.Fatalf("unexpected delete response: %q", deleteResp.Data.Content)
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m2"); !errorsIsNotFound(err) {
		t.Fatalf("expected deleted entry to be missing, got err=%v", err)
	}

	// Default mode is off, so CRUD without apply_now should not call remote validation.
	if rec.webhookLookupCount() != 0 {
		t.Fatalf("expected no webhook lookup calls in default off mode, got %d", rec.webhookLookupCount())
	}
	if rec.messageLookupCount() != 0 {
		t.Fatalf("expected no message lookup calls in default off mode, got %d", rec.messageLookupCount())
	}
}

func TestWebhookEmbedCreateApplyNowInteraction(t *testing.T) {
	const (
		guildID = "guild-apply"
		ownerID = "owner-apply"
	)

	session, rec := newConfigCommandTestSession(t)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_create", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-apply"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/555/token-apply"),
		stringOpt(optionEmbedJSON, `{"title":"apply-now"}`),
		boolOpt(optionApplyNow, true),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "apply_now=true") {
		t.Fatalf("unexpected apply_now response: %q", resp.Data.Content)
	}

	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch call, got %d", rec.webhookPatchCount())
	}
	if !strings.Contains(rec.webhookPath(), "/webhooks/555/token-apply/messages/m-apply") {
		t.Fatalf("unexpected webhook patch path: %q", rec.webhookPath())
	}

	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-apply"); err != nil {
		t.Fatalf("expected created entry with apply_now to persist: %v", err)
	}
}

func TestWebhookEmbedUpdateApplyNowFailureInteraction(t *testing.T) {
	const (
		guildID = "guild-update-fail"
		ownerID = "owner-update-fail"
	)

	session, rec := newConfigCommandTestSessionWithWebhookPatchStatus(t, http.StatusInternalServerError)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.CreateWebhookEmbedUpdate(guildID, files.WebhookEmbedUpdateConfig{
		MessageID:  "m-old",
		WebhookURL: "https://discord.com/api/webhooks/700/token-old",
		Embed:      json.RawMessage(`{"title":"before"}`),
	}); err != nil {
		t.Fatalf("failed to seed webhook update: %v", err)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_update", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-old"),
		stringOpt(optionNewMessage, "m-new"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/700/token-new"),
		stringOpt(optionEmbedJSON, `{"title":"after"}`),
		boolOpt(optionApplyNow, true),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "apply_now failed") {
		t.Fatalf("unexpected update apply_now failure response: %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "restored") {
		t.Fatalf("expected rollback message in response, got: %q", resp.Data.Content)
	}

	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch attempt, got %d", rec.webhookPatchCount())
	}

	restored, err := cm.GetWebhookEmbedUpdate(guildID, "m-old")
	if err != nil {
		t.Fatalf("expected original entry to be restored after failed apply_now: %v", err)
	}
	if restored.WebhookURL != "https://discord.com/api/webhooks/700/token-old" {
		t.Fatalf("unexpected restored webhook url: %+v", restored)
	}
	if strings.TrimSpace(string(restored.Embed)) != `{"title":"before"}` {
		t.Fatalf("unexpected restored embed payload: %s", restored.Embed)
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-new"); !errorsIsNotFound(err) {
		t.Fatalf("expected new message_id to be rolled back, got err=%v", err)
	}
}

func TestWebhookEmbedDeleteApplyNowFailureInteraction(t *testing.T) {
	const (
		guildID = "guild-delete-fail"
		ownerID = "owner-delete-fail"
	)

	session, rec := newConfigCommandTestSessionWithWebhookPatchStatus(t, http.StatusInternalServerError)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	if err := cm.CreateWebhookEmbedUpdate(guildID, files.WebhookEmbedUpdateConfig{
		MessageID:  "m-del",
		WebhookURL: "https://discord.com/api/webhooks/701/token-del",
		Embed:      json.RawMessage(`{"title":"to-delete"}`),
	}); err != nil {
		t.Fatalf("failed to seed webhook update: %v", err)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_delete", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-del"),
		boolOpt(optionApplyNow, true),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "Delete aborted because apply_now failed") {
		t.Fatalf("unexpected delete apply_now failure response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch attempt, got %d", rec.webhookPatchCount())
	}

	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-del"); err != nil {
		t.Fatalf("expected entry to remain after failed delete apply_now: %v", err)
	}
}

func TestWebhookEmbedCreateApplyNowFailureInteraction(t *testing.T) {
	const (
		guildID = "guild-create-fail"
		ownerID = "owner-create-fail"
	)

	session, rec := newConfigCommandTestSessionWithWebhookPatchStatus(t, http.StatusInternalServerError)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_create", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-create"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/702/token-create"),
		stringOpt(optionEmbedJSON, `{"title":"create"}`),
		boolOpt(optionApplyNow, true),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "Create aborted because apply_now failed") {
		t.Fatalf("unexpected create apply_now failure response: %q", resp.Data.Content)
	}
	if !strings.Contains(resp.Data.Content, "rolled back") {
		t.Fatalf("expected rollback message in create response, got: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 1 {
		t.Fatalf("expected one webhook patch attempt, got %d", rec.webhookPatchCount())
	}

	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-create"); !errorsIsNotFound(err) {
		t.Fatalf("expected created entry to be rolled back after failed apply_now, got err=%v", err)
	}
}

func TestWebhookEmbedCreateStrictValidationFailureBlocksPersist(t *testing.T) {
	const (
		guildID = "guild-create-strict"
		ownerID = "owner-create-strict"
	)

	session, rec := newConfigCommandTestSessionWithWebhookValidationStatus(t, http.StatusOK, http.StatusNotFound)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	cm.Config().RuntimeConfig.WebhookEmbedValidation = files.WebhookEmbedValidationConfig{
		Mode:      files.WebhookEmbedValidationModeStrict,
		TimeoutMS: 1000,
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_create", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-strict"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/800/token-strict"),
		stringOpt(optionEmbedJSON, `{"title":"strict"}`),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "strict mode") {
		t.Fatalf("unexpected strict validation response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected no patch calls, got %d", rec.webhookPatchCount())
	}
	if rec.webhookLookupCount() != 1 || rec.messageLookupCount() != 1 {
		t.Fatalf("expected one webhook+message lookup call, got webhook=%d message=%d", rec.webhookLookupCount(), rec.messageLookupCount())
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-strict"); !errorsIsNotFound(err) {
		t.Fatalf("expected strict validation failure to avoid persistence, got err=%v", err)
	}
}

func TestWebhookEmbedCreateSoftValidationFailurePersistsWithWarning(t *testing.T) {
	const (
		guildID = "guild-create-soft"
		ownerID = "owner-create-soft"
	)

	session, rec := newConfigCommandTestSessionWithWebhookValidationStatus(t, http.StatusOK, http.StatusNotFound)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	cm.Config().RuntimeConfig.WebhookEmbedValidation = files.WebhookEmbedValidationConfig{
		Mode:      files.WebhookEmbedValidationModeSoft,
		TimeoutMS: 1000,
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_create", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-soft"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/801/token-soft"),
		stringOpt(optionEmbedJSON, `{"title":"soft"}`),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "Warning: webhook target validation failed in soft mode") {
		t.Fatalf("unexpected soft validation response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected no patch calls, got %d", rec.webhookPatchCount())
	}
	if rec.webhookLookupCount() != 1 || rec.messageLookupCount() != 1 {
		t.Fatalf("expected one webhook+message lookup call, got webhook=%d message=%d", rec.webhookLookupCount(), rec.messageLookupCount())
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-soft"); err != nil {
		t.Fatalf("expected soft validation failure to still persist entry: %v", err)
	}
}

func TestWebhookEmbedUpdateStrictValidationFailureBlocksPersist(t *testing.T) {
	const (
		guildID = "guild-update-strict"
		ownerID = "owner-update-strict"
	)

	session, rec := newConfigCommandTestSessionWithWebhookValidationStatus(t, http.StatusOK, http.StatusNotFound)
	router, cm := newConfigCommandTestRouter(t, session, guildID, ownerID)
	cm.Config().RuntimeConfig.WebhookEmbedValidation = files.WebhookEmbedValidationConfig{
		Mode:      files.WebhookEmbedValidationModeStrict,
		TimeoutMS: 1000,
	}

	if err := cm.CreateWebhookEmbedUpdate(guildID, files.WebhookEmbedUpdateConfig{
		MessageID:  "m-old-strict",
		WebhookURL: "https://discord.com/api/webhooks/802/token-old",
		Embed:      json.RawMessage(`{"title":"old"}`),
	}); err != nil {
		t.Fatalf("failed to seed strict update entry: %v", err)
	}

	router.HandleInteraction(session, newConfigSlashInteraction(guildID, ownerID, "webhook_embed_update", []*discordgo.ApplicationCommandInteractionDataOption{
		stringOpt(optionMessageID, "m-old-strict"),
		stringOpt(optionNewMessage, "m-new-strict"),
		stringOpt(optionWebhookURL, "https://discord.com/api/webhooks/802/token-new"),
		stringOpt(optionEmbedJSON, `{"title":"new"}`),
	}))

	resp := rec.lastResponse(t)
	assertEphemeral(t, resp)
	if !strings.Contains(resp.Data.Content, "strict mode") {
		t.Fatalf("unexpected strict update response: %q", resp.Data.Content)
	}
	if rec.webhookPatchCount() != 0 {
		t.Fatalf("expected no patch calls, got %d", rec.webhookPatchCount())
	}
	if rec.webhookLookupCount() != 1 || rec.messageLookupCount() != 1 {
		t.Fatalf("expected one webhook+message lookup call, got webhook=%d message=%d", rec.webhookLookupCount(), rec.messageLookupCount())
	}

	original, err := cm.GetWebhookEmbedUpdate(guildID, "m-old-strict")
	if err != nil {
		t.Fatalf("expected old entry to remain after strict update validation failure: %v", err)
	}
	if original.WebhookURL != "https://discord.com/api/webhooks/802/token-old" {
		t.Fatalf("unexpected old entry after strict validation failure: %+v", original)
	}
	if _, err := cm.GetWebhookEmbedUpdate(guildID, "m-new-strict"); !errorsIsNotFound(err) {
		t.Fatalf("expected new message_id not to be persisted, got err=%v", err)
	}
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, files.ErrWebhookEmbedUpdateNotFound)
}
