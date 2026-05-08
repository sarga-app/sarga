package web

import "github.com/go-chi/chi/v5"

func Mount(r chi.Router) {
	r.Handle("/*", newHandler())
}
