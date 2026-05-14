package docs

import (
	_ "embed"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/openapi"
)

//go:embed viewer.html
var viewerHTML []byte

//go:embed scalar-standalone.js
var scalarJS []byte

var specJSON []byte

func init() {
	buf, err := openapi.GetSpecJSON()
	if err != nil {
		panic(fmt.Sprintf("load embedded openapi spec: %v", err))
	}
	specJSON = buf
}

func Mount(r chi.Router) {
	r.Get("/docs", serveViewer)
	r.Get("/docs/openapi.json", serveSpec)
	r.Get("/docs/scalar.js", serveScalarJS)
	r.Get("/docs/*", serveViewer)
}

func serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(specJSON)
}

func serveScalarJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(scalarJS)
}

func serveViewer(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy",
		"default-src 'none'; "+
			"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; "+
			"style-src 'self' 'unsafe-inline'; "+
			"font-src 'self' data:; "+
			"img-src 'self' data:; "+
			"connect-src 'self'; "+
			"worker-src 'self' blob:; "+
			"base-uri 'none'; "+
			"form-action 'none'")
	_, _ = w.Write(viewerHTML)
}
