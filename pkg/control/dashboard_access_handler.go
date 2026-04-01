package control

import "net/http"

const publicDashboardBrandAssetPath = "brand/alicebot.webp"

type dashboardAccessHandler struct {
	next   http.Handler
}

func newProtectedEmbeddedDashboardHandler(_ *Server) http.Handler {
	return &dashboardAccessHandler{
		next:   newEmbeddedDashboardHandler(),
	}
}

func (h *dashboardAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.next.ServeHTTP(w, r)
}

func (s *Server) hasAuthenticatedDashboardSession(r *http.Request) bool {
	if s == nil || s.discordOAuth == nil {
		return false
	}

	_, err := s.discordOAuth.sessionFromRequest(r)
	return err == nil
}
