package control

import "net/http"

func (s *Server) registerHTTPRoutes(mux *http.ServeMux) {
	if mux == nil {
		return
	}

	s.registerAuthRoutes(mux)
	s.registerAPIRoutes(mux)
	s.registerDashboardRoutes(mux)
}

func (s *Server) registerAuthRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/auth/discord/login", s.handleDiscordOAuthLogin)
	mux.HandleFunc("/auth/discord/status", s.handleDiscordOAuthStatus)
	mux.HandleFunc("/auth/discord/callback", s.handleDiscordOAuthCallback)
	mux.HandleFunc("/auth/me", s.handleDiscordOAuthMe)
	mux.HandleFunc("/auth/logout", s.handleDiscordOAuthLogout)
	mux.HandleFunc("/auth/guilds/access", s.handleDiscordOAuthAccessibleGuilds)
	mux.HandleFunc("/auth/guilds/manageable", s.handleDiscordOAuthManageableGuilds)
}

func (s *Server) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/features", s.handleFeatureRoutes)
	mux.HandleFunc("/v1/features/", s.handleFeatureRoutes)
	mux.HandleFunc("/v1/settings", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/settings/", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/runtime-config", s.handleRuntimeConfig)
	mux.HandleFunc("/v1/guilds/", s.handleGuildConfigRoutes)
}

func (s *Server) registerDashboardRoutes(mux *http.ServeMux) {
	mux.Handle("/", newLandingHandler())
	mux.HandleFunc("/dashboard", s.handleDashboardRoot)
	mux.Handle(dashboardRoutePrefix, newProtectedEmbeddedDashboardHandler(s))
}

func (s *Server) handleDashboardRoot(w http.ResponseWriter, r *http.Request) {
	if !s.hasAuthenticatedDashboardSession(r) {
		if s.discordOAuthConfigured() {
			http.Redirect(w, r, s.publicDiscordOAuthLoginURL(dashboardRoutePrefix), http.StatusFound)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	http.Redirect(w, r, s.publicDashboardURL(dashboardRoutePrefix), http.StatusFound)
}
