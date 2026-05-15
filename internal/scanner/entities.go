package scanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/internal/tags"
)

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

func lookupTrackByMBID(ctx context.Context, q *queries.Queries, meta tags.Metadata) (id string, found bool, err error) {
	if meta.MBRecordingID == "" {
		return "", false, nil
	}
	mbid := meta.MBRecordingID
	row, err := q.GetTrackByMBID(ctx, &mbid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if meta.DurationMs > 0 {
		if uerr := q.UpdateTrackDurationIfNull(ctx, queries.UpdateTrackDurationIfNullParams{
			DurationMs: &meta.DurationMs,
			ID:         row.ID,
		}); uerr != nil {
			slog.Error("scan: update track duration", "track_id", row.ID, "err", uerr)
		}
	}
	return row.ID, true, nil
}

func getOrCreateTrack(ctx context.Context, q *queries.Queries, meta tags.Metadata) (string, error) {
	title := meta.Title
	if title == "" {
		title = "Unknown Track"
	}

	if id, ok, err := lookupTrackByMBID(ctx, q, meta); err != nil {
		return "", err
	} else if ok {
		return id, nil
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
