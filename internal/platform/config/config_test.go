package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_DefaultsOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	clearSargaEnv(t)

	c, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\"): %v", err)
	}
	if c.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q, want :8080", c.Server.Addr)
	}
	if c.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want info", c.Log.Level)
	}
	if c.Log.Format != "text" {
		t.Errorf("Log.Format = %q, want text", c.Log.Format)
	}
}

func TestLoad_MissingFileFallsBackToDefaults(t *testing.T) {
	clearSargaEnv(t)

	c, err := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("Load(missing): %v", err)
	}
	if c.Server.Addr != ":8080" {
		t.Errorf("Server.Addr = %q, want :8080", c.Server.Addr)
	}
}

func TestLoad_YAMLOverridesDefaults(t *testing.T) {
	clearSargaEnv(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte("server:\n  addr: 127.0.0.1:9000\nlog:\n  level: debug\n  format: json\n")
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Server.Addr != "127.0.0.1:9000" {
		t.Errorf("Server.Addr = %q, want 127.0.0.1:9000", c.Server.Addr)
	}
	if c.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want debug", c.Log.Level)
	}
	if c.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want json", c.Log.Format)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	clearSargaEnv(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := []byte("server:\n  addr: 127.0.0.1:9000\nlog:\n  level: debug\n  format: json\n")
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("SARGA_SERVER_ADDR", ":7777")
	t.Setenv("SARGA_LOG_LEVEL", "warn")

	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Server.Addr != ":7777" {
		t.Errorf("Server.Addr = %q, want :7777 (env override)", c.Server.Addr)
	}
	if c.Log.Level != "warn" {
		t.Errorf("Log.Level = %q, want warn (env override)", c.Log.Level)
	}
	if c.Log.Format != "json" {
		t.Errorf("Log.Format = %q, want json (yaml, no env)", c.Log.Format)
	}
}

func TestLoad_BadYAMLReturnsError(t *testing.T) {
	clearSargaEnv(t)

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("server: [unterminated\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load(bad yaml): want error, got nil")
	}
}

func TestLoad_InvalidValuesFailValidation(t *testing.T) {
	clearSargaEnv(t)

	t.Setenv("SARGA_LOG_LEVEL", "trace")

	_, err := Load("")
	if err == nil {
		t.Fatal("Load(invalid log.level): want error, got nil")
	}
	if !strings.Contains(err.Error(), "log.level") {
		t.Errorf("error %q does not mention log.level", err.Error())
	}
}

func TestValidate(t *testing.T) {
	okDB := DatabaseConfig{
		Host: "localhost", Port: 5432, User: "sarga", Name: "sarga",
		Password: "sarga", SSLMode: "disable",
	}
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "ok",
			cfg:  Config{Server: ServerConfig{Addr: ":8080"}, Log: LogConfig{Level: "info", Format: "text"}, Database: okDB},
		},
		{
			name:    "empty addr",
			cfg:     Config{Server: ServerConfig{Addr: ""}, Log: LogConfig{Level: "info", Format: "text"}, Database: okDB},
			wantErr: "server.addr",
		},
		{
			name:    "addr without port",
			cfg:     Config{Server: ServerConfig{Addr: "localhost"}, Log: LogConfig{Level: "info", Format: "text"}, Database: okDB},
			wantErr: "server.addr",
		},
		{
			name:    "bad level",
			cfg:     Config{Server: ServerConfig{Addr: ":8080"}, Log: LogConfig{Level: "trace", Format: "text"}, Database: okDB},
			wantErr: "log.level",
		},
		{
			name:    "bad format",
			cfg:     Config{Server: ServerConfig{Addr: ":8080"}, Log: LogConfig{Level: "info", Format: "xml"}, Database: okDB},
			wantErr: "log.format",
		},
		{
			name: "empty db password",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Log:      LogConfig{Level: "info", Format: "text"},
				Database: DatabaseConfig{Host: "localhost", Port: 5432, User: "sarga", Name: "sarga", SSLMode: "disable"},
			},
			wantErr: "database.password",
		},
		{
			name: "bad sslmode",
			cfg: Config{
				Server:   ServerConfig{Addr: ":8080"},
				Log:      LogConfig{Level: "info", Format: "text"},
				Database: DatabaseConfig{Host: "localhost", Port: 5432, User: "sarga", Name: "sarga", Password: "x", SSLMode: "yes"},
			},
			wantErr: "database.sslmode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate: unexpected error %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate: want error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func clearSargaEnv(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i > 0 {
			k := kv[:i]
			if strings.HasPrefix(k, "SARGA_") {
				t.Setenv(k, "")
				_ = os.Unsetenv(k)
			}
		}
	}
}
