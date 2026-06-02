package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/discord/commands/moderation"
)

// handleModerationHealthRoute serves the GET /v1/health/moderation snapshot.
// Mirrors /v1/health/qotd: the payload is moderation.MetricsSnapshot so
// operators can poll one endpoint and see /clean attempt rate, success
// ratio, per-cause failure breakdown, and per-class delete failure totals
// without grepping the structured-log stream.
//
// Auth and method gating are delegated to serveHealthRoute. The two 503
// branches encode the operator-facing distinction: "moderation metrics
// not enabled" (no setter ever called) versus "moderation metrics not
// enabled (no SnapshotProvider attached)" (a NopMetrics value was wired
// but does not expose a snapshot). The fix for the latter is wiring
// NewInMemoryMetrics in app startup, not hunting a missing route.
func (s *Server) handleModerationHealthRoute(w http.ResponseWriter, r *http.Request) {
	s.serveHealthRoute(w, r, func() (any, string) {
		if s.health.moderationMetrics == nil {
			return nil, "moderation metrics not enabled"
		}
		provider, ok := s.health.moderationMetrics.(moderation.SnapshotProvider)
		if !ok {
			return nil, "moderation metrics not enabled (no SnapshotProvider attached)"
		}
		return provider.Snapshot(), ""
	})
}
