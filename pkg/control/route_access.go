package control

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

func (s *Server) authorizeRequest(w http.ResponseWriter, r *http.Request) (requestAuthorization, bool) {
	if s == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return requestAuthorization{}, false
	}

	token := strings.TrimSpace(s.authBearerToken)
	oauthControl := s.oauthControl()
	oauthConfigured := oauthControl.configured()
	if token == "" && !oauthConfigured {
		http.Error(w, "control authentication is not configured", http.StatusServiceUnavailable)
		return requestAuthorization{}, false
	}

	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz != "" {
		if !strings.HasPrefix(authz, "Bearer ") {
			http.Error(w, "invalid authorization scheme", http.StatusUnauthorized)
			return requestAuthorization{}, false
		}
		provided := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
		if provided == "" {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return requestAuthorization{}, false
		}
		if token == "" {
			http.Error(w, "control bearer authentication is not configured", http.StatusServiceUnavailable)
			return requestAuthorization{}, false
		}
		if strings.TrimSpace(r.Header.Get("Origin")) != "" {
			http.Error(w, "bearer authentication is restricted to internal automation", http.StatusForbidden)
			return requestAuthorization{}, false
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "forbidden", http.StatusForbidden)
			return requestAuthorization{}, false
		}
		return requestAuthorization{mode: requestAuthModeBearer}, true
	}

	if oauthConfigured {
		if session, err := oauthControl.sessionFromRequest(r); err == nil {
			if err := oauthControl.validateSessionCSRFToken(r, session); err != nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return requestAuthorization{}, false
			}
			return requestAuthorization{
				mode:         requestAuthModeDiscordOAuthSession,
				oauthSession: session,
			}, true
		}
	}

	http.Error(w, "missing authorization", http.StatusUnauthorized)
	return requestAuthorization{}, false
}

func (s *Server) authorizeGlobalControlAccess(
	w http.ResponseWriter,
	r *http.Request,
	auth requestAuthorization,
	requiredAccess guildAccessLevel,
) bool {
	if requiredAccess == guildAccessLevelRead {
		return true
	}

	switch auth.mode {
	case requestAuthModeBearer:
		return true
	case requestAuthModeDiscordOAuthSession:
		s.log().LogAttrs(r.Context(), slog.LevelWarn,
			"Global control mutation denied",
			slog.String("operation", "control.route_access.authorize_global"),
			slog.String("guildID", ""),
			slog.String("channelID", ""),
			slog.String("userID", auth.oauthSession.User.ID),
			slog.String("reason", "global mutations require bearer authentication"),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	default:
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return false
	}
}

func (s *Server) authorizeGuildControlAccess(
	w http.ResponseWriter,
	r *http.Request,
	auth requestAuthorization,
	guildID string,
	requiredAccess guildAccessLevel,
) bool {
	guildID = strings.TrimSpace(guildID)
	if guildID == "" {
		http.Error(w, "guild_id is required", http.StatusBadRequest)
		return false
	}

	switch auth.mode {
	case requestAuthModeBearer:
		// Fixed bearer token remains available for trusted internal automation.
		return true
	case requestAuthModeDiscordOAuthSession:
		ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
		defer cancel()

		if r.Context().Err() != nil {
			return false
		}

		resolveAccessible := s.resolveAccessibleGuilds

		accessible, err := resolveAccessible(ctx, auth.oauthSession)
		if err != nil {
			if shouldSuppressAccessibleGuildsRequestError(r.Context(), err) {
				return false
			}
			status := statusForAccessibleGuildsError(err)
			message := "failed to authorize guild access"
			if status == http.StatusUnauthorized {
				message = "oauth session requires re-authentication"
			}
			s.log().LogAttrs(r.Context(), slog.LevelError,
				"Failed to authorize guild route access",
				slog.String("operation", "control.route_access.authorize_guild"),
				slog.String("userID", auth.oauthSession.User.ID),
				slog.String("guildID", guildID),
				slog.String("requiredAccess", string(requiredAccess)),
				slog.Any("err", err),
			)
			http.Error(w, message, status)
			return false
		}

		for _, guild := range accessible {
			if strings.TrimSpace(guild.ID) != guildID {
				continue
			}
			if guildAccessIncludes(guild.AccessLevel, requiredAccess) {
				return true
			}

			s.log().LogAttrs(r.Context(), slog.LevelWarn,
				"Guild route access denied",
				slog.String("operation", "control.route_access.authorize_guild"),
				slog.String("userID", auth.oauthSession.User.ID),
				slog.String("guildID", guildID),
				slog.String("reason", "insufficient dashboard access level"),
				slog.String("requiredAccess", string(requiredAccess)),
				slog.String("actualAccess", string(guild.AccessLevel)),
			)
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}

		s.log().LogAttrs(r.Context(), slog.LevelWarn,
			"Guild route access denied",
			slog.String("operation", "control.route_access.authorize_guild"),
			slog.String("userID", auth.oauthSession.User.ID),
			slog.String("guildID", guildID),
			slog.String("reason", "guild not accessible by authenticated user"),
			slog.String("requiredAccess", string(requiredAccess)),
		)
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	default:
		http.Error(w, "missing authorization", http.StatusUnauthorized)
		return false
	}
}

func requiredControlAccessLevel(method string) guildAccessLevel {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return guildAccessLevelRead
	default:
		return guildAccessLevelWrite
	}
}

func guildAccessIncludes(actual, required guildAccessLevel) bool {
	if required == guildAccessLevelRead {
		return actual == guildAccessLevelRead || actual == guildAccessLevelWrite
	}
	return actual == guildAccessLevelWrite
}
