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
	"github.com/sarga-app/sarga/internal/domain/library"
	"github.com/sarga-app/sarga/internal/support/config"
	"github.com/sarga-app/sarga/internal/support/crypto"
	"github.com/sarga-app/sarga/internal/support/datadir"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/routes/api"
	"github.com/sarga-app/sarga/internal/transport/routes/api/auth"
	"github.com/sarga-app/sarga/internal/transport/routes/api/catalog"
	"github.com/sarga-app/sarga/internal/transport/routes/api/docs"
	"github.com/sarga-app/sarga/internal/transport/routes/api/libraries"
	"github.com/sarga-app/sarga/internal/transport/routes/art"
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
	masterKey, err := resolveMasterKey(dirs)
	if err != nil {
		return fmt.Errorf("resolve master key: %w", err)
	}

	q := queries.New(dbInst.Write())
	authH := auth.NewHandler(q, domauth.JWTConfig{Secret: secret})

	artDir, err := dirs.CacheDir("art")
	if err != nil {
		return fmt.Errorf("init art dir: %w", err)
	}
	artMgr, err := library.NewCoverArtManager(artDir)
	if err != nil {
		return fmt.Errorf("init cover art manager: %w", err)
	}
	libStore := newLibraryStore(dbInst)
	scanner := library.NewScanner(libStore, artMgr, dbInst.Write())

	matcher := library.NewMatcher(nil, nil, nil)
	enricher := library.NewEnricher(libStore, artMgr, matcher, nil, nil, nil, nil)

	scheduler := library.NewScheduler(scanner)
	watcher, werr := library.NewWatcher(scanner)
	if werr != nil {
		slog.Warn("fsnotify unavailable; file watch disabled", "err", werr)
		watcher = nil
	}

	jwt := domauth.JWTConfig{Secret: secret}
	libsH := libraries.NewHandler(dbInst, scanner, jwt)
	docsH, err := docs.NewHandler()
	if err != nil {
		return fmt.Errorf("init docs handler: %w", err)
	}
	catalogH := catalog.NewHandler(dbInst, jwt)
	artH := art.NewHandler(artDir)

	deps := Deps{
		DB:        dbInst,
		MasterKey: masterKey,
		API: api.Deps{
			Auth:      authH,
			Docs:      docsH,
			Libraries: libsH,
			Catalog:   catalogH,
			Art:       artH,
		},
	}

	go resumeInterruptedScans(ctx, dbInst, scanner)

	// Start background workers.
	enrichCtx, enrichCancel := context.WithCancel(ctx)
	defer enrichCancel()
	go enricher.Run(enrichCtx)

	// Load scheduled scans from library_settings.
	if err := loadSchedules(enrichCtx, dbInst, scheduler); err != nil {
		slog.Error("failed to load scan schedules", "err", err)
	}
	scheduler.Start()
	defer scheduler.Stop()

	if watcher != nil {
		watchCtx, watchCancel := context.WithCancel(ctx)
		defer watchCancel()
		pathMap, err := buildPathMap(enrichCtx, dbInst)
		if err != nil {
			slog.Error("failed to build watcher path map", "err", err)
		} else {
			for root := range pathMap {
				if err := watcher.Watch(root); err != nil {
					slog.Error("watcher: failed to watch path", "path", root, "err", err)
				}
			}
			go watcher.Run(watchCtx, pathMap)
		}
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

func resumeInterruptedScans(ctx context.Context, d *db.DB, sc *library.Scanner) {
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

func resolveMasterKey(dirs *datadir.Dirs) (*crypto.MasterKey, error) {
	path, err := dirs.SecretFile("master.key")
	if err != nil {
		return nil, err
	}
	return crypto.LoadOrGenerate(path)
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
