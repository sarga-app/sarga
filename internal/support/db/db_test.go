//nolint:testpackage // tests cover unexported buildDSN; pool tests need testcontainers (Phase 1+).
package db

import (
	"strings"
	"testing"

	"github.com/sarga-app/sarga/internal/support/config"
)

func TestBuildDSN_BasicFields(t *testing.T) {
	t.Parallel()
	got := buildDSN(config.DatabaseConfig{
		Host: "localhost", Port: 5432, User: "sarga", Name: "sarga",
		Password: "secret", SSLMode: "require",
	})
	for _, want := range []string{
		"host='localhost'",
		"port=5432",
		"user='sarga'",
		"password='secret'",
		"dbname='sarga'",
		"sslmode='require'",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("DSN %q missing %q", got, want)
		}
	}
}

func TestBuildDSN_EscapesQuote(t *testing.T) {
	t.Parallel()
	got := buildDSN(config.DatabaseConfig{
		Host: "h", Port: 1, User: "u", Name: "n",
		Password: "p'ass", SSLMode: "disable",
	})
	if !strings.Contains(got, `password='p\'ass'`) {
		t.Errorf("expected escaped quote in DSN, got %q", got)
	}
}

func TestBuildDSN_EscapesBackslash(t *testing.T) {
	t.Parallel()
	got := buildDSN(config.DatabaseConfig{
		Host: "h", Port: 1, User: "u", Name: "n",
		Password: `p\ass`, SSLMode: "disable",
	})
	if !strings.Contains(got, `password='p\\ass'`) {
		t.Errorf("expected escaped backslash in DSN, got %q", got)
	}
}

func TestBuildDSN_EscapeOrderBackslashThenQuote(t *testing.T) {
	t.Parallel()
	// Input: backslash followed by quote. Backslashes must be escaped FIRST,
	// then quotes — otherwise the backslash inserted by quote-escaping would
	// be re-escaped on a second pass and corrupt the output.
	got := buildDSN(config.DatabaseConfig{
		Host: "h", Port: 1, User: "u", Name: "n",
		Password: `\'`, SSLMode: "disable",
	})
	if !strings.Contains(got, `password='\\\''`) {
		t.Errorf("escape order wrong, got %q", got)
	}
}
