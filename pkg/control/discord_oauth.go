package control

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
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
	Permissions int64  `json:"permissions"`
}

func (g *discordOAuthGuild) UnmarshalJSON(data []byte) error {
	type rawDiscordOAuthGuild struct {
		ID          string          `json:"id"`
		Name        string          `json:"name"`
		Icon        string          `json:"icon,omitempty"`
		Owner       bool            `json:"owner"`
		Permissions json.RawMessage `json:"permissions"`
	}

	var raw rawDiscordOAuthGuild
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	permissions, err := parseDiscordOAuthPermissionBits(raw.Permissions)
	if err != nil {
		return fmt.Errorf("parse permissions: %w", err)
	}

	g.ID = raw.ID
	g.Name = raw.Name
	g.Icon = raw.Icon
	g.Owner = raw.Owner
	g.Permissions = permissions
	return nil
}

func parseDiscordOAuthPermissionBits(raw json.RawMessage) (int64, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return 0, nil
	}

	if trimmed[0] == '"' {
		var encoded string
		if err := json.Unmarshal(trimmed, &encoded); err != nil {
			return 0, fmt.Errorf("decode string value: %w", err)
		}
		encoded = strings.TrimSpace(encoded)
		if encoded == "" {
			return 0, nil
		}

		value, err := strconv.ParseInt(encoded, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse string value %q: %w", encoded, err)
		}
		return value, nil
	}

	var number json.Number
	if err := json.Unmarshal(trimmed, &number); err != nil {
		return 0, fmt.Errorf("decode numeric value: %w", err)
	}

	value, err := number.Int64()
	if err != nil {
		return 0, fmt.Errorf("parse numeric value %q: %w", number.String(), err)
	}
	return value, nil
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
