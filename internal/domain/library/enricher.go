package library

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/support/db/queries"
)

const (
	enrichBatchSize = 10
	enrichPollDelay = 5 * time.Second
	enrichErrDelay  = 30 * time.Second
)

type Enricher struct {
	store   Store
	art     *CoverArtManager
	matcher *Matcher

	artistMeta  []ArtistMetaProvider
	albumMeta   []AlbumMetaProvider
	artProvider []ArtworkProvider
	lyrics      LyricsProvider
}

func NewEnricher(
	store Store,
	art *CoverArtManager,
	matcher *Matcher,
	artistMeta []ArtistMetaProvider,
	albumMeta []AlbumMetaProvider,
	artProvider []ArtworkProvider,
	lyrics LyricsProvider,
) *Enricher {
	return &Enricher{
		store:       store,
		art:         art,
		matcher:     matcher,
		artistMeta:  artistMeta,
		albumMeta:   albumMeta,
		artProvider: artProvider,
		lyrics:      lyrics,
	}
}

func (e *Enricher) Run(ctx context.Context) {
	if err := e.store.ResetStuckEnrichments(ctx); err != nil {
		slog.Error("enricher: reset stuck enrichments", "err", err)
	}

	ticker := time.NewTicker(enrichPollDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.processBatch(ctx); err != nil {
				slog.Error("enricher: batch error", "err", err)
				ticker.Reset(enrichErrDelay)
			} else {
				ticker.Reset(enrichPollDelay)
			}
		}
	}
}

func (e *Enricher) processBatch(ctx context.Context) error {
	batch, err := e.store.ClaimEnrichmentBatch(ctx, enrichBatchSize)
	if err != nil {
		return err
	}
	if len(batch) > 0 {
		slog.Debug("enricher: processing batch", "count", len(batch))
	}
	for _, item := range batch {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		e.enrichOne(ctx, item)
	}
	return nil
}

func (e *Enricher) enrichOne(ctx context.Context, item queries.EnrichmentQueue) {
	slog.Debug("enricher: processing track", "track_file_id", item.TrackFileID, "attempts", item.Attempts)
	info, err := e.store.GetEnrichmentTrackInfo(ctx, item.TrackFileID)
	if err != nil {
		e.fail(ctx, item.ID, fmt.Errorf("fetch track info: %w", err))
		return
	}

	// Build a synthetic Metadata from what's stored in the DB.
	meta := Metadata{
		Title:         info.TrackTitle,
		MBRecordingID: ptrStr(info.TrackMbid),
		MBReleaseID:   ptrStr(info.ReleaseMbid),
		MBArtistID:    ptrStr(info.ArtistMbid),
		ISRC:          ptrStr(info.Isrc),
	}
	if info.ArtistName != nil {
		meta.Artists = []string{*info.ArtistName}
	}
	if info.ReleaseTitle != nil {
		meta.Album = *info.ReleaseTitle
	}

	alreadyIdentified := info.MatchSource == "mbid_tag" || info.MatchSource == "mb_fuzzy" ||
		info.MatchSource == "discogs_tag" || info.MatchSource == "discogs_fuzzy" ||
		info.MatchSource == "acoustid"
	if !alreadyIdentified { //nolint:nestif
		result, merr := e.matcher.Match(ctx, meta)
		if merr != nil {
			slog.Warn("enricher: match failed", "track_file_id", item.TrackFileID, "err", merr)
		}
		if result != nil {
			slog.Info("enricher: matched",
				"track_file_id", item.TrackFileID,
				"title", info.TrackTitle,
				"source", result.Source,
				"confidence", result.Confidence,
			)
			if err := e.store.UpdateTrackFileMatch(ctx, queries.UpdateTrackFileMatchParams{
				MatchConfidence: result.Confidence,
				MatchSource:     result.Source,
				ID:              item.TrackFileID,
			}); err != nil {
				slog.Error("enricher: update match", "err", err)
			}
			if result.MBRecordingID != "" {
				meta.MBRecordingID = result.MBRecordingID
			}
			if result.MBReleaseID != "" {
				meta.MBReleaseID = result.MBReleaseID
			}
			if result.MBArtistID != "" {
				meta.MBArtistID = result.MBArtistID
			}
		}
	}

	if info.ArtistID != nil {
		e.enrichArtist(ctx, *info.ArtistID, meta.MBArtistID, ptrStr(info.ArtistName))
	}
	if info.ReleaseID != nil {
		e.enrichRelease(ctx, *info.ReleaseID, meta.MBReleaseID, ptrStr(info.ArtistName), ptrStr(info.ReleaseTitle))
	}
	releaseGroupMBID := ptrStr(info.ReleaseGroupMbid)
	if info.ReleaseGroupID != nil {
		e.enrichArt(ctx, info.ArtistID, *info.ReleaseGroupID, meta.MBArtistID, releaseGroupMBID)
	}
	e.enrichLyrics(ctx, info.TrackID, ptrStr(info.ArtistName), info.TrackTitle, ptrStr(info.ReleaseTitle), info.DurationMs)

	if err := e.store.MarkEnrichmentDone(ctx, item.ID); err != nil {
		slog.Error("enricher: mark done", "id", item.ID, "err", err)
	}
	slog.Debug("enricher: done", "track_file_id", item.TrackFileID, "title", info.TrackTitle)
}

func (e *Enricher) enrichArtist(ctx context.Context, artistID, mbid, name string) {
	for _, p := range e.artistMeta {
		a, err := p.FetchArtist(ctx, mbid, name)
		if err != nil {
			slog.Debug("enricher: artist meta", "provider", p.Name(), "err", err)
			continue
		}
		if a == nil {
			continue
		}
		params := queries.UpdateArtistEnrichmentParams{ID: artistID}
		if a.Country != "" {
			params.Country = &a.Country
		}
		if a.Bio != "" {
			params.Bio = &a.Bio
		}
		if a.BeginYear > 0 {
			v := int64(a.BeginYear)
			params.BeginYear = &v
		}
		if a.EndYear > 0 {
			v := int64(a.EndYear)
			params.EndYear = &v
		}
		if a.DiscogsID != "" {
			params.DiscogsID = &a.DiscogsID
		}
		if a.TADBID != "" {
			params.TadbID = &a.TADBID
		}
		if err := e.store.UpdateArtistEnrichment(ctx, params); err != nil {
			slog.Error("enricher: update artist enrichment", "err", err)
		} else {
			slog.Debug("enricher: artist enriched", "artist_id", artistID, "provider", p.Name())
		}
		return // first provider wins
	}
}

func (e *Enricher) enrichRelease(ctx context.Context, releaseID, mbid, artist, title string) {
	for _, p := range e.albumMeta {
		a, err := p.FetchAlbum(ctx, mbid, artist, title)
		if err != nil {
			slog.Debug("enricher: album meta", "provider", p.Name(), "err", err)
			continue
		}
		if a == nil {
			continue
		}
		if err := e.store.UpdateReleaseEnrichment(ctx, queries.UpdateReleaseEnrichmentParams{
			DiscogsID: nilStr(a.DiscogsID),
			ID:        releaseID,
		}); err != nil {
			slog.Error("enricher: update release enrichment", "err", err)
		} else {
			slog.Debug("enricher: release enriched", "release_id", releaseID, "provider", p.Name())
		}
		return
	}
}

func (e *Enricher) enrichArt(ctx context.Context, artistID *string, releaseGroupID, artistMBID, releaseGroupMBID string) {
	// Album art: try each ArtworkProvider; cache the first result.
	for _, p := range e.artProvider {
		results, err := p.FetchAlbumArt(ctx, releaseGroupMBID)
		if err != nil {
			slog.Debug("enricher: album art", "provider", p.Name(), "err", err)
			continue
		}
		for _, r := range results {
			hash, cerr := e.art.ProcessResult(ctx, r)
			if cerr != nil || hash == "" {
				continue
			}
			if _, err := e.store.UpsertCoverArt(ctx, queries.UpsertCoverArtParams{
				ID:             uuid.NewString(),
				ReleaseGroupID: &releaseGroupID,
				SourceKind:     r.Source,
				Uri:            hash,
				Width:          nilInt64(int64(r.Width)),
				Height:         nilInt64(int64(r.Height)),
				IsPreferred:    true,
			}); err != nil {
				slog.Error("enricher: upsert album art", "release_group_id", releaseGroupID, "err", err)
			}
			break
		}
		if results != nil {
			break
		}
	}

	// Artist art: try each ArtworkProvider.
	if artistID != nil {
		for _, p := range e.artProvider {
			results, err := p.FetchArtistArt(ctx, artistMBID)
			if err != nil {
				slog.Debug("enricher: artist art", "provider", p.Name(), "err", err)
				continue
			}
			for _, r := range results {
				hash, cerr := e.art.ProcessResult(ctx, r)
				if cerr != nil || hash == "" {
					continue
				}
				if _, err := e.store.UpsertCoverArt(ctx, queries.UpsertCoverArtParams{
					ID:          uuid.NewString(),
					ArtistID:    artistID,
					SourceKind:  r.Source,
					Uri:         hash,
					Width:       nilInt64(int64(r.Width)),
					Height:      nilInt64(int64(r.Height)),
					IsPreferred: true,
				}); err != nil {
					slog.Error("enricher: upsert artist art", "err", err)
				}
				break
			}
			if results != nil {
				break
			}
		}
	}
}

func (e *Enricher) enrichLyrics(ctx context.Context, trackID, artist, title, album string, durationMs *int64) {
	if e.lyrics == nil {
		return
	}
	dur := 0
	if durationMs != nil {
		dur = int(*durationMs / 1000)
	}
	result, err := e.lyrics.FetchLyrics(ctx, artist, title, album, dur)
	if err != nil || result == nil {
		return
	}
	source := result.Source
	if source == "" {
		source = "lrclib"
	}
	if err := e.store.UpsertLyrics(ctx, queries.UpsertLyricsParams{
		ID:       uuid.NewString(),
		TrackID:  trackID,
		Source:   source,
		IsSynced: result.IsSynced,
		Content:  result.Content,
	}); err != nil {
		slog.Error("enricher: upsert lyrics", "track_id", trackID, "err", err)
	}
}

func (e *Enricher) fail(ctx context.Context, queueID string, err error) {
	msg := err.Error()
	if err := e.store.MarkEnrichmentFailed(ctx, queries.MarkEnrichmentFailedParams{
		LastError: &msg,
		ID:        queueID,
	}); err != nil {
		slog.Error("enricher: mark failed", "queue_id", queueID, "err", err)
	}
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nilInt64(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}
