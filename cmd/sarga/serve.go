package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the sarga server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("serve: not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}
