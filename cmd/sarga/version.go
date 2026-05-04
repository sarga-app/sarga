package main

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is overridable at build time via -ldflags "-X main.version=...".
// When unset, we derive build info from runtime/debug.ReadBuildInfo() — which
// gives the VCS commit, build time, and dirty flag for free.
var version = ""

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(_ *cobra.Command, _ []string) error {
		v, commit, buildTime, dirty := buildInfo()
		fmt.Printf("sarga %s\n", v)
		fmt.Printf("  commit:    %s%s\n", commit, dirtyMarker(dirty))
		fmt.Printf("  built:     %s\n", buildTime)
		fmt.Printf("  go:        %s\n", runtime.Version())
		fmt.Printf("  platform:  %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func buildInfo() (v, commit, buildTime string, dirty bool) {
	v = version
	commit = "unknown"
	buildTime = "unknown"

	info, ok := debug.ReadBuildInfo()
	if !ok {
		if v == "" {
			v = "dev"
		}
		return v, commit, buildTime, dirty
	}

	if v == "" {
		v = info.Main.Version
		if v == "" || v == "(devel)" {
			v = "dev"
		}
	}

	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			commit = s.Value
		case "vcs.time":
			buildTime = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	return v, commit, buildTime, dirty
}

func dirtyMarker(dirty bool) string {
	if dirty {
		return " (dirty)"
	}
	return ""
}
