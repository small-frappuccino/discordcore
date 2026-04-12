package control

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

var errDiscordOAuthUnavailable = errors.New("discord oauth is not configured")

type discordOAuthControlService struct {
	server   *Server
	provider *discordOAuthProvider
}

func newDiscordOAuthControlService(server *Server, provider *discordOAuthProvider) *discordOAuthControlService {
	return &discordOAuthControlService{
		server:   server,
		provider: provider,
	}
}

func (s *Server) oauthControl() *discordOAuthControlService {
	if s == nil {
		return nil
	}
	return newDiscordOAuthControlService(s, s.discordOAuth)
}

func (s *Server) oauthControlWithProvider(provider *discordOAuthProvider) *discordOAuthControlService {
	if s == nil {
		return nil
	}
	return newDiscordOAuthControlService(s, provider)
}

func (svc *discordOAuthControlService) configured() bool {
	return svc != nil && svc.provider != nil
}

func (svc *discordOAuthControlService) sessionFromRequest(r *http.Request) (discordOAuthSession, error) {
	if !svc.configured() {
		return discordOAuthSession{}, errDiscordOAuthUnavailable
	}
	return svc.provider.sessionFromRequest(r)
}

func (svc *discordOAuthControlService) validateSessionCSRFToken(r *http.Request, session discordOAuthSession) error {
	if !svc.configured() {
		return errDiscordOAuthUnavailable
	}
	return svc.provider.validateSessionCSRFToken(r, session)
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

	state, err := svc.provider.generateState()
	if err != nil {
		log.ApplicationLogger().Error("Failed to generate OAuth state", "operation", "control.oauth.login.generate_state", "err", err)
		http.Error(w, "failed to initialize oauth state", http.StatusInternalServerError)
		return
	}
	svc.provider.setStateCookie(w, r, state)
	if next := sanitizeControlRedirectTarget(r.URL.Query().Get("next")); next != "" {
		svc.provider.setNextCookie(w, r, next)
	} else {
		svc.provider.clearNextCookie(w, r)
	}

	redirectURL, err := svc.provider.buildAuthorizationURL(state)
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
	defer svc.provider.clearStateCookie(w, r)
	defer svc.provider.clearNextCookie(w, r)

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
	if err := svc.provider.validateState(r, providedState); err != nil {
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

	tokenPayload, status, err := svc.provider.exchangeCode(ctx, code)
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

	accessToken, refreshToken, tokenType, scopes, tokenTTL, err := parseTokenResponse(tokenPayload, svc.provider.scopes)
	if err != nil {
		log.ApplicationLogger().Warn("Discord OAuth token payload validation failed", "operation", "control.oauth.callback.parse_token", "err", err)
		http.Error(w, "invalid oauth token response", http.StatusBadGateway)
		return
	}

	user, err := svc.provider.fetchUser(ctx, accessToken, tokenType)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth user fetch failed", "operation", "control.oauth.callback.fetch_user", "err", err)
		http.Error(w, "failed to resolve oauth user", http.StatusBadGateway)
		return
	}

	session, err := svc.provider.sessions.Create(
		user,
		scopes,
		accessToken,
		refreshToken,
		tokenType,
		tokenTTL,
		svc.provider.sessionTTL,
	)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth session creation failed", "operation", "control.oauth.callback.create_session", "err", err)
		http.Error(w, "failed to create oauth session", http.StatusInternalServerError)
		return
	}
	svc.provider.setSessionCookie(w, r, session.ID, session.ExpiresAt)
	if next := svc.provider.nextFromRequest(r); next != "" {
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
		"dashboard_url":    svc.server.publicDashboardURL(dashboardRoutePrefix),
		"login_url":        "",
	}
	if !svc.configured() {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, response)
		return
	}

	response["login_url"] = svc.server.publicDiscordOAuthLoginURL(next)
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

	if err := svc.provider.sessions.Delete(session.ID); err != nil {
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
	svc.provider.clearSessionCookie(w, r)

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
	return svc.resolveAccessibleGuildsWithCache(ctx, session, true, true)
}

func (svc *discordOAuthControlService) resolveAccessibleGuildsFresh(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return svc.resolveAccessibleGuildsWithCache(ctx, session, false, false)
}

func (svc *discordOAuthControlService) resolveAccessibleGuildsRefreshed(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	return svc.resolveAccessibleGuildsWithCache(ctx, session, false, true)
}

func (svc *discordOAuthControlService) resolveAccessibleGuildsWithCache(
	ctx context.Context,
	session discordOAuthSession,
	useCache bool,
	storeCache bool,
) ([]accessibleGuildResponse, error) {
	if !svc.configured() {
		return nil, errDiscordOAuthUnavailable
	}
	if useCache {
		if cached, ok := svc.cachedAccessibleGuilds(session.ID, time.Now().UTC()); ok {
			return svc.materializeAccessibleGuilds(cached, session.User.ID), nil
		}
	}

	botGuildSet, err := svc.server.resolveBotGuildIDSet(ctx)
	if err != nil {
		if !errors.Is(err, errBotGuildIDsProviderUnavailable) {
			return nil, err
		}
		botGuildSet = map[string]struct{}{}
	}

	freshSession, err := svc.provider.ensureFreshSessionAccessToken(ctx, session)
	if err != nil {
		return nil, fmt.Errorf("resolve accessible guilds: refresh oauth access token: %w", err)
	}

	userGuilds, err := svc.provider.fetchUserGuilds(ctx, freshSession.AccessToken, freshSession.TokenType)
	if err != nil {
		return nil, fmt.Errorf("resolve accessible guilds: fetch user guilds: %w", err)
	}

	cachedGuilds := make([]cachedAccessibleGuild, 0, len(userGuilds))
	for _, guild := range userGuilds {
		guildID := strings.TrimSpace(guild.ID)
		if guildID == "" {
			continue
		}
		_, botPresent := botGuildSet[guildID]

		cachedGuilds = append(cachedGuilds, cachedAccessibleGuild{
			guild:      guild,
			botPresent: botPresent,
		})
	}

	if useCache || storeCache {
		svc.storeAccessibleGuilds(session, cachedGuilds, time.Now().UTC())
	}
	return svc.materializeAccessibleGuilds(cachedGuilds, session.User.ID), nil
}

func (svc *discordOAuthControlService) resolveManageableGuilds(
	ctx context.Context,
	session discordOAuthSession,
) ([]accessibleGuildResponse, error) {
	accessible, err := svc.resolveAccessibleGuilds(ctx, session)
	if err != nil {
		return nil, err
	}
	return filterAccessibleGuildsByLevel(accessible, guildAccessLevelWrite), nil
}

func (svc *discordOAuthControlService) resolveGuildAccessLevel(
	guild discordOAuthGuild,
	userID string,
) (guildAccessLevel, bool) {
	if isGuildManageableByUser(guild) {
		return guildAccessLevelWrite, true
	}
	if svc == nil || svc.server == nil || svc.server.configManager == nil {
		return "", false
	}

	guildID := strings.TrimSpace(guild.ID)
	userID = strings.TrimSpace(userID)
	if guildID == "" || userID == "" {
		return "", false
	}

	guildConfig := svc.server.configManager.GuildConfig(guildID)
	if guildConfig == nil {
		return "", false
	}
	if len(guildConfig.Roles.DashboardRead) == 0 && len(guildConfig.Roles.DashboardWrite) == 0 {
		return "", false
	}

	session, err := svc.server.discordSessionForGuild(guildID)
	if err != nil || session == nil {
		return "", false
	}

	member, err := resolveGuildMemberFromDiscordSession(session, guildID, userID)
	if err != nil || member == nil {
		return "", false
	}

	memberRoleSet := make(map[string]struct{}, len(member.Roles))
	for _, roleID := range member.Roles {
		roleID = strings.TrimSpace(roleID)
		if roleID == "" {
			continue
		}
		memberRoleSet[roleID] = struct{}{}
	}

	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardWrite) {
		return guildAccessLevelWrite, true
	}
	if matchesAnyRole(memberRoleSet, guildConfig.Roles.DashboardRead) {
		return guildAccessLevelRead, true
	}

	return "", false
}

func (svc *discordOAuthControlService) materializeAccessibleGuilds(
	guilds []cachedAccessibleGuild,
	userID string,
) []accessibleGuildResponse {
	if len(guilds) == 0 {
		return nil
	}

	out := make([]accessibleGuildResponse, 0, len(guilds))
	for _, cached := range guilds {
		accessLevel, ok := svc.resolveGuildAccessLevel(cached.guild, userID)
		if !ok {
			continue
		}

		icon := strings.TrimSpace(cached.guild.Icon)
		if !cached.botPresent {
			icon = ""
		}

		out = append(out, accessibleGuildResponse{
			ID:          strings.TrimSpace(cached.guild.ID),
			Name:        strings.TrimSpace(cached.guild.Name),
			Icon:        icon,
			BotPresent:  cached.botPresent,
			Owner:       cached.guild.Owner,
			Permissions: cached.guild.Permissions,
			AccessLevel: accessLevel,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		li := strings.ToLower(strings.TrimSpace(out[i].Name))
		lj := strings.ToLower(strings.TrimSpace(out[j].Name))
		if li == lj {
			return out[i].ID < out[j].ID
		}
		return li < lj
	})

	return out
}

func (svc *discordOAuthControlService) cachedAccessibleGuilds(
	sessionID string,
	now time.Time,
) ([]cachedAccessibleGuild, bool) {
	if svc == nil || svc.server == nil || svc.server.accessibleGuildsTTL <= 0 {
		return nil, false
	}

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false
	}

	svc.server.accessibleGuildsMu.RLock()
	entry, ok := svc.server.accessibleGuilds[sessionID]
	svc.server.accessibleGuildsMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
		svc.server.accessibleGuildsMu.Lock()
		delete(svc.server.accessibleGuilds, sessionID)
		svc.server.accessibleGuildsMu.Unlock()
		return nil, false
	}
	return cloneCachedAccessibleGuilds(entry.guilds), true
}

func (svc *discordOAuthControlService) storeAccessibleGuilds(
	session discordOAuthSession,
	guilds []cachedAccessibleGuild,
	now time.Time,
) {
	if svc == nil || svc.server == nil || svc.server.accessibleGuildsTTL <= 0 {
		return
	}

	sessionID := strings.TrimSpace(session.ID)
	if sessionID == "" {
		return
	}

	expiresAt := now.Add(svc.server.accessibleGuildsTTL)
	if !session.ExpiresAt.IsZero() && session.ExpiresAt.Before(expiresAt) {
		expiresAt = session.ExpiresAt
	}

	svc.server.accessibleGuildsMu.Lock()
	svc.server.accessibleGuilds[sessionID] = accessibleGuildCacheEntry{
		guilds:    cloneCachedAccessibleGuilds(guilds),
		expiresAt: expiresAt,
	}
	svc.server.accessibleGuildsMu.Unlock()
}

func (svc *discordOAuthControlService) invalidateAccessibleGuildsCache() {
	if svc == nil || svc.server == nil {
		return
	}

	svc.server.accessibleGuildsMu.Lock()
	svc.server.accessibleGuilds = make(map[string]accessibleGuildCacheEntry)
	svc.server.accessibleGuildsMu.Unlock()
}
