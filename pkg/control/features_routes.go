package control

import "net/http"

func (s *Server) handleFeatureRoutes(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.authorizeRequest(w, r)
	if !ok {
		return
	}
	if s.configManager == nil {
		http.Error(w, "config manager unavailable", http.StatusInternalServerError)
		return
	}

	path := normalizeFeatureRoutePath(r.URL.Path)
	switch {
	case path == "/v1/features/catalog":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
			return
		}
		s.handleFeatureCatalogGet(w)
	case path == "/v1/features":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
			return
		}
		s.handleGlobalFeaturesList(w)
	default:
		featureID, ok := splitGlobalFeatureRoute(path)
		if !ok {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelRead) {
				return
			}
			s.handleGlobalFeatureGet(w, featureID)
		case http.MethodPatch:
			if !s.authorizeGlobalControlAccess(w, r, auth, guildAccessLevelWrite) {
				return
			}
			s.handleGlobalFeaturePatch(w, r, featureID)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
