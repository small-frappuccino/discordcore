package control

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

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
	if cleanPath == "/manage" {
		cleanPath = dashboardRoutePrefix
	}
	if cleanPath == "/dashboard" {
		cleanPath = dashboardRoutePrefix
	}
	if strings.HasPrefix(cleanPath, dashboardLegacyRoutePrefix) {
		cleanPath = dashboardRoutePrefix + strings.TrimPrefix(cleanPath, dashboardLegacyRoutePrefix)
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
