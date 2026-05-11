package docs

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router, h *Handler) {
	r.Route("/docs", func(r chi.Router) {
		// Specific assets first — these must not be shadowed by the SPA fallback.
		r.Get("/openapi.json", h.Spec)
		r.Get("/scalar.js", h.Bundle)
		// Scalar pushes client-side URLs like /docs/tag/<x>/operation/<y>.
		// Serve the same viewer HTML for the root and any nested path so a
		// reload or shared link resolves instead of 404'ing.
		r.Get("/", h.Viewer)
		r.Get("/*", h.Viewer)
	})
}
