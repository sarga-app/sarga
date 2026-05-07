package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sarga-app/sarga/internal/support/config"
	"github.com/sarga-app/sarga/internal/support/db"
)

var migrateConfigPath string

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage database schema migrations",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply all pending migrations",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(migrateConfigPath)
		if err != nil {
			return err
		}
		return db.MigrateUp(cmd.Context(), dbPathFromConfig(cfg))
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Roll back the most recent migration",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(migrateConfigPath)
		if err != nil {
			return err
		}
		return db.MigrateDown(cmd.Context(), dbPathFromConfig(cfg))
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show applied and pending migrations",
	RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(migrateConfigPath)
		if err != nil {
			return err
		}
		statuses, err := db.MigrateStatus(cmd.Context(), dbPathFromConfig(cfg))
		if err != nil {
			return err
		}
		for _, s := range statuses {
			marker := "pending"
			if s.Applied {
				marker = "applied"
			}
			fmt.Printf("  %4d  %-8s  %s\n", s.Version, marker, s.Name)
		}
		return nil
	},
}

func dbPathFromConfig(cfg *config.Config) string {
	if cfg.Database.Path != "" {
		return cfg.Database.Path
	}
	return filepath.Join(cfg.Data.Dir, "sarga.db")
}

func init() {
	migrateCmd.PersistentFlags().StringVarP(&migrateConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	migrateCmd.AddCommand(migrateUpCmd, migrateDownCmd, migrateStatusCmd)
	rootCmd.AddCommand(migrateCmd)
}
