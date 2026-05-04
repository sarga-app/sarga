package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const requestTimeout = 60 * time.Second

func newRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(securityHeaders)
	r.Use(middleware.Recoverer)
	r.Use(slogRequestLogger)
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", healthz)

	return r
}
