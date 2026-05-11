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

// setAccessCookie sets a short-lived non-HttpOnly cookie carrying the JWT so
// that browser <img> requests to /art/* can carry auth without JS involvement.
func setAccessCookie(w http.ResponseWriter, r *http.Request, token string) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // non-HttpOnly is required; browser img tags can't send headers
		Name:     "sarga_token",
		Value:    token,
		Path:     "/art",
		MaxAge:   int(domauth.AccessTTL.Seconds()),
		HttpOnly: false,
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
