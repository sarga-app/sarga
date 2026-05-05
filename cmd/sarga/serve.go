package main

import (
	"github.com/spf13/cobra"

	"github.com/sarga-app/sarga/internal/platform/config"
	"github.com/sarga-app/sarga/internal/platform/db"
	"github.com/sarga-app/sarga/internal/server"
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

		pool, err := db.NewPool(cmd.Context(), cfg.Database)
		if err != nil {
			return err
		}
		defer pool.Close()

		return server.Run(cmd.Context(), cfg, pool)
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	rootCmd.AddCommand(serveCmd)
}
