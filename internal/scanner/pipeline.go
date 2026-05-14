package scanner

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/catalog"
	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/internal/providers"
)

type preparedFolder struct {
	dir         string
	files       []preparedFile
	artHash     string
	artFromFile bool
}

type preparedFile struct {
	ev   FileEvent
	hash []byte
	meta catalog.Metadata
}

func (s *Scanner) processPath(
	ctx context.Context,
	libraryID, root string,
	cfg ScanSettings,
	workerN int,
	pr *progress,
) error {
	walkStart := time.Now()
	folders, removed, err := s.rec.ReconcileFolders(ctx, libraryID, root, cfg)
	if err != nil {
		return err
	}
	slog.Debug("scan: walk",
		"elapsed", time.Since(walkStart).Round(time.Millisecond),
		"folders", len(folders), "removed", len(removed))

	for _, f := range folders {
		for _, ev := range f.Files {
			pr.recordFound(ev.AbsPath)
			if ev.Kind == EventUnchanged {
				pr.recordUnchanged()
			}
		}
	}
	if len(removed) > 0 {
		pr.recordRemoved(int64(len(removed)))
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
				pf := s.prepareFolder(f, pr)
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
	var persisted int
	for pf := range out {
		if err := s.persistFolder(ctx, libraryID, pf, pr); err != nil {
			slog.Error("scan: persist folder", "dir", pf.dir, "err", err)
		}
		persisted++
	}

	slog.Debug("scan: pipeline",
		"folders_with_work", persisted,
		"prepare_wall", persistStart.Sub(pipeStart).Round(time.Millisecond),
		"total_wall", time.Since(pipeStart).Round(time.Millisecond),
	)
	return nil
}

func (s *Scanner) prepareFolder(f FolderEvent, pr *progress) *preparedFolder {
	pf := &preparedFolder{dir: f.Dir}
	for _, ev := range f.Files {
		if ev.Kind != EventAdded && ev.Kind != EventChanged {
			continue
		}
		hash, err := HashFile(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: hash failed", "path", ev.AbsPath, "err", err)
			pr.recordErrored()
			continue
		}
		meta, err := catalog.Extract(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: tag extract failed", "path", ev.AbsPath, "err", err)
			pr.recordErrored()
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
	pr *progress,
) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

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
		if err := s.persistFile(ctx, q, libraryID, releaseID, artistID, file, pr); err != nil {
			slog.Error("scan: persist file", "path", file.ev.AbsPath, "err", err)
			pr.recordErrored()
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

	pr.flushIfDue(ctx, s.wq)
	return nil
}

func (s *Scanner) persistFile(
	ctx context.Context,
	q *queries.Queries,
	libraryID, releaseID, artistID string,
	file preparedFile,
	pr *progress,
) error {
	if file.ev.Kind == EventChanged && file.ev.DBEntry != nil {
		if refreshUnchangedFile(ctx, q, libraryID, file, pr) {
			return nil
		}
	}

	trackID, err := getOrCreateTrack(ctx, q, file.meta)
	if err != nil {
		return err
	}

	q.LinkTrackArtist(ctx, queries.LinkTrackArtistParams{ //nolint:errcheck
		TrackID: trackID, ArtistID: artistID, Position: 1, Role: "primary",
	})
	q.UpsertReleaseTrack(ctx, queries.UpsertReleaseTrackParams{ //nolint:errcheck
		ReleaseID:  releaseID,
		TrackID:    trackID,
		DiscNumber: int64(max(file.meta.DiscNum, 1)),
		Position:   int64(file.meta.TrackNum),
	})

	match := s.matcher.Match(ctx, file.meta)
	trackFileID, err := s.writeTrackFile(ctx, q, libraryID, trackID, file, match, pr)
	if err != nil {
		return err
	}

	attachFileExtras(ctx, q, trackID, trackFileID, file.meta)
	return nil
}

func refreshUnchangedFile(
	ctx context.Context,
	q *queries.Queries,
	libraryID string,
	file preparedFile,
	pr *progress,
) bool {
	existing, err := q.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
		LibraryID:    libraryID,
		RelativePath: file.ev.AbsPath,
	})
	if err != nil {
		return false
	}
	if !bytes.Equal(existing.ContentHash, file.hash) {
		return false
	}
	if err := q.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
		TrackID:       existing.TrackID,
		Format:        existing.Format,
		Codec:         existing.Codec,
		BitrateKbps:   existing.BitrateKbps,
		SampleRateHz:  existing.SampleRateHz,
		BitDepth:      existing.BitDepth,
		Channels:      existing.Channels,
		FileSizeBytes: file.ev.Info.Size(),
		Mtime:         file.ev.Info.ModTime().UTC().Format(time.RFC3339),
		ContentHash:   file.hash,
		IsLossless:    existing.IsLossless,
		RgTrackGain:   existing.RgTrackGain,
		RgTrackPeak:   existing.RgTrackPeak,
		RgAlbumGain:   existing.RgAlbumGain,
		RgAlbumPeak:   existing.RgAlbumPeak,
		ID:            existing.ID,
	}); err != nil {
		slog.Error("scan: refresh unchanged file core", "path", file.ev.AbsPath, "err", err)
	}
	pr.recordUnchanged()
	return true
}

func tryRepathByHash(
	ctx context.Context,
	q *queries.Queries,
	libraryID string,
	file preparedFile,
	mtime string,
) (id string, found bool, err error) {
	row, err := q.FindMissingTrackFileByHash(ctx, queries.FindMissingTrackFileByHashParams{
		LibraryID:   libraryID,
		ContentHash: file.hash,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if rerr := q.RepathTrackFile(ctx, queries.RepathTrackFileParams{
		RelativePath:  file.ev.AbsPath,
		FileSizeBytes: file.ev.Info.Size(),
		Mtime:         mtime,
		ID:            row.ID,
	}); rerr != nil {
		return "", false, rerr
	}
	slog.Info("scan: file moved",
		"library_id", libraryID,
		"from", row.RelativePath,
		"to", file.ev.AbsPath,
		"track_file_id", row.ID,
	)
	return row.ID, true, nil
}

func insertTrackFile(
	ctx context.Context,
	q *queries.Queries,
	libraryID, trackID string,
	file preparedFile,
	match *providers.MatchResult,
	mtime string,
	lossless bool,
	channels int64,
	pr *progress,
) (string, error) {
	if id, ok, err := tryRepathByHash(ctx, q, libraryID, file, mtime); err != nil {
		return "", err
	} else if ok {
		pr.recordMoved()
		return id, nil
	}
	preferred, err := q.HasPreferredTrackFile(ctx, trackID)
	if err != nil {
		return "", err
	}
	matchSource := "raw"
	matchConfidence := 0.0
	if match != nil {
		matchSource = match.Source
		matchConfidence = match.Confidence
	}
	meta := file.meta
	ev := file.ev
	row, err := q.InsertTrackFile(ctx, queries.InsertTrackFileParams{
		ID:              uuid.NewString(),
		TrackID:         trackID,
		LibraryID:       libraryID,
		RelativePath:    ev.AbsPath,
		Format:          meta.Format,
		Codec:           meta.Codec,
		BitrateKbps:     optInt64(meta.Bitrate),
		SampleRateHz:    optInt64(meta.SampleRate),
		BitDepth:        optInt64(meta.BitsPerSample),
		Channels:        channels,
		FileSizeBytes:   ev.Info.Size(),
		Mtime:           mtime,
		ContentHash:     file.hash,
		SourceKind:      "import",
		IsLossless:      lossless,
		IsPreferred:     !preferred,
		MatchConfidence: matchConfidence,
		MatchSource:     matchSource,
		RgTrackGain:     optFloat(meta.RGTrackGain),
		RgTrackPeak:     optFloat(meta.RGTrackPeak),
		RgAlbumGain:     optFloat(meta.RGAlbumGain),
		RgAlbumPeak:     optFloat(meta.RGAlbumPeak),
	})
	if err != nil {
		return "", err
	}
	pr.recordAdded()
	return row.ID, nil
}

func (s *Scanner) writeTrackFile(
	ctx context.Context,
	q *queries.Queries,
	libraryID, trackID string,
	file preparedFile,
	match *providers.MatchResult,
	pr *progress,
) (string, error) {
	meta := file.meta
	ev := file.ev
	mtime := ev.Info.ModTime().UTC().Format(time.RFC3339)
	lossless := meta.Format == "flac" || meta.Format == "alac" || meta.Format == "wav" || meta.Format == "aiff"

	channels := int64(meta.Channels)
	if channels == 0 {
		channels = 2
	}

	if ev.Kind == EventAdded {
		return insertTrackFile(ctx, q, libraryID, trackID, file, match, mtime, lossless, channels, pr)
	}

	existing, err := q.GetTrackFileByPath(ctx, queries.GetTrackFileByPathParams{
		LibraryID:    libraryID,
		RelativePath: ev.AbsPath,
	})
	if err != nil {
		return "", err
	}
	if err := q.UpdateTrackFileCore(ctx, queries.UpdateTrackFileCoreParams{
		TrackID:       trackID,
		Format:        meta.Format,
		Codec:         meta.Codec,
		BitrateKbps:   optInt64(meta.Bitrate),
		SampleRateHz:  optInt64(meta.SampleRate),
		BitDepth:      optInt64(meta.BitsPerSample),
		Channels:      channels,
		FileSizeBytes: ev.Info.Size(),
		Mtime:         mtime,
		ContentHash:   file.hash,
		IsLossless:    lossless,
		RgTrackGain:   optFloat(meta.RGTrackGain),
		RgTrackPeak:   optFloat(meta.RGTrackPeak),
		RgAlbumGain:   optFloat(meta.RGAlbumGain),
		RgAlbumPeak:   optFloat(meta.RGAlbumPeak),
		ID:            existing.ID,
	}); err != nil {
		slog.Error("scan: update track file core", "path", ev.AbsPath, "err", err)
	}
	pr.recordUpdated()
	return existing.ID, nil
}

func attachFileExtras(
	ctx context.Context,
	q *queries.Queries,
	trackID, trackFileID string,
	meta catalog.Metadata,
) {
	_ = trackFileID
	if meta.Lyrics == "" {
		return
	}
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
