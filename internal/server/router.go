package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
	"github.com/sarga-app/sarga/internal/server/auth"
)

const requestTimeout = 60 * time.Second

func newRouter(db pinger, authH *auth.Handler, jwt coreauth.JWTConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(securityHeaders)
	r.Use(middleware.Recoverer)
	r.Use(slogRequestLogger)
	r.Use(middleware.Timeout(requestTimeout))

	r.Get("/healthz", healthzHandler(db))

	mountAuthRoutes(r, authH, jwt)

	return r
}
