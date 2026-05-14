package api

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/sarga-app/sarga/internal/db/queries"
)

func (a *API) mountCatalogRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(requireAuth(a.jwt))

		r.Get("/albums", a.listAlbums)
		r.Get("/albums/{id}", a.getAlbum)
		r.Get("/artists", a.listArtists)
		r.Get("/artists/{id}", a.getArtist)
		r.Get("/tracks", a.listTracks)
		r.Get("/playlists", a.listPlaylists)
		r.Get("/playlists/{id}", a.getPlaylist)
		r.Get("/search", a.search)
	})
}

func (a *API) listAlbums(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "date"
	}

	ctx := r.Context()
	q := queries.New(a.db.Read())

	var albums []map[string]any
	var total int64
	var err error

	switch sort {
	case "title":
		rows, rerr := q.ListReleasesByTitle(ctx, queries.ListReleasesByTitleParams{
			LibraryID: libraryID, Limit: int64(limit), Offset: offset,
		})
		err = rerr
		if err == nil {
			total, err = q.CountReleases(ctx, libraryID)
			albums = make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				albums = append(albums, releaseRowToMap(row.ID, row.Title, row.ReleaseDate, row.EditionNotes,
					row.LabelName, row.PrimaryType, row.ReleaseGroupID,
					row.PrimaryArtistID, row.PrimaryArtistName, row.IsVariousArtists, row.CoverArtUri))
			}
		}
	case "added":
		rows, rerr := q.ListReleasesByAdded(ctx, queries.ListReleasesByAddedParams{
			LibraryID: libraryID, Limit: int64(limit), Offset: offset,
		})
		err = rerr
		if err == nil {
			total, err = q.CountReleases(ctx, libraryID)
			albums = make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				albums = append(albums, releaseRowToMap(row.ID, row.Title, row.ReleaseDate, row.EditionNotes,
					row.LabelName, row.PrimaryType, row.ReleaseGroupID,
					row.PrimaryArtistID, row.PrimaryArtistName, row.IsVariousArtists, row.CoverArtUri))
			}
		}
	default:
		rows, rerr := q.ListReleasesByDate(ctx, queries.ListReleasesByDateParams{
			LibraryID: libraryID, Limit: int64(limit), Offset: offset,
		})
		err = rerr
		if err == nil {
			total, err = q.CountReleases(ctx, libraryID)
			albums = make([]map[string]any, 0, len(rows))
			for _, row := range rows {
				albums = append(albums, releaseRowToMap(row.ID, row.Title, row.ReleaseDate, row.EditionNotes,
					row.LabelName, row.PrimaryType, row.ReleaseGroupID,
					row.PrimaryArtistID, row.PrimaryArtistName, row.IsVariousArtists, row.CoverArtUri))
			}
		}
	}
	if err != nil {
		catalogDBErr(w, "failed to list albums", err)
		return
	}

	writeJSON(w, http.StatusOK, paginatedResponse(albums, page, limit, int(total)))
}

func (a *API) getAlbum(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	rel, err := q.GetReleaseByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "album not found")
			return
		}
		catalogDBErr(w, "failed to get album", err)
		return
	}

	tracks, err := q.ListReleaseTracks(ctx, queries.ListReleaseTracksParams{
		LibraryID: libraryID, ReleaseID: id,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list tracks", err)
		return
	}
	total, err := q.CountReleaseTracks(ctx, id)
	if err != nil {
		catalogDBErr(w, "failed to count tracks", err)
		return
	}

	trackList := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		trackList = append(trackList, map[string]any{
			"track_id":         t.TrackID,
			"track_file_id":    t.TrackFileID,
			"disc_number":      t.DiscNumber,
			"position":         t.Position,
			"title":            t.Title,
			"mbid":             t.TrackMbid,
			"duration_ms":      t.DurationMs,
			"format":           t.Format,
			"codec":            t.Codec,
			"is_lossless":      t.IsLossless,
			"is_preferred":     t.IsPreferred,
			"match_source":     t.MatchSource,
			"match_confidence": t.MatchConfidence,
		})
	}

	out := map[string]any{
		"id":                 rel.ID,
		"title":              rel.Title,
		"mbid":               rel.Mbid,
		"edition_notes":      rel.EditionNotes,
		"release_date":       rel.ReleaseDate,
		"label_name":         rel.LabelName,
		"catalog_number":     rel.CatalogNumber,
		"total_discs":        rel.TotalDiscs,
		"release_type":       rel.PrimaryType,
		"release_group_id":   rel.ReleaseGroupID,
		"release_group_mbid": rel.ReleaseGroupMbid,
		"cover_art_hash":     rel.CoverArtUri,
		"is_compilation":     false,
		"tracks":             trackList,
		"pagination":         pagMeta(page, limit, int(total)),
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) listArtists(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	rows, err := q.ListArtists(ctx, queries.ListArtistsParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list artists", err)
		return
	}
	total, err := q.CountArtists(ctx, libraryID)
	if err != nil {
		catalogDBErr(w, "failed to count artists", err)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, ar := range rows {
		out = append(out, map[string]any{
			"id":            ar.ID,
			"name":          ar.Name,
			"sort_name":     ar.SortName,
			"mbid":          ar.Mbid,
			"country":       ar.Country,
			"release_count": ar.ReleaseCount,
		})
	}
	writeJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

func (a *API) getArtist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	ar, err := q.GetArtistByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "artist not found")
			return
		}
		catalogDBErr(w, "failed to get artist", err)
		return
	}

	releases, err := q.ListReleasesByArtistByDate(ctx, queries.ListReleasesByArtistByDateParams{
		ArtistID: id, LibraryID: libraryID,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list releases", err)
		return
	}
	total, err := q.CountReleasesByArtist(ctx, queries.CountReleasesByArtistParams{
		ArtistID: id, LibraryID: libraryID,
	})
	if err != nil {
		catalogDBErr(w, "failed to count releases", err)
		return
	}

	rels := make([]map[string]any, 0, len(releases))
	for _, row := range releases {
		rels = append(rels, releaseRowToMap(row.ID, row.Title, row.ReleaseDate, row.EditionNotes,
			row.LabelName, row.PrimaryType, row.ReleaseGroupID,
			row.PrimaryArtistID, row.PrimaryArtistName, row.IsVariousArtists, row.CoverArtUri))
	}

	out := map[string]any{
		"id":         ar.ID,
		"name":       ar.Name,
		"sort_name":  ar.SortName,
		"mbid":       ar.Mbid,
		"country":    ar.Country,
		"bio":        ar.Bio,
		"begin_year": ar.BeginYear,
		"end_year":   ar.EndYear,
		"releases":   rels,
		"pagination": pagMeta(page, limit, int(total)),
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) listTracks(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	albumID := r.URL.Query().Get("album_id")
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	if albumID != "" {
		rows, err := q.ListReleaseTracks(ctx, queries.ListReleaseTracksParams{
			LibraryID: libraryID, ReleaseID: albumID,
			Limit: int64(limit), Offset: offset,
		})
		if err != nil {
			catalogDBErr(w, "failed to list tracks", err)
			return
		}
		total, err := q.CountReleaseTracks(ctx, albumID)
		if err != nil {
			catalogDBErr(w, "failed to count tracks", err)
			return
		}
		out := make([]map[string]any, 0, len(rows))
		for _, t := range rows {
			out = append(out, map[string]any{
				"id": t.TrackID, "title": t.Title, "duration_ms": t.DurationMs,
				"format": t.Format, "codec": t.Codec, "is_lossless": t.IsLossless,
				"match_source": t.MatchSource, "match_confidence": t.MatchConfidence,
			})
		}
		writeJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
		return
	}

	rows, err := q.ListTracks(ctx, queries.ListTracksParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list tracks", err)
		return
	}
	total, err := q.CountTracks(ctx, libraryID)
	if err != nil {
		catalogDBErr(w, "failed to count tracks", err)
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		out = append(out, map[string]any{
			"id": t.ID, "title": t.Title, "duration_ms": t.DurationMs,
			"format": t.Format, "codec": t.Codec, "is_lossless": t.IsLossless,
			"match_source": t.MatchSource,
		})
	}
	writeJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

func (a *API) listPlaylists(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	rows, err := q.ListPlaylists(ctx, queries.ListPlaylistsParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list playlists", err)
		return
	}
	total, err := q.CountPlaylists(ctx, libraryID)
	if err != nil {
		catalogDBErr(w, "failed to count playlists", err)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "description": p.Description,
			"track_count": p.TrackCount, "is_public": p.IsPublic,
		})
	}
	writeJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

func (a *API) getPlaylist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(a.db.Read())

	pl, err := q.GetPlaylistByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "playlist not found")
			return
		}
		catalogDBErr(w, "failed to get playlist", err)
		return
	}

	tracks, err := q.ListPlaylistTracks(ctx, queries.ListPlaylistTracksParams{
		LibraryID: libraryID, PlaylistID: id,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		catalogDBErr(w, "failed to list playlist tracks", err)
		return
	}
	total, err := q.CountPlaylistTracks(ctx, id)
	if err != nil {
		catalogDBErr(w, "failed to count playlist tracks", err)
		return
	}

	tl := make([]map[string]any, 0, len(tracks))
	for _, t := range tracks {
		tl = append(tl, map[string]any{
			"position": t.Position, "track_id": t.TrackID, "title": t.Title,
			"duration_ms": t.DurationMs, "format": t.Format, "codec": t.Codec,
			"is_lossless": t.IsLossless, "is_preferred": t.IsPreferred,
		})
	}
	out := map[string]any{
		"id": pl.ID, "name": pl.Name, "description": pl.Description,
		"track_count": pl.TrackCount, "is_public": pl.IsPublic,
		"tracks":     tl,
		"pagination": pagMeta(page, limit, int(total)),
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "q is required")
		return
	}
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	slog.Debug("search", "q", q, "library_id", libraryID, "page", page, "limit", limit)

	ctx := r.Context()
	dbConn := a.db.Read()

	albumResults, albumTotal, err := searchReleases(ctx, dbConn, q, libraryID, int64(limit), offset)
	if err != nil {
		slog.Error("search: releases", "q", q, "err", err)
	}
	artistResults, artistTotal, err := searchArtists(ctx, dbConn, q, libraryID, int64(limit), offset)
	if err != nil {
		slog.Error("search: artists", "q", q, "err", err)
	}
	trackResults, trackTotal, err := searchTracks(ctx, dbConn, q, libraryID, int64(limit), offset)
	if err != nil {
		slog.Error("search: tracks", "q", q, "err", err)
	}

	slog.Debug("search: results", "albums", albumTotal, "artists", artistTotal, "tracks", trackTotal)

	writeJSON(w, http.StatusOK, map[string]any{
		"query": q,
		"albums": map[string]any{
			"data":       albumResults,
			"total":      albumTotal,
			"pagination": pagMeta(page, limit, albumTotal),
		},
		"artists": map[string]any{
			"data":       artistResults,
			"total":      artistTotal,
			"pagination": pagMeta(page, limit, artistTotal),
		},
		"tracks": map[string]any{
			"data":       trackResults,
			"total":      trackTotal,
			"pagination": pagMeta(page, limit, trackTotal),
		},
	})
}

func releaseRowToMap(
	id, title string,
	releaseDate, editionNotes, labelName *string,
	primaryType string,
	releaseGroupID string,
	primaryArtistID, primaryArtistName string,
	isVariousArtists bool,
	coverArtURI string,
) map[string]any {
	var artistID, artistName any
	if primaryArtistID != "" {
		artistID = primaryArtistID
	}
	if primaryArtistName != "" {
		artistName = primaryArtistName
	}
	return map[string]any{
		"id":                  id,
		"title":               title,
		"edition_notes":       editionNotes,
		"release_date":        releaseDate,
		"label_name":          labelName,
		"release_type":        primaryType,
		"release_group_id":    releaseGroupID,
		"primary_artist_id":   artistID,
		"primary_artist_name": artistName,
		"is_compilation":      isVariousArtists,
		"cover_art_hash":      coverArtURI,
	}
}

func paginatedResponse(data any, page, limit, total int) map[string]any {
	return map[string]any{
		"data":       data,
		"pagination": pagMeta(page, limit, total),
	}
}

func pagMeta(page, limit, total int) map[string]int {
	totalPages := total / limit
	if total%limit != 0 || total == 0 {
		totalPages++
	}
	return map[string]int{
		"page":        page,
		"limit":       limit,
		"total":       total,
		"total_pages": totalPages,
	}
}

func searchReleases(ctx context.Context, rawDB *sql.DB, q, libraryID string, limit, offset int64) ([]map[string]any, int, error) { //nolint:gocritic
	rows, err := rawDB.QueryContext(ctx,
		`SELECT DISTINCT r.id, r.title,
		    (SELECT uri FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_hash
		 FROM releases_fts
		 JOIN releases r ON r.id = releases_fts.release_id
		 WHERE releases_fts MATCH ?
		   AND EXISTS (
		       SELECT 1 FROM release_tracks rt
		       JOIN track_files tf ON tf.track_id = rt.track_id
		       WHERE rt.release_id = r.id AND tf.library_id = ?
		   )
		 LIMIT ? OFFSET ?`,
		q, libraryID, limit, offset,
	)
	if err != nil {
		return []map[string]any{}, 0, err
	}
	defer rows.Close() //nolint:errcheck
	var out []map[string]any
	for rows.Next() {
		var id, title string
		var coverHash *string
		if err := rows.Scan(&id, &title, &coverHash); err == nil {
			out = append(out, map[string]any{"id": id, "title": title, "cover_art_hash": coverHash})
		}
	}
	if err := rows.Err(); err != nil {
		return []map[string]any{}, 0, err
	}
	var total int
	rawDB.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(DISTINCT r.id) FROM releases_fts
		 JOIN releases r ON r.id = releases_fts.release_id
		 WHERE releases_fts MATCH ?
		   AND EXISTS (SELECT 1 FROM release_tracks rt JOIN track_files tf ON tf.track_id = rt.track_id WHERE rt.release_id = r.id AND tf.library_id = ?)`,
		q, libraryID,
	).Scan(&total) //nolint:errcheck
	if out == nil {
		out = []map[string]any{}
	}
	return out, total, nil
}

func searchArtists(ctx context.Context, rawDB *sql.DB, q, libraryID string, limit, offset int64) ([]map[string]any, int, error) { //nolint:gocritic
	rows, err := rawDB.QueryContext(ctx,
		`SELECT DISTINCT a.id, a.name
		 FROM artists_fts
		 JOIN artists a ON a.id = artists_fts.artist_id
		 WHERE artists_fts MATCH ?
		   AND EXISTS (
		       SELECT 1 FROM release_artists ra2
		       JOIN release_tracks rt ON rt.release_id = ra2.release_id
		       JOIN track_files tf ON tf.track_id = rt.track_id
		       WHERE ra2.artist_id = a.id AND tf.library_id = ?
		   )
		 LIMIT ? OFFSET ?`,
		q, libraryID, limit, offset,
	)
	if err != nil {
		return []map[string]any{}, 0, err
	}
	defer rows.Close() //nolint:errcheck
	var out []map[string]any
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err == nil {
			out = append(out, map[string]any{"id": id, "name": name})
		}
	}
	if err := rows.Err(); err != nil {
		return []map[string]any{}, 0, err
	}
	var total int
	rawDB.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(DISTINCT a.id) FROM artists_fts
		 JOIN artists a ON a.id = artists_fts.artist_id
		 WHERE artists_fts MATCH ?
		   AND EXISTS (
		       SELECT 1 FROM release_artists ra2
		       JOIN release_tracks rt ON rt.release_id = ra2.release_id
		       JOIN track_files tf ON tf.track_id = rt.track_id
		       WHERE ra2.artist_id = a.id AND tf.library_id = ?
		   )`,
		q, libraryID,
	).Scan(&total) //nolint:errcheck
	if out == nil {
		out = []map[string]any{}
	}
	return out, total, nil
}

func searchTracks(ctx context.Context, rawDB *sql.DB, q, libraryID string, limit, offset int64) ([]map[string]any, int, error) { //nolint:gocritic
	rows, err := rawDB.QueryContext(ctx,
		`SELECT DISTINCT t.id, t.title, tf.format, tf.is_lossless,
		    a.name AS primary_artist_name, r.title AS release_title
		 FROM tracks_fts
		 JOIN tracks t ON t.id = tracks_fts.track_id
		 JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ? AND tf.is_preferred = 1
		 LEFT JOIN track_artists ta ON ta.track_id = t.id AND ta.role = 'primary' AND ta.position = 1
		 LEFT JOIN artists a ON a.id = ta.artist_id
		 LEFT JOIN release_tracks rt ON rt.track_id = t.id
		 LEFT JOIN releases r ON r.id = rt.release_id
		 WHERE tracks_fts MATCH ?
		 LIMIT ? OFFSET ?`,
		libraryID, q, limit, offset,
	)
	if err != nil {
		return []map[string]any{}, 0, err
	}
	defer rows.Close() //nolint:errcheck
	var out []map[string]any
	for rows.Next() {
		var id, title string
		var format string
		var isLossless bool
		var artistName, releaseTitle *string
		if err := rows.Scan(&id, &title, &format, &isLossless, &artistName, &releaseTitle); err == nil {
			out = append(out, map[string]any{
				"id": id, "title": title, "format": format, "is_lossless": isLossless,
				"primary_artist_name": artistName, "release_title": releaseTitle,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return []map[string]any{}, 0, err
	}
	var total int
	rawDB.QueryRowContext(ctx, //nolint:errcheck
		`SELECT COUNT(DISTINCT t.id) FROM tracks_fts
		 JOIN tracks t ON t.id = tracks_fts.track_id
		 JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ?
		 WHERE tracks_fts MATCH ?`,
		libraryID, q,
	).Scan(&total) //nolint:errcheck
	if out == nil {
		out = []map[string]any{}
	}
	return out, total, nil
}

func catalogDBErr(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, "err", err)
	writeError(w, http.StatusInternalServerError, "db_error", msg)
}
