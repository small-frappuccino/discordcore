package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/discord/cleanup"
	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const healthModerationTestToken = "test-bearer-token"

// TestModerationHealthRouteReturns503WhenMetricsNotWired pins that the route
// honors the "metrics not enabled" contract instead of panicking when the
// runtime is partially booted (the embedded server starts before moderation
// wiring completes in some startup orderings).
func TestModerationHealthRouteReturns503WhenMetricsNotWired(t *testing.T) {
	t.Parallel()

	srv := newModerationHealthTestServer(t)
	srv.SetBearerToken(healthModerationTestToken)
	// Intentionally NO srv.SetModerationMetrics — simulates the gap.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/moderation", nil)
	req.Header.Set("Authorization", "Bearer "+healthModerationTestToken)
	srv.handleModerationHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when moderation metrics is not wired, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestModerationHealthRouteReturns503WhenMetricsAreNopOnly pins that the bot
// running without observability wiring surfaces the gap as a 503 (not a 200
// with an empty payload). Distinct status lets operators detect "moderation
// is up, telemetry is off" without parsing JSON.
func TestModerationHealthRouteReturns503WhenMetricsAreNopOnly(t *testing.T) {
	t.Parallel()

	srv := newModerationHealthTestServer(t)
	srv.SetBearerToken(healthModerationTestToken)
	// NopMetrics does NOT satisfy SnapshotProvider, so the route falls into
	// the "metrics not enabled" branch even though Metrics is set.
	srv.SetModerationMetrics(moderation.NopMetrics{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/moderation", nil)
	req.Header.Set("Authorization", "Bearer "+healthModerationTestToken)
	srv.handleModerationHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for NopMetrics, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestModerationHealthRouteSurfacesRecordedMetrics pins the headline behavior:
// when the bot is wired with the in-memory metrics implementation, the route
// returns a 200 with a JSON snapshot that reflects every counter the clean
// command recorded.
func TestModerationHealthRouteSurfacesRecordedMetrics(t *testing.T) {
	t.Parallel()

	srv := newModerationHealthTestServer(t)
	srv.SetBearerToken(healthModerationTestToken)

	metrics := &moderation.InMemoryMetrics{}
	metrics.RecordCleanAttempt()
	metrics.RecordCleanAttempt()
	metrics.RecordCleanSuccess(1200*time.Millisecond, 7)
	metrics.RecordCleanFailure(moderation.CleanFailureCauseFetchRateLimited, 50*time.Millisecond)
	metrics.RecordCleanDeleteFailure(cleanup.FailureClassForbidden)
	metrics.RecordCleanDeleteFailure(cleanup.FailureClassForbidden)
	metrics.RecordCleanDeleteFailure(cleanup.FailureClassTransient)
	metrics.RecordCleanAuditLogFailure()

	srv.SetModerationMetrics(metrics)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/moderation", nil)
	req.Header.Set("Authorization", "Bearer "+healthModerationTestToken)
	srv.handleModerationHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap moderation.MetricsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	if snap.Clean.AttemptsTotal != 2 {
		t.Fatalf("AttemptsTotal=%d want 2 (%+v)", snap.Clean.AttemptsTotal, snap.Clean)
	}
	if snap.Clean.SuccessTotal != 1 || snap.Clean.DeletedMessagesTotal != 7 {
		t.Fatalf("success/deleted=%+v want 1/7", snap.Clean)
	}
	if snap.Clean.FailureByCause[moderation.CleanFailureCauseFetchRateLimited] != 1 {
		t.Fatalf("FailureByCause=%+v want fetch_rate_limited=1", snap.Clean.FailureByCause)
	}
	if snap.Clean.DeleteFailureByClass[moderation.FailureClassToken(cleanup.FailureClassForbidden)] != 2 {
		t.Fatalf("DeleteFailureByClass=%+v want forbidden=2", snap.Clean.DeleteFailureByClass)
	}
	if snap.Clean.AuditLogFailureTotal != 1 {
		t.Fatalf("AuditLogFailureTotal=%d want 1 (%+v)", snap.Clean.AuditLogFailureTotal, snap.Clean)
	}
}

// TestModerationHealthRouteRejectsUnauthenticatedRequests pins the auth gate.
// /v1/health/moderation exposes mod-action telemetry an attacker could use
// to fingerprint cleanup cadence or watch when permissions break; it must
// require the same Bearer token as the rest of /v1/*.
func TestModerationHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newModerationHealthTestServer(t)
	srv.SetBearerToken(healthModerationTestToken)
	srv.SetModerationMetrics(&moderation.InMemoryMetrics{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/moderation", nil)
	// No Authorization header.
	srv.handleModerationHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

// TestModerationHealthRouteRejectsNonGETMethods pins that the route is
// read-only. POST/PUT/DELETE on a metrics endpoint should fail loudly;
// allowing them silently would let a misconfigured client think a write
// was accepted.
func TestModerationHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newModerationHealthTestServer(t)
	srv.SetBearerToken(healthModerationTestToken)
	srv.SetModerationMetrics(&moderation.InMemoryMetrics{})

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/moderation", nil)
			req.Header.Set("Authorization", "Bearer "+healthModerationTestToken)
			srv.handleModerationHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newModerationHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
