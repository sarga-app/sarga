package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sarga-app/sarga/internal/platform/config"
	"github.com/sarga-app/sarga/internal/platform/db"
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
		return db.MigrateUp(cmd.Context(), cfg.Database)
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
		return db.MigrateDown(cmd.Context(), cfg.Database)
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
		statuses, err := db.MigrateStatus(cmd.Context(), cfg.Database)
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

func init() {
	migrateCmd.PersistentFlags().StringVarP(&migrateConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	migrateCmd.AddCommand(migrateUpCmd, migrateDownCmd, migrateStatusCmd)
	rootCmd.AddCommand(migrateCmd)
}
