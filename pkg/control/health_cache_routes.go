package control

import (
	"encoding/json"
	"net/http"
)

// handleCacheHealthRoute serves the GET /v1/health/cache snapshot. The
// payload mirrors the JSON shape of cache.CacheMetricsSnapshot so operators
// (and any monitoring scraper that speaks JSON) can poll one endpoint and
// see per-segment entries/hits/misses/evictions/hit-rate, the persisted
// cache totals, and the last warmup timestamp without grepping the
// structured-log stream.
//
// Auth: same gate as the rest of /v1/* (Bearer token or Discord OAuth
// session). The metrics are bot-global and contain no per-user data, so
// global read access is sufficient; we do not require guild-scoped
// authorization here.
//
// 200 OK with the snapshot when the cache resolver returns a non-nil
// UnifiedCache. 503 Service Unavailable when the resolver is unset or
// returns nil — operators can distinguish "control server up, cache
// observability not wired" from "control server unavailable".
func (s *Server) handleCacheHealthRoute(w http.ResponseWriter, r *http.Request) {
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
	if s.cacheSnapshotResolve == nil {
		http.Error(w, "cache observability not wired", http.StatusServiceUnavailable)
		return
	}
	uc := s.cacheSnapshotResolve()
	if uc == nil {
		// The resolver exists but no runtime has built a UnifiedCache yet
		// (early boot or monitoring disabled). 503 signals "the endpoint
		// is reachable, the cache is not". Operators get a clear "wait or
		// fix wiring" rather than a 200 with zeroed counters that would
		// look like a healthy idle bot.
		http.Error(w, "cache observability not available", http.StatusServiceUnavailable)
		return
	}

	snapshot := uc.Snapshot(r.Context(), s.cacheSnapshotStore)
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
