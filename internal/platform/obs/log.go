package obs

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/sarga-app/sarga/internal/platform/config"
)

func NewLogger(cfg config.LogConfig) (*slog.Logger, error) {
	return newLoggerTo(os.Stdout, cfg)
}

func newLoggerTo(w io.Writer, cfg config.LogConfig) (*slog.Logger, error) {
	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}
	opts := &slog.HandlerOptions{Level: level}

	var h slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "text":
		h = slog.NewTextHandler(w, opts)
	case "json":
		h = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("log.format must be one of text/json, got %q", cfg.Format)
	}
	return slog.New(h), nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("log.level must be one of debug/info/warn/error, got %q", s)
	}
}
