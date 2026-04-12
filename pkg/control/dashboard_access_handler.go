package control

import (
	"net/http"
	"strings"
)

const publicDashboardBrandAssetPath = "brand/alicebot.webp"

type dashboardAccessHandler struct {
	next   http.Handler
	server *Server
}

func newProtectedEmbeddedDashboardHandler(server *Server) http.Handler {
	return &dashboardAccessHandler{
		next:   newEmbeddedDashboardHandler(),
		server: server,
	}
}

func (h *dashboardAccessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.isPublicDashboardAsset(r) || (h.server != nil && h.server.hasAuthenticatedDashboardSession(r)) {
		h.next.ServeHTTP(w, r)
		return
	}

	redirectTarget := "/"
	if h.server != nil {
		redirectTarget = strings.TrimSpace(h.server.publicControlURL("/"))
		if redirectTarget == "" {
			redirectTarget = "/"
		}
	}
	http.Redirect(w, r, redirectTarget, http.StatusFound)
}

func (h *dashboardAccessHandler) isPublicDashboardAsset(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}

	assetPath, ok := normalizeDashboardAssetPath(r.URL.Path)
	if !ok {
		return false
	}
	return strings.TrimSpace(assetPath) == publicDashboardBrandAssetPath
}

func (s *Server) hasAuthenticatedDashboardSession(r *http.Request) bool {
	oauthControl := s.oauthControl()
	if !oauthControl.configured() {
		return false
	}

	_, err := oauthControl.sessionFromRequest(r)
	return err == nil
}
