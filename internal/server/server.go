package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
	"github.com/sarga-app/sarga/internal/platform/config"
	"github.com/sarga-app/sarga/internal/platform/db/queries"
	"github.com/sarga-app/sarga/internal/platform/obs"
	"github.com/sarga-app/sarga/internal/server/api"
	"github.com/sarga-app/sarga/internal/server/api/auth"
)

const (
	shutdownTimeout = 30 * time.Second

	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20

	jwtSecretEnvOverride = "SARGA_AUTH_JWT_SECRET" //nolint:gosec // env var name, not a credential
	jwtSecretMinBytes    = 32
)

func Run(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) error {
	slog.SetDefault(obs.NewLogger(cfg.Log))

	secret, err := resolveJWTSecret(cfg.State.Dir)
	if err != nil {
		return fmt.Errorf("resolve jwt secret: %w", err)
	}

	q := queries.New(pool)
	authH := auth.NewHandler(q, coreauth.JWTConfig{Secret: secret})

	deps := Deps{
		DB:  pool,
		API: api.Deps{Auth: authH},
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           newRouter(deps),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("listening", "addr", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		slog.Info("shutdown signal received", "timeout", shutdownTimeout)
		// Fresh context: gctx is already cancelled. A new deadline lets
		// in-flight requests drain instead of being aborted immediately.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx) //nolint:contextcheck // see comment above
	})

	if err := g.Wait(); err != nil {
		return err
	}
	slog.Info("shutdown complete")
	return nil
}

func resolveJWTSecret(stateDir string) ([]byte, error) {
	if env := os.Getenv(jwtSecretEnvOverride); env != "" {
		if len(env) < jwtSecretMinBytes {
			return nil, fmt.Errorf("%s must be at least %d bytes", jwtSecretEnvOverride, jwtSecretMinBytes)
		}
		slog.Info("jwt secret: using env override")
		return []byte(env), nil
	}
	return coreauth.LoadOrCreateSecretFile(stateDir)
}
