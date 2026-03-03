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
	if redirectRec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected /dashboard redirect, got %d body=%q", redirectRec.Code, redirectRec.Body.String())
	}
	if location := strings.TrimSpace(redirectRec.Header().Get("Location")); location != dashboardRoutePrefix {
		t.Fatalf("expected redirect location %q, got %q", dashboardRoutePrefix, location)
	}

	dashboardReq := httptest.NewRequest(http.MethodGet, "/dashboard/", nil)
	dashboardRec := httptest.NewRecorder()
	handler.ServeHTTP(dashboardRec, dashboardReq)
	if dashboardRec.Code != http.StatusOK {
		t.Fatalf("expected /dashboard/ to serve embedded index, got %d body=%q", dashboardRec.Code, dashboardRec.Body.String())
	}
	if contentType := dashboardRec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("expected /dashboard/ to serve html, got content-type %q", contentType)
	}
	if body := strings.ToLower(dashboardRec.Body.String()); !strings.Contains(body, "<!doctype html") {
		t.Fatalf("expected embedded index html, got %q", dashboardRec.Body.String())
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

func TestDashboardEndpointInteraction(t *testing.T) {
	t.Parallel()

	srv, _ := newControlTestServer(t)
	if err := srv.Start(); err != nil {
		t.Fatalf("start control server: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Stop(ctx)
	})

	resp, err := http.Get("http://" + srv.listener.Addr().String() + "/dashboard/")
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

	resp, err := http.Get("http://" + srv.listener.Addr().String() + "/dashboard/")
	if err != nil {
		t.Fatalf("GET /dashboard/: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /dashboard/ over live server to return 200, got %d", resp.StatusCode)
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
