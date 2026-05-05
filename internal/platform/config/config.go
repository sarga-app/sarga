package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"strings"
	"time"

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
}

type ServerConfig struct {
	Addr string `koanf:"addr"`
}

type LogConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // text | json
}

type AuthConfig struct {
	JWTSecret    string        `koanf:"jwtsecret"`
	JWTIssuer    string        `koanf:"jwtissuer"`
	JWTAudience  string        `koanf:"jwtaudience"`
	AccessTTL    time.Duration `koanf:"accessttl"`
	RefreshTTL   time.Duration `koanf:"refreshttl"`
	CookieSecure bool          `koanf:"cookiesecure"`
}

func (a AuthConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("jwtsecret", redactedMarker(a.JWTSecret)),
		slog.String("jwtissuer", a.JWTIssuer),
		slog.String("jwtaudience", a.JWTAudience),
		slog.Duration("accessttl", a.AccessTTL),
		slog.Duration("refreshttl", a.RefreshTTL),
		slog.Bool("cookiesecure", a.CookieSecure),
	)
}

type DatabaseConfig struct {
	Host        string `koanf:"host"`
	Port        int    `koanf:"port"`
	User        string `koanf:"user"`
	Name        string `koanf:"name"`
	Password    string `koanf:"password"`
	SSLMode     string `koanf:"sslmode"`
	MaxConns    int32  `koanf:"maxconns"`
	AutoMigrate bool   `koanf:"automigrate"`
}

func (d DatabaseConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("host", d.Host),
		slog.Int("port", d.Port),
		slog.String("user", d.User),
		slog.String("name", d.Name),
		slog.String("sslmode", d.SSLMode),
		slog.Int("maxconns", int(d.MaxConns)),
		slog.Bool("automigrate", d.AutoMigrate),
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
	"server.addr":          ":8080",
	"log.level":            "info",
	"log.format":           "text",
	"database.host":        "localhost",
	"database.port":        5432,
	"database.user":        "sarga",
	"database.name":        "sarga",
	"database.password":    "sarga",
	"database.sslmode":     "disable",
	"database.maxconns":    0,
	"database.automigrate": true,
	"auth.jwtissuer":       "sarga",
	"auth.jwtaudience":     "sarga-web",
	"auth.accessttl":       "30m",
	"auth.refreshttl":      "720h",
	"auth.cookiesecure":    true,
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
		return fmt.Errorf("database.maxconns must be >= 0, got %d", c.Database.MaxConns)
	}
	if c.Database.Password == "" {
		return errors.New("database.password must not be empty")
	}

	if len(c.Auth.JWTSecret) < 32 {
		return errors.New(
			"auth.jwtsecret must be at least 32 bytes; generate one with `openssl rand -base64 48` " +
				"and set via SARGA_AUTH_JWT_SECRET or the auth.jwtsecret YAML key",
		)
	}
	if c.Auth.JWTIssuer == "" {
		return errors.New("auth.jwtissuer must not be empty")
	}
	if c.Auth.JWTAudience == "" {
		return errors.New("auth.jwtaudience must not be empty")
	}
	if c.Auth.AccessTTL <= 0 {
		return fmt.Errorf("auth.accessttl must be positive, got %s", c.Auth.AccessTTL)
	}
	if c.Auth.RefreshTTL <= c.Auth.AccessTTL {
		return fmt.Errorf("auth.refreshttl (%s) must exceed access_ttl (%s)", c.Auth.RefreshTTL, c.Auth.AccessTTL)
	}

	return nil
}
