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
	"github.com/sarga-app/sarga/internal/enricher"
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

func Run(ctx context.Context, cfg *config.Config, dbInst *db.DB) error {
	dirs, err := data.New(cfg.Data.Dir)
	if err != nil {
		return fmt.Errorf("init data dir: %w", err)
	}
	secret, err := resolveJWTSecret(cfg.Secrets.JWT, dirs)
	if err != nil {
		return fmt.Errorf("resolve jwt secret: %w", err)
	}
	masterKey, err := resolveMasterKey(cfg.Secrets.Master, dirs)
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

	matcher := enricher.NewMatcher(nil, nil, nil)
	enr := enricher.NewEnricher(dbInst, artMgr, matcher, nil, nil, nil, nil)

	scheduler := scanner.NewScheduler(sc)
	watcher, werr := scanner.NewWatcher(sc)
	if werr != nil {
		slog.Warn("fsnotify unavailable; file watch disabled", "err", werr)
		watcher = nil
	}

	initialSettings, err := queries.New(dbInst.Read()).GetServerSettings(ctx)
	if err != nil {
		slog.Warn("art: failed to load initial cache limit, defaulting to 500 MB", "err", err)
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

	go resumeInterruptedScans(ctx, dbInst, sc)

	enrichCtx, enrichCancel := context.WithCancel(ctx)
	defer enrichCancel()
	go enr.Run(enrichCtx)

	if err := scheduler.LoadFromDB(enrichCtx, dbInst); err != nil {
		slog.Error("failed to load scan schedules", "err", err)
	}
	scheduler.Start()
	defer scheduler.Stop()

	if watcher == nil {
		slog.Info("watcher: disabled (fsnotify unavailable)")
	} else {
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()
		startWatcher(watchCtx, watcher, dbInst)
	}

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
		slog.Info("listening", "addr", cfg.Server.Addr)
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

func resumeInterruptedScans(ctx context.Context, d *db.DB, sc *scanner.Scanner) {
	rows, err := d.Read().QueryContext(ctx, `SELECT DISTINCT library_id FROM scan_runs WHERE status = 'running'`)
	if err != nil {
		slog.Error("resume scans: query failed", "err", err)
		return
	}
	defer rows.Close() //nolint:errcheck
	var libs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			libs = append(libs, id)
		}
	}
	if err := rows.Err(); err != nil {
		slog.Error("resume scans: iterate rows", "err", err)
	}
	if len(libs) == 0 {
		return
	}
	if _, err := d.Write().ExecContext(ctx,
		`UPDATE scan_runs SET status = 'failed', error = 'interrupted by server restart' WHERE status = 'running'`,
	); err != nil {
		slog.Error("resume scans: mark failed", "err", err)
	}
	for _, libID := range libs {
		slog.Info("resume: restarting interrupted scan", "library_id", libID)
		go func(id string) {
			if err := sc.Scan(ctx, id); err != nil {
				slog.Error("resume: scan failed", "library_id", id, "err", err)
			}
		}(libID)
	}
}

func resolveMasterKey(configured string, dirs *data.Dirs) (*crypto.MasterKey, error) {
	if configured != "" {
		k, err := crypto.ParseMasterKey(configured)
		if err != nil {
			return nil, err
		}
		slog.Info("master key: using configured override")
		return k, nil
	}
	path, err := dirs.SecretFile("master.key")
	if err != nil {
		return nil, err
	}
	return crypto.LoadOrGenerateMasterKey(path)
}

func startWatcher(ctx context.Context, w *scanner.Watcher, dbInst *db.DB) {
	pathMap, err := w.LoadPathsFromDB(ctx, dbInst)
	if err != nil {
		slog.Error("failed to build watcher path map", "err", err)
		return
	}
	for root := range pathMap {
		if err := w.Watch(root); err != nil {
			slog.Error("watcher: failed to watch path", "path", root, "err", err)
		}
	}
	slog.Info("watcher: monitoring library roots", "count", len(pathMap))
	go w.Run(ctx, pathMap)
}

func resolveJWTSecret(configured string, dirs *data.Dirs) ([]byte, error) {
	if configured != "" {
		slog.Info("jwt secret: using configured override")
		return []byte(configured), nil
	}
	path, err := dirs.SecretFile(crypto.JWTSecretFileName)
	if err != nil {
		return nil, err
	}
	return crypto.LoadOrGenerateJWTSecret(path)
}
