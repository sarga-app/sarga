package server

import (
	"context"
	"log/slog"

	"github.com/rohan-k02/sarga/internal/platform/config"
	"github.com/rohan-k02/sarga/internal/platform/obs"
)

func Run(ctx context.Context, cfg *config.Config) error {
	logger, err := obs.NewLogger(cfg.Log)
	if err != nil {
		return err
	}
	slog.SetDefault(logger)

	slog.Debug("config loaded",
		"addr", cfg.Server.Addr,
		"log_level", cfg.Log.Level,
		"log_format", cfg.Log.Format,
	)
	slog.Info("starting sarga", "addr", cfg.Server.Addr)
	slog.Warn("serve: not yet implemented")

	_ = ctx
	return nil
}
