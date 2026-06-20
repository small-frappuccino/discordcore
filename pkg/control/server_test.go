package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestServer_GracefulDegradation(t *testing.T) {
	// Terminação de Conexão Degradada
	srv := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/long", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Second) // Purposefully long
	})
	srv.httpServer = &http.Server{Handler: mux}

	// Start a mock server listener
	ts := httptest.NewUnstartedServer(mux)
	srv.httpServer = ts.Config
	ts.Start()
	defer ts.Close()

	// Hit the long route
	go http.Get(ts.URL + "/long")

	time.Sleep(100 * time.Millisecond)

	start := time.Now()
	// Stop should enforce 5s deadline
	err := srv.Stop(context.Background())
	duration := time.Since(start)

	if err == nil {
		t.Fatal("Expected deadline exceeded error from Shutdown")
	}

	if duration < 4*time.Second || duration > 6*time.Second {
		t.Fatalf("Stop did not enforce the strict 5s limit: took %v", duration)
	}
}

func TestMain(m *testing.M) {
	// Goleak verification as explicitly requested
	goleak.VerifyTestMain(m)
}
