package art

import (
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/transport/httpx"
)

var validHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Handler struct {
	artDir string
}

func NewHandler(artDir string) *Handler {
	return &Handler{artDir: artDir}
}

func Mount(r chi.Router, h *Handler) {
	r.Get("/art/{hash}", h.ServeArt)
}

func (h *Handler) ServeArt(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !validHash.MatchString(hash) {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "art not found")
		return
	}
	path := filepath.Join(h.artDir, hash)
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path) //nolint:gosec // hash validated by regex above; filepath.Join cleans traversal
}
