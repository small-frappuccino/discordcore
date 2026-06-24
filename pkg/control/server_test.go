package control

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/goleak"
	"golang.org/x/sync/errgroup"
)

func TestServer_GracefulDegradation(t *testing.T) {
	t.Parallel()
	// Terminação de Conexão Degradada
	srv, _ := NewServer("127.0.0.1:0", nil, nil)
	mux := http.NewServeMux()
	started := make(chan struct{})
	mux.HandleFunc("/long", func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done() // Block deterministically until shutdown deadline exceeds
	})
	srv.httpServer = &http.Server{Handler: mux}

	// Start a mock server listener
	ts := httptest.NewUnstartedServer(mux)
	srv.httpServer = ts.Config
	ts.Start()
	defer ts.Close()

	// Hit the long route
	reqCtx, cancelReq := context.WithCancel(context.Background())
	defer cancelReq()

	req, err := http.NewRequestWithContext(reqCtx, "GET", ts.URL+"/long", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	eg, ctx := errgroup.WithContext(context.Background())
	eg.Go(func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		return nil
	})

	<-started // Wait deterministically for the request to reach the server

	start := time.Now()
	// Stop should enforce 5s deadline
	stopErr := srv.Stop(context.Background())
	duration := time.Since(start)

	// Cancel client request to clean up connection and prevent deadlock in ts.Close()
	cancelReq()

	_ = eg.Wait()

	if stopErr == nil {
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
