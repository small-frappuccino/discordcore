package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/telemetry"
)

// handleMonitoringHealthRoute serves the GET /v1/health/monitoring snapshot.
// Mirrors /v1/health/qotd and /v1/health/moderation: the payload is
// telemetry.MetricsSnapshot so operators can poll one endpoint and see Discord
// API call counts (audit_log/guild_member/messages_sent) and the in-process
// cache hit counters monitoring uses to avoid those API calls — without
// grepping the structured-log stream.
//
// Auth and method gating are delegated to serveHealthRoute. The three 503
// branches encode operator-facing distinctions: "not wired" (resolver
// never installed), "not available" (resolver returns nil — runtime still
// booting), and "not enabled (no SnapshotProvider attached)" (runtime up
// but only NopMetrics is attached, i.e. observability disabled). The fix
// for the last case is wiring NewInMemoryMetrics in app startup, not
// hunting a missing route.
type monitoringHealthResolver struct {
	s *Server
}

func (res monitoringHealthResolver) resolve() (any, string) {
	if res.s.health.monitoringMetricsResolve == nil {
		return nil, "monitoring metrics not wired"
	}
	metrics := res.s.health.monitoringMetricsResolve()
	if metrics == nil {
		return nil, "monitoring metrics not available"
	}
	provider, ok := metrics.(telemetry.SnapshotProvider)
	if !ok {
		return nil, "monitoring metrics not enabled (no SnapshotProvider attached)"
	}
	return provider.Snapshot(), ""
}

func (s *Server) handleMonitoringHealthRoute(w http.ResponseWriter, r *http.Request) {
	serveHealthRoute(s, w, r, monitoringHealthResolver{s: s})
}
