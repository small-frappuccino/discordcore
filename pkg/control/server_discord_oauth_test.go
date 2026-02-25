package control

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiscordOAuthRoutesRequireConfig(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/auth/discord/login"},
		{method: http.MethodGet, path: "/auth/discord/callback?code=test&state=test"},
		{method: http.MethodGet, path: "/auth/me"},
		{method: http.MethodPost, path: "/auth/logout"},
	}

	for _, tc := range tests {
		rec := performHandlerJSONRequestWithAuth(t, handler, tc.method, tc.path, nil, "")
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 when oauth is not configured for %s %s, got %d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestDiscordOAuthLoginRedirect(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	}); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	rec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d body=%q", rec.Code, rec.Body.String())
	}

	state, stateCookie, redirect := parseOAuthLoginRedirect(t, rec)
	if state == "" {
		t.Fatal("expected non-empty state in oauth redirect")
	}
	if stateCookie == nil {
		t.Fatal("expected oauth state cookie")
	}
	if stateCookie.Value != state {
		t.Fatalf("expected cookie state to match query state, cookie=%q state=%q", stateCookie.Value, state)
	}

	query := redirect.Query()
	if query.Get("client_id") != "1234567890" {
		t.Fatalf("unexpected client_id query value: %q", query.Get("client_id"))
	}
	if query.Get("response_type") != "code" {
		t.Fatalf("unexpected response_type query value: %q", query.Get("response_type"))
	}
	if query.Get("redirect_uri") != "http://127.0.0.1:8080/auth/discord/callback" {
		t.Fatalf("unexpected redirect_uri query value: %q", query.Get("redirect_uri"))
	}

	scopeSet := toSet(strings.Fields(query.Get("scope")))
	if !scopeSet[discordOAuthScopeIdentify] || !scopeSet[discordOAuthScopeGuilds] {
		t.Fatalf("expected identify and guilds scopes, got %q", query.Get("scope"))
	}
	if scopeSet[discordOAuthScopeGuildsMembersRead] {
		t.Fatalf("did not expect %s without optional flag, got scope %q", discordOAuthScopeGuildsMembersRead, query.Get("scope"))
	}
}

func TestDiscordOAuthLoginRedirectIncludesGuildMembersScope(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{
		ClientID:                 "1234567890",
		ClientSecret:             "super-secret",
		RedirectURI:              "http://127.0.0.1:8080/auth/discord/callback",
		IncludeGuildsMembersRead: true,
	}); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	rec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 redirect, got %d body=%q", rec.Code, rec.Body.String())
	}

	_, _, redirect := parseOAuthLoginRedirect(t, rec)
	scopeSet := toSet(strings.Fields(redirect.Query().Get("scope")))
	if !scopeSet[discordOAuthScopeIdentify] || !scopeSet[discordOAuthScopeGuilds] || !scopeSet[discordOAuthScopeGuildsMembersRead] {
		t.Fatalf("expected scopes identify guilds %s, got %q", discordOAuthScopeGuildsMembersRead, redirect.Query().Get("scope"))
	}
}

func TestDiscordOAuthCallbackCreatesSessionAndHidesTokenPayload(t *testing.T) {
	t.Parallel()

	type exchangeRequest struct {
		method      string
		contentType string
		body        string
	}

	exchangeCapture := make(chan exchangeRequest, 1)
	userCapture := make(chan string, 1)
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			raw, _ := io.ReadAll(r.Body)
			exchangeCapture <- exchangeRequest{
				method:      r.Method,
				contentType: r.Header.Get("Content-Type"),
				body:        string(raw),
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			userCapture <- strings.TrimSpace(r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice","avatar":"abc123","discriminator":"0001"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:     discordAPI.URL + "/token",
		UserInfoURL:  discordAPI.URL + "/users/@me",
		HTTPClient:   discordAPI.Client(),
	}); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	req.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from callback, got %d body=%q", rec.Code, rec.Body.String())
	}

	var response struct {
		Status    string           `json:"status"`
		User      discordOAuthUser `json:"user"`
		Scopes    []string         `json:"scopes"`
		ExpiresAt string           `json:"expires_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode callback response: %v", err)
	}
	if response.Status != "ok" {
		t.Fatalf("unexpected response status: %+v", response)
	}
	if response.User.ID != "u1" || response.User.Username != "alice" {
		t.Fatalf("unexpected callback user payload: %+v", response.User)
	}
	if len(response.Scopes) == 0 {
		t.Fatalf("expected scopes in callback payload, got %+v", response)
	}
	if strings.TrimSpace(response.ExpiresAt) == "" {
		t.Fatalf("expected expires_at in callback payload, got %+v", response)
	}

	sessionCookie := findCookie(rec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q session cookie from callback", defaultDiscordOAuthSessionCookie)
	}
	if strings.TrimSpace(sessionCookie.Value) == "" {
		t.Fatalf("expected non-empty %q cookie value", defaultDiscordOAuthSessionCookie)
	}

	// Callback response must not expose oauth_token payload.
	if strings.Contains(strings.ToLower(rec.Body.String()), "access_token") {
		t.Fatalf("callback leaked access token payload: %q", rec.Body.String())
	}

	select {
	case capture := <-exchangeCapture:
		if capture.method != http.MethodPost {
			t.Fatalf("expected token exchange to use POST, got %s", capture.method)
		}
		if !strings.HasPrefix(strings.ToLower(capture.contentType), "application/x-www-form-urlencoded") {
			t.Fatalf("expected form-urlencoded content type, got %q", capture.contentType)
		}
		form, err := url.ParseQuery(capture.body)
		if err != nil {
			t.Fatalf("parse exchange body: %v body=%q", err, capture.body)
		}
		if form.Get("client_id") != "1234567890" {
			t.Fatalf("unexpected client_id form value: %q", form.Get("client_id"))
		}
		if form.Get("client_secret") != "super-secret" {
			t.Fatalf("unexpected client_secret form value: %q", form.Get("client_secret"))
		}
		if form.Get("grant_type") != "authorization_code" {
			t.Fatalf("unexpected grant_type form value: %q", form.Get("grant_type"))
		}
		if form.Get("code") != "auth-code-123" {
			t.Fatalf("unexpected code form value: %q", form.Get("code"))
		}
		if form.Get("redirect_uri") != "http://127.0.0.1:8080/auth/discord/callback" {
			t.Fatalf("unexpected redirect_uri form value: %q", form.Get("redirect_uri"))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected callback to call discord token endpoint")
	}

	select {
	case authz := <-userCapture:
		if authz != "Bearer access-token" {
			t.Fatalf("expected bearer auth on /users/@me request, got %q", authz)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected callback to call discord user info endpoint")
	}

	// Session cookie authenticates /auth/me.
	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected /auth/me to succeed with session cookie, got %d body=%q", meRec.Code, meRec.Body.String())
	}

	// Session auth also unlocks control routes without bearer.
	controlReq := httptest.NewRequest(http.MethodGet, "/v1/guilds/g1/partner-board", nil)
	controlReq.AddCookie(sessionCookie)
	controlRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(controlRec, controlReq)
	if controlRec.Code != http.StatusOK {
		t.Fatalf("expected control route to accept session auth, got %d body=%q", controlRec.Code, controlRec.Body.String())
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("expected /auth/logout to succeed, got %d body=%q", logoutRec.Code, logoutRec.Body.String())
	}
	if deleted := findCookie(logoutRec.Result().Cookies(), defaultDiscordOAuthSessionCookie); deleted == nil || deleted.MaxAge >= 0 {
		t.Fatalf("expected /auth/logout to clear session cookie, got %+v", deleted)
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meAfterLogoutReq.AddCookie(sessionCookie)
	meAfterLogoutRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(meAfterLogoutRec, meAfterLogoutReq)
	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected /auth/me to fail after logout, got %d body=%q", meAfterLogoutRec.Code, meAfterLogoutRec.Body.String())
	}
}

func TestDiscordOAuthCallbackRejectsInvalidState(t *testing.T) {
	t.Parallel()

	var exchangeCalls atomic.Int32
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		exchangeCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"unexpected"}`))
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:     discordAPI.URL + "/token",
		UserInfoURL:  discordAPI.URL + "/users/@me",
		HTTPClient:   discordAPI.Client(),
	}); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	_, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	req := httptest.NewRequest(http.MethodGet, "/auth/discord/callback?code=auth-code-123&state=wrong-state", nil)
	req.AddCookie(stateCookie)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid state, got %d body=%q", rec.Code, rec.Body.String())
	}
	if got := exchangeCalls.Load(); got != 0 {
		t.Fatalf("expected no token exchange call when state is invalid, got %d", got)
	}
}

func TestSetDiscordOAuthConfigValidatesRequiredFields(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{}); err == nil {
		t.Fatal("expected oauth config validation error for empty config")
	}
}

func parseOAuthLoginRedirect(t *testing.T, rec *httptest.ResponseRecorder) (string, *http.Cookie, *url.URL) {
	t.Helper()

	location := strings.TrimSpace(rec.Header().Get("Location"))
	if location == "" {
		t.Fatal("missing redirect location")
	}
	redirect, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse redirect URL: %v", err)
	}

	state := strings.TrimSpace(redirect.Query().Get("state"))
	if state == "" {
		t.Fatalf("missing state query parameter in redirect URL: %s", location)
	}

	stateCookie := findCookie(rec.Result().Cookies(), defaultDiscordOAuthStateCookieName)
	if stateCookie == nil {
		t.Fatalf("missing %q cookie", defaultDiscordOAuthStateCookieName)
	}

	return state, stateCookie, redirect
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	return nil
}

func toSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		out[trimmed] = true
	}
	return out
}
