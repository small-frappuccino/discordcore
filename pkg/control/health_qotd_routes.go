package control

import (
	"net/http"

	"github.com/small-frappuccino/discordcore/pkg/qotd"
)

// handleQOTDHealthRoute serves the GET /v1/health/qotd snapshot. The
// payload mirrors the JSON shape of qotd.MetricsSnapshot so operators
// (and any monitoring scraper that speaks JSON) can poll one endpoint
// and see publish ratios, reconcile cadence, divergence counts, and
// the side-event totals without grepping the structured-log stream.
//
// Auth and method gating are delegated to serveHealthRoute. The two 503
// branches encode the operator-facing distinction: "qotd service
// unavailable" (the service was never wired) versus "qotd metrics not
// enabled" (the service runs with NopMetrics, the default constructor's
// no-snapshot sink). The fix for the latter is wiring NewInMemoryMetrics
// in app startup, not hunting a missing route.
func (s *Server) handleQOTDHealthRoute(w http.ResponseWriter, r *http.Request) {
	s.serveHealthRoute(w, r, func() (any, string) {
		if s.qotdService == nil {
			return nil, "qotd service unavailable"
		}
		provider, ok := s.qotdService.Metrics().(qotd.SnapshotProvider)
		if !ok {
			return nil, "qotd metrics not enabled (no SnapshotProvider attached)"
		}
		return provider.Snapshot(), ""
	})
}
