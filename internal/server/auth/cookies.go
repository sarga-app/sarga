package auth

import (
	"net/http"
	"time"
)

const (
	RefreshCookieName = "sarga_refresh"
	RefreshCookiePath = "/api/v1/auth"
)

func setRefreshCookie(w http.ResponseWriter, plaintext string, ttl time.Duration, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is dynamic so dev over HTTP works; prod default is true
		Name:     RefreshCookieName,
		Value:    plaintext,
		Path:     RefreshCookiePath,
		Expires:  time.Now().Add(ttl),
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearRefreshCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // Secure is dynamic so dev over HTTP works; prod default is true
		Name:     RefreshCookieName,
		Value:    "",
		Path:     RefreshCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}
