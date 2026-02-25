package control

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	defaultDiscordOAuthStateCookieName  = "alice_discord_oauth_state"
	defaultDiscordOAuthSessionCookie    = "alice_control_session"
	defaultDiscordOAuthStateTTL         = 10 * time.Minute
	defaultDiscordOAuthSessionTTL       = 12 * time.Hour
	defaultDiscordOAuthExchangeTimeout  = 10 * time.Second
)

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
	scopes            []string
	stateCookieName   string
	sessionCookieName string
	stateTTL          time.Duration
	sessionTTL        time.Duration
	httpClient        *http.Client
	sessions          *discordOAuthSessionStore
}

type discordOAuthUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator,omitempty"`
	GlobalName    string `json:"global_name,omitempty"`
	Avatar        string `json:"avatar,omitempty"`
}

type discordOAuthSession struct {
	ID        string
	User      discordOAuthUser
	Scopes    []string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type discordOAuthSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]discordOAuthSession
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

	return &discordOAuthProvider{
		clientID:          clientID,
		clientSecret:      clientSecret,
		redirectURI:       redirectURI,
		authorizationURL:  authorizationURL,
		tokenURL:          tokenURL,
		userInfoURL:       userInfoURL,
		scopes:            DiscordOAuthScopes(config.IncludeGuildsMembersRead),
		stateCookieName:   stateCookieName,
		sessionCookieName: sessionCookieName,
		stateTTL:          stateTTL,
		sessionTTL:        sessionTTL,
		httpClient:        client,
		sessions:          newDiscordOAuthSessionStore(),
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

	accessToken, tokenType, scopes, err := parseTokenResponse(tokenPayload, oauth.scopes)
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

	session, err := oauth.sessions.Create(user, scopes, oauth.sessionTTL)
	if err != nil {
		log.ApplicationLogger().Error("Discord OAuth session creation failed", "operation", "control.oauth.callback.create_session", "err", err)
		http.Error(w, "failed to create oauth session", http.StatusInternalServerError)
		return
	}
	oauth.setSessionCookie(w, r, session.ID, session.ExpiresAt)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"user":       session.User,
		"scopes":     session.Scopes,
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
		"expires_at": session.ExpiresAt.UTC().Format(time.RFC3339),
	})
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

	sessionID, err := oauth.sessionIDFromRequest(r)
	if err == nil {
		oauth.sessions.Delete(sessionID)
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

func (o *discordOAuthProvider) setStateCookie(w http.ResponseWriter, r *http.Request, state string) {
	secure := requestUsesHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name:     o.stateCookieName,
		Value:    state,
		Path:     "/auth/discord/callback",
		Expires:  time.Now().Add(o.stateTTL),
		MaxAge:   int(o.stateTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func (o *discordOAuthProvider) clearStateCookie(w http.ResponseWriter, r *http.Request) {
	secure := requestUsesHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name:     o.stateCookieName,
		Value:    "",
		Path:     "/auth/discord/callback",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
}

func (o *discordOAuthProvider) setSessionCookie(w http.ResponseWriter, r *http.Request, sessionID string, expiresAt time.Time) {
	secure := requestUsesHTTPS(r)
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
		Secure:   secure,
	})
}

func (o *discordOAuthProvider) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	secure := requestUsesHTTPS(r)
	http.SetCookie(w, &http.Cookie{
		Name:     o.sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})
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
	session, ok := o.sessions.Get(sessionID, time.Now())
	if !ok {
		return discordOAuthSession{}, fmt.Errorf("oauth session not found")
	}
	return session, nil
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBodyBytes))
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, defaultMaxBodyBytes))
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

func parseTokenResponse(payload map[string]any, fallbackScopes []string) (accessToken string, tokenType string, scopes []string, err error) {
	accessToken, _ = payload["access_token"].(string)
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		err = fmt.Errorf("missing access_token")
		return
	}

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

	return
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

func requestUsesHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https")
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

func newDiscordOAuthSessionStore() *discordOAuthSessionStore {
	return &discordOAuthSessionStore{
		sessions: map[string]discordOAuthSession{},
	}
}

func (s *discordOAuthSessionStore) Create(user discordOAuthUser, scopes []string, ttl time.Duration) (discordOAuthSession, error) {
	sessionID, err := generateRandomToken(32)
	if err != nil {
		return discordOAuthSession{}, fmt.Errorf("generate session id: %w", err)
	}

	now := time.Now()
	session := discordOAuthSession{
		ID:        sessionID,
		User:      user,
		Scopes:    slices.Clone(scopes),
		CreatedAt: now.UTC(),
		ExpiresAt: now.Add(ttl).UTC(),
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = session
	s.pruneExpiredLocked(now)
	return session, nil
}

func (s *discordOAuthSessionStore) Get(sessionID string, now time.Time) (discordOAuthSession, bool) {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()
	if !ok {
		return discordOAuthSession{}, false
	}
	if now.After(session.ExpiresAt) {
		s.Delete(sessionID)
		return discordOAuthSession{}, false
	}
	return session, true
}

func (s *discordOAuthSessionStore) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *discordOAuthSessionStore) pruneExpiredLocked(now time.Time) {
	for sessionID, session := range s.sessions {
		if now.After(session.ExpiresAt) {
			delete(s.sessions, sessionID)
		}
	}
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
