package scanner

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/catalog"
	"github.com/sarga-app/sarga/internal/coverart"
	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
)

var ErrScanAlreadyRunning = errors.New("scanner: scan already running for this library")

const progressFlushEvery = 500

type Scanner struct {
	rq  *queries.Queries
	wq  *queries.Queries
	art *coverart.Manager
	rec *Reconciler
	db  *sql.DB
}

func New(d *db.DB, art *coverart.Manager) *Scanner {
	rq := queries.New(d.Read())
	wq := queries.New(d.Write())
	return &Scanner{
		rq:  rq,
		wq:  wq,
		art: art,
		rec: NewReconciler(rq),
		db:  d.Write(),
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

	active, err := s.rq.GetActiveScanRun(ctx, libraryID)
	if err == nil && active.ID != "" {
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
	if err := s.wq.FinishScanRun(ctx, queries.FinishScanRunParams{
		Status: status,
		Error:  errMsg,
		ID:     run.ID,
	}); err != nil {
		slog.Error("scan: finish run", "run_id", run.ID, "err", err)
	}
	return scanErr
}

func (s *Scanner) backfillDurations(ctx context.Context, libraryID string) {
	rows, err := s.rq.ListTrackFilesNullDuration(ctx, libraryID)
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
		meta, err := catalog.Extract(row.RelativePath)
		if err != nil || meta.DurationMs <= 0 {
			slog.Warn("scan: duration unreadable", "path", row.RelativePath)
			continue
		}
		dur := meta.DurationMs
		if err := s.wq.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
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

	st.mu.Lock()
	fp := make([]string, len(st.foundPaths))
	copy(fp, st.foundPaths)
	removedDelta := st.removed - st.flushed.removed
	st.mu.Unlock()

	if err := markMissingTrackFiles(ctx, s.db, libraryID, fp); err != nil {
		slog.Error("scan: mark missing tracks", "library_id", libraryID, "err", err)
	}

	st.mu.Lock()
	total := st.total
	if err := s.wq.UpdateScanRunProgress(ctx, queries.UpdateScanRunProgressParams{
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

// processPath walks one library root, groups files by folder, and runs the
// pipeline: parallel hash+tag-extract per folder → serial per-folder tx.
func (s *Scanner) processPath(
	ctx context.Context,
	libraryID, root string,
	cfg ScanSettings,
	workerN int,
	st *scanProgress,
) error {
	walkStart := time.Now()
	folders, removed, err := s.rec.ReconcileFolders(ctx, libraryID, root, cfg)
	if err != nil {
		return err
	}
	slog.Debug("scan: walk", "elapsed", time.Since(walkStart),
		"folders", len(folders), "removed", len(removed))

	// Account for stat-clean files and removed files now; the pipeline below
	// only touches folders with actual work.
	for _, f := range folders {
		for _, ev := range f.Files {
			st.mu.Lock()
			st.foundPaths = append(st.foundPaths, ev.AbsPath)
			if ev.Kind == EventUnchanged {
				st.total++
				st.unchanged++
			}
			st.mu.Unlock()
		}
	}
	if len(removed) > 0 {
		st.mu.Lock()
		st.removed += int64(len(removed))
		st.mu.Unlock()
	}

	pipeStart := time.Now()
	in := make(chan FolderEvent, workerN)
	out := make(chan *preparedFolder, workerN)

	go func() {
		defer close(in)
		for _, f := range folders {
			if !f.HasWork() {
				continue
			}
			select {
			case in <- f:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for range workerN {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range in {
				pf := s.prepareFolder(f, st)
				if pf == nil {
					continue
				}
				select {
				case out <- pf:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(out) }()

	persistStart := time.Now()
	var persistedFolders int
	for pf := range out {
		if err := s.persistFolder(ctx, libraryID, pf, st); err != nil {
			slog.Error("scan: persist folder", "dir", pf.dir, "err", err)
		}
		persistedFolders++
	}

	slog.Debug("scan: pipeline",
		"folders_with_work", persistedFolders,
		"prepare_wall", persistStart.Sub(pipeStart).Round(time.Millisecond),
		"total_wall", time.Since(pipeStart).Round(time.Millisecond),
	)
	return nil
}

// preparedFolder is the output of phase 1a (parallel hash + tag extract +
// cover-art resolve). Phase 1b (persistFolder) consumes it inside one tx.
type preparedFolder struct {
	dir         string
	files       []preparedFile
	artHash     string
	artFromFile bool // true = embedded, false = folder.jpg
}

type preparedFile struct {
	ev   FileEvent
	hash []byte
	meta catalog.Metadata
}

func (s *Scanner) prepareFolder(f FolderEvent, st *scanProgress) *preparedFolder {
	pf := &preparedFolder{dir: f.Dir}
	for _, ev := range f.Files {
		if ev.Kind != EventAdded && ev.Kind != EventChanged {
			continue
		}
		hash, err := HashFile(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: hash failed", "path", ev.AbsPath, "err", err)
			st.recordErr()
			continue
		}
		meta, err := catalog.Extract(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: tag extract failed", "path", ev.AbsPath, "err", err)
			st.recordErr()
			continue
		}
		slog.Debug("scan: file",
			"path", ev.AbsPath, "title", meta.Title,
			"duration_ms", meta.DurationMs, "format", meta.Format)
		pf.files = append(pf.files, preparedFile{ev: ev, hash: hash, meta: meta})
	}
	if len(pf.files) == 0 {
		return nil
	}

	for _, pfile := range pf.files {
		if len(pfile.meta.EmbeddedArt) > 0 {
			if h, err := s.art.ProcessEmbedded(pfile.meta.EmbeddedArt); err == nil && h != "" {
				pf.artHash = h
				pf.artFromFile = true
				break
			}
		}
	}
	if pf.artHash == "" {
		if h, err := s.art.FindFolderArt(pf.dir); err == nil && h != "" {
			pf.artHash = h
		}
	}
	return pf
}

func (s *Scanner) persistFolder(
	ctx context.Context,
	libraryID string,
	pf *preparedFolder,
	st *scanProgress,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // commit replaces this when successful

	q := queries.New(tx)
	lead := pf.files[0].meta

	artistID, err := getOrCreateArtist(ctx, q, lead)
	if err != nil {
		return fmt.Errorf("artist: %w", err)
	}
	rgID, err := getOrCreateReleaseGroup(ctx, q, lead)
	if err != nil {
		return fmt.Errorf("release_group: %w", err)
	}
	releaseID, err := getOrCreateRelease(ctx, q, lead, rgID)
	if err != nil {
		return fmt.Errorf("release: %w", err)
	}

	q.LinkReleaseArtist(ctx, queries.LinkReleaseArtistParams{ //nolint:errcheck
		ReleaseID: releaseID, ArtistID: artistID, Position: 1, Role: "primary",
	})

	for _, file := range pf.files {
		if err := s.persistFile(ctx, q, libraryID, releaseID, artistID, file, st); err != nil {
			slog.Error("scan: persist file", "path", file.ev.AbsPath, "err", err)
			st.mu.Lock()
			st.errored++
			st.mu.Unlock()
		}
	}

	if pf.artHash != "" {
		kind := "folder"
		if pf.artFromFile {
			kind = "embedded"
		}
		if _, err := q.UpsertCoverArt(ctx, queries.UpsertCoverArtParams{
			ID:          uuid.NewString(),
			ReleaseID:   &releaseID,
			SourceKind:  kind,
			Uri:         pf.artHash,
			IsPreferred: true,
		}); err != nil {
			slog.Error("scan: upsert cover art", "release_id", releaseID, "err", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	st.mu.Lock()
	st.maybeFlush(ctx, s.wq)
	st.mu.Unlock()
	return nil
}

func (s *Scanner) persistFile(
	ctx context.Context,
	q *queries.Queries,
	libraryID, releaseID, artistID string,
	file preparedFile,
	st *scanProgress,
) error {
	ev := file.ev
	meta := file.meta
	hash := file.hash

	if ev.Kind == EventChanged && ev.DBEntry != nil { //nolint:nestif
		existing, err := q.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
			LibraryID:    libraryID,
			RelativePath: ev.AbsPath,
		})
		if err == nil && bytes.Equal(existing.ContentHash, hash) {
			if err2 := q.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
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
				slog.Error("scan: refresh unchanged file core", "path", ev.AbsPath, "err", err2)
			}
			st.mu.Lock()
			st.unchanged++
			st.mu.Unlock()
			return nil
		}
	}

	trackID, err := getOrCreateTrack(ctx, q, meta)
	if err != nil {
		return err
	}

	q.LinkTrackArtist(ctx, queries.LinkTrackArtistParams{ //nolint:errcheck
		TrackID: trackID, ArtistID: artistID, Position: 1, Role: "primary",
	})
	q.UpsertReleaseTrack(ctx, queries.UpsertReleaseTrackParams{ //nolint:errcheck
		ReleaseID:  releaseID,
		TrackID:    trackID,
		DiscNumber: int64(max(meta.DiscNum, 1)),
		Position:   int64(meta.TrackNum),
	})

	mtime := ev.Info.ModTime().UTC().Format(time.RFC3339)
	lossless := meta.Format == "flac" || meta.Format == "alac" || meta.Format == "wav" || meta.Format == "aiff"
	rgGain, rgPeak, raGain, raPeak := optFloat(meta.RGTrackGain), optFloat(meta.RGTrackPeak), optFloat(meta.RGAlbumGain), optFloat(meta.RGAlbumPeak)
	bitrate, sampleRate, bitDepth := optInt64(meta.Bitrate), optInt64(meta.SampleRate), optInt64(meta.BitsPerSample)
	channels := int64(meta.Channels)
	if channels == 0 {
		channels = 2
	}

	var trackFileID string
	if ev.Kind == EventAdded {
		row, err := q.InsertTrackFile(ctx, queries.InsertTrackFileParams{
			ID:              uuid.NewString(),
			TrackID:         trackID,
			LibraryID:       libraryID,
			RelativePath:    ev.AbsPath,
			Format:          meta.Format,
			Codec:           meta.Codec,
			BitrateKbps:     bitrate,
			SampleRateHz:    sampleRate,
			BitDepth:        bitDepth,
			Channels:        channels,
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
		st.mu.Lock()
		st.added++
		st.mu.Unlock()
	} else {
		existing, err := q.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
			LibraryID:    libraryID,
			RelativePath: ev.AbsPath,
		})
		if err != nil {
			return err
		}
		trackFileID = existing.ID
		if err2 := q.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
			TrackID:       trackID,
			Format:        meta.Format,
			Codec:         meta.Codec,
			BitrateKbps:   bitrate,
			SampleRateHz:  sampleRate,
			BitDepth:      bitDepth,
			Channels:      channels,
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
		st.mu.Lock()
		st.updated++
		st.mu.Unlock()
	}

	if meta.Lyrics != "" {
		if err := q.UpsertLyrics(ctx, queries.UpsertLyricsParams{
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
	if err := q.EnqueueTrackFile(ctx, queries.EnqueueTrackFileParams{
		ID:          uuid.NewString(),
		TrackFileID: trackFileID,
		LibraryID:   libraryID,
		Priority:    priority,
	}); err != nil {
		slog.Error("scan: enqueue track file", "track_file_id", trackFileID, "err", err)
	}
	return nil
}

func getOrCreateArtist(ctx context.Context, q *queries.Queries, meta catalog.Metadata) (string, error) {
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
		row, err := q.GetArtistByMBID(ctx, &mbid)
		if err == nil {
			return row.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	row, err := q.GetArtistByName(ctx, name)
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
	inserted, err := q.InsertArtist(ctx, queries.InsertArtistParams{
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

func getOrCreateReleaseGroup(ctx context.Context, q *queries.Queries, meta catalog.Metadata) (string, error) {
	title := meta.Album
	if title == "" {
		title = "Unknown Album"
	}

	if meta.MBReleaseID != "" {
		mbid := meta.MBReleaseID
		rg, err := q.GetReleaseGroupByMBID(ctx, &mbid)
		if err == nil {
			return rg.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	rg, err := q.GetReleaseGroupByTitle(ctx, title)
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
	inserted, err := q.InsertReleaseGroup(ctx, queries.InsertReleaseGroupParams{
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

func getOrCreateRelease(ctx context.Context, q *queries.Queries, meta catalog.Metadata, rgID string) (string, error) {
	title := meta.Album
	if title == "" {
		title = "Unknown Album"
	}

	if meta.MBReleaseID != "" {
		mbid := meta.MBReleaseID
		rel, err := q.GetReleaseByMBID(ctx, &mbid)
		if err == nil {
			return rel.ID, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
	}

	rel, err := q.GetReleaseByGroupAndTitle(ctx, queries.GetReleaseByGroupAndTitleParams{
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
	inserted, err := q.InsertRelease(ctx, queries.InsertReleaseParams{
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

func getOrCreateTrack(ctx context.Context, q *queries.Queries, meta catalog.Metadata) (string, error) {
	title := meta.Title
	if title == "" {
		title = "Unknown Track"
	}

	if meta.MBRecordingID != "" { //nolint:nestif
		mbid := meta.MBRecordingID
		row, err := q.GetTrackByMBID(ctx, &mbid)
		if err == nil {
			if meta.DurationMs > 0 {
				if err2 := q.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
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
	inserted, err := q.InsertTrack(ctx, queries.InsertTrackParams{
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

// maybeFlush assumes st.mu is held by the caller.
func (st *scanProgress) maybeFlush(ctx context.Context, wq *queries.Queries) {
	st.total++
	if st.total-st.flushed.total < progressFlushEvery {
		return
	}
	if err := wq.UpdateScanRunProgress(ctx, queries.UpdateScanRunProgressParams{
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

func optInt64(n int) *int64 {
	if n <= 0 {
		return nil
	}
	v := int64(n)
	return &v
}

func markMissingTrackFiles(ctx context.Context, sqlDB *sql.DB, libraryID string, foundPaths []string) error {
	if len(foundPaths) == 0 {
		_, err := sqlDB.ExecContext(ctx,
			`UPDATE track_files SET missing_since = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
             WHERE library_id = ? AND missing_since IS NULL`, libraryID)
		return err
	}
	placeholders := strings.Repeat("?,", len(foundPaths))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf( //nolint:gosec // placeholders is "?,?,?" — no user data in the format verb
		`UPDATE track_files SET missing_since = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
         WHERE library_id = ? AND relative_path NOT IN (%s) AND missing_since IS NULL`,
		placeholders,
	)
	args := make([]any, 0, len(foundPaths)+1)
	args = append(args, libraryID)
	for _, p := range foundPaths {
		args = append(args, p)
	}
	_, err := sqlDB.ExecContext(ctx, query, args...)
	return err
}
