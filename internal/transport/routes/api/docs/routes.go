package docs

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Route("/docs", func(r chi.Router) {
		r.Get("/", h.Viewer)
		r.Get("/openapi.json", h.Spec)
		r.Get("/scalar.js", h.Bundle)
	})
}
