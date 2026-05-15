package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/sarga-app/sarga/internal/auth"
	"github.com/sarga-app/sarga/internal/config"
	"github.com/sarga-app/sarga/internal/coverart"
	"github.com/sarga-app/sarga/internal/crypto"
	"github.com/sarga-app/sarga/internal/data"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/internal/library"
	"github.com/sarga-app/sarga/internal/scanner"
	"github.com/sarga-app/sarga/internal/server/api"
	"github.com/sarga-app/sarga/internal/server/public"
)

const (
	shutdownTimeout = 30 * time.Second

	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 1 << 20
)

func Run(ctx context.Context, cfg *config.Config, dbInst *db.DB, startedAt time.Time) error {
	dirs, err := data.New(cfg.Data.Dir)
	if err != nil {
		return fmt.Errorf("init data dir: %w", err)
	}
	jwtPath, err := dirs.SecretFile(crypto.JWTSecretFileName)
	if err != nil {
		return err
	}
	secret, err := crypto.ResolveJWTSecret(cfg.Secrets.JWT, jwtPath)
	if err != nil {
		return fmt.Errorf("resolve jwt secret: %w", err)
	}
	masterPath, err := dirs.SecretFile("master.key")
	if err != nil {
		return err
	}
	masterKey, err := crypto.ResolveMasterKey(cfg.Secrets.Master, masterPath)
	if err != nil {
		return fmt.Errorf("resolve master key: %w", err)
	}

	artDir, err := dirs.CacheDir("art")
	if err != nil {
		return fmt.Errorf("init art dir: %w", err)
	}
	artMgr, err := coverart.NewManager(artDir)
	if err != nil {
		return fmt.Errorf("init cover art manager: %w", err)
	}
	sc := scanner.New(dbInst, artMgr)

	scheduler := library.NewScheduler(sc)
	watcher, werr := library.NewWatcher(sc)
	if werr != nil {
		slog.Warn("watcher: disabled",
			"reason", "fsnotify unavailable",
			"err", werr,
			"action", "rebuild on a platform with inotify/kqueue support")
		watcher = nil
	}

	initialSettings, err := queries.New(dbInst.Read()).GetServerSettings(ctx)
	if err != nil {
		slog.Warn("art: cache limit fallback",
			"err", err,
			"default_mb", 500,
			"action", "set art.thumb_cache_mb in server_settings to override")
	}
	var initialLimitMB int64 = 500
	if err == nil {
		initialLimitMB = initialSettings.ArtThumbCacheMb
	}

	publicH := public.New(artDir, initialLimitMB)

	cipher, err := crypto.NewCipher(masterKey)
	if err != nil {
		return fmt.Errorf("init secret box: %w", err)
	}

	jwt := auth.JWTConfig{Secret: secret}
	apiH := api.New(dbInst, sc, jwt, cipher, publicH)

	go sc.ResumeAll(ctx, dbInst)

	if err := scheduler.LoadFromDB(ctx, dbInst); err != nil {
		slog.Error("scheduler: load schedules failed", "err", err)
	}
	scheduler.Start()
	defer scheduler.Stop()

	if watcher != nil {
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()
		watcher.Start(watchCtx, dbInst)
	}

	auth.LogConfig()
	logFirstBootHint(ctx, dbInst)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           newRouter(dbInst, apiH, publicH),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		slog.Info("sarga: ready",
			"addr", cfg.Server.Addr,
			"startup", time.Since(startedAt).Round(time.Millisecond),
			"tls", "off")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		slog.Info("shutdown signal received", "timeout", shutdownTimeout)
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

func logFirstBootHint(ctx context.Context, d *db.DB) {
	var n int64
	if err := d.Read().QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&n); err != nil {
		return
	}
	if n == 0 {
		slog.Info("setup: no users found",
			"action", "run 'sarga admin create-user' or enable registration in server settings")
	}
}
