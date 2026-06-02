package control

import "net/http"

// handleCacheHealthRoute serves the GET /v1/health/cache snapshot. The
// payload mirrors the JSON shape of cache.CacheMetricsSnapshot so operators
// (and any monitoring scraper that speaks JSON) can poll one endpoint and
// see per-segment entries/hits/misses/evictions/hit-rate, the persisted
// cache totals, and the last warmup timestamp without grepping the
// structured-log stream.
//
// Auth and method gating are delegated to serveHealthRoute. The two 503
// branches encode the operator-facing distinction: "cache observability
// not wired" (the resolver was never installed) versus "cache observability
// not available" (the resolver exists but no runtime has built a
// UnifiedCache yet — early boot or monitoring disabled). Distinct bodies
// let operators tell wiring bugs from boot races rather than a 200 with
// zeroed counters that would look like a healthy idle bot.
func (s *Server) handleCacheHealthRoute(w http.ResponseWriter, r *http.Request) {
	s.serveHealthRoute(w, r, func() (any, string) {
		if s.health.cacheSnapshotResolve == nil {
			return nil, "cache observability not wired"
		}
		uc := s.health.cacheSnapshotResolve()
		if uc == nil {
			return nil, "cache observability not available"
		}
		return uc.Snapshot(r.Context(), s.health.cacheSnapshotStore), ""
	})
}
