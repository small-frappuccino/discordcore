package control

import (
	"net/http"
	"strings"
)

const publicDashboardBrandAssetPath = "brand/discordmain.webp"

type dashboardAccessHandler struct {
	next           http.Handler
	oauthAvailable bool
	redirectTarget string
	hasSession     func(*http.Request) bool
}

func newProtectedEmbeddedDashboardHandler(oauthAvailable bool, redirectTarget string, hasSession func(*http.Request) bool) http.Handler {
	return &dashboardAccessHandler{
		next:           newEmbeddedDashboardHandler(),
		oauthAvailable: oauthAvailable,
		redirectTarget: redirectTarget,
		hasSession:     hasSession,
	}
}

func (h *dashboardAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.oauthAvailable {
		http.Error(w, "Dashboard access requires Discord OAuth configuration.", http.StatusForbidden)
		return
	}

	if h.hasSession != nil && h.hasSession(r) {
		h.next.ServeHTTP(w, r)
		return
	}

	redirect := strings.TrimSpace(h.redirectTarget)
	if redirect == "" {
		redirect = "/"
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (s *Server) hasAuthenticatedDashboardSession(r *http.Request) bool {
	oauthControl := s.oauthControl()
	if !oauthControl.configured() {
		return false
	}

	_, err := oauthControl.sessionFromRequest(r)
	return err == nil
}
