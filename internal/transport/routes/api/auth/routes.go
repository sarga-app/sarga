package auth

import (
	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/transport/middleware"
)

const bodyLimit = 4 * 1024

func Mount(r chi.Router, h *Handler) {
	r.Route("/auth", func(r chi.Router) {
		r.Use(middleware.BodyLimit(bodyLimit))
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.jwt))
		r.Get("/me", h.Me)
	})
}
