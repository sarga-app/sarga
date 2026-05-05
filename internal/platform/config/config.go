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
}

type ServerConfig struct {
	Addr string `koanf:"addr"`
}

type LogConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // text | json
}

type DatabaseConfig struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	User     string `koanf:"user"`
	Name     string `koanf:"name"`
	Password string `koanf:"password"`
	SSLMode  string `koanf:"sslmode"`
	MaxConns int32  `koanf:"max_conns"`
}

func (d DatabaseConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("host", d.Host),
		slog.Int("port", d.Port),
		slog.String("user", d.User),
		slog.String("name", d.Name),
		slog.String("sslmode", d.SSLMode),
		slog.Int64("max_conns", int64(d.MaxConns)),
		slog.String("password", redactedMarker(d.Password)),
	)
}

func redactedMarker(s string) string {
	if s == "" {
		return "<unset>"
	}
	return "<redacted>"
}

var defaults = map[string]any{
	"server.addr":        ":8080",
	"log.level":          "info",
	"log.format":         "text",
	"database.host":      "localhost",
	"database.port":      5432,
	"database.user":      "sarga",
	"database.name":      "sarga",
	"database.password":  "sarga",
	"database.sslmode":   "disable",
	"database.max_conns": 0,
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

func (c *Config) Validate() error {
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

	if c.Database.Host == "" {
		return errors.New("database.host must not be empty")
	}
	if c.Database.Port <= 0 || c.Database.Port > 65535 {
		return fmt.Errorf("database.port must be 1..65535, got %d", c.Database.Port)
	}
	if c.Database.User == "" {
		return errors.New("database.user must not be empty")
	}
	if c.Database.Name == "" {
		return errors.New("database.name must not be empty")
	}
	switch c.Database.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("database.sslmode invalid: %q", c.Database.SSLMode)
	}
	if c.Database.MaxConns < 0 {
		return fmt.Errorf("database.max_conns must be >= 0, got %d", c.Database.MaxConns)
	}
	if c.Database.Password == "" {
		return errors.New("database.password must not be empty")
	}

	return nil
}
