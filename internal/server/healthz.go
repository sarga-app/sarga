package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
)

const pingTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

func mountHealthz(r chi.Router, db Pinger) {
	r.Get("/healthz", healthzHandler(db))
}

func healthzHandler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeHealthJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"db":     "down",
			})
			return
		}
		writeHealthJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "ok",
		})
	}
}

func writeHealthJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("healthz: write json", "err", err)
	}
}
