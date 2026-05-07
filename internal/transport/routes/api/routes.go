package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/docs"
	"github.com/sarga-app/sarga/internal/transport/routes/api/libraries"
)

func Mount(r chi.Router, d Deps) {
	r.Route("/api/v1", func(r chi.Router) {
		auth.Mount(r, d.Auth)
		docs.Mount(r, d.Docs)
		libraries.Mount(r, d.Libraries, d.Auth.JWTConfig())
	})
}
