package control

import (
	"encoding/json"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

// handleQOTDHealthRoute serves the GET /v1/health/qotd snapshot. The
// payload mirrors the JSON shape of qotd.MetricsSnapshot so operators
// (and any monitoring scraper that speaks JSON) can poll one endpoint
// and see publish ratios, reconcile cadence, divergence counts, and
// the side-event totals without grepping the structured-log stream.
//
// Auth: same gate as the rest of /v1/* (Bearer token or Discord OAuth
// session). The metrics are guild-aggregated and contain no per-user
// data, so global read access is sufficient; we do not require
// guild-scoped authorization here.
//
// 200 OK with the snapshot when the service exposes a SnapshotProvider.
// 503 Service Unavailable when QOTD is wired without an in-memory
// metrics sink (i.e. only NopMetrics is attached) — operators can
// distinguish "QOTD healthy, no metrics enabled" from "QOTD unavailable".
func (s *Server) handleQOTDHealthRoute(w http.ResponseWriter, r *http.Request) {
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
	if s.qotdService == nil {
		http.Error(w, "qotd service unavailable", http.StatusServiceUnavailable)
		return
	}

	provider, ok := s.qotdService.Metrics().(qotd.SnapshotProvider)
	if !ok {
		// The default constructor uses NopMetrics, which does not expose
		// a snapshot. Returning 503 (not 404) signals "the endpoint is
		// wired but the bot is running without observability enabled" —
		// the operator's fix is to wire NewInMemoryMetrics in app
		// startup, not to look for a missing route.
		http.Error(w, "qotd metrics not enabled (no SnapshotProvider attached)", http.StatusServiceUnavailable)
		return
	}

	snapshot := provider.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		// The response status header is already in flight when Encode
		// hits the wire; there is no recovery beyond logging. We rely
		// on the standard library writing the partial body and the
		// client treating a truncated JSON as a transient failure.
		_ = err
	}
}
