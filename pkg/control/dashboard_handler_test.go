package control

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/files"
)

func TestDashboardHandlerServesStaticAsset(t *testing.T) {
	t.Parallel()

	handler := mustNewDashboardTestHandler(t, fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte("<!doctype html><html><body>index</body></html>")},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('dashboard');")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{background:#fff;}")},
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected asset request to succeed, got %d body=%q", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != "console.log('dashboard');" {
		t.Fatalf("unexpected asset body: %q", body)
	}
}

func TestDashboardHandlerFallsBackToIndexForSPARoute(t *testing.T) {
	t.Parallel()

	handler := mustNewDashboardTestHandler(t, fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<!doctype html><html><body>spa index</body></html>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('dashboard');")},
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/settings/guilds", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected SPA fallback to succeed, got %d body=%q", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, "spa index") {
		t.Fatalf("expected SPA fallback to serve index, got %q", body)
	}
}

func TestDashboardHandlerMissingAssetWithExtensionReturnsNotFound(t *testing.T) {
	t.Parallel()

	handler := mustNewDashboardTestHandler(t, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><html><body>spa index</body></html>")},
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard/assets/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected missing asset path to return 404, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestServerDashboardRoutesDoNotInterceptAPIOrAuth(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	handler := srv.httpServer.Handler

	redirectReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	redirectRec := httptest.NewRecorder()
	handler.ServeHTTP(redirectRec, redirectReq)
	if redirectRec.Code != http.StatusFound {
		t.Fatalf("expected /dashboard to redirect unauthenticated users, got %d body=%q", redirectRec.Code, redirectRec.Body.String())
	}
	if location := strings.TrimSpace(redirectRec.Header().Get("Location")); location != "/" {
		t.Fatalf("expected redirect location %q, got %q", "/", location)
	}

	rootReq := httptest.NewRequest(http.MethodGet, "/", nil)
	rootRec := httptest.NewRecorder()
	handler.ServeHTTP(rootRec, rootReq)
	if rootRec.Code != http.StatusOK {
		t.Fatalf("expected / to serve landing page, got %d body=%q", rootRec.Code, rootRec.Body.String())
	}
	if body := rootRec.Body.String(); !strings.Contains(body, "Login com Discord") {
		t.Fatalf("expected landing page login button, got %q", body)
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	dashboardRec := httptest.NewRecorder()
	handler.ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusFound {
		t.Fatalf("expected /dashboard/ to redirect unauthenticated users, got %d body=%q", dashboardRec.Code, dashboardRec.Body.String())
	}
	if location := strings.TrimSpace(dashboardRec.Header().Get("Location")); location != "/" {
		t.Fatalf("expected unauthenticated /dashboard/ redirect to root, got %q", location)
	}

	apiRec := performHandlerJSONRequest(t, handler, http.MethodGet, "/v1/runtime-config", nil)
	if apiRec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected /v1/runtime-config to remain bound to API handler, got %d body=%q", apiRec.Code, apiRec.Body.String())
	}

	authRec := performHandlerJSONRequestWithAuth(t, handler, http.MethodGet, "/auth/me", nil, "")
	if authRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected /auth/me to remain bound to auth handler, got %d body=%q", authRec.Code, authRec.Body.String())
	}
}

func TestDashboardRoutesRedirectToDiscordLoginWhenOAuthConfigured(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	})); err != nil {
		t.Fatalf("configure dashboard oauth: %v", err)
	}

	handler := srv.httpServer.Handler

	rootDashboardReq := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rootDashboardRec := httptest.NewRecorder()
	handler.ServeHTTP(rootDashboardRec, rootDashboardReq)
	if rootDashboardRec.Code != http.StatusFound {
		t.Fatalf("expected /dashboard to redirect to oauth login, got %d body=%q", rootDashboardRec.Code, rootDashboardRec.Body.String())
	}
	if location := strings.TrimSpace(rootDashboardRec.Header().Get("Location")); location != "/auth/discord/login?next=%2Fdashboard%2F" {
		t.Fatalf("unexpected /dashboard redirect target: %q", location)
	}

	spaReq := httptest.NewRequest(http.MethodGet, "/dashboard/settings/guilds?tab=access", nil)
	spaRec := httptest.NewRecorder()
	handler.ServeHTTP(spaRec, spaReq)
	if spaRec.Code != http.StatusFound {
		t.Fatalf("expected dashboard SPA route to redirect to oauth login, got %d body=%q", spaRec.Code, spaRec.Body.String())
	}
	if location := strings.TrimSpace(spaRec.Header().Get("Location")); location != "/auth/discord/login?next=%2Fdashboard%2Fsettings%2Fguilds%3Ftab%3Daccess" {
		t.Fatalf("unexpected dashboard SPA redirect target: %q", location)
	}
}

func TestDashboardEndpointInteraction(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	sessionCookie := configureDashboardSession(t, srv)
	if err := srv.Start(); err != nil {
		t.Fatalf("start control server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	req, err := http.NewRequest(http.MethodGet, "http://"+srv.listener.Addr().String()+"/dashboard/", nil)
	if err != nil {
		t.Fatalf("build GET /dashboard/ request: %v", err)
	}
	req.AddCookie(sessionCookie)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /dashboard/ over live server to return 200, got %d", resp.StatusCode)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected live dashboard response to be html, got %q", contentType)
	}
}

func TestDashboardEndpointInteractionWithoutConfiguredAuth(t *testing.T) {
	t.Parallel()

	cm := files.NewConfigManagerWithPath(filepath.Join(t.TempDir(), "settings.json"))
	srv := NewServer("127.0.0.1:0", cm, nil)
	if srv == nil {
		t.Fatal("expected non-nil control server")
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("start control server without auth: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	client := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get("http://" + srv.listener.Addr().String() + "/dashboard/")
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected /dashboard/ over live server to redirect to root, got %d", resp.StatusCode)
	}
	if location := strings.TrimSpace(resp.Header.Get("Location")); location != "/" {
		t.Fatalf("expected redirect to root, got %q", location)
	}

	apiResp, err := http.Get("http://" + srv.listener.Addr().String() + "/v1/runtime-config")
	if err != nil {
		t.Fatalf("GET /v1/runtime-config: %v", err)
	}
	defer apiResp.Body.Close()

	if apiResp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected unauthenticated runtime-config to return 503 when auth is not configured, got %d", apiResp.StatusCode)
	}
}

func mustNewDashboardTestHandler(t *testing.T, assets fs.FS) http.Handler {
	t.Helper()

	handler, err := newDashboardHandler(assets)
	if err != nil {
		t.Fatalf("newDashboardHandler() error = %v", err)
	}
	return handler
}

func configureDashboardSession(t *testing.T, srv *Server) *http.Cookie {
	t.Helper()

	if err := srv.SetDiscordOAuthConfig(withTestOAuthSessionStorePath(t, DiscordOAuthConfig{
		ClientID:     "1234567890",
		ClientSecret: "super-secret",
		RedirectURI:  "http://127.0.0.1:8080/auth/discord/callback",
	})); err != nil {
		t.Fatalf("configure dashboard oauth: %v", err)
	}

	session, err := srv.discordOAuth.sessions.Create(
		discordOAuthUser{ID: "u1", Username: "alice"},
		[]string{discordOAuthScopeIdentify, discordOAuthScopeGuilds},
		"access-token",
		"refresh-token",
		"Bearer",
		time.Hour,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create dashboard oauth session: %v", err)
	}

	return &http.Cookie{
		Name:  defaultDiscordOAuthSessionCookie,
		Value: session.ID,
		Path:  "/",
	}
}
