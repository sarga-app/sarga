package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

//go:embed all:dist
var webDist embed.FS

type frontendHandler struct {
	fileServer http.Handler
	root       fs.FS
	hasIndex   bool
}

func Mount(r chi.Router) {
	r.Handle("/*", newHandler())
}

func newHandler() *frontendHandler {
	sub, err := fs.Sub(webDist, "dist")
	if err != nil {
		panic("web: embed invariant violated: " + err.Error())
	}
	_, indexErr := fs.Stat(sub, "index.html")
	return &frontendHandler{
		fileServer: http.FileServer(http.FS(sub)),
		root:       sub,
		hasIndex:   indexErr == nil,
	}
}

func (h *frontendHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.hasIndex {
		http.Error(w, "Frontend not built. Run: make web-build", http.StatusServiceUnavailable)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(h.root, path); err != nil {
		r.URL.Path = "/"
	}
	h.fileServer.ServeHTTP(w, r)
}
