package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/coverart"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
)

var ErrScanAlreadyRunning = errors.New("scanner: scan already running for this library")

type Scanner struct {
	rq      *queries.Queries
	wq      *queries.Queries
	art     *coverart.Manager
	rec     *Reconciler
	db      *sql.DB
	matcher *Matcher
}

func New(d *db.DB, art *coverart.Manager, matcher *Matcher) *Scanner {
	rq := queries.New(d.Read())
	wq := queries.New(d.Write())
	return &Scanner{
		rq:      rq,
		wq:      wq,
		art:     art,
		rec:     NewReconciler(rq),
		db:      d.Write(),
		matcher: matcher,
	}
}

func (s *Scanner) Scan(ctx context.Context, libraryID string) error {
	settings, err := s.rq.GetLibrarySettings(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("scanner: load settings: %w", err)
	}
	paths, err := s.rq.ListLibraryPaths(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("scanner: load paths: %w", err)
	}
	if len(paths) == 0 {
		return nil
	}

	if active, aerr := s.rq.GetActiveScanRun(ctx, libraryID); aerr == nil && active.ID != "" {
		return ErrScanAlreadyRunning
	}

	run, err := s.wq.InsertScanRun(ctx, queries.InsertScanRunParams{
		ID:        uuid.NewString(),
		LibraryID: libraryID,
	})
	if err != nil {
		return fmt.Errorf("scanner: create scan run: %w", err)
	}

	slog.Info("scan: started", "library_id", libraryID, "run_id", run.ID, "paths", len(paths))
	for _, p := range paths {
		slog.Debug("scan: path", "path", p.Path)
	}

	pr, scanErr := s.runScan(ctx, libraryID, run.ID, paths, settings)

	status, errMsg := "completed", (*string)(nil)
	if scanErr != nil && !errors.Is(scanErr, context.Canceled) {
		errStr := scanErr.Error()
		status, errMsg = "failed", &errStr
		slog.Error("scan: failed", "library_id", libraryID, "run_id", run.ID, "err", scanErr)
	} else {
		t := pr.totals()
		slog.Info("scan: completed",
			"library_id", libraryID, "run_id", run.ID,
			"total", t.Total, "added", t.Added, "updated", t.Updated,
			"unchanged", t.Unchanged, "moved", t.Moved,
			"removed", t.Removed, "errors", t.Errored,
		)
	}
	if err := s.wq.FinishScanRun(ctx, queries.FinishScanRunParams{
		Status: status,
		Error:  errMsg,
		ID:     run.ID,
	}); err != nil {
		slog.Error("scan: finish run", "run_id", run.ID, "err", err)
	}
	return scanErr
}

func (s *Scanner) runScan(
	ctx context.Context,
	libraryID, runID string,
	paths []queries.LibraryPath,
	settings queries.LibrarySetting,
) (*progress, error) {
	scanCfg := ScanSettings{
		Extensions:     ParseExtensions(settings.FileExtensions),
		FollowSymlinks: settings.FollowSymlinks,
	}
	workerN := min(runtime.NumCPU(), 8)
	slog.Debug("scan: worker pool", "workers", workerN)

	pr := &progress{runID: runID}

	for _, lp := range paths {
		if ctx.Err() != nil {
			return pr, ctx.Err()
		}
		slog.Info("scan: walking path", "path", lp.Path)
		if err := s.processPath(ctx, libraryID, lp.Path, scanCfg, workerN, pr); err != nil {
			return pr, err
		}
	}

	if err := markMissingTrackFiles(ctx, s.db, libraryID, pr.snapshotFound()); err != nil {
		slog.Error("scan: mark missing tracks", "library_id", libraryID, "err", err)
	}

	pr.flushFinal(ctx, s.wq)

	if _, err := s.db.ExecContext(ctx,
		"UPDATE scan_runs SET total_files = ? WHERE id = ?",
		pr.totals().Total, runID,
	); err != nil {
		slog.Error("scan: set total_files", "run_id", runID, "err", err)
	}

	return pr, nil
}
