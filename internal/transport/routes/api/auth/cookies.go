package auth

import (
	"net/http"
	"time"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
)

const (
	RefreshCookieName = "sarga_refresh"
	RefreshCookiePath = "/api/v1/auth"
)

func setRefreshCookie(w http.ResponseWriter, r *http.Request, plaintext string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure derived from request TLS state
		Name:     RefreshCookieName,
		Value:    plaintext,
		Path:     RefreshCookiePath,
		Expires:  time.Now().Add(domauth.RefreshTTL),
		MaxAge:   int(domauth.RefreshTTL.Seconds()),
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure derived from request TLS state
		Name:     RefreshCookieName,
		Value:    "",
		Path:     RefreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})
}
