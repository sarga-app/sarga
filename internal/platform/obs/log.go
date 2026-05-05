package obs

import (
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/sarga-app/sarga/internal/platform/config"
)

func NewLogger(cfg config.LogConfig) *slog.Logger {
	return newLoggerTo(os.Stdout, cfg)
}

func newLoggerTo(w io.Writer, cfg config.LogConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.Level)}

	var h slog.Handler
	if strings.EqualFold(cfg.Format, "json") {
		h = slog.NewJSONHandler(w, opts)
	} else {
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
