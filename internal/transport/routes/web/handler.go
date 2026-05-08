package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

type handler struct {
	fileServer http.Handler
	root       fs.FS
	hasIndex   bool
}

func newHandler() *handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("web: embed invariant violated: " + err.Error())
	}
	_, indexErr := fs.Stat(sub, "index.html")
	return &handler{
		fileServer: http.FileServer(http.FS(sub)),
		root:       sub,
		hasIndex:   indexErr == nil,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
