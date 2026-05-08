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
}

func newHandler() *handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic("web: embed invariant violated: " + err.Error())
	}
	return &handler{
		fileServer: http.FileServer(http.FS(sub)),
		root:       sub,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(h.root, path); err != nil {
		// Path not found in dist — serve index.html for client-side routing
		r.URL.Path = "/"
	}
	h.fileServer.ServeHTTP(w, r)
}
