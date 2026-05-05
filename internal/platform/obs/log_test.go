//nolint:testpackage // tests cover unexported parseLevel and newLoggerTo (the io.Writer seam).
package obs

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/sarga-app/sarga/internal/platform/config"
)

func TestParseLevel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"trace", slog.LevelInfo},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := parseLevel(tc.in); got != tc.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewLoggerTo_TextFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := newLoggerTo(&buf, config.LogConfig{Level: "info", Format: "text"})
	l.Info("hello", "k", "v")

	out := buf.String()
	if !strings.Contains(out, "msg=hello") {
		t.Errorf("text output missing msg=hello: %q", out)
	}
	if !strings.Contains(out, "k=v") {
		t.Errorf("text output missing k=v: %q", out)
	}
}

func TestNewLoggerTo_JSONFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := newLoggerTo(&buf, config.LogConfig{Level: "info", Format: "json"})
	l.Info("hello", "k", "v")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("not valid json: %v: %q", err, buf.String())
	}
	if got["msg"] != "hello" {
		t.Errorf("json msg = %v, want hello", got["msg"])
	}
	if got["k"] != "v" {
		t.Errorf("json k = %v, want v", got["k"])
	}
}

func TestNewLoggerTo_LevelFilters(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := newLoggerTo(&buf, config.LogConfig{Level: "warn", Format: "text"})
	l.Info("ignored")
	l.Warn("kept")

	out := buf.String()
	if strings.Contains(out, "ignored") {
		t.Errorf("info should be filtered out at warn level: %q", out)
	}
	if !strings.Contains(out, "kept") {
		t.Errorf("warn should pass at warn level: %q", out)
	}
}

func TestNewLoggerTo_UnknownFormatFallsBackToText(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	l := newLoggerTo(&buf, config.LogConfig{Level: "info", Format: "garbage"})
	l.Info("hello")
	if !strings.Contains(buf.String(), "msg=hello") {
		t.Errorf("expected text fallback, got %q", buf.String())
	}
}
