package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

const healthQOTDTestToken = "test-bearer-token"

// TestQOTDHealthRouteReturns503WhenServiceNotWired pins that the route
// honors the "qotd service unavailable" contract instead of panicking
// when the runtime is partially booted (the embedded server starts
// before QOTD wiring completes in some startup orderings).
func TestQOTDHealthRouteReturns503WhenServiceNotWired(t *testing.T) {
	t.Parallel()

	srv := newQOTDHealthTestServer(t)
	srv.SetBearerToken(healthQOTDTestToken)
	// Intentionally NO srv.SetQOTDService — simulates the gap.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/qotd", nil)
	req.Header.Set("Authorization", "Bearer "+healthQOTDTestToken)
	srv.handleQOTDHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when QOTD service is not wired, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestQOTDHealthRouteReturns503WhenMetricsAreNopOnly pins that the bot
// running without observability wiring surfaces the gap as a 503 (not
// a 200 with an empty payload). Distinct status lets operators detect
// "QOTD is up, telemetry is off" without parsing JSON.
func TestQOTDHealthRouteReturns503WhenMetricsAreNopOnly(t *testing.T) {
	t.Parallel()

	srv := newQOTDHealthTestServer(t)
	srv.SetBearerToken(healthQOTDTestToken)
	// NewService uses NopMetrics. NopMetrics does NOT satisfy
	// SnapshotProvider, so the route falls into the "metrics not
	// enabled" branch even though the QOTD service is healthy.
	srv.SetQOTDService(qotd.NewService(srv.configManager, nil, nil))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/qotd", nil)
	req.Header.Set("Authorization", "Bearer "+healthQOTDTestToken)
	srv.handleQOTDHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for NopMetrics, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestQOTDHealthRouteSurfacesRecordedMetrics pins the headline behavior:
// when the bot is wired with the in-memory metrics implementation, the
// route returns a 200 with a JSON snapshot that reflects every counter
// the service recorded.
func TestQOTDHealthRouteSurfacesRecordedMetrics(t *testing.T) {
	t.Parallel()

	srv := newQOTDHealthTestServer(t)
	srv.SetBearerToken(healthQOTDTestToken)

	metrics := &qotd.InMemoryMetrics{}
	metrics.RecordPublishAttempt(qotd.PublishModeScheduled)
	metrics.RecordPublishSuccess(qotd.PublishModeScheduled, 1200*time.Millisecond)
	metrics.RecordPublishFailure(qotd.PublishModeManual, "no_questions", 400*time.Millisecond)
	metrics.RecordReconcileCycle(150*time.Millisecond, nil)
	metrics.RecordOfficialPostAbandoned()
	metrics.RecordOrphanReclaim(2)
	metrics.RecordSuppressionCleared()

	srv.SetQOTDService(qotd.NewServiceWithMetrics(srv.configManager, nil, nil, metrics))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/qotd", nil)
	req.Header.Set("Authorization", "Bearer "+healthQOTDTestToken)
	srv.handleQOTDHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap qotd.MetricsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	scheduled, ok := snap.Publishes[string(qotd.PublishModeScheduled)]
	if !ok || scheduled.SuccessTotal != 1 {
		t.Fatalf("expected scheduled success total to be 1, got %+v", snap.Publishes)
	}
	manual, ok := snap.Publishes[string(qotd.PublishModeManual)]
	if !ok || manual.FailureTotal != 1 || manual.FailureByCause["no_questions"] != 1 {
		t.Fatalf("expected manual failure with no_questions cause, got %+v", snap.Publishes)
	}
	if snap.Reconcile.CyclesTotal != 1 {
		t.Fatalf("expected one reconcile cycle, got %+v", snap.Reconcile)
	}
	if snap.State.AbandonedTotal != 1 || snap.State.SuppressionsCleared != 1 || snap.State.OrphanReservationsReclaimed != 2 {
		t.Fatalf("expected side-event counters to match recordings, got %+v", snap.State)
	}
}

// TestQOTDHealthRouteRejectsUnauthenticatedRequests pins the auth gate.
// /v1/health/qotd exposes operational telemetry that an attacker could
// use to fingerprint guild count and publish cadence; it must require
// the same Bearer token as the rest of /v1/*.
func TestQOTDHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newQOTDHealthTestServer(t)
	srv.SetBearerToken(healthQOTDTestToken)
	srv.SetQOTDService(qotd.NewServiceWithMetrics(srv.configManager, nil, nil, &qotd.InMemoryMetrics{}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/qotd", nil)
	// No Authorization header.
	srv.handleQOTDHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

// TestQOTDHealthRouteRejectsNonGETMethods pins that the route is read-only.
// POST/PUT/DELETE on a metrics endpoint should fail loudly; allowing them
// silently would let a misconfigured client think a write was accepted.
func TestQOTDHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newQOTDHealthTestServer(t)
	srv.SetBearerToken(healthQOTDTestToken)
	srv.SetQOTDService(qotd.NewServiceWithMetrics(srv.configManager, nil, nil, &qotd.InMemoryMetrics{}))

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/qotd", nil)
			req.Header.Set("Authorization", "Bearer "+healthQOTDTestToken)
			srv.handleQOTDHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newQOTDHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
