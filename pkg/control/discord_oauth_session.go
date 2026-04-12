package control

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"
)

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
