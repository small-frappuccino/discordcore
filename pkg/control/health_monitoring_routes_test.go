package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
	"github.com/small-frappuccino/discordcore/pkg/files"
)

const healthMonitoringTestToken = "test-bearer-token"

// TestMonitoringHealthRouteReturns503WhenResolverNotWired pins that the
// route honors the "monitoring metrics not wired" contract instead of
// panicking when the control server starts before any bot runtime has
// published its Metrics value.
func TestMonitoringHealthRouteReturns503WhenResolverNotWired(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)
	// Intentionally NO srv.SetMonitoringMetricsResolver.

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/monitoring", nil)
	req.Header.Set("Authorization", "Bearer "+healthMonitoringTestToken)
	srv.handleMonitoringHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver is not wired, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestMonitoringHealthRouteReturns503WhenResolverYieldsNil pins that a
// wired resolver returning nil (no runtime ready yet) still surfaces as
// 503 — operators must distinguish "monitoring off" from "monitoring up
// with zero counters".
func TestMonitoringHealthRouteReturns503WhenResolverYieldsNil(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)
	srv.SetMonitoringMetricsResolver(func() logging.Metrics { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/monitoring", nil)
	req.Header.Set("Authorization", "Bearer "+healthMonitoringTestToken)
	srv.handleMonitoringHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver yields nil, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestMonitoringHealthRouteReturns503WhenMetricsAreNopOnly pins that a bot
// running with NopMetrics (the default constructor) surfaces the gap as a
// 503, not a 200 with zero counters. Distinct body lets operators tell
// "monitoring is up, telemetry is off" from "no runtime".
func TestMonitoringHealthRouteReturns503WhenMetricsAreNopOnly(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)
	srv.SetMonitoringMetricsResolver(func() logging.Metrics { return logging.NopMetrics{} })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/monitoring", nil)
	req.Header.Set("Authorization", "Bearer "+healthMonitoringTestToken)
	srv.handleMonitoringHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for NopMetrics, got %d body=%q", rec.Code, rec.Body.String())
	}
}

// TestMonitoringHealthRouteSurfacesRecordedMetrics pins the headline
// behavior: a wired in-memory metrics value reflects every counter the
// monitoring service recorded into the JSON snapshot.
func TestMonitoringHealthRouteSurfacesRecordedMetrics(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)

	metrics := logging.NewInMemoryMetrics()
	metrics.RecordAuditLogCall()
	metrics.RecordAuditLogCall()
	metrics.RecordGuildMemberCall()
	metrics.RecordMessageSent()
	metrics.RecordStateMemberCacheHit()
	metrics.RecordRolesCacheMemoryHit()
	metrics.RecordRolesCacheStoreHit()
	metrics.RecordRolesAuditCacheHit()

	srv.SetMonitoringMetricsResolver(func() logging.Metrics { return metrics })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/monitoring", nil)
	req.Header.Set("Authorization", "Bearer "+healthMonitoringTestToken)
	srv.handleMonitoringHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap logging.MetricsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	if snap.API.AuditLogCallsTotal != 2 || snap.API.GuildMemberCallsTotal != 1 || snap.API.MessagesSentTotal != 1 {
		t.Fatalf("expected API counters 2/1/1, got %+v", snap.API)
	}
	if snap.Cache.StateMemberHitsTotal != 1 || snap.Cache.RolesMemoryHitsTotal != 1 || snap.Cache.RolesStoreHitsTotal != 1 || snap.Cache.RolesAuditHitsTotal != 1 {
		t.Fatalf("expected cache hit counters 1/1/1/1, got %+v", snap.Cache)
	}
}

// TestMonitoringHealthRouteRejectsUnauthenticatedRequests pins the auth gate.
// /v1/health/monitoring exposes operational telemetry; it must require the
// same Bearer token as the rest of /v1/*.
func TestMonitoringHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)
	srv.SetMonitoringMetricsResolver(func() logging.Metrics { return logging.NewInMemoryMetrics() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/monitoring", nil)
	// No Authorization header.
	srv.handleMonitoringHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

// TestMonitoringHealthRouteRejectsNonGETMethods pins that the route is read-only.
func TestMonitoringHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newMonitoringHealthTestServer(t)
	srv.SetBearerToken(healthMonitoringTestToken)
	srv.SetMonitoringMetricsResolver(func() logging.Metrics { return logging.NewInMemoryMetrics() })

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/monitoring", nil)
			req.Header.Set("Authorization", "Bearer "+healthMonitoringTestToken)
			srv.handleMonitoringHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newMonitoringHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewMemoryConfigManager()
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
