package main

import (
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/sarga-app/sarga/internal/config"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/log"
	"github.com/sarga-app/sarga/internal/server"
)

var serveConfigPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the sarga server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		startedAt := time.Now()
		cfg, err := config.Load(serveConfigPath)
		if err != nil {
			return err
		}
		slog.SetDefault(log.NewLogger(cfg.Log))
		v, commit, _, dirty := buildInfo()
		printBanner(os.Stderr, v, commit, dirty)
		slog.Info("log: configured", "level", cfg.Log.Level, "format", cfg.Log.Format)
		slog.Info("sarga: starting", "version", v, "addr", cfg.Server.Addr, "data_dir", cfg.Data.Dir)

		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		dbPath := cfg.Database.Path
		if dbPath == "" {
			dbPath = filepath.Join(cfg.Data.Dir, "sarga.db")
		}

		if cfg.Database.AutoMigrate {
			if mErr := db.MigrateUp(ctx, dbPath); mErr != nil {
				return mErr
			}
		}

		dbInst, err := db.New(dbPath)
		if err != nil {
			return err
		}
		defer func() {
			if err := dbInst.Close(); err != nil {
				slog.Error("db: close failed", "err", err)
			}
		}()

		return server.Run(ctx, cfg, dbInst, startedAt)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	rootCmd.AddCommand(serveCmd)
}
