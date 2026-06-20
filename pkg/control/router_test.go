package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouter_Go122MethodMultiplexing(t *testing.T) {
	srv := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	// Test GET vs POST on /v1/features
	req := httptest.NewRequest("PATCH", "/v1/features", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	// Since we registered GET and POST, PATCH should fail natively with 405
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("Expected 405 Method Not Allowed natively from Go 1.22 mux, got %d", w.Code)
	}
}
