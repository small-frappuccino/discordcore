package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const controlTestAuthToken = "test-control-token"

type partnerBoardResponse struct {
	Status       string                           `json:"status"`
	GuildID      string                           `json:"guild_id"`
	Target       files.EmbedUpdateTargetConfig    `json:"target"`
	Template     files.PartnerBoardTemplateConfig `json:"template"`
	Partner      files.PartnerEntryConfig         `json:"partner"`
	Partners     []files.PartnerEntryConfig       `json:"partners"`
	PartnerBoard files.PartnerBoardConfig         `json:"partner_board"`
	Deleted      string                           `json:"deleted"`
	Synced       bool                             `json:"synced"`
}

type syncExecutorStub struct {
	calls int
	last  string
	err   error
}

func (s *syncExecutorStub) SyncGuild(_ context.Context, guildID string) error {
	s.calls++
	s.last = guildID
	return s.err
}

func newControlTestServer(t *testing.T) (*Server, *files.ConfigManager) {
	t.Helper()

	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil || srv.httpServer == nil || srv.httpServer.Handler == nil {
		t.Fatal("expected non-nil control server with configured handler")
	}
	srv.SetBearerToken(controlTestAuthToken)
	return srv, cm
}

func performHandlerJSONRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	payload any,
) *httptest.ResponseRecorder {
	return performHandlerJSONRequestWithAuth(t, handler, method, path, payload, "Bearer "+controlTestAuthToken)
}

func performHandlerJSONRequestWithAuth(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	payload any,
	authHeader string,
) *httptest.ResponseRecorder {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode payload: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &body)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(authHeader) != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func decodePartnerBoardResponse(t *testing.T, rec *httptest.ResponseRecorder) partnerBoardResponse {
	t.Helper()

	var out partnerBoardResponse
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v body=%q", err, rec.Body.String())
	}
	return out
}

func TestSplitGuildRoute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      string
		wantGuild string
		wantTail  []string
		wantOK    bool
	}{
		{
			name:      "partner board path",
			path:      "/v1/guilds/g1/partner-board",
			wantGuild: "g1",
			wantTail:  []string{"partner-board"},
			wantOK:    true,
		},
		{
			name:      "target with trailing slash",
			path:      "/v1/guilds/g2/partner-board/target/",
			wantGuild: "g2",
			wantTail:  []string{"partner-board", "target"},
			wantOK:    true,
		},
		{
			name:      "template path",
			path:      "/v1/guilds/g3/partner-board/template",
			wantGuild: "g3",
			wantTail:  []string{"partner-board", "template"},
			wantOK:    true,
		},
		{
			name:   "wrong prefix",
			path:   "/v1/runtime-config",
			wantOK: false,
		},
		{
			name:   "missing guild id",
			path:   "/v1/guilds/",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotGuild, gotTail, gotOK := splitGuildRoute(tt.path)
			if gotOK != tt.wantOK {
				t.Fatalf("ok mismatch: got=%v want=%v", gotOK, tt.wantOK)
			}
			if gotGuild != tt.wantGuild {
				t.Fatalf("guild mismatch: got=%q want=%q", gotGuild, tt.wantGuild)
			}
			if strings.Join(gotTail, "|") != strings.Join(tt.wantTail, "|") {
				t.Fatalf("tail mismatch: got=%v want=%v", gotTail, tt.wantTail)
			}
		})
	}
}

func TestPartnerBoardRoutesHandlerCRUD(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	targetPayload := files.EmbedUpdateTargetConfig{
		Type:       files.EmbedUpdateTargetTypeWebhookMessage,
		MessageID:  "123456789012345678",
		WebhookURL: "https://discord.com/api/webhooks/123456789012345678/token",
	}
	targetRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g1/partner-board/target", targetPayload)
	if targetRec.Code != http.StatusOK {
		t.Fatalf("put target: status=%d body=%q", targetRec.Code, targetRec.Body.String())
	}
	targetResp := decodePartnerBoardResponse(t, targetRec)
	if targetResp.Target.Type != files.EmbedUpdateTargetTypeWebhookMessage {
		t.Fatalf("unexpected target type: %+v", targetResp.Target)
	}

	getTargetRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/partner-board/target", nil)
	if getTargetRec.Code != http.StatusOK {
		t.Fatalf("get target: status=%d body=%q", getTargetRec.Code, getTargetRec.Body.String())
	}
	getTargetResp := decodePartnerBoardResponse(t, getTargetRec)
	if getTargetResp.Target.MessageID != "123456789012345678" {
		t.Fatalf("unexpected target message_id: %+v", getTargetResp.Target)
	}

	templatePayload := files.PartnerBoardTemplateConfig{
		Title:                     "Partners Board",
		LineTemplate:              "* `{{name}}` - {{link}}",
		SectionHeaderTemplate:     "## {{fandom}}",
		EmptyStateText:            "No partners yet",
		OtherFandomLabel:          "Other Servers",
		DisableFandomSorting:      true,
		DisablePartnerSorting:     false,
		ContinuationTitle:         "Partners (cont.)",
		SectionContinuationSuffix: "(cont.)",
	}
	templateRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g1/partner-board/template", templatePayload)
	if templateRec.Code != http.StatusOK {
		t.Fatalf("put template: status=%d body=%q", templateRec.Code, templateRec.Body.String())
	}
	templateResp := decodePartnerBoardResponse(t, templateRec)
	if templateResp.Template.Title != "Partners Board" {
		t.Fatalf("unexpected template title: %+v", templateResp.Template)
	}

	getTemplateRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/partner-board/template", nil)
	if getTemplateRec.Code != http.StatusOK {
		t.Fatalf("get template: status=%d body=%q", getTemplateRec.Code, getTemplateRec.Body.String())
	}
	getTemplateResp := decodePartnerBoardResponse(t, getTemplateRec)
	if getTemplateResp.Template.SectionHeaderTemplate != "## {{fandom}}" {
		t.Fatalf("unexpected template section header: %+v", getTemplateResp.Template)
	}

	createFirst := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board/partners", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	})
	if createFirst.Code != http.StatusCreated {
		t.Fatalf("create first partner: status=%d body=%q", createFirst.Code, createFirst.Body.String())
	}

	createSecond := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board/partners", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Alice Mains",
		Link:   "https://discord.com/invite/alice",
	})
	if createSecond.Code != http.StatusCreated {
		t.Fatalf("create second partner: status=%d body=%q", createSecond.Code, createSecond.Body.String())
	}

	listRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/partner-board/partners", nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list partners: status=%d body=%q", listRec.Code, listRec.Body.String())
	}
	listResp := decodePartnerBoardResponse(t, listRec)
	if len(listResp.Partners) != 2 {
		t.Fatalf("unexpected partner list length: %+v", listResp.Partners)
	}
	if listResp.Partners[0].Name != "Alice Mains" || listResp.Partners[1].Name != "Citlali Mains" {
		t.Fatalf("expected deterministic name order, got %+v", listResp.Partners)
	}
	if listResp.Partners[0].Link != "https://discord.gg/alice" {
		t.Fatalf("expected invite URL normalization, got %+v", listResp.Partners[0])
	}

	updateRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g1/partner-board/partners", map[string]any{
		"current_name": "Citlali Mains",
		"partner": files.PartnerEntryConfig{
			Fandom: "Genshin Impact",
			Name:   "Citlali Mains EN",
			Link:   "https://discord.gg/citlali_en",
		},
	})
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update partner: status=%d body=%q", updateRec.Code, updateRec.Body.String())
	}
	updateResp := decodePartnerBoardResponse(t, updateRec)
	if updateResp.Partner.Name != "Citlali Mains EN" {
		t.Fatalf("unexpected updated partner: %+v", updateResp.Partner)
	}

	deleteRec := performHandlerJSONRequest(t, handler, http.MethodDelete, "/v1/guilds/g1/partner-board/partners?name=Alice%20Mains", nil)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete partner: status=%d body=%q", deleteRec.Code, deleteRec.Body.String())
	}

	boardRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/partner-board", nil)
	if boardRec.Code != http.StatusOK {
		t.Fatalf("get board: status=%d body=%q", boardRec.Code, boardRec.Body.String())
	}
	boardResp := decodePartnerBoardResponse(t, boardRec)
	if len(boardResp.PartnerBoard.Partners) != 1 {
		t.Fatalf("expected one partner after delete, got %+v", boardResp.PartnerBoard.Partners)
	}
	if boardResp.PartnerBoard.Partners[0].Name != "Citlali Mains EN" {
		t.Fatalf("unexpected partner left in board: %+v", boardResp.PartnerBoard.Partners[0])
	}
	if boardResp.PartnerBoard.Template.Title != "Partners Board" {
		t.Fatalf("unexpected persisted template: %+v", boardResp.PartnerBoard.Template)
	}
}

func TestPartnerBoardRoutesHandlerErrors(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	methodRec := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board", nil)
	if methodRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for invalid method, got %d body=%q", methodRec.Code, methodRec.Body.String())
	}

	invalidTargetRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g1/partner-board/target", map[string]any{
		"type":        files.EmbedUpdateTargetTypeWebhookMessage,
		"message_id":  "not-numeric",
		"webhook_url": "https://discord.com/api/webhooks/123/token",
	})
	if invalidTargetRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid target payload, got %d body=%q", invalidTargetRec.Code, invalidTargetRec.Body.String())
	}

	createRec := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board/partners", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("seed create partner: status=%d body=%q", createRec.Code, createRec.Body.String())
	}

	duplicateRec := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board/partners", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Citlali Mains",
		Link:   "discord.gg/citlali",
	})
	if duplicateRec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate partner, got %d body=%q", duplicateRec.Code, duplicateRec.Body.String())
	}

	missingCurrentNameRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/g1/partner-board/partners", map[string]any{
		"partner": files.PartnerEntryConfig{
			Fandom: "Genshin Impact",
			Name:   "Citlali Mains Updated",
			Link:   "discord.gg/citlali_updated",
		},
	})
	if missingCurrentNameRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when current_name is missing, got %d body=%q", missingCurrentNameRec.Code, missingCurrentNameRec.Body.String())
	}

	deleteMissingNameRec := performHandlerJSONRequest(t, handler, http.MethodDelete, "/v1/guilds/g1/partner-board/partners", nil)
	if deleteMissingNameRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for delete without name query, got %d body=%q", deleteMissingNameRec.Code, deleteMissingNameRec.Body.String())
	}

	missingGuildReadRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/missing/partner-board", nil)
	if missingGuildReadRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing guild board read, got %d body=%q", missingGuildReadRec.Code, missingGuildReadRec.Body.String())
	}

	missingGuildUpdateRec := performHandlerJSONRequest(t, handler, http.MethodPut, "/v1/guilds/missing/partner-board/partners", map[string]any{
		"current_name": "Citlali Mains",
		"partner": files.PartnerEntryConfig{
			Fandom: "Genshin Impact",
			Name:   "Citlali Mains Updated",
			Link:   "discord.gg/citlali_updated",
		},
	})
	if missingGuildUpdateRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing guild partner update, got %d body=%q", missingGuildUpdateRec.Code, missingGuildUpdateRec.Body.String())
	}

	missingGuildDeleteRec := performHandlerJSONRequest(t, handler, http.MethodDelete, "/v1/guilds/missing/partner-board/partners?name=Citlali%20Mains", nil)
	if missingGuildDeleteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing guild partner delete, got %d body=%q", missingGuildDeleteRec.Code, missingGuildDeleteRec.Body.String())
	}

	notFoundRouteRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/guilds/g1/partner-board/unknown", nil)
	if notFoundRouteRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown partner-board route, got %d body=%q", notFoundRouteRec.Code, notFoundRouteRec.Body.String())
	}
}

func TestPartnerBoardSyncRouteHandler(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	exec := &syncExecutorStub{}
	srv.SetPartnerBoardSyncExecutor(exec)

	okRec := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/g1/partner-board/sync", nil)
	if okRec.Code != http.StatusOK {
		t.Fatalf("sync route status=%d body=%q", okRec.Code, okRec.Body.String())
	}
	okResp := decodePartnerBoardResponse(t, okRec)
	if !okResp.Synced {
		t.Fatalf("expected synced=true response, got %+v", okResp)
	}
	if exec.calls != 1 || exec.last != "g1" {
		t.Fatalf("expected sync executor called for g1, got calls=%d last=%q", exec.calls, exec.last)
	}

	exec.err = fmt.Errorf("sync failed: %w", files.ErrGuildConfigNotFound)
	failRec := performHandlerJSONRequest(t, handler, http.MethodPost, "/v1/guilds/missing/partner-board/sync", nil)
	if failRec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for sync error, got %d body=%q", failRec.Code, failRec.Body.String())
	}
}

func TestControlAuthEnforcement(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	noAuth := performHandlerJSONRequestWithAuth(
		t,
		handler,
		http.MethodGet,
		"/v1/guilds/g1/partner-board",
		nil,
		"",
	)
	if noAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without auth, got %d body=%q", noAuth.Code, noAuth.Body.String())
	}

	wrongAuth := performHandlerJSONRequestWithAuth(
		t,
		handler,
		http.MethodGet,
		"/v1/guilds/g1/partner-board",
		nil,
		"Bearer wrong",
	)
	if wrongAuth.Code != http.StatusForbidden {
		t.Fatalf("expected 403 with wrong token, got %d body=%q", wrongAuth.Code, wrongAuth.Body.String())
	}

	browserBearerReq := httptest.NewRequest(http.MethodGet, "/v1/guilds/g1/partner-board", nil)
	browserBearerReq.Header.Set("Authorization", "Bearer "+controlTestAuthToken)
	browserBearerReq.Header.Set("Origin", "https://dashboard.example")
	browserBearerRec := httptest.NewRecorder()
	handler.ServeHTTP(browserBearerRec, browserBearerReq)
	if browserBearerRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for bearer auth with browser origin, got %d body=%q", browserBearerRec.Code, browserBearerRec.Body.String())
	}

	okAuth := performHandlerJSONRequest(
		t,
		handler,
		http.MethodGet,
		"/v1/guilds/g1/partner-board",
		nil,
	)
	if okAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d body=%q", okAuth.Code, okAuth.Body.String())
	}
}

func TestControlServerStartWithoutConfiguredAuth(t *testing.T) {
	t.Parallel()

	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("expected server start without configured auth to succeed, got: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("stop server: %v", err)
	}
}
