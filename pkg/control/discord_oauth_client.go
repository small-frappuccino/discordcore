package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"
)

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
