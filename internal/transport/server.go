package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/config"
	"github.com/sarga-app/sarga/internal/support/datadir"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/routes/api"
	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/docs"
)

const (
	shutdownTimeout = 30 * time.Second

	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20
)

func Run(ctx context.Context, cfg *config.Config, dbInst *db.DB) error {
	dirs, err := datadir.New(cfg.Data.Dir)
	if err != nil {
		return fmt.Errorf("init data dir: %w", err)
	}
	secret, err := resolveJWTSecret(cfg.Auth.JWT.Secret, dirs)
	if err != nil {
		return fmt.Errorf("resolve jwt secret: %w", err)
	}

	q := queries.New(dbInst.Write())
	authH := auth.NewHandler(q, domauth.JWTConfig{Secret: secret})
	docsH, err := docs.NewHandler()
	if err != nil {
		return fmt.Errorf("init docs handler: %w", err)
	}

	deps := Deps{
		DB:  dbInst,
		API: api.Deps{Auth: authH, Docs: docsH},
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

func resolveJWTSecret(configured string, dirs *datadir.Dirs) ([]byte, error) {
	if configured != "" {
		slog.Info("jwt secret: using configured override")
		return []byte(configured), nil
	}
	path, err := dirs.SecretFile(domauth.JWTSecretFileName)
	if err != nil {
		return nil, err
	}
	return domauth.LoadOrCreateSecretFile(path)
}
