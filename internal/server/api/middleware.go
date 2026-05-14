package api

import (
	"net/http"
	"strings"

	"github.com/sarga-app/sarga/internal/auth"
)

func requireAuth(jwt auth.JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || tok == "" {
				if c, err := r.Cookie("sarga_token"); err == nil && c.Value != "" {
					tok = c.Value
					ok = true
				}
			}
			if !ok || tok == "" {
				writeError(w, http.StatusUnauthorized, "missing_token", "missing bearer token")
				return
			}
			claims, err := auth.ParseAccessToken(jwt, tok)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "invalid or expired token")
				return
			}
			uid, err := claims.UserID()
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid_token", "invalid subject")
				return
			}
			next.ServeHTTP(w, r.WithContext(auth.WithUser(r.Context(), uid, claims.IsAdmin)))
		})
	}
}

func requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !auth.IsAdminFrom(r.Context()) {
			writeError(w, http.StatusForbidden, "forbidden", "admin required")
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
