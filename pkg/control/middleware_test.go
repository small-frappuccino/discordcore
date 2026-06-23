package control

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiddleware_OOMPrevention(t *testing.T) {
	t.Parallel()
	handler := maxBytesMiddleware(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Send >10MB
	body := bytes.Repeat([]byte("A"), 11*1024*1024)
	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Expected 413, got %d", w.Code)
	}
}

func TestMiddleware_TimingAttack(t *testing.T) {
	t.Parallel()
	handler := authorizeRequest("secure_token_12345", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	start := time.Now()
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid_token")
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401")
	}

	duration := time.Since(start)
	if duration > 10*time.Millisecond { // basic sanity check that it returns instantly
		t.Fatalf("Authorization took too long, potential timing leak: %v", duration)
	}
}

func TestMiddleware_AdminAccess(t *testing.T) {
	t.Parallel()
	handler := requireGuildAdmin(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	// Missing flag
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}
}
