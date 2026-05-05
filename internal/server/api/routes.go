package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/server/api/auth"
)

func Mount(r chi.Router, d Deps) {
	r.Route("/api/v1", func(r chi.Router) {
		auth.Mount(r, d.Auth)
	})
}
