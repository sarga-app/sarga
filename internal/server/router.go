package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/sarga-app/sarga/internal/server/middleware"
	"github.com/sarga-app/sarga/internal/server/routes/api"
	"github.com/sarga-app/sarga/internal/server/routes/health"
)

const requestTimeout = 60 * time.Second

func newRouter(d Deps) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(middleware.SecurityHeaders)
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestLogger)
	r.Use(chimw.Timeout(requestTimeout))

	health.Mount(r, d.DB)
	api.Mount(r, d.API)

	return r
}
