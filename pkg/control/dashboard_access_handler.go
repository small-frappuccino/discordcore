package control

import (
	"net/http"
	"strings"
)

const publicDashboardBrandAssetPath = "brand/alicebot.webp"

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
	if isPublicDashboardBrandAssetPath(r) {
		h.next.ServeHTTP(w, r)
		return
	}

	if !h.server.hasAuthenticatedDashboardSession(r) {
		if h.server.discordOAuthConfigured() {
			http.Redirect(w, r, buildDiscordOAuthLoginPath(dashboardRequestRedirectTarget(r)), http.StatusFound)
			return
		}
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

func dashboardRequestRedirectTarget(r *http.Request) string {
	if r == nil || r.URL == nil {
		return dashboardRoutePrefix
	}

	target := r.URL.Path
	if strings.TrimSpace(r.URL.RawQuery) != "" {
		target += "?" + r.URL.RawQuery
	}
	return target
}

func isPublicDashboardBrandAssetPath(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}

	assetPath, ok := normalizeDashboardAssetPath(r.URL.Path)
	return ok && assetPath == publicDashboardBrandAssetPath
}
