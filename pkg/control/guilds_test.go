package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGuilds_SimpleGet(t *testing.T) {
	srv := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	req := httptest.NewRequest("GET", "/v1/guilds/123/channels", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}
}
