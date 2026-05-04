package main

import (
	"context"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sarga",
	Short: "A self-hosted music server",
	Long: `Sarga is a single-binary, self-hosted music server.

Bit-perfect lossless playback, library curation, OpenSubsonic-compatible.`,
	SilenceUsage: true,
}

func Execute() error {
	return rootCmd.ExecuteContext(context.Background())
}
