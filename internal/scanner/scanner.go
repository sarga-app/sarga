package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

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

	dirtyMu sync.Mutex
	dirty   map[string]bool
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
		dirty:   make(map[string]bool),
	}
}

func (s *Scanner) markDirty(libraryID string) {
	s.dirtyMu.Lock()
	s.dirty[libraryID] = true
	s.dirtyMu.Unlock()
}

func (s *Scanner) takeDirty(libraryID string) bool {
	s.dirtyMu.Lock()
	defer s.dirtyMu.Unlock()
	was := s.dirty[libraryID]
	delete(s.dirty, libraryID)
	return was
}

func (s *Scanner) Scan(ctx context.Context, libraryID string) error {
	settings, paths, err := s.loadLibrary(ctx, libraryID)
	if err != nil || len(paths) == 0 {
		return err
	}

	if active, aerr := s.rq.GetActiveScanRun(ctx, libraryID); aerr == nil && active.ID != "" {
		s.markDirty(libraryID)
		slog.Info("scan: coalesced — another scan is running",
			"library_id", libraryID, "active_run", active.ID)
		return ErrScanAlreadyRunning
	}

	run, err := s.wq.InsertScanRun(ctx, queries.InsertScanRunParams{
		ID:        uuid.NewString(),
		LibraryID: libraryID,
	})
	if err != nil {
		return fmt.Errorf("scanner: create scan run: %w", err)
	}

	return s.doScan(ctx, libraryID, run.ID, "", paths, settings)
}

func (s *Scanner) Resume(ctx context.Context, libraryID string) error {
	settings, paths, err := s.loadLibrary(ctx, libraryID)
	if err != nil || len(paths) == 0 {
		return err
	}

	active, aerr := s.rq.GetActiveScanRun(ctx, libraryID)
	if aerr != nil || active.ID == "" {
		return nil //nolint:nilerr // no interrupted run is not an error condition
	}
	cursor := ""
	if active.LastCompletedFolder != nil {
		cursor = *active.LastCompletedFolder
	}
	slog.Info("scan: resuming interrupted run",
		"library_id", libraryID, "run_id", active.ID, "cursor", cursor)
	return s.doScan(ctx, libraryID, active.ID, cursor, paths, settings)
}

func (s *Scanner) loadLibrary(ctx context.Context, libraryID string) (queries.LibrarySetting, []queries.LibraryPath, error) {
	settings, err := s.rq.GetLibrarySettings(ctx, libraryID)
	if err != nil {
		return settings, nil, fmt.Errorf("scanner: load settings: %w", err)
	}
	paths, err := s.rq.ListLibraryPaths(ctx, libraryID)
	if err != nil {
		return settings, nil, fmt.Errorf("scanner: load paths: %w", err)
	}
	return settings, paths, nil
}

func (s *Scanner) doScan(
	ctx context.Context,
	libraryID, runID, cursor string,
	paths []queries.LibraryPath,
	settings queries.LibrarySetting,
) error {
	slog.Info("scan: started",
		"library_id", libraryID, "run_id", runID,
		"paths", len(paths), "cursor", cursor)
	for _, p := range paths {
		slog.Debug("scan: path", "path", p.Path)
	}

	pr, scanErr := s.runScan(ctx, libraryID, runID, cursor, paths, settings)

	status, errMsg := "completed", (*string)(nil)
	if scanErr != nil && !errors.Is(scanErr, context.Canceled) {
		errStr := scanErr.Error()
		status, errMsg = "failed", &errStr
		slog.Error("scan: failed", "library_id", libraryID, "run_id", runID, "err", scanErr)
	} else {
		t := pr.totals()
		walk, pipeline, missing, gc := pr.timings()
		slog.Info("scan: completed",
			"library_id", libraryID, "run_id", runID,
			"total", t.Total, "added", t.Added, "updated", t.Updated,
			"unchanged", t.Unchanged, "moved", t.Moved,
			"removed", t.Removed, "errors", t.Errored,
			"walk", walk.Round(time.Millisecond),
			"pipeline", pipeline.Round(time.Millisecond),
			"missing", missing.Round(time.Millisecond),
			"gc", gc.Round(time.Millisecond),
		)
	}
	if err := s.wq.FinishScanRun(ctx, queries.FinishScanRunParams{
		Status: status,
		Error:  errMsg,
		ID:     runID,
	}); err != nil {
		slog.Error("scan: finish run", "run_id", runID, "err", err)
	}

	if s.takeDirty(libraryID) {
		slog.Info("scan: respawning for queued changes", "library_id", libraryID)
		go func() { //nolint:gosec,contextcheck // respawn must outlive caller's request-scoped ctx
			respawnCtx := context.Background()
			if rerr := s.Scan(respawnCtx, libraryID); rerr != nil && !errors.Is(rerr, ErrScanAlreadyRunning) {
				slog.Error("scan: respawn failed", "library_id", libraryID, "err", rerr)
			}
		}()
	}
	return scanErr
}

func (s *Scanner) runScan(
	ctx context.Context,
	libraryID, runID, cursor string,
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
		if err := s.processPath(ctx, libraryID, lp.Path, scanCfg, cursor, workerN, pr); err != nil {
			return pr, err
		}
	}

	missingStart := time.Now()
	if err := markMissingTrackFiles(ctx, s.db, libraryID, pr.snapshotFound()); err != nil {
		slog.Error("scan: mark missing tracks", "library_id", libraryID, "err", err)
	}
	pr.setMissingDur(time.Since(missingStart))

	gcStart := time.Now()
	if _, err := pruneOrphans(ctx, s.db); err != nil {
		slog.Error("scan: gc", "library_id", libraryID, "err", err)
	}
	pr.setGCDur(time.Since(gcStart))

	pr.flushFinal(ctx, s.wq)

	if _, err := s.db.ExecContext(ctx,
		"UPDATE scan_runs SET total_files = ? WHERE id = ?",
		pr.totals().Total, runID,
	); err != nil {
		slog.Error("scan: set total_files", "run_id", runID, "err", err)
	}

	return pr, nil
}
