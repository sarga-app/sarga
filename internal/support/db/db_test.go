package db

import (
	"strings"
	"testing"
)

func TestBuildDSN_ContainsPragmas(t *testing.T) {
	t.Parallel()
	dsn := buildDSN("/data/sarga.db")
	for _, want := range []string{
		"file:/data/sarga.db",
		"journal_mode(WAL)",
		"foreign_keys(ON)",
		"busy_timeout(",
	} {
		if !strings.Contains(dsn, want) {
			t.Errorf("DSN %q missing %q", dsn, want)
		}
	}
}
