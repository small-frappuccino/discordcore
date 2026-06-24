package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/sync/errgroup"
)

func TestFeaturesSettings_RaceConditions(t *testing.T) {
	t.Parallel()
	srv, _ := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	eg, _ := errgroup.WithContext(context.Background())

	// Condições de Corrida em Hot-Reload
	// Start 50 concurrent GETs and PUTs
	for i := 0; i < 50; i++ {
		eg.Go(func() error {
			req := httptest.NewRequest("GET", "/v1/settings", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			return nil
		})
		eg.Go(func() error {
			req := httptest.NewRequest("PUT", "/v1/runtime-config", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			return nil
		})
	}

	_ = eg.Wait()
	// If run with -race, the compiler will flag any data races occurring here
}
