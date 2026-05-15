package control

import (
	"encoding/json"
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
)

// handleModerationHealthRoute serves the GET /v1/health/moderation snapshot.
// Mirrors /v1/health/qotd: the payload is moderation.MetricsSnapshot so
// operators can poll one endpoint and see /clean attempt rate, success
// ratio, per-cause failure breakdown, and per-class delete failure totals
// without grepping the structured-log stream.
//
// Auth: same gate as the rest of /v1/* (Bearer token or Discord OAuth
// session). The metrics are aggregate and contain no per-user data, so
// global read access is sufficient.
//
// 200 OK with the snapshot when the server is wired with a SnapshotProvider.
// 503 Service Unavailable when moderation metrics are not wired (no setter
// called) or only NopMetrics is attached — operators can distinguish
// "moderation up, telemetry off" from "moderation unavailable".
func (s *Server) handleModerationHealthRoute(w http.ResponseWriter, r *http.Request) {
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
	if s.moderationMetrics == nil {
		http.Error(w, "moderation metrics not enabled", http.StatusServiceUnavailable)
		return
	}

	provider, ok := s.moderationMetrics.(moderation.SnapshotProvider)
	if !ok {
		// NopMetrics is a valid Metrics implementation but does not expose
		// a snapshot. Returning 503 (not 404) signals "the endpoint is
		// wired but the bot is running without observability enabled" —
		// the operator's fix is to wire NewInMemoryMetrics in app
		// startup, not to look for a missing route.
		http.Error(w, "moderation metrics not enabled (no SnapshotProvider attached)", http.StatusServiceUnavailable)
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
