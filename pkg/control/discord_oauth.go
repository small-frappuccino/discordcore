package control

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
)

const (
	defaultDiscordOAuthAuthorizationURL = "https://discord.com/oauth2/authorize"
	defaultDiscordOAuthTokenURL         = "https://discord.com/api/oauth2/token"
	defaultDiscordOAuthUserInfoURL      = "https://discord.com/api/users/@me"
	defaultDiscordOAuthUserGuildsURL    = "https://discord.com/api/users/@me/guilds"
	discordOAuthCSRFHeaderName          = "X-CSRF-Token"
	defaultDiscordOAuthStateCookieName  = "alice_discord_oauth_state"
	defaultDiscordOAuthNextCookieName   = "alice_discord_oauth_next"
	defaultDiscordOAuthSessionCookie    = "alice_control_session"
	defaultDiscordOAuthStateTTL         = 10 * time.Minute
	defaultDiscordOAuthSessionTTL       = 12 * time.Hour
	defaultDiscordOAuthExchangeTimeout  = 10 * time.Second
	defaultDiscordOAuthReadBodyLimit    = 1024 * 1024
	discordOAuthGuildsPageLimit         = 200
)

var errDiscordOAuthSessionReauthenticationRequired = errors.New("oauth session requires re-authentication")

const (
	discordOAuthScopeIdentify          = "identify"
	discordOAuthScopeGuilds            = "guilds"
	discordOAuthScopeGuildsMembersRead = "guilds.members.read"
)

// DiscordOAuthConfig configures Discord OAuth2 flow for control server routes.
type DiscordOAuthConfig struct {
	ClientID                 string
	ClientSecret             string
	RedirectURI              string
	IncludeGuildsMembersRead bool
	AuthorizationURL         string
	TokenURL                 string
	UserInfoURL              string
	UserGuildsURL            string
	SessionStorePath         string
	StateCookieName          string
	SessionCookieName        string
	StateTTL                 time.Duration
	SessionTTL               time.Duration
	HTTPClient               *http.Client
}

type discordOAuthProvider struct {
	clientID          string
	clientSecret      string
	redirectURI       string
	authorizationURL  string
	tokenURL          string
	userInfoURL       string
	userGuildsURL     string
	scopes            []string
	stateCookieName   string
	sessionCookieName string
	stateTTL          time.Duration
	sessionTTL        time.Duration
	httpClient        *http.Client
	sessions          discordOAuthSessionStore
	tokenRefreshMu    sync.Mutex
}

type discordOAuthUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator,omitempty"`
	GlobalName    string `json:"global_name,omitempty"`
	Avatar        string `json:"avatar,omitempty"`
}

type discordOAuthSession struct {
	ID                   string
	User                 discordOAuthUser
	Scopes               []string
	CSRFToken            string
	AccessToken          string
	RefreshToken         string
	AccessTokenExpiresAt time.Time
	TokenType            string
	CreatedAt            time.Time
	ExpiresAt            time.Time
}

type discordOAuthSessionStore interface {
	Create(
		user discordOAuthUser,
		scopes []string,
		accessToken string,
		refreshToken string,
		tokenType string,
		tokenTTL time.Duration,
		ttl time.Duration,
	) (discordOAuthSession, error)
	Get(sessionID string, now time.Time) (discordOAuthSession, bool, error)
	Save(session discordOAuthSession) error
	Delete(sessionID string) error
}

type discordOAuthSessionStoreFilePayload struct {
	Sessions []discordOAuthSession `json:"sessions"`
}

type discordOAuthSessionDiskStore struct {
	mu       sync.RWMutex
	path     string
	sessions map[string]discordOAuthSession
}

type discordOAuthGuild struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Icon        string `json:"icon,omitempty"`
	Owner       bool   `json:"owner"`
	Permissions int64  `json:"permissions,string"`
}

// SetDiscordOAuthConfig enables /auth/discord/login and /auth/discord/callback routes.
func (s *Server) SetDiscordOAuthConfig(config DiscordOAuthConfig) error {
	if s == nil {
		return nil
	}

	provider, err := newDiscordOAuthProvider(config)
	if err != nil {
		return fmt.Errorf("configure discord oauth: %w", err)
	}

	s.discordOAuth = provider
	return nil
}

func newDiscordOAuthProvider(config DiscordOAuthConfig) (*discordOAuthProvider, error) {
	clientID := strings.TrimSpace(config.ClientID)
	if clientID == "" {
		return nil, fmt.Errorf("client id is required")
	}

	clientSecret := strings.TrimSpace(config.ClientSecret)
	if clientSecret == "" {
		return nil, fmt.Errorf("client secret is required")
	}

	redirectURI := strings.TrimSpace(config.RedirectURI)
	if redirectURI == "" {
		return nil, fmt.Errorf("redirect uri is required")
	}
	if _, err := parseAbsoluteURL(redirectURI); err != nil {
		return nil, fmt.Errorf("invalid redirect uri: %w", err)
	}

	authorizationURL := strings.TrimSpace(config.AuthorizationURL)
	if authorizationURL == "" {
		authorizationURL = defaultDiscordOAuthAuthorizationURL
	}
	if _, err := parseAbsoluteURL(authorizationURL); err != nil {
		return nil, fmt.Errorf("invalid authorization url: %w", err)
	}

	tokenURL := strings.TrimSpace(config.TokenURL)
	if tokenURL == "" {
		tokenURL = defaultDiscordOAuthTokenURL
	}
	if _, err := parseAbsoluteURL(tokenURL); err != nil {
		return nil, fmt.Errorf("invalid token url: %w", err)
	}

	userInfoURL := strings.TrimSpace(config.UserInfoURL)
	if userInfoURL == "" {
		userInfoURL = defaultDiscordOAuthUserInfoURL
	}
	if _, err := parseAbsoluteURL(userInfoURL); err != nil {
		return nil, fmt.Errorf("invalid user info url: %w", err)
	}

	userGuildsURL := strings.TrimSpace(config.UserGuildsURL)
	if userGuildsURL == "" {
		userGuildsURL = defaultDiscordOAuthUserGuildsURL
	}
	if _, err := parseAbsoluteURL(userGuildsURL); err != nil {
		return nil, fmt.Errorf("invalid user guilds url: %w", err)
	}

	stateCookieName := strings.TrimSpace(config.StateCookieName)
	if stateCookieName == "" {
		stateCookieName = defaultDiscordOAuthStateCookieName
	}

	sessionCookieName := strings.TrimSpace(config.SessionCookieName)
	if sessionCookieName == "" {
		sessionCookieName = defaultDiscordOAuthSessionCookie
	}

	stateTTL := config.StateTTL
	if stateTTL <= 0 {
		stateTTL = defaultDiscordOAuthStateTTL
	}

	sessionTTL := config.SessionTTL
	if sessionTTL <= 0 {
		sessionTTL = defaultDiscordOAuthSessionTTL
	}

	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultDiscordOAuthExchangeTimeout}
	}

	sessionStore, err := newDiscordOAuthSessionStore(strings.TrimSpace(config.SessionStorePath))
	if err != nil {
		return nil, fmt.Errorf("configure oauth session store: %w", err)
	}

	return &discordOAuthProvider{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       redirectURI,
		authorizationURL:  authorizationURL,
		tokenURL:          tokenURL,
		userInfoURL:       userInfoURL,
		userGuildsURL:     userGuildsURL,
		scopes:            DiscordOAuthScopes(config.IncludeGuildsMembersRead),
		stateCookieName:   stateCookieName,
		sessionCookieName: sessionCookieName,
		stateTTL:          stateTTL,
		sessionTTL:        sessionTTL,
		httpClient:        client,
		sessions:          sessionStore,
	}, nil
}

func (s *Server) handleDiscordOAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oauth, ok := s.requireDiscordOAuth(w)
	if !ok {
		return
	}

	state, err := oauth.generateState()
	if err != nil {
		log.ApplicationLogger().Error("Failed to generate OAuth state", "operation", "control.oauth.login.generate_state", "err", err)
		http.Error(w, "failed to initialize oauth state", http.StatusInternalServerError)
		return
	}
	oauth.setStateCookie(w, r, state)
	if next := sanitizeControlRedirectTarget(r.URL.Query().Get("next")); next != "" {
		oauth.setNextCookie(w, r, next)
	} else {
		oauth.clearNextCookie(w, r)
	}

	redirectURL, err := oauth.buildAuthorizationURL(state)
	if err != nil {
		log.ApplicationLogger().Error("Failed to build OAuth login URL", "operation", "control.oauth.login.build_url", "err", err)
		http.Error(w, "failed to build oauth redirect", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (s *Server) handleDiscordOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oauth, ok := s.requireDiscordOAuth(w)
	if !ok {
		return
	}
	defer oauth.clearStateCookie(w, r)
	defer oauth.clearNextCookie(w, r)

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
	if err := oauth.validateState(r, providedState); err != nil {
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

	tokenPayload, status, err := oauth.exchangeCode(ctx, code)
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

	accessToken, refreshToken, tokenType, scopes, tokenTTL, err := parseTokenResponse(tokenPayload, oauth.scopes)
	if err != nil {
		log.ApplicationLogger().Warn("Discord OAuth token payload validation failed", "operation", "control.oauth.callback.parse_token", "err", err)
		http.Error(w, "invalid oauth token response", http.StatusBadGateway)
		return
	}

	user, err := oauth.fetchUser(ctx, accessToken, tokenType)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth user fetch failed", "operation", "control.oauth.callback.fetch_user", "err", err)
		http.Error(w, "failed to resolve oauth user", http.StatusBadGateway)
		return
	}

	session, err := oauth.sessions.Create(
		user,
		scopes,
		accessToken,
		refreshToken,
		tokenType,
		tokenTTL,
		oauth.sessionTTL,
	)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth session creation failed", "operation", "control.oauth.callback.create_session", "err", err)
		http.Error(w, "failed to create oauth session", http.StatusInternalServerError)
		return
	}
	oauth.setSessionCookie(w, r, session.ID, session.ExpiresAt)
	if next := oauth.nextFromRequest(r); next != "" {
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

func (s *Server) handleDiscordOAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oauth, ok := s.requireDiscordOAuth(w)
	if !ok {
		return
	}

	session, err := oauth.sessionFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
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

func (s *Server) handleDiscordOAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	next := sanitizeControlRedirectTarget(r.URL.Query().Get("next"))
	response := map[string]any{
		"status":           "ok",
		"oauth_configured": s.discordOAuthConfigured(),
		"authenticated":    false,
		"dashboard_url":    s.publicDashboardURL(dashboardRoutePrefix),
		"login_url":        "",
	}
	if !s.discordOAuthConfigured() {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeJSON(w, http.StatusOK, response)
		return
	}

	response["login_url"] = s.publicDiscordOAuthLoginURL(next)
	session, err := s.discordOAuth.sessionFromRequest(r)
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

func (s *Server) handleDiscordOAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	oauth, ok := s.requireDiscordOAuth(w)
	if !ok {
		return
	}

	session, err := oauth.sessionFromRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := oauth.validateSessionCSRFToken(r, session); err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := oauth.sessions.Delete(session.ID); err != nil {
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
	oauth.clearSessionCookie(w, r)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func (s *Server) requireDiscordOAuth(w http.ResponseWriter) (*discordOAuthProvider, bool) {
	if s == nil {
		http.Error(w, "control server unavailable", http.StatusInternalServerError)
		return nil, false
	}
	if s.discordOAuth == nil {
		http.Error(w, "discord oauth is not configured", http.StatusServiceUnavailable)
		return nil, false
	}
	return s.discordOAuth, true
}

func (s *Server) discordOAuthConfigured() bool {
	return s != nil && s.discordOAuth != nil
}

func (o *discordOAuthProvider) generateState() (string, error) {
	state, err := generateRandomToken(32)
	if err != nil {
		return "", fmt.Errorf("generate state nonce: %w", err)
	}
	return state, nil
}

func (o *discordOAuthProvider) buildAuthorizationURL(state string) (string, error) {
	authURL, err := url.Parse(o.authorizationURL)
	if err != nil {
		return "", fmt.Errorf("parse authorization url: %w", err)
	}

	query := authURL.Query()
	query.Set("response_type", "code")
	query.Set("client_id", o.clientID)
	query.Set("redirect_uri", o.redirectURI)
	query.Set("scope", strings.Join(o.scopes, " "))
	query.Set("state", state)
	authURL.RawQuery = query.Encode()

	return authURL.String(), nil
}

func (o *discordOAuthProvider) setStateCookie(w http.ResponseWriter, _ *http.Request, state string) {
	http.SetCookie(w, &http.Cookie{
		Name:     o.stateCookieName,
		Value:    state,
		Path:     "/auth/discord/callback",
		Expires:  time.Now().Add(o.stateTTL),
		MaxAge:   int(o.stateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func (o *discordOAuthProvider) setNextCookie(w http.ResponseWriter, _ *http.Request, next string) {
	http.SetCookie(w, &http.Cookie{
		Name:     defaultDiscordOAuthNextCookieName,
		Value:    next,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   int(o.stateTTL.Seconds()),
	})
}

func (o *discordOAuthProvider) clearStateCookie(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     o.stateCookieName,
		Value:    "",
		Path:     "/auth/discord/callback",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func (o *discordOAuthProvider) clearNextCookie(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     defaultDiscordOAuthNextCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
		MaxAge:   -1,
	})
}

func (o *discordOAuthProvider) setSessionCookie(w http.ResponseWriter, _ *http.Request, sessionID string, expiresAt time.Time) {
	ttl := time.Until(expiresAt)
	maxAge := int(ttl.Seconds())
	if maxAge < 0 {
		maxAge = 0
	}

	http.SetCookie(w, &http.Cookie{
		Name:     o.sessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func (o *discordOAuthProvider) nextFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(defaultDiscordOAuthNextCookieName)
	if err != nil {
		return ""
	}
	return sanitizeControlRedirectTarget(cookie.Value)
}

func (o *discordOAuthProvider) clearSessionCookie(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     o.sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   true,
	})
}

func sanitizeControlRedirectTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	target, err := url.Parse(trimmed)
	if err != nil || target.IsAbs() || target.Host != "" {
		return ""
	}

	cleanPath := path.Clean("/" + strings.TrimSpace(target.Path))
	if cleanPath == "/" {
		if target.RawQuery != "" {
			return cleanPath + "?" + target.RawQuery
		}
		return cleanPath
	}
	if cleanPath == "/dashboard" {
		cleanPath = dashboardRoutePrefix
	}
	if !strings.HasPrefix(cleanPath, dashboardRoutePrefix) {
		return ""
	}

	if target.RawQuery != "" {
		return cleanPath + "?" + target.RawQuery
	}
	return cleanPath
}

func buildDiscordOAuthLoginPath(next string) string {
	target := sanitizeControlRedirectTarget(next)
	if target == "" {
		target = dashboardRoutePrefix
	}
	return "/auth/discord/login?next=" + url.QueryEscape(target)
}

func (o *discordOAuthProvider) validateState(r *http.Request, provided string) error {
	provided = strings.TrimSpace(provided)
	if provided == "" {
		return fmt.Errorf("missing oauth state query parameter")
	}

	cookie, err := r.Cookie(o.stateCookieName)
	if err != nil {
		return fmt.Errorf("missing oauth state cookie: %w", err)
	}
	expected := strings.TrimSpace(cookie.Value)
	if expected == "" {
		return fmt.Errorf("empty oauth state cookie")
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return fmt.Errorf("oauth state mismatch")
	}

	return nil
}

func (o *discordOAuthProvider) sessionIDFromRequest(r *http.Request) (string, error) {
	cookie, err := r.Cookie(o.sessionCookieName)
	if err != nil {
		return "", fmt.Errorf("missing oauth session cookie: %w", err)
	}

	sessionID := strings.TrimSpace(cookie.Value)
	if sessionID == "" {
		return "", fmt.Errorf("empty oauth session id")
	}
	return sessionID, nil
}

func (o *discordOAuthProvider) sessionFromRequest(r *http.Request) (discordOAuthSession, error) {
	sessionID, err := o.sessionIDFromRequest(r)
	if err != nil {
		return discordOAuthSession{}, err
	}
	session, ok, err := o.sessions.Get(sessionID, time.Now())
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("oauth session lookup failed: %w", err)
	}
	if !ok {
		return discordOAuthSession{}, fmt.Errorf("oauth session not found")
	}
	return session, nil
}

func (o *discordOAuthProvider) validateSessionCSRFToken(r *http.Request, session discordOAuthSession) error {
	if !requiresSessionCSRFToken(r.Method) {
		return nil
	}

	expected := strings.TrimSpace(session.CSRFToken)
	if expected == "" {
		return fmt.Errorf("oauth session csrf token is empty")
	}
	provided := strings.TrimSpace(r.Header.Get(discordOAuthCSRFHeaderName))
	if provided == "" {
		return fmt.Errorf("missing csrf token header")
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return fmt.Errorf("invalid csrf token")
	}
	return nil
}

func (o *discordOAuthProvider) ensureFreshSessionAccessToken(ctx context.Context, session discordOAuthSession) (discordOAuthSession, error) {
	now := time.Now().UTC()
	if !accessTokenNeedsRefresh(session, now) {
		return session, nil
	}

	if strings.TrimSpace(session.RefreshToken) == "" {
		if err := o.sessions.Delete(session.ID); err != nil {
			return discordOAuthSession{}, fmt.Errorf(
				"drop oauth session with missing refresh token: %w",
				err,
			)
		}
		return discordOAuthSession{}, fmt.Errorf(
			"%w: oauth access token expired and refresh token is missing",
			errDiscordOAuthSessionReauthenticationRequired,
		)
	}

	o.tokenRefreshMu.Lock()
	defer o.tokenRefreshMu.Unlock()

	current, ok, err := o.sessions.Get(session.ID, time.Now())
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("reload oauth session for refresh: %w", err)
	}
	if !ok {
		return discordOAuthSession{}, fmt.Errorf("oauth session not found for refresh")
	}
	if !accessTokenNeedsRefresh(current, time.Now().UTC()) {
		return current, nil
	}

	payload, status, err := o.refreshAccessToken(ctx, current.RefreshToken)
	if err != nil {
		if status >= 400 && status < 500 {
			if deleteErr := o.sessions.Delete(current.ID); deleteErr != nil {
				return discordOAuthSession{}, fmt.Errorf(
					"refresh oauth token rejected (status %d) and failed to drop oauth session: %w",
					status,
					deleteErr,
				)
			}
			return discordOAuthSession{}, fmt.Errorf(
				"%w: refresh oauth token rejected (status %d)",
				errDiscordOAuthSessionReauthenticationRequired,
				status,
			)
		}
		return discordOAuthSession{}, fmt.Errorf("refresh oauth token (status %d): %w", status, err)
	}

	accessToken, refreshToken, tokenType, scopes, tokenTTL, err := parseTokenResponse(payload, current.Scopes)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("parse refreshed oauth token: %w", err)
	}
	if refreshToken == "" {
		refreshToken = current.RefreshToken
	}

	current.AccessToken = accessToken
	current.RefreshToken = refreshToken
	current.TokenType = tokenType
	current.Scopes = slices.Clone(scopes)
	current.AccessTokenExpiresAt = resolveAccessTokenExpiry(time.Now().UTC(), tokenTTL, current.ExpiresAt)

	if err := o.sessions.Save(current); err != nil {
		return discordOAuthSession{}, fmt.Errorf("persist refreshed oauth session: %w", err)
	}

	return current, nil
}

func accessTokenNeedsRefresh(session discordOAuthSession, now time.Time) bool {
	expiresAt := session.AccessTokenExpiresAt
	if expiresAt.IsZero() {
		return strings.TrimSpace(session.AccessToken) == ""
	}
	return !now.Before(expiresAt)
}

func (o *discordOAuthProvider) exchangeCode(ctx context.Context, code string) (map[string]any, int, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("authorization code is required")
	}

	form := url.Values{}
	form.Set("client_id", o.clientID)
	form.Set("client_secret", o.clientSecret)
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", o.redirectURI)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("build token exchange request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultDiscordOAuthReadBodyLimit))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read token exchange response: %w", err)
	}

	payload, err := parseDiscordOAuthPayload(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode token exchange response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return payload, resp.StatusCode, fmt.Errorf("discord token endpoint returned status %d", resp.StatusCode)
	}

	return payload, resp.StatusCode, nil
}

func (o *discordOAuthProvider) refreshAccessToken(ctx context.Context, refreshToken string) (map[string]any, int, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("refresh token is required")
	}

	form := url.Values{}
	form.Set("client_id", o.clientID)
	form.Set("client_secret", o.clientSecret)
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refreshToken)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("build token refresh request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("execute token refresh request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultDiscordOAuthReadBodyLimit))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read token refresh response: %w", err)
	}

	payload, err := parseDiscordOAuthPayload(body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("decode token refresh response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return payload, resp.StatusCode, fmt.Errorf("discord refresh token endpoint returned status %d", resp.StatusCode)
	}

	return payload, resp.StatusCode, nil
}

func (o *discordOAuthProvider) fetchUser(ctx context.Context, accessToken, tokenType string) (discordOAuthUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.userInfoURL, nil)
	if err != nil {
		return discordOAuthUser{}, fmt.Errorf("build user info request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, accessToken))

	resp, err := o.httpClient.Do(req)
	if err != nil {
		return discordOAuthUser{}, fmt.Errorf("execute user info request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultDiscordOAuthReadBodyLimit))
	if err != nil {
		return discordOAuthUser{}, fmt.Errorf("read user info response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discordOAuthUser{}, fmt.Errorf("discord user info endpoint returned status %d", resp.StatusCode)
	}

	var user discordOAuthUser
	if err := json.Unmarshal(body, &user); err != nil {
		return discordOAuthUser{}, fmt.Errorf("decode user info response: %w", err)
	}
	if strings.TrimSpace(user.ID) == "" || strings.TrimSpace(user.Username) == "" {
		return discordOAuthUser{}, fmt.Errorf("discord user info response missing required user fields")
	}

	return user, nil
}

func parseTokenResponse(payload map[string]any, fallbackScopes []string) (accessToken string, refreshToken string, tokenType string, scopes []string, tokenTTL time.Duration, err error) {
	accessToken, _ = payload["access_token"].(string)
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		err = fmt.Errorf("missing access_token")
		return
	}
	refreshToken, _ = payload["refresh_token"].(string)
	refreshToken = strings.TrimSpace(refreshToken)

	tokenType, _ = payload["token_type"].(string)
	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}

	scopeRaw, _ := payload["scope"].(string)
	scopeRaw = strings.TrimSpace(scopeRaw)
	if scopeRaw == "" {
		scopes = slices.Clone(fallbackScopes)
	} else {
		scopes = uniqueSortedTokens(scopeRaw)
	}

	if seconds, ok := parseTokenExpirySeconds(payload["expires_in"]); ok && seconds > 0 {
		tokenTTL = time.Duration(seconds) * time.Second
	}

	return
}

func parseTokenExpirySeconds(raw any) (int64, bool) {
	switch value := raw.(type) {
	case float64:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	case int64:
		if value <= 0 {
			return 0, false
		}
		return value, true
	case int:
		if value <= 0 {
			return 0, false
		}
		return int64(value), true
	case json.Number:
		v, err := value.Int64()
		if err != nil || v <= 0 {
			return 0, false
		}
		return v, true
	default:
		return 0, false
	}
}

func resolveAccessTokenExpiry(now time.Time, tokenTTL time.Duration, sessionExpiresAt time.Time) time.Time {
	now = now.UTC()
	expiresAt := now
	switch {
	case tokenTTL > 0:
		expiresAt = now.Add(tokenTTL)
	case !sessionExpiresAt.IsZero():
		expiresAt = sessionExpiresAt.UTC()
	default:
		expiresAt = now
	}

	if !sessionExpiresAt.IsZero() && expiresAt.After(sessionExpiresAt.UTC()) {
		return sessionExpiresAt.UTC()
	}
	return expiresAt
}

func parseDiscordOAuthPayload(raw []byte) (map[string]any, error) {
	if len(strings.TrimSpace(string(raw))) == 0 {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	if payload == nil {
		return map[string]any{}, nil
	}
	return payload, nil
}

func (o *discordOAuthProvider) fetchUserGuilds(ctx context.Context, accessToken, tokenType string) ([]discordOAuthGuild, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, fmt.Errorf("oauth access token is required")
	}

	tokenType = strings.TrimSpace(tokenType)
	if tokenType == "" {
		tokenType = "Bearer"
	}

	after := ""
	out := make([]discordOAuthGuild, 0, discordOAuthGuildsPageLimit)
	for page := 0; page < 1000; page++ {
		url, err := o.buildUserGuildsURL(after)
		if err != nil {
			return nil, fmt.Errorf("build user guilds URL: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("build user guilds request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, accessToken))

		resp, err := o.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("execute user guilds request: %w", err)
		}

		body, readErr := io.ReadAll(io.LimitReader(resp.Body, defaultDiscordOAuthReadBodyLimit))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read user guilds response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("discord user guilds endpoint returned status %d", resp.StatusCode)
		}

		var pageGuilds []discordOAuthGuild
		if err := json.Unmarshal(body, &pageGuilds); err != nil {
			return nil, fmt.Errorf("decode user guilds response: %w", err)
		}

		out = append(out, pageGuilds...)
		if len(pageGuilds) < discordOAuthGuildsPageLimit {
			return out, nil
		}

		nextAfter := strings.TrimSpace(pageGuilds[len(pageGuilds)-1].ID)
		if nextAfter == "" || nextAfter == after {
			return nil, fmt.Errorf("invalid pagination cursor returned by discord user guilds endpoint")
		}
		after = nextAfter
	}

	return nil, fmt.Errorf("discord user guilds pagination exceeded maximum page limit")
}

func (o *discordOAuthProvider) buildUserGuildsURL(after string) (string, error) {
	baseURL, err := url.Parse(o.userGuildsURL)
	if err != nil {
		return "", fmt.Errorf("parse user guilds url: %w", err)
	}

	query := baseURL.Query()
	query.Set("limit", fmt.Sprintf("%d", discordOAuthGuildsPageLimit))
	after = strings.TrimSpace(after)
	if after != "" {
		query.Set("after", after)
	}
	baseURL.RawQuery = query.Encode()
	return baseURL.String(), nil
}

func uniqueSortedTokens(raw string) []string {
	parts := strings.Fields(strings.TrimSpace(raw))
	if len(parts) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for token := range set {
		out = append(out, token)
	}
	slices.Sort(out)
	return out
}

func parseAbsoluteURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("absolute URL is required")
	}
	return parsed, nil
}

// DiscordOAuthScopes returns required Discord OAuth scopes with optional guild member scope.
func DiscordOAuthScopes(includeGuildMembersRead bool) []string {
	scopes := []string{
		discordOAuthScopeIdentify,
		discordOAuthScopeGuilds,
	}
	if includeGuildMembersRead {
		scopes = append(scopes, discordOAuthScopeGuildsMembersRead)
	}
	return scopes
}

func newDiscordOAuthSessionStore(path string) (discordOAuthSessionStore, error) {
	storePath := strings.TrimSpace(path)
	if storePath == "" {
		return nil, fmt.Errorf("oauth session store path is required")
	}

	store := &discordOAuthSessionDiskStore{
		path:     storePath,
		sessions: map[string]discordOAuthSession{},
	}
	if err := store.loadFromDisk(time.Now().UTC()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *discordOAuthSessionDiskStore) Create(
	user discordOAuthUser,
	scopes []string,
	accessToken string,
	refreshToken string,
	tokenType string,
	tokenTTL time.Duration,
	ttl time.Duration,
) (discordOAuthSession, error) {
	sessionID, err := generateRandomToken(32)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("generate session id: %w", err)
	}
	csrfToken, err := generateRandomToken(32)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("generate csrf token: %w", err)
	}

	now := time.Now().UTC()
	sessionExpiresAt := now.Add(ttl).UTC()
	session := discordOAuthSession{
		ID:                   sessionID,
		User:                 user,
		Scopes:               slices.Clone(scopes),
		CSRFToken:            csrfToken,
		AccessToken:          strings.TrimSpace(accessToken),
		RefreshToken:         strings.TrimSpace(refreshToken),
		AccessTokenExpiresAt: resolveAccessTokenExpiry(now, tokenTTL, sessionExpiresAt),
		TokenType:            strings.TrimSpace(tokenType),
		CreatedAt:            now,
		ExpiresAt:            sessionExpiresAt,
	}
	session = canonicalizeDiscordOAuthSession(session)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(now)
	s.sessions[sessionID] = session
	if err := s.persistLocked(); err != nil {
		delete(s.sessions, sessionID)
		return discordOAuthSession{}, err
	}
	return cloneDiscordOAuthSession(session), nil
}

func (s *discordOAuthSessionDiskStore) Get(sessionID string, now time.Time) (discordOAuthSession, bool, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return discordOAuthSession{}, false, nil
	}
	now = now.UTC()

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return discordOAuthSession{}, false, nil
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, sessionID)
		if err := s.persistLocked(); err != nil {
			return discordOAuthSession{}, false, err
		}
		return discordOAuthSession{}, false, nil
	}
	return cloneDiscordOAuthSession(session), true, nil
}

func (s *discordOAuthSessionDiskStore) Save(session discordOAuthSession) error {
	session = canonicalizeDiscordOAuthSession(session)
	if session.ID == "" {
		return fmt.Errorf("oauth session id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[session.ID]; !ok {
		return fmt.Errorf("oauth session not found")
	}

	now := time.Now().UTC()
	s.pruneExpiredLocked(now)
	if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
		delete(s.sessions, session.ID)
	} else {
		s.sessions[session.ID] = session
	}
	return s.persistLocked()
}

func (s *discordOAuthSessionDiskStore) Delete(sessionID string) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[sessionID]; !ok {
		return nil
	}
	delete(s.sessions, sessionID)
	return s.persistLocked()
}

func (s *discordOAuthSessionDiskStore) pruneExpiredLocked(now time.Time) bool {
	now = now.UTC()
	pruned := false
	for sessionID, session := range s.sessions {
		if !session.ExpiresAt.IsZero() && now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
			pruned = true
		}
	}
	return pruned
}

func (s *discordOAuthSessionDiskStore) loadFromDisk(now time.Time) error {
	now = now.UTC()
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read oauth session store file %q: %w", s.path, err)
	}

	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil
	}

	var payload discordOAuthSessionStoreFilePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return fmt.Errorf("decode oauth session store file %q: %w", s.path, err)
	}

	loaded := make(map[string]discordOAuthSession, len(payload.Sessions))
	for _, item := range payload.Sessions {
		session := canonicalizeDiscordOAuthSession(item)
		if session.ID == "" {
			continue
		}
		if session.ExpiresAt.IsZero() || now.After(session.ExpiresAt) {
			continue
		}
		loaded[session.ID] = session
	}
	s.sessions = loaded
	return nil
}

func (s *discordOAuthSessionDiskStore) persistLocked() error {
	dir := filepath.Dir(s.path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create oauth session store directory %q: %w", dir, err)
	}

	payload := discordOAuthSessionStoreFilePayload{
		Sessions: make([]discordOAuthSession, 0, len(s.sessions)),
	}
	for _, session := range s.sessions {
		payload.Sessions = append(payload.Sessions, cloneDiscordOAuthSession(session))
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode oauth session store file %q: %w", s.path, err)
	}

	tmpFile, err := os.CreateTemp(dir, "oauth_sessions_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp oauth session store file in %q: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(raw); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp oauth session store file %q: %w", tmpPath, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp oauth session store file %q: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp oauth session store file %q: %w", tmpPath, err)
	}

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale oauth session store file %q: %w", s.path, err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace oauth session store file %q: %w", s.path, err)
	}
	cleanupTmp = false
	return nil
}

func canonicalizeDiscordOAuthSession(session discordOAuthSession) discordOAuthSession {
	session.ID = strings.TrimSpace(session.ID)
	session.User.ID = strings.TrimSpace(session.User.ID)
	session.User.Username = strings.TrimSpace(session.User.Username)
	session.User.Discriminator = strings.TrimSpace(session.User.Discriminator)
	session.User.GlobalName = strings.TrimSpace(session.User.GlobalName)
	session.User.Avatar = strings.TrimSpace(session.User.Avatar)
	session.Scopes = slices.Clone(session.Scopes)
	session.CSRFToken = strings.TrimSpace(session.CSRFToken)
	session.AccessToken = strings.TrimSpace(session.AccessToken)
	session.RefreshToken = strings.TrimSpace(session.RefreshToken)
	session.TokenType = strings.TrimSpace(session.TokenType)
	if !session.AccessTokenExpiresAt.IsZero() {
		session.AccessTokenExpiresAt = session.AccessTokenExpiresAt.UTC()
	}
	if !session.CreatedAt.IsZero() {
		session.CreatedAt = session.CreatedAt.UTC()
	}
	if !session.ExpiresAt.IsZero() {
		session.ExpiresAt = session.ExpiresAt.UTC()
	}
	return session
}

func cloneDiscordOAuthSession(session discordOAuthSession) discordOAuthSession {
	return canonicalizeDiscordOAuthSession(session)
}

func generateRandomToken(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("token length must be positive")
	}
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func requiresSessionCSRFToken(method string) bool {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost, http.MethodPut, http.MethodDelete:
		return true
	default:
		return false
	}
}
