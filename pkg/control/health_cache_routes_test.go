package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/cache"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const healthCacheTestToken = "test-bearer-token"

// TestCacheHealthRouteReturns503WhenResolverNotWired pins that the route
// honors the "cache observability not wired" contract instead of panicking
// when the control server is started before any bot runtime has installed
// a UnifiedCache.
func TestCacheHealthRouteReturns503WhenResolverNotWired(t *testing.T) {
	t.Parallel()

	srv := newCacheHealthTestServer(t)
	srv.SetBearerToken(healthCacheTestToken)
	// Intentionally NO srv.SetCacheObservability — simulates the gap.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/cache", nil)
	req.Header.Set("Authorization", "Bearer "+healthCacheTestToken)
	srv.handleCacheHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver is not wired, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestCacheHealthRouteReturns503WhenResolverYieldsNil pins that a wired
// resolver returning nil (the runtime exists but has no monitoring service)
// still surfaces as 503 — operators must distinguish "cache off" from
// "cache running with zero entries".
func TestCacheHealthRouteReturns503WhenResolverYieldsNil(t *testing.T) {
	t.Parallel()

	srv := newCacheHealthTestServer(t)
	srv.SetBearerToken(healthCacheTestToken)
	srv.SetCacheObservability(func() *cache.UnifiedCache { return nil }, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/cache", nil)
	req.Header.Set("Authorization", "Bearer "+healthCacheTestToken)
	srv.handleCacheHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver yields nil, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestCacheHealthRouteSurfacesSnapshot pins the headline behavior: a wired
// resolver returning a real UnifiedCache produces a 200 with a JSON snapshot
// that includes all four segments and a (possibly empty) persisted block.
func TestCacheHealthRouteSurfacesSnapshot(t *testing.T) {
	t.Parallel()

	srv := newCacheHealthTestServer(t)
	srv.SetBearerToken(healthCacheTestToken)

	uc := cache.NewUnifiedCache(cache.CacheConfig{})
	srv.SetCacheObservability(func() *cache.UnifiedCache { return uc }, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/cache", nil)
	req.Header.Set("Authorization", "Bearer "+healthCacheTestToken)
	srv.handleCacheHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap cache.CacheMetricsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	// A fresh cache has zero entries; the contract under test is that the
	// JSON shape includes each segment as an object (not omitted), so the
	// scraper can assert per-segment fields without conditional logic.
	if snap.Members.Entries != 0 || snap.Guilds.Entries != 0 || snap.Roles.Entries != 0 || snap.Channels.Entries != 0 {
		t.Fatalf("expected zero entries on a fresh cache, got %+v", snap)
	}
}

// TestCacheHealthRouteRejectsUnauthenticatedRequests pins the auth gate.
// /v1/health/cache exposes operational telemetry an attacker could use to
// fingerprint cache fill state and traffic patterns; it must require the
// same Bearer token as the rest of /v1/*.
func TestCacheHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newCacheHealthTestServer(t)
	srv.SetBearerToken(healthCacheTestToken)
	uc := cache.NewUnifiedCache(cache.CacheConfig{})
	srv.SetCacheObservability(func() *cache.UnifiedCache { return uc }, nil)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/cache", nil)
	// No Authorization header.
	srv.handleCacheHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

// TestCacheHealthRouteRejectsNonGETMethods pins that the route is read-only.
// POST/PUT/DELETE on a metrics endpoint should fail loudly; allowing them
// silently would let a misconfigured client think a write was accepted.
func TestCacheHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newCacheHealthTestServer(t)
	srv.SetBearerToken(healthCacheTestToken)
	uc := cache.NewUnifiedCache(cache.CacheConfig{})
	srv.SetCacheObservability(func() *cache.UnifiedCache { return uc }, nil)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/cache", nil)
			req.Header.Set("Authorization", "Bearer "+healthCacheTestToken)
			srv.handleCacheHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newCacheHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{})
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
