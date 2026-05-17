package frontend

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler serves embedded frontend files and falls back to index.html for SPA routes.
type Handler struct {
	distFS     fs.FS
	fileServer http.Handler
	indexHTML  []byte
}

// NewHandler creates a frontend handler from an fs.FS containing the dist directory.
func NewHandler(fsys fs.FS) (*Handler, error) {
	distFS, err := fs.Sub(fsys, "dist")
	if err != nil {
		return nil, err
	}
	idx, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return nil, err
	}
	return &Handler{
		distFS:     distFS,
		fileServer: http.FileServer(http.FS(distFS)),
		indexHTML:  idx,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// API and v2 routes should never reach the frontend handler (handled by routing),
	// but guard against it just in case.
	if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/v2") {
		http.NotFound(w, r)
		return
	}

	// Try serving a static asset (js, css, images, fonts, etc.)
	if path != "/" && strings.Contains(path, ".") {
		name := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(h.distFS, name); err == nil {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			h.fileServer.ServeHTTP(w, r)
			return
		}
	}

	// SPA fallback: serve index.html for all other routes
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(h.indexHTML)
}