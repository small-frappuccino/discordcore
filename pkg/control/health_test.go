package control

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockQOTDHealth struct {
	Active bool `json:"active"`
}

func TestHealth_GenericReflection(t *testing.T) {
	// Consolidação de Tipos Genéricos
	handler := serveHealthRoute(func() mockQOTDHealth {
		return mockQOTDHealth{Active: true}
	})

	req := httptest.NewRequest("GET", "/v1/health/mock", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "{\"active\":true}\n" {
		t.Fatalf("JSON marshal failed, got: %s", body)
	}
}
