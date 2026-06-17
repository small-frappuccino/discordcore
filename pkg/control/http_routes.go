package control

import (
	"net/http"

	embeddedui "github.com/small-frappuccino/discordcore/ui"
)

func (s *Server) registerHTTPRoutes(mux *http.ServeMux) {
	if mux == nil {
		return
	}

	s.registerAuthRoutes(mux)
	s.registerAPIRoutes(mux)
	s.registerDashboardRoutes(mux)
}

func (s *Server) registerAuthRoutes(mux *http.ServeMux) {

	mux.HandleFunc("/v1/guilds/{guildId}/data/moderation", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDangerPurgeModeration(w, r)
	})
	mux.HandleFunc("/v1/guilds/{guildId}/data/qotd", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDangerPurgeQOTD(w, r)
	})
	mux.HandleFunc("/v1/guilds/{guildId}/data/engagement", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDangerPurgeEngagement(w, r)
	})
	mux.HandleFunc("/v1/guilds/{guildId}/data/cache", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDangerPurgeCache(w, r)
	})
	mux.HandleFunc("/v1/guilds/{guildId}/wipe", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.handleDangerWipeGuild(w, r)
	})

	mux.HandleFunc("/auth/discord/login", s.handleDiscordOAuthLogin)
	mux.HandleFunc("/auth/discord/status", s.handleDiscordOAuthStatus)
	mux.HandleFunc("/auth/discord/callback", s.handleDiscordOAuthCallback)
	mux.HandleFunc("/auth/me", s.handleDiscordOAuthMe)
	mux.HandleFunc("/auth/logout", s.handleDiscordOAuthLogout)
	mux.HandleFunc("/auth/guilds/access", s.handleDiscordOAuthAccessibleGuilds)
	mux.HandleFunc("/auth/guilds/manageable", s.handleDiscordOAuthManageableGuilds)
}

func (s *Server) registerAPIRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/users/@me/preferences", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleUserPreferencesGet(w, r)
		case http.MethodPut:
			s.handleUserPreferencesPut(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/features", s.handleFeatureRoutes)
	mux.HandleFunc("/v1/features/", s.handleFeatureRoutes)
	mux.HandleFunc("/v1/settings", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/settings/", s.handleSettingsRoutes)
	mux.HandleFunc("/v1/runtime-config", s.handleRuntimeConfig)
	mux.HandleFunc("/v1/guilds/", s.handleGuildConfigRoutes)
	mux.HandleFunc("/v1/events/guilds", s.handleGuildEvents)
	mux.HandleFunc("/v1/telemetry/performance", s.handleUITelemetry)
	mux.HandleFunc("/v1/telemetry/logs", s.handleUILogs)
	mux.HandleFunc("/v1/health/qotd", s.handleQOTDHealthRoute)
	mux.HandleFunc("/v1/health/moderation", s.handleModerationHealthRoute)
	mux.HandleFunc("/v1/health/monitoring", s.handleMonitoringHealthRoute)
	mux.HandleFunc("/v1/health/cache", s.handleCacheHealthRoute)
	mux.HandleFunc("/v1/health/live", s.handleLiveHealthRoute)
}

func (s *Server) registerDashboardRoutes(mux *http.ServeMux) {
	isOAuthConfigured := func() bool {
		if oauthControl := s.oauthControl(); oauthControl != nil {
			return oauthControl.configured()
		}
		return false
	}

	mux.Handle("/", newLandingHandler(isOAuthConfigured))
	mux.HandleFunc("/manage", s.handleManageRoot)

	// Public assets segregation in router required by Go 1.22
	if assets, err := embeddedui.DistFS(); err == nil {
		fs := http.FileServer(http.FS(assets))
		mux.Handle("GET "+dashboardRoutePrefix+"brand/", http.StripPrefix(dashboardRoutePrefix, fs))
		mux.Handle("GET "+dashboardLegacyRoutePrefix+"brand/", http.StripPrefix(dashboardLegacyRoutePrefix, fs))
	}

	publicControlURL := func() string {
		return s.publicControlURL("/")
	}

	mux.Handle(dashboardRoutePrefix, newProtectedEmbeddedDashboardHandler(s.newEmbeddedDashboardHandler(), isOAuthConfigured, publicControlURL, s.hasAuthenticatedDashboardSession))
	mux.HandleFunc("/dashboard", s.handleDashboardRoot)
	mux.Handle(dashboardLegacyRoutePrefix, newProtectedEmbeddedDashboardHandler(s.newEmbeddedDashboardHandler(), isOAuthConfigured, publicControlURL, s.hasAuthenticatedDashboardSession))
}

func (s *Server) handleManageRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.publicDashboardURL(dashboardRoutePrefix), http.StatusFound)
}

func (s *Server) handleDashboardRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, s.publicDashboardURL(dashboardRoutePrefix), http.StatusFound)
}
