package middleware

import (
	"net/http"
	"strings"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/transport/httpx"
)

// RequireAuth parses Authorization: Bearer <jwt>, validates, and stores user
// identity on the request context. 401 on any failure.
func RequireAuth(jwt domauth.JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || tok == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing_token", "missing bearer token")
				return
			}
			claims, err := domauth.ParseAccessToken(jwt, tok)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "invalid or expired token")
				return
			}
			uid, err := claims.UserID()
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "invalid subject")
				return
			}
			next.ServeHTTP(w, r.WithContext(domauth.WithUser(r.Context(), uid, claims.IsAdmin)))
		})
	}
}

// RequireAdmin must run after RequireAuth.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !domauth.IsAdminFrom(r.Context()) {
			httpx.WriteError(w, http.StatusForbidden, "forbidden", "admin required")
			return
		}
		next.ServeHTTP(w, r)
	})
}
