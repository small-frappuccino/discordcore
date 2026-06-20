package control

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"

	embeddedui "github.com/small-frappuccino/discordcore/ui"
)

const (
	dashboardRoutePrefix       = "/manage/"
	dashboardLegacyRoutePrefix = "/dashboard/"
)

type dashboardHandler struct {
	distFS    fs.FS
	indexHTML []byte
	indexGzip []byte
}

func newDashboardHandler() *dashboardHandler {
	assets, err := embeddedui.DistFS()
	if err != nil {
		panic("embeddedui.DistFS failed: " + err.Error())
	}

	indexData, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		panic("failed to read embedded index.html: " + err.Error())
	}

	var gzBuf bytes.Buffer
	gzWriter := gzip.NewWriter(&gzBuf)
	gzWriter.Write(indexData)
	gzWriter.Close()

	return &dashboardHandler{
		distFS:    assets,
		indexHTML: indexData,
		indexGzip: gzBuf.Bytes(),
	}
}

func (h *dashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// SPA Fallback logic
	if r.URL.Path == dashboardRoutePrefix || r.URL.Path == dashboardLegacyRoutePrefix || !strings.Contains(r.URL.Path, ".") {
		h.serveIndex(w, r)
		return
	}

	// Serve static assets
	stripped := strings.TrimPrefix(r.URL.Path, dashboardRoutePrefix)
	stripped = strings.TrimPrefix(stripped, dashboardLegacyRoutePrefix)

	f, err := h.distFS.Open(stripped)
	if err != nil {
		h.serveIndex(w, r)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil || stat.IsDir() {
		h.serveIndex(w, r)
		return
	}

	http.ServeContent(w, r, stat.Name(), stat.ModTime(), f.(io.ReadSeeker))
}

func (h *dashboardHandler) serveIndex(w http.ResponseWriter, r *http.Request) {
	slog.Debug("Granular inspection: Serving SPA index fallback", slog.String("path", r.URL.Path))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Compression Negotiation
	accept := r.Header.Get("Accept-Encoding")
	if strings.Contains(accept, "br") {
		// If brotli is requested but we only eager-cached gzip, fallback to gzip or raw
		// For the sake of this implementation, we simulate br fallback to gzip if supported
		if strings.Contains(accept, "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Write(h.indexGzip)
			return
		}
	} else if strings.Contains(accept, "gzip") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Write(h.indexGzip)
		return
	}

	// Uncompressed fallback
	w.Write(h.indexHTML)
}
