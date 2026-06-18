package control

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/small-frappuccino/discordcore/pkg/files"
	"github.com/small-frappuccino/discordcore/pkg/members"
	"github.com/small-frappuccino/discordcore/pkg/messages"
)

const healthDomainTestToken = "test-bearer-token"

func TestMembersHealthRouteReturns503WhenResolverNotWired(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/members", nil)
	req.Header.Set("Authorization", "Bearer "+healthDomainTestToken)
	srv.handleMembersHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver is not wired, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMessagesHealthRouteReturns503WhenResolverYieldsNil(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)
	srv.SetMessagesMetricsResolver(func() messages.Metrics { return nil })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/messages", nil)
	req.Header.Set("Authorization", "Bearer "+healthDomainTestToken)
	srv.handleMessagesHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when resolver yields nil, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMembersHealthRouteReturns503WhenMetricsAreNopOnly(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)
	srv.SetMembersMetricsResolver(func() members.Metrics { return members.NopMetrics{} })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/members", nil)
	req.Header.Set("Authorization", "Bearer "+healthDomainTestToken)
	srv.handleMembersHealthRoute(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for NopMetrics, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMembersHealthRouteSurfacesRecordedMetrics(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)

	metrics := members.NewInMemoryMetrics()
	metrics.RecordAuditLogCall()
	metrics.RecordAuditLogCall()
	metrics.RecordGuildMemberCall()
	metrics.RecordStateMemberCacheHit()
	metrics.RecordRolesCacheMemoryHit()
	metrics.RecordRolesCacheStoreHit()
	metrics.RecordRolesAuditCacheHit()

	srv.SetMembersMetricsResolver(func() members.Metrics { return metrics })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/members", nil)
	req.Header.Set("Authorization", "Bearer "+healthDomainTestToken)
	srv.handleMembersHealthRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected application/json content type, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache control, got %q", got)
	}

	var snap members.MetricsSnapshot
	if err := json.Unmarshal(rec.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode snapshot: %v body=%q", err, rec.Body.String())
	}
	if snap.AuditLogCallsTotal != 2 || snap.GuildMemberCallsTotal != 1 {
		t.Fatalf("expected API counters 2/1, got %+v", snap)
	}
	if snap.StateMemberHitsTotal != 1 || snap.RolesMemoryHitsTotal != 1 || snap.RolesStoreHitsTotal != 1 || snap.RolesAuditHitsTotal != 1 {
		t.Fatalf("expected cache hit counters 1/1/1/1, got %+v", snap)
	}
}

func TestMembersHealthRouteRejectsUnauthenticatedRequests(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)
	srv.SetMembersMetricsResolver(func() members.Metrics { return members.NewInMemoryMetrics() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/health/members", nil)
	// No Authorization header.
	srv.handleMembersHealthRoute(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("expected unauthenticated request to be rejected, got 200")
	}
}

func TestMembersHealthRouteRejectsNonGETMethods(t *testing.T) {
	t.Parallel()

	srv := newDomainHealthTestServer(t)
	srv.SetBearerToken(healthDomainTestToken)
	srv.SetMembersMetricsResolver(func() members.Metrics { return members.NewInMemoryMetrics() })

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/v1/health/members", nil)
			req.Header.Set("Authorization", "Bearer "+healthDomainTestToken)
			srv.handleMembersHealthRoute(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 for %s, got %d", method, rec.Code)
			}
		})
	}
}

func newDomainHealthTestServer(t *testing.T) *Server {
	t.Helper()

	cm := files.NewConfigManagerWithStore(&files.MemoryConfigStore{}, nil)
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
	return srv
}
