package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server   ServerConfig   `koanf:"server"`
	Log      LogConfig      `koanf:"log"`
	Database DatabaseConfig `koanf:"database"`
	Secrets  SecretsConfig  `koanf:"secrets"`
	Data     DataConfig     `koanf:"data"`
}

type SecretsConfig struct {
	JWT    string `koanf:"jwt"`
	Master string `koanf:"master"`
}

func (s SecretsConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("jwt", redactedMarker(s.JWT)),
		slog.String("master", redactedMarker(s.Master)),
	)
}

const JWTSecretMinBytes = 32

type ServerConfig struct {
	Addr string `koanf:"addr"`
}

type LogConfig struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

type DatabaseConfig struct {
	Path        string `koanf:"path"`
	AutoMigrate bool   `koanf:"automigrate"`
}

type DataConfig struct {
	Dir string `koanf:"dir"`
}

func redactedMarker(s string) string {
	if s == "" {
		return "<unset>"
	}
	return "<redacted>"
}

func defaults() map[string]any {
	return map[string]any{
		"server.addr":          ":8080",
		"log.level":            "info",
		"log.format":           "text",
		"database.path":        "",
		"database.automigrate": true,
		"data.dir":             defaultDataDir(),
	}
}

func defaultDataDir() string {
	const app = "sarga"
	if inContainer() {
		return "/data"
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "./data"
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", app)
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, app)
		}
		if v := os.Getenv("APPDATA"); v != "" {
			return filepath.Join(v, app)
		}
		return filepath.Join(home, "AppData", "Local", app)
	default:
		if v := os.Getenv("XDG_DATA_HOME"); v != "" {
			return filepath.Join(v, app)
		}
		return filepath.Join(home, ".local", "share", app)
	}
}

func inContainer() bool {
	for _, p := range []string{"/.dockerenv", "/run/.containerenv"} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return nil, fmt.Errorf("load defaults: %w", err)
	}

	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("load %s: %w", path, err)
			}
		}
	}

	envProvider := env.Provider("SARGA_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(s, "SARGA_")), "_", ".")
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	var c Config
	if err := k.Unmarshal("", &c); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c Config) Validate() error {
	if c.Server.Addr == "" {
		return errors.New("server.addr must not be empty")
	}
	if _, _, err := net.SplitHostPort(c.Server.Addr); err != nil {
		return fmt.Errorf("server.addr %q: %w", c.Server.Addr, err)
	}

	switch c.Log.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("log.level must be one of debug/info/warn/error, got %q", c.Log.Level)
	}

	switch c.Log.Format {
	case "text", "json":
	default:
		return fmt.Errorf("log.format must be one of text/json, got %q", c.Log.Format)
	}

	if s := c.Secrets.JWT; s != "" && len(s) < JWTSecretMinBytes {
		return fmt.Errorf("secrets.jwt must be at least %d bytes when set, got %d", JWTSecretMinBytes, len(s))
	}

	if c.Data.Dir == "" {
		return errors.New("data.dir must not be empty")
	}

	return nil
}
