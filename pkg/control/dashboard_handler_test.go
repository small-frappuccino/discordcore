package control

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
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
	if body := dashboardRec.Body.String(); !strings.Contains(body, "Dashboard assets not built") {
		t.Fatalf("expected embedded placeholder index, got %q", body)
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

func mustNewDashboardTestHandler(t *testing.T, assets fs.FS) http.Handler {
	t.Helper()

	handler, err := newDashboardHandler(assets)
	if err != nil {
		t.Fatalf("newDashboardHandler() error = %v", err)
	}
	return handler
}
