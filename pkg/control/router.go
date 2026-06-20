package control

import (
	"net/http"
)

func (s *Server) registerRoutes(mux *http.ServeMux) {
	// API Routes (Go 1.22 Method Routing)
	mux.HandleFunc("GET /v1/features", s.handleGetFeatures)
	mux.HandleFunc("POST /v1/features", maxBytesMiddleware(s.handlePostFeatures))

	mux.HandleFunc("GET /v1/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /v1/runtime-config", maxBytesMiddleware(s.handlePutRuntimeConfig))

	mux.HandleFunc("GET /v1/guilds/{guildID}/channels", s.handleGetGuildChannels)
	mux.HandleFunc("GET /v1/guilds/{guildID}/roles", s.handleGetGuildRoles)

	// Generic Health Routes
	mux.HandleFunc("GET /v1/health/qotd", serveHealthRoute(s.qotdHealthResolver))
	mux.HandleFunc("GET /v1/health/moderation", serveHealthRoute(s.moderationHealthResolver))
	mux.HandleFunc("GET /v1/health/cache", serveHealthRoute(s.cacheHealthResolver))

	// OAuth Routes
	mux.HandleFunc("GET /auth/discord/login", s.handleOAuthLogin)
	mux.HandleFunc("GET /auth/discord/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /auth/logout", s.handleOAuthLogout)

	// Dashboard SPA
	dashboard := newDashboardHandler()

	// Mount on /manage/ and legacy /dashboard/
	mux.Handle("GET /manage/", http.StripPrefix("/manage", dashboard))
	mux.Handle("GET /dashboard/", http.StripPrefix("/dashboard", dashboard))

	// Root redirect
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/manage/", http.StatusFound)
		} else {
			http.NotFound(w, r)
		}
	})
}
