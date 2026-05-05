package server

import (
	"github.com/go-chi/chi/v5"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
	"github.com/sarga-app/sarga/internal/server/auth"
)

const authBodyLimit = 4 * 1024

func mountAuthRoutes(r chi.Router, h *auth.Handler, jwt coreauth.JWTConfig) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Use(bodyLimit(authBodyLimit))
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(requireAuth(jwt))
		r.Get("/me", h.Me)
	})
}
