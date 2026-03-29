package control

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/small-frappuccino/discordcore/pkg/files"
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

func TestDiscordOAuthStatusReportsUnavailableWithoutConfig(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	rec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/status?next=%2Fdashboard%2F", nil, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from oauth status without config, got %d body=%q", rec.Code, rec.Body.String())
	}

	var response struct {
		Status          string `json:"status"`
		OAuthConfigured bool   `json:"oauth_configured"`
		Authenticated   bool   `json:"authenticated"`
		DashboardURL    string `json:"dashboard_url"`
		LoginURL        string `json:"login_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode oauth status response: %v body=%q", err, rec.Body.String())
	}
	if response.Status != "ok" {
		t.Fatalf("unexpected oauth status payload: %+v", response)
	}
	if response.OAuthConfigured {
		t.Fatalf("expected oauth_configured=false, got %+v", response)
	}
	if response.Authenticated {
		t.Fatalf("expected authenticated=false, got %+v", response)
	}
	if response.DashboardURL != dashboardRoutePrefix {
		t.Fatalf("expected dashboard_url=%q, got %+v", dashboardRoutePrefix, response)
	}
	if response.LoginURL != "" {
		t.Fatalf("expected empty login_url without oauth config, got %+v", response)
	}
}

func TestDiscordOAuthGuildPermissionsUnmarshalAcceptsStringOrNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload string
		want    int64
	}{
		{
			name:    "string",
			payload: `{"id":"g1","name":"Guild One","owner":false,"permissions":"32"}`,
			want:    32,
		},
		{
			name:    "number",
			payload: `{"id":"g2","name":"Guild Two","owner":false,"permissions":8}`,
			want:    8,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var guild discordOAuthGuild
			if err := json.Unmarshal([]byte(tc.payload), &guild); err != nil {
				t.Fatalf("unmarshal guild payload: %v", err)
			}
			if guild.Permissions != tc.want {
				t.Fatalf("unexpected permissions: got=%d want=%d", guild.Permissions, tc.want)
			}
		})
	}
}

func TestDiscordOAuthStatusReportsConfiguredSessionState(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	signedOutRec := performHandlerJSONRequestWithAuth(
		t,
		srv.httpServer.Handler,
		http.MethodGet,
		"/auth/discord/status?next=%2Fdashboard%2Fsettings%3Ftab%3Dmoderation",
		nil,
		"",
	)
	if signedOutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 from oauth status with config, got %d body=%q", signedOutRec.Code, signedOutRec.Body.String())
	}

	var signedOut struct {
		OAuthConfigured bool   `json:"oauth_configured"`
		Authenticated   bool   `json:"authenticated"`
		DashboardURL    string `json:"dashboard_url"`
		LoginURL        string `json:"login_url"`
	}
	if err := json.NewDecoder(signedOutRec.Body).Decode(&signedOut); err != nil {
		t.Fatalf("decode signed-out oauth status response: %v body=%q", err, signedOutRec.Body.String())
	}
	if !signedOut.OAuthConfigured {
		t.Fatalf("expected oauth_configured=true, got %+v", signedOut)
	}
	if signedOut.Authenticated {
		t.Fatalf("expected authenticated=false without session, got %+v", signedOut)
	}
	if signedOut.DashboardURL != "http://127.0.0.1:8080/dashboard/" {
		t.Fatalf("unexpected dashboard_url for signed-out status: %+v", signedOut)
	}
	if signedOut.LoginURL != "http://127.0.0.1:8080/auth/discord/login?next=%2Fdashboard%2Fsettings%3Ftab%3Dmoderation" {
		t.Fatalf("unexpected login_url for signed-out status: %+v", signedOut)
	}

	sessionCookie := configureDashboardSession(t, srv)
	req := httptest.NewRequest(http.MethodGet, "/auth/discord/status?next=%2Fdashboard%2Fcontrol-panel", nil)
	req.AddCookie(sessionCookie)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from oauth status with session, got %d body=%q", rec.Code, rec.Body.String())
	}

	var signedIn struct {
		OAuthConfigured bool             `json:"oauth_configured"`
		Authenticated   bool             `json:"authenticated"`
		DashboardURL    string           `json:"dashboard_url"`
		LoginURL        string           `json:"login_url"`
		User            discordOAuthUser `json:"user"`
		CSRFToken       string           `json:"csrf_token"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&signedIn); err != nil {
		t.Fatalf("decode signed-in oauth status response: %v body=%q", err, rec.Body.String())
	}
	if !signedIn.OAuthConfigured || !signedIn.Authenticated {
		t.Fatalf("expected authenticated oauth status, got %+v", signedIn)
	}
	if signedIn.User.ID != "u1" || signedIn.User.Username != "alice" {
		t.Fatalf("unexpected oauth status user payload: %+v", signedIn.User)
	}
	if strings.TrimSpace(signedIn.CSRFToken) == "" {
		t.Fatalf("expected csrf token in signed-in oauth status, got %+v", signedIn)
	}
	if signedIn.DashboardURL != "http://127.0.0.1:8080/dashboard/" {
		t.Fatalf("unexpected dashboard_url for signed-in status: %+v", signedIn)
	}
	if signedIn.LoginURL != "http://127.0.0.1:8080/auth/discord/login?next=%2Fdashboard%2Fcontrol-panel" {
		t.Fatalf("unexpected login_url for signed-in status: %+v", signedIn)
	}
}

func TestDiscordOAuthStatusPrefersServerPublicOrigin(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetPublicOrigin("https://alice.localhost:8443"); err != nil {
		t.Fatalf("set public origin: %v", err)
	}
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "https://alice.localhost:8443/auth/discord/callback",
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"https://alice.localhost:8443/auth/discord/status?next=%2Fdashboard%2Fcontrol-panel",
		nil,
	)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from oauth status, got %d body=%q", rec.Code, rec.Body.String())
	}

	var payload struct {
		DashboardURL string `json:"dashboard_url"`
		LoginURL     string `json:"login_url"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode oauth status payload: %v body=%q", err, rec.Body.String())
	}
	if payload.DashboardURL != "https://alice.localhost:8443/dashboard/" {
		t.Fatalf("unexpected public dashboard url: %+v", payload)
	}
	if payload.LoginURL != "https://alice.localhost:8443/auth/discord/login?next=%2Fdashboard%2Fcontrol-panel" {
		t.Fatalf("unexpected public login url: %+v", payload)
	}
}

func TestDiscordOAuthLoginRedirect(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	})); err != nil {
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
	if !stateCookie.HttpOnly {
		t.Fatalf("expected oauth state cookie to be HttpOnly, got %+v", stateCookie)
	}
	if stateCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected oauth state cookie SameSite=Lax, got %v", stateCookie.SameSite)
	}
	if !stateCookie.Secure {
		t.Fatalf("expected oauth state cookie Secure=true, got %+v", stateCookie)
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
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:                 "1234567890",
		ClientSecret:             "super-secret",
		RedirectURI:              "http://127.0.0.1:8080/auth/discord/callback",
		IncludeGuildsMembersRead: true,
	})); err != nil {
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

func TestDiscordOAuthCallbackRedirectsToDashboardWhenRequested(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice","avatar":"abc123","discriminator":"0001"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login?next=%2Fdashboard%2F", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)
	nextCookie := findCookie(loginRec.Result().Cookies(), defaultDiscordOAuthNextCookieName)
	if nextCookie == nil {
		t.Fatalf("expected %q cookie from login redirect", defaultDiscordOAuthNextCookieName)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	req.AddCookie(stateCookie)
	req.AddCookie(nextCookie)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 callback redirect, got %d body=%q", rec.Code, rec.Body.String())
	}
	if location := strings.TrimSpace(rec.Header().Get("Location")); location != "/dashboard/" {
		t.Fatalf("expected callback redirect to /dashboard/, got %q", location)
	}
}

func TestDiscordOAuthCallbackRedirectsToRootWhenRequested(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice","avatar":"abc123","discriminator":"0001"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login?next=%2F", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)
	nextCookie := findCookie(loginRec.Result().Cookies(), defaultDiscordOAuthNextCookieName)
	if nextCookie == nil {
		t.Fatalf("expected %q cookie from login redirect", defaultDiscordOAuthNextCookieName)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	req.AddCookie(stateCookie)
	req.AddCookie(nextCookie)
	rec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302 callback redirect, got %d body=%q", rec.Code, rec.Body.String())
	}
	if location := strings.TrimSpace(rec.Header().Get("Location")); location != "/" {
		t.Fatalf("expected callback redirect to /, got %q", location)
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
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
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
		CSRFToken string           `json:"csrf_token"`
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
	if strings.TrimSpace(response.CSRFToken) == "" {
		t.Fatalf("expected csrf_token in callback payload, got %+v", response)
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
	if !sessionCookie.HttpOnly {
		t.Fatalf("expected oauth session cookie to be HttpOnly, got %+v", sessionCookie)
	}
	if sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("expected oauth session cookie SameSite=Lax, got %v", sessionCookie.SameSite)
	}
	if !sessionCookie.Secure {
		t.Fatalf("expected oauth session cookie Secure=true, got %+v", sessionCookie)
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
	var meResponse struct {
		Status    string `json:"status"`
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(meRec.Body).Decode(&meResponse); err != nil {
		t.Fatalf("decode /auth/me response: %v body=%q", err, meRec.Body.String())
	}
	if strings.TrimSpace(meResponse.CSRFToken) == "" {
		t.Fatalf("expected csrf_token in /auth/me response, got %+v", meResponse)
	}
	if response.CSRFToken != meResponse.CSRFToken {
		t.Fatalf("expected same csrf token from callback and /auth/me, callback=%q me=%q", response.CSRFToken, meResponse.CSRFToken)
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
	if logoutRec.Code != http.StatusForbidden {
		t.Fatalf("expected /auth/logout without csrf token to fail, got %d body=%q", logoutRec.Code, logoutRec.Body.String())
	}

	logoutWithCSRFReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	logoutWithCSRFReq.AddCookie(sessionCookie)
	logoutWithCSRFReq.Header.Set(discordOAuthCSRFHeaderName, meResponse.CSRFToken)
	logoutWithCSRFRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(logoutWithCSRFRec, logoutWithCSRFReq)
	if logoutWithCSRFRec.Code != http.StatusOK {
		t.Fatalf("expected /auth/logout to succeed, got %d body=%q", logoutWithCSRFRec.Code, logoutWithCSRFRec.Body.String())
	}
	if deleted := findCookie(logoutWithCSRFRec.Result().Cookies(), defaultDiscordOAuthSessionCookie); deleted == nil || deleted.MaxAge >= 0 {
		t.Fatalf("expected /auth/logout to clear session cookie, got %+v", deleted)
	} else if !deleted.Secure {
		t.Fatalf("expected cleared session cookie Secure=true, got %+v", deleted)
	}

	meAfterLogoutReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meAfterLogoutReq.AddCookie(sessionCookie)
	meAfterLogoutRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(meAfterLogoutRec, meAfterLogoutReq)
	if meAfterLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected /auth/me to fail after logout, got %d body=%q", meAfterLogoutRec.Code, meAfterLogoutRec.Body.String())
	}
}

func TestGuildRoutesRequireCSRFForOAuthSessionMutations(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	var callbackResp struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(callbackRec.Body).Decode(&callbackResp); err != nil {
		t.Fatalf("decode callback response: %v body=%q", err, callbackRec.Body.String())
	}
	if strings.TrimSpace(callbackResp.CSRFToken) == "" {
		t.Fatalf("expected csrf_token in callback response, got %+v", callbackResp)
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	targetPayload := strings.NewReader(`{"type":"channel_message","message_id":"123456789012345678","channel_id":"223456789012345678"}`)

	noCSRFReq := httptest.NewRequest(http.MethodPut, "/v1/guilds/g1/partner-board/target", targetPayload)
	noCSRFReq.AddCookie(sessionCookie)
	noCSRFReq.Header.Set("Content-Type", "application/json")
	noCSRFRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf token on mutable guild route, got %d body=%q", noCSRFRec.Code, noCSRFRec.Body.String())
	}

	withCSRFReq := httptest.NewRequest(http.MethodPut, "/v1/guilds/g1/partner-board/target", strings.NewReader(`{"type":"channel_message","message_id":"123456789012345678","channel_id":"223456789012345678"}`))
	withCSRFReq.AddCookie(sessionCookie)
	withCSRFReq.Header.Set("Content-Type", "application/json")
	withCSRFReq.Header.Set(discordOAuthCSRFHeaderName, callbackResp.CSRFToken)
	withCSRFRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(withCSRFRec, withCSRFReq)
	if withCSRFRec.Code != http.StatusOK {
		t.Fatalf("expected mutable guild route with csrf token to succeed, got %d body=%q", withCSRFRec.Code, withCSRFRec.Body.String())
	}
}

func TestRuntimeConfigRouteRequiresCSRFForOAuthSessionMutations(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	var callbackResp struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(callbackRec.Body).Decode(&callbackResp); err != nil {
		t.Fatalf("decode callback response: %v body=%q", err, callbackRec.Body.String())
	}
	if strings.TrimSpace(callbackResp.CSRFToken) == "" {
		t.Fatalf("expected csrf_token in callback response, got %+v", callbackResp)
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	noCSRFReq := httptest.NewRequest(http.MethodPost, "/v1/runtime-config", strings.NewReader(`{"bot_theme":"default"}`))
	noCSRFReq.AddCookie(sessionCookie)
	noCSRFReq.Header.Set("Content-Type", "application/json")
	noCSRFRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(noCSRFRec, noCSRFReq)
	if noCSRFRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 without csrf token on runtime-config mutation, got %d body=%q", noCSRFRec.Code, noCSRFRec.Body.String())
	}

	withCSRFReq := httptest.NewRequest(http.MethodPost, "/v1/runtime-config", strings.NewReader(`{"bot_theme":"default"}`))
	withCSRFReq.AddCookie(sessionCookie)
	withCSRFReq.Header.Set("Content-Type", "application/json")
	withCSRFReq.Header.Set(discordOAuthCSRFHeaderName, callbackResp.CSRFToken)
	withCSRFRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(withCSRFRec, withCSRFReq)
	if withCSRFRec.Code != http.StatusOK {
		t.Fatalf("expected runtime-config mutation with csrf token to succeed, got %d body=%q", withCSRFRec.Code, withCSRFRec.Body.String())
	}
}

func TestGuildRoutesDenyOAuthSessionWithoutGuildAuthorization(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":false,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	guildReq := httptest.NewRequest(http.MethodGet, "/v1/guilds/g1/partner-board", nil)
	guildReq.AddCookie(sessionCookie)
	guildRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(guildRec, guildReq)
	if guildRec.Code != http.StatusForbidden {
		t.Fatalf("expected guild route to reject oauth session without guild authorization, got %d body=%q", guildRec.Code, guildRec.Body.String())
	}
}

func TestGuildRoutesAllowReadOnlyOAuthAccessForGetAndDenyWrites(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":false,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, cm := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	srv.SetDiscordSessionProvider(func() *discordgo.Session {
		return newTestDiscordSessionWithGuildMembers("g1",
			&discordgo.Member{
				GuildID: "g1",
				User: &discordgo.User{
					ID:       "u1",
					Username: "alice",
				},
				Roles: []string{"reader-role"},
			},
		)
	})
	if _, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		for index := range cfg.Guilds {
			if strings.TrimSpace(cfg.Guilds[index].GuildID) != "g1" {
				continue
			}
			cfg.Guilds[index].Roles.DashboardRead = []string{"reader-role"}
			cfg.Guilds[index].Roles.DashboardWrite = nil
			return nil
		}
		cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
			GuildID: "g1",
			Roles: files.RolesConfig{
				DashboardRead: []string{"reader-role"},
			},
		})
		return nil
	}); err != nil {
		t.Fatalf("seed dashboard read roles: %v", err)
	}
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	var callbackResp struct {
		CSRFToken string `json:"csrf_token"`
	}
	if err := json.NewDecoder(callbackRec.Body).Decode(&callbackResp); err != nil {
		t.Fatalf("decode callback response: %v body=%q", err, callbackRec.Body.String())
	}
	if strings.TrimSpace(callbackResp.CSRFToken) == "" {
		t.Fatalf("expected csrf_token in callback response, got %+v", callbackResp)
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/guilds/g1/settings", nil)
	getReq.AddCookie(sessionCookie)
	getRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected GET guild settings to allow read-only oauth access, got %d body=%q", getRec.Code, getRec.Body.String())
	}

	putReq := httptest.NewRequest(
		http.MethodPut,
		"/v1/guilds/g1/settings",
		strings.NewReader(`{"roles":{"dashboard_read":["reader-role"]}}`),
	)
	putReq.AddCookie(sessionCookie)
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set(discordOAuthCSRFHeaderName, callbackResp.CSRFToken)
	putRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusForbidden {
		t.Fatalf("expected PUT guild settings to reject read-only oauth access, got %d body=%q", putRec.Code, putRec.Body.String())
	}
}

func TestDiscordOAuthGuildAccessEndpoints(t *testing.T) {
	t.Parallel()

	var (
		guildsQueryMu sync.Mutex
		guildsQueries []string
	)

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			limit := strings.TrimSpace(r.URL.Query().Get("limit"))
			after := strings.TrimSpace(r.URL.Query().Get("after"))

			guildsQueryMu.Lock()
			guildsQueries = append(guildsQueries, fmt.Sprintf("limit=%s after=%s", limit, after))
			guildsQueryMu.Unlock()

			if limit != "200" {
				http.Error(w, "invalid limit", http.StatusBadRequest)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			switch after {
			case "":
				page := make([]discordOAuthGuild, 0, 200)
				for i := 0; i < 200; i++ {
					guild := discordOAuthGuild{
						ID:   fmt.Sprintf("p%03d", i),
						Name: fmt.Sprintf("Page One %03d", i),
					}
					if i == 10 {
						guild.ID = "g-owner"
						guild.Name = "Owner Guild"
						guild.Owner = true
					}
					if i == 20 {
						guild.ID = "g-admin"
						guild.Name = "Admin Guild"
						guild.Permissions = discordgo.PermissionAdministrator
					}
					page = append(page, guild)
				}
				if err := json.NewEncoder(w).Encode(page); err != nil {
					t.Fatalf("encode first guilds page: %v", err)
				}
			case "p199":
				page := []discordOAuthGuild{
					{ID: "g-manage", Name: "Manage Guild", Permissions: discordgo.PermissionManageGuild},
					{ID: "g-read", Name: "Read Only", Permissions: 0},
					{ID: "g-other", Name: "Other Guild", Owner: true},
				}
				if err := json.NewEncoder(w).Encode(page); err != nil {
					t.Fatalf("encode second guilds page: %v", err)
				}
			default:
				http.Error(w, "unexpected after cursor", http.StatusBadRequest)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, cm := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g-owner", "g-admin", "g-manage", "g-read"}, nil
	})
	srv.SetDiscordSessionProvider(func() *discordgo.Session {
		return newTestDiscordSessionWithGuildMembers("g-read",
			&discordgo.Member{
				GuildID: "g-read",
				User: &discordgo.User{
					ID:       "u1",
					Username: "alice",
				},
				Roles: []string{"reader-role"},
			},
		)
	})
	if _, err := cm.UpdateConfig(func(cfg *files.BotConfig) error {
		cfg.Guilds = append(cfg.Guilds, files.GuildConfig{
			GuildID: "g-read",
			Roles: files.RolesConfig{
				DashboardRead: []string{"reader-role"},
			},
		})
		return nil
	}); err != nil {
		t.Fatalf("seed readable guild config: %v", err)
	}
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	accessReq := httptest.NewRequest(http.MethodGet, "/auth/guilds/access", nil)
	accessReq.AddCookie(sessionCookie)
	accessRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(accessRec, accessReq)
	if accessRec.Code != http.StatusOK {
		t.Fatalf("expected accessible guilds endpoint to succeed, got %d body=%q", accessRec.Code, accessRec.Body.String())
	}

	var accessResponse struct {
		Status string                    `json:"status"`
		Count  int                       `json:"count"`
		Guilds []accessibleGuildResponse `json:"guilds"`
	}
	if err := json.NewDecoder(accessRec.Body).Decode(&accessResponse); err != nil {
		t.Fatalf("decode accessible guilds response: %v body=%q", err, accessRec.Body.String())
	}
	if accessResponse.Status != "ok" {
		t.Fatalf("unexpected accessible response status: %+v", accessResponse)
	}
	if accessResponse.Count != 4 || len(accessResponse.Guilds) != 4 {
		t.Fatalf("expected 4 accessible guilds, got count=%d guilds=%+v", accessResponse.Count, accessResponse.Guilds)
	}

	gotAccessIDs := []string{
		accessResponse.Guilds[0].ID,
		accessResponse.Guilds[1].ID,
		accessResponse.Guilds[2].ID,
		accessResponse.Guilds[3].ID,
	}
	wantAccessIDs := []string{"g-admin", "g-manage", "g-owner", "g-read"}
	if strings.Join(gotAccessIDs, ",") != strings.Join(wantAccessIDs, ",") {
		t.Fatalf("unexpected accessible guild IDs: got=%v want=%v", gotAccessIDs, wantAccessIDs)
	}
	if accessResponse.Guilds[0].AccessLevel != guildAccessLevelWrite ||
		accessResponse.Guilds[1].AccessLevel != guildAccessLevelWrite ||
		accessResponse.Guilds[2].AccessLevel != guildAccessLevelWrite ||
		accessResponse.Guilds[3].AccessLevel != guildAccessLevelRead {
		t.Fatalf("unexpected accessible guild access levels: %+v", accessResponse.Guilds)
	}

	manageableReq := httptest.NewRequest(http.MethodGet, "/auth/guilds/manageable", nil)
	manageableReq.AddCookie(sessionCookie)
	manageableRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(manageableRec, manageableReq)
	if manageableRec.Code != http.StatusOK {
		t.Fatalf("expected manageable guilds endpoint to succeed, got %d body=%q", manageableRec.Code, manageableRec.Body.String())
	}

	var response struct {
		Status string                    `json:"status"`
		Count  int                       `json:"count"`
		Guilds []accessibleGuildResponse `json:"guilds"`
	}
	if err := json.NewDecoder(manageableRec.Body).Decode(&response); err != nil {
		t.Fatalf("decode manageable guilds response: %v body=%q", err, manageableRec.Body.String())
	}
	if response.Status != "ok" {
		t.Fatalf("unexpected response status: %+v", response)
	}
	if response.Count != 3 || len(response.Guilds) != 3 {
		t.Fatalf("expected 3 manageable guilds, got count=%d guilds=%+v", response.Count, response.Guilds)
	}

	gotIDs := []string{response.Guilds[0].ID, response.Guilds[1].ID, response.Guilds[2].ID}
	wantIDs := []string{"g-admin", "g-manage", "g-owner"}
	if strings.Join(gotIDs, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("unexpected manageable guild IDs: got=%v want=%v", gotIDs, wantIDs)
	}

	guildsQueryMu.Lock()
	defer guildsQueryMu.Unlock()
	if len(guildsQueries) != 2 {
		t.Fatalf("expected exactly two /users/@me/guilds calls, got %d (%v)", len(guildsQueries), guildsQueries)
	}
	if guildsQueries[0] != "limit=200 after=" {
		t.Fatalf("unexpected first guild query: %q", guildsQueries[0])
	}
	if guildsQueries[1] != "limit=200 after=p199" {
		t.Fatalf("unexpected second guild query: %q", guildsQueries[1])
	}
}

func TestDiscordOAuthManageableGuildsEndpointRequiresBotGuildProvider(t *testing.T) {
	t.Parallel()

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	manageableReq := httptest.NewRequest(http.MethodGet, "/auth/guilds/manageable", nil)
	manageableReq.AddCookie(sessionCookie)
	manageableRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(manageableRec, manageableReq)
	if manageableRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected manageable guilds endpoint to fail without bot guild provider, got %d body=%q", manageableRec.Code, manageableRec.Body.String())
	}
}

func TestDiscordOAuthSessionPersistsAcrossServerRestart(t *testing.T) {
	t.Parallel()

	sessionStorePath := filepath.Join(t.TempDir(), "oauth_sessions.json")
	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"access-token","refresh_token":"refresh-token","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	oauthCfg := DiscordOAuthConfig{
		ClientID:         "1234567890",
		ClientSecret:     "super-secret",
		RedirectURI:      "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:         discordAPI.URL + "/token",
		UserInfoURL:      discordAPI.URL + "/users/@me",
		SessionStorePath: sessionStorePath,
		HTTPClient:       discordAPI.Client(),
	}

	firstServer, _ := newControlTestServer(t)
	if err := firstServer.SetDiscordOAuthConfig(oauthCfg); err != nil {
		t.Fatalf("configure oauth on first server: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, firstServer.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	firstServer.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	secondServer, _ := newControlTestServer(t)
	if err := secondServer.SetDiscordOAuthConfig(oauthCfg); err != nil {
		t.Fatalf("configure oauth on second server: %v", err)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	secondServer.httpServer.Handler.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected /auth/me to succeed after server restart, got %d body=%q", meRec.Code, meRec.Body.String())
	}

	var meResp struct {
		Status string           `json:"status"`
		User   discordOAuthUser `json:"user"`
	}
	if err := json.NewDecoder(meRec.Body).Decode(&meResp); err != nil {
		t.Fatalf("decode /auth/me response: %v body=%q", err, meRec.Body.String())
	}
	if meResp.Status != "ok" || meResp.User.ID != "u1" {
		t.Fatalf("unexpected /auth/me response after restart: %+v", meResp)
	}
}

func TestDiscordOAuthManageableGuildsRefreshesAccessTokenWithRotation(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	refreshTokenInputs := make([]string, 0, 2)
	guildAuthHeaders := make([]string, 0, 2)

	discordAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			body, _ := io.ReadAll(r.Body)
			form, _ := url.ParseQuery(string(body))
			switch form.Get("grant_type") {
			case "authorization_code":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"access-initial","refresh_token":"refresh-1","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
			case "refresh_token":
				mu.Lock()
				refreshTokenInputs = append(refreshTokenInputs, form.Get("refresh_token"))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"access_token":"access-refreshed","refresh_token":"refresh-2","token_type":"Bearer","scope":"identify guilds","expires_in":3600}`))
			default:
				http.Error(w, "unsupported grant type", http.StatusBadRequest)
			}
		case "/users/@me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"u1","username":"alice","global_name":"Alice"}`))
		case "/users/@me/guilds":
			mu.Lock()
			guildAuthHeaders = append(guildAuthHeaders, strings.TrimSpace(r.Header.Get("Authorization")))
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":"g1","name":"Guild One","owner":true,"permissions":"0"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer discordAPI.Close()

	srv, _ := newControlTestServer(t)
	srv.SetBotGuildIDsProvider(func(_ context.Context) ([]string, error) {
		return []string{"g1"}, nil
	})
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:      "1234567890",
		ClientSecret:  "super-secret",
		RedirectURI:   "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:      discordAPI.URL + "/token",
		UserInfoURL:   discordAPI.URL + "/users/@me",
		UserGuildsURL: discordAPI.URL + "/users/@me/guilds",
		HTTPClient:    discordAPI.Client(),
	})); err != nil {
		t.Fatalf("configure oauth: %v", err)
	}

	loginRec := performHandlerJSONRequestWithAuth(t, srv.httpServer.Handler, http.MethodGet, "/auth/discord/login", nil, "")
	state, stateCookie, _ := parseOAuthLoginRedirect(t, loginRec)

	callbackReq := httptest.NewRequest(
		http.MethodGet,
		"/auth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state),
		nil,
	)
	callbackReq.AddCookie(stateCookie)
	callbackRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("expected oauth callback to succeed, got %d body=%q", callbackRec.Code, callbackRec.Body.String())
	}

	sessionCookie := findCookie(callbackRec.Result().Cookies(), defaultDiscordOAuthSessionCookie)
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie after callback", defaultDiscordOAuthSessionCookie)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	sessionReq.AddCookie(sessionCookie)
	session, err := srv.discordOAuth.sessionFromRequest(sessionReq)
	if err != nil {
		t.Fatalf("load oauth session: %v", err)
	}
	session.AccessToken = "access-expired"
	session.AccessTokenExpiresAt = time.Now().Add(-time.Minute).UTC()
	session.RefreshToken = "refresh-1"
	if err := srv.discordOAuth.sessions.Save(session); err != nil {
		t.Fatalf("save expired oauth session: %v", err)
	}

	manageableReq := httptest.NewRequest(http.MethodGet, "/auth/guilds/manageable", nil)
	manageableReq.AddCookie(sessionCookie)
	manageableRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(manageableRec, manageableReq)
	if manageableRec.Code != http.StatusOK {
		t.Fatalf("expected manageable guilds endpoint to succeed after refresh, got %d body=%q", manageableRec.Code, manageableRec.Body.String())
	}

	refreshed, ok, err := srv.discordOAuth.sessions.Get(session.ID, time.Now())
	if err != nil {
		t.Fatalf("reload refreshed oauth session: %v", err)
	}
	if !ok {
		t.Fatal("expected refreshed oauth session to remain stored")
	}
	if refreshed.AccessToken != "access-refreshed" {
		t.Fatalf("expected refreshed access token to be persisted, got %q", refreshed.AccessToken)
	}
	if refreshed.RefreshToken != "refresh-2" {
		t.Fatalf("expected rotated refresh token to be persisted, got %q", refreshed.RefreshToken)
	}
	if !refreshed.AccessTokenExpiresAt.After(time.Now().UTC()) {
		t.Fatalf("expected refreshed access token expiry in the future, got %s", refreshed.AccessTokenExpiresAt.UTC().Format(time.RFC3339))
	}

	mu.Lock()
	if len(refreshTokenInputs) != 1 {
		mu.Unlock()
		t.Fatalf("expected one refresh token exchange, got %d (%v)", len(refreshTokenInputs), refreshTokenInputs)
	}
	if refreshTokenInputs[0] != "refresh-1" {
		mu.Unlock()
		t.Fatalf("expected refresh exchange to use previous refresh token, got %v", refreshTokenInputs)
	}
	if len(guildAuthHeaders) == 0 {
		mu.Unlock()
		t.Fatal("expected at least one /users/@me/guilds call")
	}
	lastAuthHeader := guildAuthHeaders[len(guildAuthHeaders)-1]
	mu.Unlock()
	if lastAuthHeader != "Bearer access-refreshed" {
		t.Fatalf("expected /users/@me/guilds to use refreshed access token, got %q", lastAuthHeader)
	}

	secondManageableReq := httptest.NewRequest(http.MethodGet, "/auth/guilds/manageable", nil)
	secondManageableReq.AddCookie(sessionCookie)
	secondManageableRec := httptest.NewRecorder()
	srv.httpServer.Handler.ServeHTTP(secondManageableRec, secondManageableReq)
	if secondManageableRec.Code != http.StatusOK {
		t.Fatalf("expected second manageable guilds call to succeed, got %d body=%q", secondManageableRec.Code, secondManageableRec.Body.String())
	}
	mu.Lock()
	defer mu.Unlock()
	if len(refreshTokenInputs) != 1 {
		t.Fatalf("expected no additional refresh exchange while access token is fresh, got %d (%v)", len(refreshTokenInputs), refreshTokenInputs)
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
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
		TokenURL:     discordAPI.URL + "/token",
		UserInfoURL:  discordAPI.URL + "/users/@me",
		HTTPClient:   discordAPI.Client(),
	})); err != nil {
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
	if err := srv.SetDiscordOAuthConfig(DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	}); err == nil {
		t.Fatal("expected oauth config validation error when session store path is missing")
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

func withTestOAuthSessionStorePath(t *testing.T, cfg DiscordOAuthConfig) DiscordOAuthConfig {
	t.Helper()
	if strings.TrimSpace(cfg.SessionStorePath) == "" {
		cfg.SessionStorePath = filepath.Join(t.TempDir(), "oauth_sessions.json")
	}
	return cfg
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
