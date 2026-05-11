package library

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/support/db/queries"
)

var ErrScanAlreadyRunning = errors.New("scanner: scan already running for this library")

const progressFlushEvery = 500

type Scanner struct {
	store Store
	art   *CoverArtManager
	rec   *Reconciler
	db    *sql.DB
}

func NewScanner(store Store, art *CoverArtManager, db *sql.DB) *Scanner {
	return &Scanner{
		store: store,
		art:   art,
		rec:   NewReconciler(store),
		db:    db,
	}
}

func (s *Scanner) Scan(ctx context.Context, libraryID string) error {
	settings, err := s.store.GetLibrarySettings(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("scanner: load settings: %w", err)
	}
	paths, err := s.store.ListLibraryPaths(ctx, libraryID)
	if err != nil {
		return fmt.Errorf("scanner: load paths: %w", err)
	}
	if len(paths) == 0 {
		return nil
	}

	// 409 guard: reject if a scan is already running for this library.
	active, err := s.store.GetActiveScanRun(ctx, libraryID)
	if err == nil && active.ID != "" {
		return ErrScanAlreadyRunning
	}

	run, err := s.store.InsertScanRun(ctx, queries.InsertScanRunParams{
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
	st, scanErr := s.runScan(ctx, libraryID, run.ID, paths, settings)

	status, errMsg := "completed", (*string)(nil)
	if scanErr != nil && !errors.Is(scanErr, context.Canceled) {
		errStr := scanErr.Error()
		status, errMsg = "failed", &errStr
		slog.Error("scan: failed", "library_id", libraryID, "run_id", run.ID, "err", scanErr)
	} else {
		slog.Info("scan: completed",
			"library_id", libraryID,
			"run_id", run.ID,
			"added", st.added,
			"updated", st.updated,
			"unchanged", st.unchanged,
			"removed", st.removed,
			"errors", st.errored,
		)
		s.backfillDurations(ctx, libraryID)
	}
	if err := s.store.FinishScanRun(ctx, queries.FinishScanRunParams{
		Status: status,
		Error:  errMsg,
		ID:     run.ID,
	}); err != nil {
		slog.Error("scan: finish run", "run_id", run.ID, "err", err)
	}
	return scanErr
}

func (s *Scanner) backfillDurations(ctx context.Context, libraryID string) {
	rows, err := s.store.ListTrackFilesNullDuration(ctx, libraryID)
	if err != nil {
		slog.Error("scan: backfill durations: query failed", "library_id", libraryID, "err", err)
		return
	}
	if len(rows) == 0 {
		return
	}
	slog.Info("scan: backfilling durations", "library_id", libraryID, "count", len(rows))
	updated := 0
	for _, row := range rows {
		dur := DurationFromFile(row.RelativePath)
		if dur <= 0 {
			slog.Warn("scan: duration unreadable", "path", row.RelativePath)
			continue
		}
		if err := s.store.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
			DurationMs: &dur,
			ID:         row.TrackID,
		}); err != nil {
			slog.Error("scan: backfill duration update failed", "track_id", row.TrackID, "err", err)
			continue
		}
		updated++
	}
	slog.Info("scan: durations backfilled", "library_id", libraryID, "updated", updated, "total", len(rows))
}

func (s *Scanner) runScan(
	ctx context.Context,
	libraryID, runID string,
	paths []queries.LibraryPath,
	settings queries.LibrarySetting,
) (*scanProgress, error) {
	scanCfg := ScanSettings{
		Extensions:     ParseExtensions(settings.FileExtensions),
		FollowSymlinks: settings.FollowSymlinks,
	}
	workerN := min(runtime.NumCPU(), 8)
	slog.Debug("scan: worker pool", "workers", workerN)

	st := &scanProgress{runID: runID}

	for _, lp := range paths {
		if ctx.Err() != nil {
			return st, ctx.Err()
		}
		slog.Info("scan: walking path", "path", lp.Path)
		if err := s.processPath(ctx, libraryID, lp.Path, scanCfg, workerN, st); err != nil {
			return st, err
		}
	}

	// Bulk-mark track_files no longer on disk.
	st.mu.Lock()
	fp := make([]string, len(st.foundPaths))
	copy(fp, st.foundPaths)
	removedDelta := st.removed - st.flushed.removed
	st.mu.Unlock()

	if err := MarkMissingTrackFiles(ctx, s.db, libraryID, fp); err != nil {
		slog.Error("scan: mark missing tracks", "library_id", libraryID, "err", err)
	}

	// Final progress flush.
	st.mu.Lock()
	total := st.total
	if err := s.store.UpdateScanRunProgress(ctx, queries.UpdateScanRunProgressParams{
		FilesScanned:   total - st.flushed.total,
		FilesAdded:     st.added - st.flushed.added,
		FilesUpdated:   st.updated - st.flushed.updated,
		FilesRemoved:   removedDelta,
		FilesUnchanged: st.unchanged - st.flushed.unchanged,
		FilesErrored:   st.errored - st.flushed.errored,
		ID:             runID,
	}); err != nil {
		slog.Error("scan: update progress", "run_id", runID, "err", err)
	}
	st.mu.Unlock()

	if _, err := s.db.ExecContext(ctx,
		"UPDATE scan_runs SET total_files = ? WHERE id = ?",
		total, runID,
	); err != nil {
		slog.Error("scan: set total_files", "run_id", runID, "err", err)
	}

	return st, nil
}

func (s *Scanner) processPath(
	ctx context.Context,
	libraryID, root string,
	cfg ScanSettings,
	workerN int,
	st *scanProgress,
) error {
	events, errc := s.rec.Reconcile(ctx, libraryID, root, cfg)

	sem := make(chan struct{}, workerN)
	var wg sync.WaitGroup

	for ev := range events {
		switch ev.Kind {
		case EventAdded:
			slog.Debug("scan: file added", "path", ev.AbsPath)
			st.mu.Lock()
			st.foundPaths = append(st.foundPaths, ev.AbsPath)
			st.mu.Unlock()
			wg.Add(1)
			sem <- struct{}{}
			go func(e FileEvent) {
				defer func() { <-sem; wg.Done() }()
				s.processFile(ctx, libraryID, e, st)
			}(ev)

		case EventChanged:
			slog.Debug("scan: file changed", "path", ev.AbsPath)
			st.mu.Lock()
			st.foundPaths = append(st.foundPaths, ev.AbsPath)
			st.mu.Unlock()
			wg.Add(1)
			sem <- struct{}{}
			go func(e FileEvent) {
				defer func() { <-sem; wg.Done() }()
				s.processFile(ctx, libraryID, e, st)
			}(ev)

		case EventUnchanged:
			st.mu.Lock()
			st.total++
			st.unchanged++
			st.foundPaths = append(st.foundPaths, ev.AbsPath)
			st.mu.Unlock()

		case EventRemoved:
			slog.Debug("scan: file removed", "path", ev.AbsPath)
			st.mu.Lock()
			st.removed++
			st.mu.Unlock()
		}
	}

	wg.Wait()
	return <-errc
}

func (s *Scanner) processFile(ctx context.Context, libraryID string, ev FileEvent, st *scanProgress) {
	hash, err := HashFile(ev.AbsPath)
	if err != nil {
		slog.Warn("scan: hash failed", "path", ev.AbsPath, "err", err)
		st.recordErr()
		return
	}
	meta, err := Extract(ev.AbsPath)
	if err != nil {
		slog.Warn("scan: tag extract failed", "path", ev.AbsPath, "err", err)
		st.recordErr()
		return
	}
	if meta.DurationMs == 0 {
		meta.DurationMs = DurationFromFile(ev.AbsPath)
	}
	slog.Debug("scan: file", "path", ev.AbsPath, "title", meta.Title, "duration_ms", meta.DurationMs, "format", meta.Format)
	artHash := ""
	if len(meta.EmbeddedArt) > 0 {
		artHash, _ = s.art.ProcessEmbedded(meta.EmbeddedArt) //nolint:errcheck
	}
	if artHash == "" {
		artHash, _ = s.art.FindFolderArt(filepath.Dir(ev.AbsPath)) //nolint:errcheck
	}

	// Serialize all DB work — prevents get-or-create races between workers.
	st.mu.Lock()
	defer st.mu.Unlock()

	if err := s.upsertFile(ctx, libraryID, ev, hash, meta, artHash, st); err != nil {
		st.errored++
	}
	st.maybeFlush(ctx, s.store)
}

func (s *Scanner) upsertFile(
	ctx context.Context,
	libraryID string,
	ev FileEvent,
	hash []byte,
	meta Metadata,
	artHash string,
	st *scanProgress,
) error {
	if ev.Kind == EventChanged && ev.DBEntry != nil { //nolint:nestif
		existing, err := s.store.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
			LibraryID:    libraryID,
			RelativePath: ev.AbsPath,
		})
		if err == nil && bytes.Equal(existing.ContentHash, hash) {
			if err2 := s.store.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
				TrackID:       existing.TrackID,
				Format:        existing.Format,
				Codec:         existing.Codec,
				BitrateKbps:   existing.BitrateKbps,
				SampleRateHz:  existing.SampleRateHz,
				BitDepth:      existing.BitDepth,
				Channels:      existing.Channels,
				FileSizeBytes: ev.Info.Size(),
				Mtime:         ev.Info.ModTime().UTC().Format(time.RFC3339),
				ContentHash:   hash,
				IsLossless:    existing.IsLossless,
				RgTrackGain:   existing.RgTrackGain,
				RgTrackPeak:   existing.RgTrackPeak,
				RgAlbumGain:   existing.RgAlbumGain,
				RgAlbumPeak:   existing.RgAlbumPeak,
				ID:            existing.ID,
			}); err2 != nil {
				slog.Error("scan: update unchanged file core", "path", ev.AbsPath, "err", err2)
			}
			if dur := DurationFromFile(ev.AbsPath); dur > 0 {
				if err2 := s.store.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
					DurationMs: &dur,
					ID:         existing.TrackID,
				}); err2 != nil {
					slog.Error("scan: update duration", "track_id", existing.TrackID, "err", err2)
				}
			}
			st.unchanged++
			return nil
		}
	}

	artistID, err := s.getOrCreateArtist(ctx, meta)
	if err != nil {
		return err
	}
	rgID, err := s.getOrCreateReleaseGroup(ctx, meta)
	if err != nil {
		return err
	}
	releaseID, err := s.getOrCreateRelease(ctx, meta, rgID)
	if err != nil {
		return err
	}
	trackID, err := s.getOrCreateTrack(ctx, meta)
	if err != nil {
		return err
	}

	s.store.LinkReleaseArtist(ctx, queries.LinkReleaseArtistParams{ //nolint:errcheck
		ReleaseID: releaseID, ArtistID: artistID, Position: 1, Role: "primary",
	})
	s.store.LinkTrackArtist(ctx, queries.LinkTrackArtistParams{ //nolint:errcheck
		TrackID: trackID, ArtistID: artistID, Position: 1, Role: "primary",
	})
	s.store.UpsertReleaseTrack(ctx, queries.UpsertReleaseTrackParams{ //nolint:errcheck
		ReleaseID:  releaseID,
		TrackID:    trackID,
		DiscNumber: int64(max(meta.DiscNum, 1)),
		Position:   int64(meta.TrackNum),
	})

	mtime := ev.Info.ModTime().UTC().Format(time.RFC3339)
	lossless := meta.Format == "flac" || meta.Format == "alac" || meta.Format == "wav" || meta.Format == "aiff"

	rgGain, rgPeak, raGain, raPeak := optFloat(meta.RGTrackGain), optFloat(meta.RGTrackPeak), optFloat(meta.RGAlbumGain), optFloat(meta.RGAlbumPeak)

	var trackFileID string
	if ev.Kind == EventAdded {
		row, err := s.store.InsertTrackFile(ctx, queries.InsertTrackFileParams{
			ID:              uuid.NewString(),
			TrackID:         trackID,
			LibraryID:       libraryID,
			RelativePath:    ev.AbsPath,
			Format:          meta.Format,
			Codec:           meta.Codec,
			Channels:        2,
			FileSizeBytes:   ev.Info.Size(),
			Mtime:           mtime,
			ContentHash:     hash,
			SourceKind:      "import",
			IsLossless:      lossless,
			IsPreferred:     true,
			MatchConfidence: 0,
			MatchSource:     "raw",
			RgTrackGain:     rgGain,
			RgTrackPeak:     rgPeak,
			RgAlbumGain:     raGain,
			RgAlbumPeak:     raPeak,
		})
		if err != nil {
			return err
		}
		trackFileID = row.ID
		st.added++
	} else {
		existing, err := s.store.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
			LibraryID:    libraryID,
			RelativePath: ev.AbsPath,
		})
		if err != nil {
			return err
		}
		trackFileID = existing.ID
		if err2 := s.store.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
			TrackID:       trackID,
			Format:        meta.Format,
			Codec:         meta.Codec,
			Channels:      2,
			FileSizeBytes: ev.Info.Size(),
			Mtime:         mtime,
			ContentHash:   hash,
			IsLossless:    lossless,
			RgTrackGain:   rgGain,
			RgTrackPeak:   rgPeak,
			RgAlbumGain:   raGain,
			RgAlbumPeak:   raPeak,
			ID:            trackFileID,
		}); err2 != nil {
			slog.Error("scan: update track file core", "path", ev.AbsPath, "err", err2)
		}
		st.updated++
	}

	if artHash != "" {
		if _, err := s.store.UpsertCoverArt(ctx, queries.UpsertCoverArtParams{
			ID:          uuid.NewString(),
			ReleaseID:   &releaseID,
			SourceKind:  artSourceKind(meta),
			Uri:         artHash,
			IsPreferred: true,
		}); err != nil {
			slog.Error("scan: upsert cover art", "release_id", releaseID, "err", err)
		}
	}

	if meta.Lyrics != "" {
		if err := s.store.UpsertLyrics(ctx, queries.UpsertLyricsParams{
			ID:       uuid.NewString(),
			TrackID:  trackID,
			Source:   "embedded",
			IsSynced: false,
			Content:  meta.Lyrics,
		}); err != nil {
			slog.Error("scan: upsert lyrics", "track_id", trackID, "err", err)
		}
	}

	priority := int64(0)
	if meta.HasMBID() {
		priority = 10
	}
	if err := s.store.EnqueueTrackFile(ctx, queries.EnqueueTrackFileParams{
		ID:          uuid.NewString(),
		TrackFileID: trackFileID,
		LibraryID:   libraryID,
		Priority:    priority,
	}); err != nil {
		slog.Error("scan: enqueue track file", "track_file_id", trackFileID, "err", err)
	}

	return nil
}

func (s *Scanner) getOrCreateArtist(ctx context.Context, meta Metadata) (string, error) {
	name := meta.AlbumArtist
	if name == "" {
		name = meta.PrimaryArtist()
	}
	if name == "" {
		name = "Unknown Artist"
	}
	sortName := name
	isVA := name == "Various Artists"

	if meta.MBArtistID != "" {
		mbid := meta.MBArtistID
		row, err := s.store.GetArtistByMBID(ctx, &mbid)
		if err == nil {
			return row.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	row, err := s.store.GetArtistByName(ctx, name)
	if err == nil {
		return row.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var mbidPtr *string
	if meta.MBArtistID != "" {
		mbidPtr = &meta.MBArtistID
	}
	slog.Debug("scan: new artist", "name", name)
	inserted, err := s.store.InsertArtist(ctx, queries.InsertArtistParams{
		ID:               uuid.NewString(),
		Mbid:             mbidPtr,
		Name:             name,
		SortName:         sortName,
		IsVariousArtists: isVA,
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func (s *Scanner) getOrCreateReleaseGroup(ctx context.Context, meta Metadata) (string, error) {
	title := meta.Album
	if title == "" {
		title = "Unknown Album"
	}

	if meta.MBReleaseID != "" {
		mbid := meta.MBReleaseID
		rg, err := s.store.GetReleaseGroupByMBID(ctx, &mbid)
		if err == nil {
			return rg.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	rg, err := s.store.GetReleaseGroupByTitle(ctx, title)
	if err == nil {
		return rg.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	year := int64(meta.Year)
	var yearPtr *int64
	if year > 0 {
		yearPtr = &year
	}
	inserted, err := s.store.InsertReleaseGroup(ctx, queries.InsertReleaseGroupParams{
		ID:               uuid.NewString(),
		Title:            title,
		PrimaryType:      "album",
		FirstReleaseYear: yearPtr,
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func (s *Scanner) getOrCreateRelease(ctx context.Context, meta Metadata, rgID string) (string, error) {
	title := meta.Album
	if title == "" {
		title = "Unknown Album"
	}

	if meta.MBReleaseID != "" {
		mbid := meta.MBReleaseID
		rel, err := s.store.GetReleaseByMBID(ctx, &mbid)
		if err == nil {
			return rel.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	rel, err := s.store.GetReleaseByGroupAndTitle(ctx, queries.GetReleaseByGroupAndTitleParams{
		ReleaseGroupID: rgID,
		Title:          title,
	})
	if err == nil {
		return rel.ID, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	var mbidPtr *string
	if meta.MBReleaseID != "" {
		mbidPtr = &meta.MBReleaseID
	}
	var datePtr *string
	if meta.Year > 0 {
		d := fmt.Sprintf("%d", meta.Year)
		datePtr = &d
	}
	totalDiscs := int64(max(meta.DiscTotal, 1))
	inserted, err := s.store.InsertRelease(ctx, queries.InsertReleaseParams{
		ID:             uuid.NewString(),
		Mbid:           mbidPtr,
		ReleaseGroupID: rgID,
		Title:          title,
		ReleaseDate:    datePtr,
		TotalDiscs:     totalDiscs,
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func (s *Scanner) getOrCreateTrack(ctx context.Context, meta Metadata) (string, error) {
	title := meta.Title
	if title == "" {
		title = "Unknown Track"
	}

	if meta.MBRecordingID != "" { //nolint:nestif
		mbid := meta.MBRecordingID
		row, err := s.store.GetTrackByMBID(ctx, &mbid)
		if err == nil {
			if meta.DurationMs > 0 {
				if err2 := s.store.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
					DurationMs: &meta.DurationMs,
					ID:         row.ID,
				}); err2 != nil {
					slog.Error("scan: update track duration", "track_id", row.ID, "err", err2)
				}
			}
			return row.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	var mbidPtr *string
	if meta.MBRecordingID != "" {
		mbidPtr = &meta.MBRecordingID
	}
	var isrcPtr *string
	if meta.ISRC != "" {
		isrcPtr = &meta.ISRC
	}
	var bpmPtr *int64
	if meta.BPM > 0 {
		b := int64(meta.BPM)
		bpmPtr = &b
	}
	var keyPtr *string
	if meta.KeySig != "" {
		keyPtr = &meta.KeySig
	}
	var durPtr *int64
	if meta.DurationMs > 0 {
		d := meta.DurationMs
		durPtr = &d
	}
	inserted, err := s.store.InsertTrack(ctx, queries.InsertTrackParams{
		ID:         uuid.NewString(),
		Mbid:       mbidPtr,
		Title:      title,
		DurationMs: durPtr,
		Isrc:       isrcPtr,
		Bpm:        bpmPtr,
		KeySig:     keyPtr,
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

type scanProgress struct {
	mu         sync.Mutex
	runID      string
	foundPaths []string

	total, added, updated, unchanged, errored, removed int64
	flushed                                            struct {
		total, added, updated, unchanged, errored, removed int64
	}
}

func (st *scanProgress) recordErr() {
	st.mu.Lock()
	st.errored++
	st.mu.Unlock()
}

func (st *scanProgress) maybeFlush(ctx context.Context, store Store) {
	st.total++
	if st.total-st.flushed.total < progressFlushEvery {
		return
	}
	if err := store.UpdateScanRunProgress(ctx, queries.UpdateScanRunProgressParams{
		FilesScanned:   st.total - st.flushed.total,
		FilesAdded:     st.added - st.flushed.added,
		FilesUpdated:   st.updated - st.flushed.updated,
		FilesRemoved:   st.removed - st.flushed.removed,
		FilesUnchanged: st.unchanged - st.flushed.unchanged,
		FilesErrored:   st.errored - st.flushed.errored,
		ID:             st.runID,
	}); err != nil {
		slog.Error("scan: flush progress", "run_id", st.runID, "err", err)
	}
	st.flushed.total = st.total
	st.flushed.added = st.added
	st.flushed.updated = st.updated
	st.flushed.unchanged = st.unchanged
	st.flushed.errored = st.errored
	st.flushed.removed = st.removed
}

func optFloat(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func artSourceKind(meta Metadata) string {
	if len(meta.EmbeddedArt) > 0 {
		return "embedded"
	}
	return "folder"
}
