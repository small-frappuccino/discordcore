package control

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestFeaturesSettings_RaceConditions(t *testing.T) {
	srv := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	srv.registerRoutes(mux)

	var wg sync.WaitGroup

	// Condições de Corrida em Hot-Reload
	// Start 50 concurrent GETs and PUTs
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/v1/settings", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
		}()
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("PUT", "/v1/runtime-config", nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
		}()
	}

	wg.Wait()
	// If run with -race, the compiler will flag any data races occurring here
}
