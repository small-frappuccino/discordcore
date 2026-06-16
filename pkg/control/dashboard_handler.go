package control

import (
	"bytes"
	"fmt"
	embeddedui "github.com/small-frappuccino/discordcore/ui"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	dashboardRoutePrefix       = "/manage/"
	dashboardLegacyRoutePrefix = "/dashboard/"
)

type dashboardHandler struct {
	assets     fs.FS
	fileServer http.Handler
	knownFiles map[string]struct{}
	knownDirs  map[string]struct{}
}

func (s *Server) newEmbeddedDashboardHandler() http.Handler {
	assets, err := embeddedui.DistFS()
	if err != nil {
		s.log().Error("Dashboard assets unavailable", "err", err)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "dashboard assets unavailable", http.StatusServiceUnavailable)
		})
	}

	handler, err := newDashboardHandler(assets)
	if err != nil {
		s.log().Error("Dashboard handler unavailable", "err", err)
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

	knownFiles := make(map[string]struct{})
	knownDirs := make(map[string]struct{})

	err := fs.WalkDir(assets, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			knownDirs["."] = struct{}{}
			return nil
		}
		if d.IsDir() {
			knownDirs[path] = struct{}{}
		} else {
			knownFiles[path] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to pre-compute embedded assets: %w", err)
	}

	go func() {
		// Eagerly pre-fault critical assets into the OS page cache to eliminate cold-start latency.
		for p := range knownFiles {
			if p == "index.html" || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") || strings.HasSuffix(p, ".br") || strings.HasSuffix(p, ".gz") {
				if content, err := fs.ReadFile(assets, p); err == nil {
					_ = content
				}
			}
		}
	}()

	return &dashboardHandler{
		assets:     assets,
		fileServer: http.FileServer(http.FS(assets)),
		knownFiles: knownFiles,
		knownDirs:  knownDirs,
	}, nil
}

// ServeHTTP serves http.
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
		if path.Ext(assetPath) != "" || strings.HasPrefix(assetPath, "assets/") {
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
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	accept := r.Header.Get("Accept-Encoding")

	if strings.Contains(accept, "br") {
		brPath := assetPath + ".br"
		if _, ok := h.knownFiles[brPath]; ok {
			if f, err := h.assets.Open(brPath); err == nil {
				defer f.Close()
				if stat, err := f.Stat(); err == nil {
					w.Header().Set("Content-Encoding", "br")
					if t := mime.TypeByExtension(path.Ext(assetPath)); t != "" {
						w.Header().Set("Content-Type", t)
					}
					http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(io.ReadSeeker))
					return
				}
			}
		}
	}

	if strings.Contains(accept, "gzip") {
		gzPath := assetPath + ".gz"
		if _, ok := h.knownFiles[gzPath]; ok {
			if f, err := h.assets.Open(gzPath); err == nil {
				defer f.Close()
				if stat, err := f.Stat(); err == nil {
					w.Header().Set("Content-Encoding", "gzip")
					if t := mime.TypeByExtension(path.Ext(assetPath)); t != "" {
						w.Header().Set("Content-Type", t)
					}
					http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(io.ReadSeeker))
					return
				}
			}
		}
	}

	assetReq := r.Clone(r.Context())
	assetURL := *r.URL
	assetURL.Path = "/" + strings.TrimPrefix(assetPath, "/")
	assetReq.URL = &assetURL
	h.fileServer.ServeHTTP(w, assetReq)
}

func (h *dashboardHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept-Encoding")
	w.Header().Set("Cache-Control", "no-cache")

	accept := r.Header.Get("Accept-Encoding")

	if strings.Contains(accept, "br") {
		if _, ok := h.knownFiles["index.html.br"]; ok {
			content, err := fs.ReadFile(h.assets, "index.html.br")
			if err == nil {
				w.Header().Set("Content-Encoding", "br")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
				return
			}
		}
	}

	if strings.Contains(accept, "gzip") {
		if _, ok := h.knownFiles["index.html.gz"]; ok {
			content, err := fs.ReadFile(h.assets, "index.html.gz")
			if err == nil {
				w.Header().Set("Content-Encoding", "gzip")
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(content))
				return
			}
		}
	}

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
	if _, ok := h.knownFiles[assetPath]; ok {
		return dashboardAssetFile
	}
	if _, ok := h.knownDirs[assetPath]; ok {
		return dashboardAssetDirectory
	}
	return dashboardAssetMissing
}

func normalizeDashboardAssetPath(requestPath string) (string, bool) {
	trimmed, ok := trimDashboardRoutePrefix(requestPath)
	if !ok {
		return "", false
	}

	cleaned := path.Clean("/" + trimmed)
	cleaned = strings.TrimPrefix(cleaned, "/")
	if cleaned == "." {
		return "", true
	}
	return cleaned, true
}

func trimDashboardRoutePrefix(requestPath string) (string, bool) {
	switch {
	case strings.HasPrefix(requestPath, dashboardRoutePrefix):
		return strings.TrimPrefix(requestPath, dashboardRoutePrefix), true
	case strings.HasPrefix(requestPath, dashboardLegacyRoutePrefix):
		return strings.TrimPrefix(requestPath, dashboardLegacyRoutePrefix), true
	default:
		return "", false
	}
}
