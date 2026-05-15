package scanner

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/internal/providers"
	"github.com/sarga-app/sarga/internal/tags"
)

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

	dir := pf.dir
	if err := q.UpdateScanRunCursor(ctx, queries.UpdateScanRunCursorParams{
		LastCompletedFolder: &dir,
		ID:                  pr.runID,
	}); err != nil {
		slog.Error("scan: update cursor", "run_id", pr.runID, "err", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	pr.setLiveCurrent(pf.dir)
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

	_, err = s.writeTrackFile(ctx, q, libraryID, trackID, file, localMatch(file.meta), pr)
	return err
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
	slog.Debug("scan: file moved",
		"library_id", libraryID,
		"from", row.RelativePath,
		"to", file.ev.AbsPath,
		"track_file_id", row.ID,
	)
	return row.ID, true, nil
}

func localMatch(meta tags.Metadata) *providers.MatchResult {
	if !meta.HasMBID() {
		return nil
	}
	return &providers.MatchResult{
		Confidence:    1.0,
		Source:        "mbid_tag",
		MBRecordingID: meta.MBRecordingID,
		MBReleaseID:   meta.MBReleaseID,
		MBArtistID:    meta.MBArtistID,
	}
}

func getOrCreateArtist(ctx context.Context, q *queries.Queries, meta tags.Metadata) (string, error) {
	name := meta.AlbumArtist
	if name == "" {
		name = meta.PrimaryArtist()
	}
	if name == "" {
		name = "Unknown Artist"
	}

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
		SortName:         name,
		IsVariousArtists: name == "Various Artists",
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func getOrCreateReleaseGroup(ctx context.Context, q *queries.Queries, meta tags.Metadata) (string, error) {
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

	var yearPtr *int64
	if meta.Year > 0 {
		y := int64(meta.Year)
		yearPtr = &y
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

func getOrCreateRelease(ctx context.Context, q *queries.Queries, meta tags.Metadata, rgID string) (string, error) {
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
	inserted, err := q.InsertRelease(ctx, queries.InsertReleaseParams{
		ID:             uuid.NewString(),
		Mbid:           mbidPtr,
		ReleaseGroupID: rgID,
		Title:          title,
		ReleaseDate:    datePtr,
		TotalDiscs:     int64(max(meta.DiscTotal, 1)),
	})
	if err != nil {
		return "", err
	}
	return inserted.ID, nil
}

func getOrCreateTrack(ctx context.Context, q *queries.Queries, meta tags.Metadata) (string, error) {
	title := meta.Title
	if title == "" {
		title = "Unknown Track"
	}

	if meta.MBRecordingID != "" {
		mbid := meta.MBRecordingID
		row, err := q.GetTrackByMBID(ctx, &mbid)
		if err == nil {
			if meta.DurationMs > 0 {
				if uerr := q.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
					DurationMs: &meta.DurationMs,
					ID:         row.ID,
				}); uerr != nil {
					slog.Error("scan: update track duration", "track_id", row.ID, "err", uerr)
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
