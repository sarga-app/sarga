package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/sarga-app/sarga/internal/server/api"
	"github.com/sarga-app/sarga/internal/server/docs"
	"github.com/sarga-app/sarga/internal/server/frontend"
	"github.com/sarga-app/sarga/internal/server/public"
)

const requestTimeout = 60 * time.Second

func newRouter(db Pinger, apiH *api.API, publicH *public.Public) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(SecurityHeaders)
	r.Use(chimw.Recoverer)
	r.Use(RequestLogger)
	r.Use(chimw.Timeout(requestTimeout))

	mountHealthz(r, db)
	docs.Mount(r)
	apiH.Mount(r)
	publicH.Mount(r)
	frontend.Mount(r)

	return r
}
