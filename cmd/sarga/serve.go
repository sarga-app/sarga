package main

import (
	"fmt"

	"github.com/rohan-k02/sarga/internal/platform/config"
	"github.com/spf13/cobra"
)

var serveConfigPath string

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the sarga server",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(serveConfigPath)
		if err != nil {
			return err
		}
		fmt.Printf("config loaded: addr=%s log=%s/%s\n",
			cfg.Server.Addr, cfg.Log.Level, cfg.Log.Format)
		fmt.Println("serve: not yet implemented")
		return nil
	},
}

func init() {
	serveCmd.Flags().StringVarP(&serveConfigPath, "config", "c", "config.yaml",
		"path to config file (optional; defaults applied if missing)")
	rootCmd.AddCommand(serveCmd)
}
