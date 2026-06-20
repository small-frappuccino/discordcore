package control

import (
	"net/http/httptest"
	"testing"
)

func TestDashboard_CompressionNegotiation(t *testing.T) {
	handler := newDashboardHandler()

	// Negociação de Compressão no Fallback
	tests := []struct {
		name             string
		acceptEncoding   string
		expectedEncoding string
	}{
		{"Gzip supported", "gzip", "gzip"},
		{"Brotli fallback to gzip", "br, gzip", "gzip"},
		{"No compression", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			enc := w.Header().Get("Content-Encoding")
			if enc != tt.expectedEncoding {
				t.Errorf("Expected Content-Encoding %q, got %q", tt.expectedEncoding, enc)
			}
		})
	}
}
