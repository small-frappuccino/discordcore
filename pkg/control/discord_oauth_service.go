package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

var errDiscordOAuthUnavailable = errors.New("discord oauth is not configured")

type discordOAuthControlService struct {
	providerSource             func() *discordOAuthProvider
	guildAccessResolver        *accessibleGuildResolver
	publicDashboardURL         func(string) string
	publicDiscordOAuthLoginURL func(string) string
}

func newDiscordOAuthControlService(
	providerSource func() *discordOAuthProvider,
	guildAccessResolver *accessibleGuildResolver,
	publicDashboardURL func(string) string,
	publicDiscordOAuthLoginURL func(string) string,
) *discordOAuthControlService {
	return &discordOAuthControlService{
		providerSource:             providerSource,
		guildAccessResolver:        guildAccessResolver,
		publicDashboardURL:         publicDashboardURL,
		publicDiscordOAuthLoginURL: publicDiscordOAuthLoginURL,
	}
}

func (s *Server) oauthControl() *discordOAuthControlService {
	if s == nil {
		return nil
	}
	return s.discordOAuthSvc
}

func (svc *discordOAuthControlService) configured() bool {
	return svc.provider() != nil
}

func (svc *discordOAuthControlService) sessionFromRequest(r *http.Request) (discordOAuthSession, error) {
	provider := svc.provider()
	if provider == nil {
		return discordOAuthSession{}, errDiscordOAuthUnavailable
	}
	return provider.sessionFromRequest(r)
}

func (svc *discordOAuthControlService) validateSessionCSRFToken(r *http.Request, session discordOAuthSession) error {
	provider := svc.provider()
	if provider == nil {
		return errDiscordOAuthUnavailable
	}
	return provider.validateSessionCSRFToken(r, session)
}

func (svc *discordOAuthControlService) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !svc.configured() {
		http.Error(w, "discord oauth is not configured", http.StatusServiceUnavailable)
		return
	}

	provider := svc.provider()
	state, err := provider.generateState()
	if err != nil {
		log.ApplicationLogger().Error("Failed to generate OAuth state", "operation", "control.oauth.login.generate_state", "err", err)
		http.Error(w, "failed to initialize oauth state", http.StatusInternalServerError)
		return
	}
	provider.setStateCookie(w, r, state)
	if next := sanitizeControlRedirectTarget(r.URL.Query().Get("next")); next != "" {
		provider.setNextCookie(w, r, next)
	} else {
		provider.clearNextCookie(w, r)
	}

	redirectURL, err := provider.buildAuthorizationURL(state)
	if err != nil {
		log.ApplicationLogger().Error("Failed to build OAuth login URL", "operation", "control.oauth.login.build_url", "err", err)
		http.Error(w, "failed to build oauth redirect", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (svc *discordOAuthControlService) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !svc.configured() {
		http.Error(w, "discord oauth is not configured", http.StatusServiceUnavailable)
		return
	}
	provider := svc.provider()
	defer provider.clearStateCookie(w, r)
	defer provider.clearNextCookie(w, r)

	if oauthErr := strings.TrimSpace(r.URL.Query().Get("error")); oauthErr != "" {
		desc := strings.TrimSpace(r.URL.Query().Get("error_description"))
		message := fmt.Sprintf("discord oauth error: %s", oauthErr)
		if desc != "" {
			message = fmt.Sprintf("%s (%s)", message, desc)
		}
		http.Error(w, message, http.StatusBadRequest)
		return
	}

	providedState := strings.TrimSpace(r.URL.Query().Get("state"))
	if err := provider.validateState(r, providedState); err != nil {
		log.ApplicationLogger().Warn("Discord OAuth state validation failed", "operation", "control.oauth.callback.state_validation", "err", err)
		http.Error(w, "invalid oauth state", http.StatusBadRequest)
		return
	}

	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if code == "" {
		http.Error(w, "missing code query parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultDiscordOAuthExchangeTimeout)
	defer cancel()

	tokenPayload, status, err := provider.exchangeCode(ctx, code)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth token exchange failed", "operation", "control.oauth.callback.exchange_token", "status", status, "err", err)
		if status < 400 {
			status = http.StatusBadGateway
		}

		response := map[string]any{
			"status":  "error",
			"message": "discord oauth token exchange failed",
		}
		if len(tokenPayload) > 0 {
			response["discord_error"] = tokenPayload
		}

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, status, response)
		return
	}

	accessToken, refreshToken, tokenType, scopes, tokenTTL, err := parseTokenResponse(tokenPayload, provider.scopes)
	if err != nil {
		log.ApplicationLogger().Warn("Discord OAuth token payload validation failed", "operation", "control.oauth.callback.parse_token", "err", err)
		http.Error(w, "invalid oauth token response", http.StatusBadGateway)
		return
	}

	user, err := provider.fetchUser(ctx, accessToken, tokenType)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth user fetch failed", "operation", "control.oauth.callback.fetch_user", "err", err)
		http.Error(w, "failed to resolve oauth user", http.StatusBadGateway)
		return
	}

	session, err := provider.sessions.Create(
		user,
		scopes,
		accessToken,
		refreshToken,
		tokenType,
		tokenTTL,
		provider.sessionTTL,
	)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth session creation failed", "operation", "control.oauth.callback.create_session", "err", err)
		http.Error(w, "failed to create oauth session", http.StatusInternalServerError)
		return
	}
	provider.setSessionCookie(w, r, session.ID, session.ExpiresAt)
	if next := provider.nextFromRequest(r); next != "" {
		http.Redirect(w, r, next, http.StatusFound)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"user":       session.User,
		"scopes":     session.Scopes,
		"csrf_token": session.CSRFToken,
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (svc *discordOAuthControlService) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := svc.sessionFromRequest(r)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, errDiscordOAuthUnavailable) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, http.StatusText(status), status)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"user":       session.User,
		"scopes":     session.Scopes,
		"csrf_token": session.CSRFToken,
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (svc *discordOAuthControlService) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	next := sanitizeControlRedirectTarget(r.URL.Query().Get("next"))
	response := map[string]any{
		"status":           "ok",
		"oauth_configured": svc.configured(),
		"authenticated":    false,
		"dashboard_url":    "",
		"login_url":        "",
	}
	if svc.publicDashboardURL != nil {
		response["dashboard_url"] = svc.publicDashboardURL(dashboardRoutePrefix)
	}
	if !svc.configured() {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, response)
		return
	}

	if svc.publicDiscordOAuthLoginURL != nil {
		response["login_url"] = svc.publicDiscordOAuthLoginURL(next)
	}
	session, err := svc.sessionFromRequest(r)
	if err == nil {
		response["authenticated"] = true
		response["user"] = session.User
		response["scopes"] = session.Scopes
		response["csrf_token"] = session.CSRFToken
		response["expires_at"] = session.ExpiresAt.UTC().Format(time.RFC3339)
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, response)
}

func (svc *discordOAuthControlService) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := svc.sessionFromRequest(r)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, errDiscordOAuthUnavailable) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, http.StatusText(status), status)
		return
	}
	if err := svc.validateSessionCSRFToken(r, session); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	provider := svc.provider()
	if err := provider.sessions.Delete(session.ID); err != nil {
		log.ApplicationLogger().Error(
			"Discord OAuth logout session delete failed",
			"operation", "control.oauth.logout.delete_session",
			"userID", session.User.ID,
			"sessionID", session.ID,
			"err", err,
		)
		http.Error(w, "failed to logout", http.StatusInternalServerError)
		return
	}
	provider.clearSessionCookie(w, r)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (svc *discordOAuthControlService) handleGuildAccessList(
	w http.ResponseWriter,
	r *http.Request,
	writeOnly bool,
) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	session, err := svc.sessionFromRequest(r)
	if err != nil {
		status := http.StatusUnauthorized
		if errors.Is(err, errDiscordOAuthUnavailable) {
			status = http.StatusServiceUnavailable
		}
		http.Error(w, http.StatusText(status), status)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultAccessibleGuildsQuery)
	defer cancel()

	resolveAccessible := svc.resolveAccessibleGuilds
	if requestWantsFreshGuildAccess(r) {
		resolveAccessible = svc.resolveAccessibleGuildsRefreshed
	}

	accessible, err := resolveAccessible(ctx, session)
	if err != nil {
		if shouldSuppressAccessibleGuildsRequestError(r.Context(), err) {
			return
		}
		status := statusForAccessibleGuildsError(err)
		message := "failed to resolve accessible guilds"
		if status == http.StatusUnauthorized {
			message = "oauth session requires re-authentication"
		}
		log.ApplicationLogger().Error(
			"Failed to resolve accessible guilds",
			"operation", "control.oauth.guild_access.resolve",
			"userID", session.User.ID,
			"writeOnly", writeOnly,
			"err", err,
		)
		http.Error(w, message, status)
		return
	}

	if writeOnly {
		accessible = filterAccessibleGuildsByLevel(accessible, guildAccessLevelWrite)
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"guilds": accessible,
		"count":  len(accessible),
	})
}

func (svc *discordOAuthControlService) resolveAccessibleGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	if svc == nil || svc.guildAccessResolver == nil {
		return nil, errDiscordOAuthUnavailable
	}
	return svc.guildAccessResolver.ResolveAccessibleGuilds(ctx, session)
}

func (svc *discordOAuthControlService) resolveAccessibleGuildsRefreshed(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	if svc == nil || svc.guildAccessResolver == nil {
		return nil, errDiscordOAuthUnavailable
	}
	return svc.guildAccessResolver.ResolveAccessibleGuildsRefreshed(ctx, session)
}

func (svc *discordOAuthControlService) resolveManageableGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	if svc == nil || svc.guildAccessResolver == nil {
		return nil, errDiscordOAuthUnavailable
	}
	return svc.guildAccessResolver.ResolveManageableGuilds(ctx, session)
}

func (svc *discordOAuthControlService) invalidateAccessibleGuildsCache() {
	if svc == nil || svc.guildAccessResolver == nil {
		return
	}
	svc.guildAccessResolver.InvalidateCache()
}

func (svc *discordOAuthControlService) provider() *discordOAuthProvider {
	if svc == nil || svc.providerSource == nil {
		return nil
	}
	return svc.providerSource()
}
