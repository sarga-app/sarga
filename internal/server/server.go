package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"

	coreauth "github.com/sarga-app/sarga/internal/core/auth"
	"github.com/sarga-app/sarga/internal/platform/config"
	"github.com/sarga-app/sarga/internal/platform/datadir"
	"github.com/sarga-app/sarga/internal/platform/db/queries"
	"github.com/sarga-app/sarga/internal/platform/obs"
	"github.com/sarga-app/sarga/internal/server/routes/api"
	"github.com/sarga-app/sarga/internal/server/routes/api/auth"
)

const (
	shutdownTimeout = 30 * time.Second

	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20
)

func Run(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool) error {
	slog.SetDefault(obs.NewLogger(cfg.Log))

	dirs, err := datadir.New(datadir.DefaultRoot)
	if err != nil {
		return fmt.Errorf("init data dir: %w", err)
	}
	secret, err := resolveJWTSecret(cfg.Auth.JWT.Secret, dirs)
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

// resolveJWTSecret returns the JWT signing secret. If configured (via YAML or
// SARGA_AUTH_JWT_SECRET env), uses that — it's already length-validated by
// Config.Validate. Otherwise loads/generates the file at <data>/secrets/jwt.key.
func resolveJWTSecret(configured string, dirs *datadir.Dirs) ([]byte, error) {
	if configured != "" {
		slog.Info("jwt secret: using configured override")
		return []byte(configured), nil
	}
	path, err := dirs.SecretFile(coreauth.JWTSecretFileName)
	if err != nil {
		return nil, err
	}
	return coreauth.LoadOrCreateSecretFile(path)
}
