package health

import (
	"context"
	"net/http"
	"time"

	"github.com/sarga-app/sarga/internal/server/httpx"
)

const pingTimeout = 2 * time.Second

type Pinger interface {
	Ping(ctx context.Context) error
}

func handler(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"db":     "down",
			})
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "ok",
		})
	}
}
