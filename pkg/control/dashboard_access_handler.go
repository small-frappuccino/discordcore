package control

import (
	"net/http"
)

type dashboardAccessHandler struct {
	server *Server
	next   http.Handler
}

func newProtectedEmbeddedDashboardHandler(server *Server) http.Handler {
	return &dashboardAccessHandler{
		server: server,
		next:   newEmbeddedDashboardHandler(),
	}
}

func (h *dashboardAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.server.hasAuthenticatedDashboardSession(r) {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	h.next.ServeHTTP(w, r)
}

func (s *Server) hasAuthenticatedDashboardSession(r *http.Request) bool {
	if s == nil || s.discordOAuth == nil {
		return false
	}

	_, err := s.discordOAuth.sessionFromRequest(r)
	return err == nil
}
