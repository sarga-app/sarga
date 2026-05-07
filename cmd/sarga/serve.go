package main

import (
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/sarga-app/sarga/internal/support/config"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/telemetry"
	"github.com/sarga-app/sarga/internal/transport"
)

var serveConfigPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the sarga server",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(serveConfigPath)
		if err != nil {
			return err
		}
		slog.SetDefault(telemetry.NewLogger(cfg.Log))

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
		defer dbInst.Close()

		return transport.Run(ctx, cfg, dbInst)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	rootCmd.AddCommand(serveCmd)
}
