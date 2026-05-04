// Package config loads and validates sarga's runtime configuration.
//
// Three sources, merged in order (later overrides earlier):
//  1. Hardcoded defaults
//  2. YAML file (optional)
//  3. SARGA_-prefixed environment variables
//
// Env mapping: SARGA_SERVER_ADDR → server.addr (prefix stripped, lowercased,
// underscores become dots).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	Server ServerConfig `koanf:"server"`
	Log    LogConfig    `koanf:"log"`
}

type ServerConfig struct {
	Addr string `koanf:"addr"`
}

type LogConfig struct {
	Level  string `koanf:"level"`  // debug | info | warn | error
	Format string `koanf:"format"` // text | json
}

var defaults = map[string]any{
	"server.addr": ":8080",
	"log.level":   "info",
	"log.format":  "text",
}

// Load builds a Config from defaults, optionally a YAML file at path, and
// SARGA_-prefixed environment variables. An empty or missing file is fine;
// any other file error is returned.
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

	return nil
}
