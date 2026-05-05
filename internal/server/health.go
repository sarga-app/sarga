package server

import (
	"context"
	"net/http"
	"time"
)

const healthPingTimeout = 2 * time.Second

type pinger interface {
	Ping(ctx context.Context) error
}

func healthzHandler(db pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), healthPingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"db":     "down",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "ok",
		})
	}
}
