package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
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
	Auth     AuthConfig     `koanf:"auth"`
	Data     DataConfig     `koanf:"data"`
}

type DataConfig struct {
	Dir string `koanf:"dir"`
}

type AuthConfig struct {
	JWT JWTConfig `koanf:"jwt"`
}

type JWTConfig struct {
	Secret string `koanf:"secret"`
}

func (a AuthConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("jwt.secret", redactedMarker(a.JWT.Secret)),
	)
}

const JWTSecretMinBytes = 32

type ServerConfig struct {
	Addr string `koanf:"addr"`
}

type LogConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // text | json
}

// DatabaseConfig describes the SQLite database. Path defaults to "" which the
// serve command resolves to <data.dir>/sarga.db.
type DatabaseConfig struct {
	Path        string `koanf:"path"`
	AutoMigrate bool   `koanf:"automigrate"`
}

func (d DatabaseConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("path", d.Path),
		slog.Bool("automigrate", d.AutoMigrate),
	)
}

func redactedMarker(s string) string {
	if s == "" {
		return "<unset>"
	}
	return "<redacted>"
}

var defaults = map[string]any{
	"server.addr":          ":8080",
	"log.level":            "info",
	"log.format":           "text",
	"database.path":        "",
	"database.automigrate": true,
	"data.dir":             "./data",
}

func Load(path string) (*Config, error) {
	k := koanf.New(".")

	if err := k.Load(confmap.Provider(defaults, "."), nil); err != nil {
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

	if s := c.Auth.JWT.Secret; s != "" && len(s) < JWTSecretMinBytes {
		return fmt.Errorf("auth.jwt.secret must be at least %d bytes when set, got %d", JWTSecretMinBytes, len(s))
	}

	if c.Data.Dir == "" {
		return errors.New("data.dir must not be empty")
	}

	return nil
}
