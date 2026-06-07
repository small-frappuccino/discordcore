package control

import (
	"net/http"
	"strings"
)

const publicDashboardBrandAssetPath = "brand/discordmain.webp"

type dashboardAccessHandler struct {
	next           http.Handler
	oauthAvailable func() bool
	redirectTarget func() string
	hasSession     func(*http.Request) bool
}

func newProtectedEmbeddedDashboardHandler(oauthAvailable func() bool, redirectTarget func() string, hasSession func(*http.Request) bool) http.Handler {
	return &dashboardAccessHandler{
		next:           newEmbeddedDashboardHandler(),
		oauthAvailable: oauthAvailable,
		redirectTarget: redirectTarget,
		hasSession:     hasSession,
	}
}

func (h *dashboardAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.oauthAvailable != nil && !h.oauthAvailable() {
		http.Error(w, "Dashboard access requires Discord OAuth configuration.", http.StatusForbidden)
		return
	}

	if h.hasSession != nil && h.hasSession(r) {
		h.next.ServeHTTP(w, r)
		return
	}

	var redirect string
	if h.redirectTarget != nil {
		redirect = strings.TrimSpace(h.redirectTarget())
	}
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
