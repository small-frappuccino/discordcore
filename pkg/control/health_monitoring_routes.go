package control

import (
	"encoding/json"
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
// Auth: same gate as the rest of /v1/* (Bearer token or Discord OAuth
// session). The metrics are aggregate and contain no per-user data, so
// global read access is sufficient.
//
// 200 OK with the snapshot when the server is wired with a SnapshotProvider.
// 503 Service Unavailable when the resolver is unset, returns nil (runtime
// still booting), or returns only NopMetrics (runtime up, observability
// disabled). Distinct 503 bodies let operators tell those cases apart.
func (s *Server) handleMonitoringHealthRoute(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
		return
	}
	if s.monitoringMetricsResolve == nil {
		http.Error(w, "monitoring metrics not wired", http.StatusServiceUnavailable)
		return
	}
	metrics := s.monitoringMetricsResolve()
	if metrics == nil {
		http.Error(w, "monitoring metrics not available", http.StatusServiceUnavailable)
		return
	}
	provider, ok := metrics.(logging.SnapshotProvider)
	if !ok {
		// NopMetrics satisfies Metrics but does not expose a snapshot.
		// 503 (not 404) signals "the endpoint is wired but the bot is
		// running without observability enabled" — the operator's fix
		// is to wire NewInMemoryMetrics in app startup, not to look for
		// a missing route.
		http.Error(w, "monitoring metrics not enabled (no SnapshotProvider attached)", http.StatusServiceUnavailable)
		return
	}

	snapshot := provider.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		// Response status header is already in flight when Encode hits
		// the wire; recovery beyond logging is impossible. The standard
		// library will write the partial body and the client must treat
		// a truncated JSON as a transient failure.
		_ = err
	}
}
