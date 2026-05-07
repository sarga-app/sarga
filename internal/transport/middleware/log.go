package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

func RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := chimw.GetReqID(r.Context())
		w.Header().Set("X-Request-Id", reqID)

		ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		defer func() {
			status := ww.Status()
			if r.URL.Path == "/healthz" && status < 400 {
				return
			}
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", status,
				"duration_ms", float64(time.Since(start).Microseconds()) / 1000.0,
				"request_id", reqID,
				"remote_ip", r.RemoteAddr,
				"bytes", ww.BytesWritten(),
			}
			switch {
			case status >= 500:
				slog.Warn("request", attrs...)
			case status >= 400:
				slog.Info("request", attrs...)
			default:
				slog.Debug("request", attrs...)
			}
		}()

		next.ServeHTTP(ww, r)
	})
}
