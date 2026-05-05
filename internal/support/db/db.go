package db

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sarga-app/sarga/internal/support/config"
)

const (
	connectTimeout    = 10 * time.Second
	maxConnLifetime   = time.Hour
	maxConnIdleTime   = 30 * time.Minute
	healthCheckPeriod = time.Minute
)

func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(buildDSN(cfg))
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	if cfg.MaxConns > 0 {
		pcfg.MaxConns = cfg.MaxConns
	} else {
		pcfg.MaxConns = int32(runtime.GOMAXPROCS(0)) * 2 //nolint:gosec
	}
	pcfg.MinConns = 0
	pcfg.MaxConnLifetime = maxConnLifetime
	pcfg.MaxConnIdleTime = maxConnIdleTime
	pcfg.HealthCheckPeriod = healthCheckPeriod

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

func buildDSN(cfg config.DatabaseConfig) string {
	kv := func(k, v string) string {
		return k + "='" + strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), `'`, `\'`) + "'"
	}
	return strings.Join([]string{
		kv("host", cfg.Host),
		fmt.Sprintf("port=%d", cfg.Port),
		kv("user", cfg.User),
		kv("password", cfg.Password),
		kv("dbname", cfg.Name),
		kv("sslmode", cfg.SSLMode),
	}, " ")
}
