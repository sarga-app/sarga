package docs

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/knadh/koanf/parsers/yaml"
)

//go:embed openapi.yaml
var openAPIYAML []byte

//go:embed viewer.html
var viewerHTML []byte

//go:embed scalar-standalone.js
var scalarJS []byte

type Handler struct {
	specJSON []byte
}

func NewHandler() (*Handler, error) {
	doc, err := yaml.Parser().Unmarshal(openAPIYAML)
	if err != nil {
		return nil, fmt.Errorf("parse openapi.yaml: %w", err)
	}
	buf, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("marshal openapi: %w", err)
	}
	return &Handler{specJSON: buf}, nil
}

func (h *Handler) Spec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(h.specJSON)
}

func (h *Handler) Bundle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(scalarJS)
}

func (h *Handler) Viewer(w http.ResponseWriter, _ *http.Request) {
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
