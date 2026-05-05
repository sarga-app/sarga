package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
)

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

func slogRequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		w.Header().Set("X-Request-Id", reqID)

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", float64(time.Since(start).Microseconds())/1000.0,
				"request_id", reqID,
				"remote_ip", r.RemoteAddr,
				"bytes", ww.BytesWritten(),
			)
		}()

		next.ServeHTTP(ww, r)
	})
}

func requireAuth(jwt coreauth.JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			tok, ok := strings.CutPrefix(authz, "Bearer ")
			if !ok || tok == "" {
				writeUnauthorized(w, "missing_token", "missing bearer token")
				return
			}
			claims, err := coreauth.ParseAccessToken(jwt, tok)
			if err != nil {
				writeUnauthorized(w, "invalid_token", "invalid or expired token")
				return
			}
			uid, err := claims.UserID()
			if err != nil {
				writeUnauthorized(w, "invalid_token", "invalid subject")
				return
			}
			next.ServeHTTP(w, r.WithContext(coreauth.WithUser(r.Context(), uid, claims.IsAdmin)))
		})
	}
}

// requireAdmin must run after requireAuth.
//
//nolint:unused // wired up in Step 11 (libraries CRUD admin gating)
func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !coreauth.IsAdminFrom(r.Context()) {
			writeUnauthorized(w, "forbidden", "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func bodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"` + code + `","message":"` + msg + `"}}`))
}
