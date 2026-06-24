package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOAuth_CSRFPurge(t *testing.T) {
	t.Parallel()
	srv, _ := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Manutenção e Validação de Sessão CSRF
	req := httptest.NewRequest("GET", "/auth/discord/callback?state=forged_state", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403 Forbidden on forged state, got %d", w.Code)
	}

	cookies := w.Header().Values("Set-Cookie")
	foundPurge := false
	for _, c := range cookies {
		if c == "session=; Path=/; Max-Age=0" {
			foundPurge = true
			break
		}
	}

	if !foundPurge {
		t.Fatalf("Expected CSRF session purge cookie, got: %v", cookies)
	}
}
