package control

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const controlTestAuthToken = "test-control-token"

func newControlTestServer(t *testing.T) (*Server, *files.ConfigManager) {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
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
