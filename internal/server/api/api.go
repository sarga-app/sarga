package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/auth"
	"github.com/sarga-app/sarga/internal/crypto"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/scanner"
)

type ThumbStats interface {
	ThumbUsedBytes() int64
	SetCacheLimit(mb int64)
}

type API struct {
	db        *db.DB
	scanner   *scanner.Scanner
	jwt       auth.JWTConfig
	cipher    *crypto.Cipher
	thumbs    ThumbStats
	authStore authStore
}

func New(d *db.DB, sc *scanner.Scanner, jwt auth.JWTConfig, cipher *crypto.Cipher, thumbs ThumbStats) *API {
	return &API{
		db:        d,
		scanner:   sc,
		jwt:       jwt,
		cipher:    cipher,
		thumbs:    thumbs,
		authStore: buildAuthStore(d),
	}
}

func (a *API) Mount(r chi.Router) {
	r.Route("/api/v1", func(r chi.Router) {
		a.mountAuthRoutes(r)
		a.mountLibrariesRoutes(r)
		a.mountCatalogRoutes(r)
		a.mountSettingsRoutes(r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write json", "err", err)
	}
}

type errorResponse struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorPayload{Code: code, Message: message}})
}
