package control

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestPartnerBoardEndpointsInteraction(t *testing.T) {
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("expected non-nil control server")
	}
	srv.SetBearerToken(controlTestAuthToken)

	if err := srv.Start(); err != nil {
		t.Fatalf("start control server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	baseURL := fmt.Sprintf("http://%s", srv.listener.Addr().String())
	client := &http.Client{Timeout: 2 * time.Second}

	targetPayload := files.EmbedUpdateTargetConfig{
		Type:      files.EmbedUpdateTargetTypeChannelMessage,
		MessageID: "123456789012345678",
		ChannelID: "223456789012345678",
	}
	targetResp := doControlRequest(t, client, http.MethodPut, baseURL+"/v1/guilds/g1/partner-board/target", targetPayload)
	if targetResp.Code != http.StatusOK {
		t.Fatalf("put target: status=%d body=%q", targetResp.Code, targetResp.Body)
	}

	createResp := doControlRequest(t, client, http.MethodPost, baseURL+"/v1/guilds/g1/partner-board/partners", files.PartnerEntryConfig{
		Fandom: "Genshin Impact",
		Name:   "Columbina Mains",
		Link:   "https://discord.com/invite/columbina",
	})
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create partner: status=%d body=%q", createResp.Code, createResp.Body)
	}

	templateResp := doControlRequest(t, client, http.MethodPut, baseURL+"/v1/guilds/g1/partner-board/template", files.PartnerBoardTemplateConfig{
		Title:                 "Regional Partners",
		LineTemplate:          "• {{name}} -> {{link}}",
		SectionHeaderTemplate: "### {{fandom}}",
	})
	if templateResp.Code != http.StatusOK {
		t.Fatalf("put template: status=%d body=%q", templateResp.Code, templateResp.Body)
	}

	boardResp := doControlRequest(t, client, http.MethodGet, baseURL+"/v1/guilds/g1/partner-board", nil)
	if boardResp.Code != http.StatusOK {
		t.Fatalf("get board: status=%d body=%q", boardResp.Code, boardResp.Body)
	}
	var payload partnerBoardResponse
	if err := json.Unmarshal(boardResp.Body, &payload); err != nil {
		t.Fatalf("decode board response: %v body=%q", err, string(boardResp.Body))
	}
	if payload.PartnerBoard.Target.Type != files.EmbedUpdateTargetTypeChannelMessage {
		t.Fatalf("unexpected target type in board payload: %+v", payload.PartnerBoard.Target)
	}
	if len(payload.PartnerBoard.Partners) != 1 || payload.PartnerBoard.Partners[0].Name != "Columbina Mains" {
		t.Fatalf("unexpected partners in board payload: %+v", payload.PartnerBoard.Partners)
	}
	if payload.PartnerBoard.Template.Title != "Regional Partners" {
		t.Fatalf("unexpected template title in board payload: %+v", payload.PartnerBoard.Template)
	}

	cmTarget, err := cm.GetPartnerBoardTarget("g1")
	if err != nil {
		t.Fatalf("read target from config manager: %v", err)
	}
	if cmTarget.ChannelID != "223456789012345678" {
		t.Fatalf("unexpected persisted channel target: %+v", cmTarget)
	}
}

func TestPartnerBoardSyncEndpointInteraction(t *testing.T) {
	cm := files.NewMemoryConfigManager()
	if err := cm.AddGuildConfig(files.GuildConfig{GuildID: "g1"}); err != nil {
		t.Fatalf("add guild config: %v", err)
	}

	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("expected non-nil control server")
	}
	srv.SetBearerToken(controlTestAuthToken)

	exec := &syncExecutorStub{}
	srv.SetPartnerBoardSyncExecutor(exec)

	if err := srv.Start(); err != nil {
		t.Fatalf("start control server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	baseURL := fmt.Sprintf("http://%s", srv.listener.Addr().String())
	client := &http.Client{Timeout: 2 * time.Second}

	ok := doControlRequest(t, client, http.MethodPost, baseURL+"/v1/guilds/g1/partner-board/sync", nil)
	if ok.Code != http.StatusOK {
		t.Fatalf("sync endpoint status=%d body=%q", ok.Code, ok.Body)
	}
	var payload partnerBoardResponse
	if err := json.Unmarshal(ok.Body, &payload); err != nil {
		t.Fatalf("decode sync response: %v body=%q", err, string(ok.Body))
	}
	if !payload.Synced {
		t.Fatalf("expected synced=true, got %+v", payload)
	}
	if exec.calls != 1 || exec.last != "g1" {
		t.Fatalf("unexpected sync executor calls=%d last=%q", exec.calls, exec.last)
	}

	exec.err = fmt.Errorf("sync failed: %w", files.ErrGuildConfigNotFound)
	fail := doControlRequest(t, client, http.MethodPost, baseURL+"/v1/guilds/missing/partner-board/sync", nil)
	if fail.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for failed sync, got %d body=%q", fail.Code, fail.Body)
	}
}

type controlHTTPResponse struct {
	Code int
	Body []byte
}

func doControlRequest(
	t *testing.T,
	client *http.Client,
	method string,
	url string,
	payload any,
) controlHTTPResponse {
	t.Helper()

	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			t.Fatalf("encode request payload: %v", err)
		}
	}

	req, err := http.NewRequest(method, url, &body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+controlTestAuthToken)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return controlHTTPResponse{
		Code: resp.StatusCode,
		Body: raw,
	}
}
