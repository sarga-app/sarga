package health

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, db Pinger) {
	r.Get("/healthz", handler(db))
}
