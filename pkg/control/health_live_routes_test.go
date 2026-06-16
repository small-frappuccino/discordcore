package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

const healthLiveTestToken = "test-bearer-token"

// TestLiveHealthRouteReturnsOKWithSnapshot pins the contract relied on by
// the external poller: a 200 + JSON body with status=ok, the core version
// string, and a non-negative uptime. If a future refactor accidentally
// short-circuits the route to 204 or breaks the JSON shape, every
// production health poller starts paging.
func TestLiveHealthRouteReturnsOKWithSnapshot(t *testing.T) {
	t.Parallel()

	srv := newLiveHealthTestServer(t)
	srv.SetBearerToken(healthLiveTestToken)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/live", nil)
	req.Header.Set("Authorization", "Bearer "+healthLiveTestToken)
	srv.handleLiveHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap LiveHealthSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	if snap.Status != "ok" {
		t.Fatalf("Status=%q want ok", snap.Status)
	}
	if snap.CoreVersion == "" {
		t.Fatal("CoreVersion is empty; external pollers use this to fingerprint deploys")
	}
	if _, err := time.Parse(time.RFC3339, snap.StartedAt); err != nil {
		t.Fatalf("StartedAt=%q is not RFC3339: %v", snap.StartedAt, err)
	}
	if snap.UptimeSeconds < 0 {
		t.Fatalf("UptimeSeconds=%d must be non-negative", snap.UptimeSeconds)
	}
}

// TestLiveHealthRouteRejectsUnauthenticatedRequests pins the auth gate.
// /v1/health/live is the headline "is the bot up" probe; leaving it
// open would let any anonymous visitor fingerprint bot identity, version,
// and uptime — useful for an attacker timing maintenance windows.
func TestLiveHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newLiveHealthTestServer(t)
	srv.SetBearerToken(healthLiveTestToken)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/live", nil)
	// No Authorization header.
	srv.handleLiveHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

// TestLiveHealthRouteRejectsNonGETMethods pins that the route is
// read-only — POST/PUT/PATCH/DELETE must fail. The external poller is
// strictly a GET consumer; allowing writes silently would let a
// misconfigured caller think a state change was accepted.
func TestLiveHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newLiveHealthTestServer(t)
	srv.SetBearerToken(healthLiveTestToken)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/live", nil)
			req.Header.Set("Authorization", "Bearer "+healthLiveTestToken)
			srv.handleLiveHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newLiveHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
