package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/discord/logging"
)

// handleMonitoringHealthRoute serves the GET /v1/health/monitoring snapshot.
// Mirrors /v1/health/qotd and /v1/health/moderation: the payload is
// logging.MetricsSnapshot so operators can poll one endpoint and see Discord
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
func (s *Server) handleMonitoringHealthRoute(w http.ResponseWriter, r *http.Request) {
	s.serveHealthRoute(w, r, func() (any, string) {
		if s.monitoringMetricsResolve == nil {
			return nil, "monitoring metrics not wired"
		}
		metrics := s.monitoringMetricsResolve()
		if metrics == nil {
			return nil, "monitoring metrics not available"
		}
		provider, ok := metrics.(logging.SnapshotProvider)
		if !ok {
			return nil, "monitoring metrics not enabled (no SnapshotProvider attached)"
		}
		return provider.Snapshot(), ""
	})
}
