package control

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/small-frappuccino/discordcore/pkg/log"
	embeddedui "github.com/small-frappuccino/discordcore/ui"
)

const dashboardRoutePrefix = "/dashboard/"

type dashboardHandler struct {
	assets     fs.FS
	fileServer http.Handler
}

func newEmbeddedDashboardHandler() http.Handler {
	assets, err := embeddedui.DistFS()
	if err != nil {
		log.ApplicationLogger().Error("Dashboard assets unavailable", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "dashboard assets unavailable", http.StatusServiceUnavailable)
		})
	}

	handler, err := newDashboardHandler(assets)
	if err != nil {
		log.ApplicationLogger().Error("Dashboard handler unavailable", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "dashboard handler unavailable", http.StatusServiceUnavailable)
		})
	}
	return handler
}

func newDashboardHandler(assets fs.FS) (http.Handler, error) {
	if assets == nil {
		return nil, fmt.Errorf("dashboard assets fs is nil")
	}

	return &dashboardHandler{
		assets:     assets,
		fileServer: http.FileServer(http.FS(assets)),
	}, nil
}

func (h *dashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assetPath, ok := normalizeDashboardAssetPath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}
	if assetPath == "" {
		h.serveIndex(w, r)
		return
	}

	switch h.assetKind(assetPath) {
	case dashboardAssetFile:
		h.serveAsset(w, r, assetPath)
		return
	case dashboardAssetDirectory:
		http.NotFound(w, r)
		return
	case dashboardAssetMissing:
		if path.Ext(assetPath) != "" {
			http.NotFound(w, r)
			return
		}
		h.serveIndex(w, r)
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (h *dashboardHandler) serveAsset(w http.ResponseWriter, r *http.Request, assetPath string) {
	assetReq := r.Clone(r.Context())
	assetURL := *r.URL
	assetURL.Path = "/" + strings.TrimPrefix(assetPath, "/")
	assetReq.URL = &assetURL
	h.fileServer.ServeHTTP(w, assetReq)
}

func (h *dashboardHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	content, err := fs.ReadFile(h.assets, "index.html")
	if err != nil {
		http.Error(w, "dashboard index unavailable", http.StatusServiceUnavailable)
		return
	}

	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
}

type dashboardAssetKind int

const (
	dashboardAssetMissing dashboardAssetKind = iota
	dashboardAssetFile
	dashboardAssetDirectory
)

func (h *dashboardHandler) assetKind(assetPath string) dashboardAssetKind {
	info, err := fs.Stat(h.assets, assetPath)
	if err != nil {
		return dashboardAssetMissing
	}
	if info.IsDir() {
		return dashboardAssetDirectory
	}
	return dashboardAssetFile
}

func normalizeDashboardAssetPath(requestPath string) (string, bool) {
	if !strings.HasPrefix(requestPath, dashboardRoutePrefix) {
		return "", false
	}

	trimmed := strings.TrimPrefix(requestPath, dashboardRoutePrefix)
	cleaned := path.Clean("/" + trimmed)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return "", true
	}
	return cleaned, true
}
