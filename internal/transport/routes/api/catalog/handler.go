package catalog

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	domauth "github.com/sarga-app/sarga/internal/domain/auth"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
	"github.com/sarga-app/sarga/internal/transport/middleware"
)

// Handler handles all catalog browse and search endpoints.
type Handler struct {
	db  *db.DB
	jwt domauth.JWTConfig
}

func NewHandler(d *db.DB, jwt domauth.JWTConfig) *Handler {
	return &Handler{db: d, jwt: jwt}
}

// Mount registers all catalog routes under r (all require auth).
func Mount(r chi.Router, h *Handler) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(h.jwt))

		r.Get("/albums", h.listAlbums)
		r.Get("/albums/{id}", h.getAlbum)
		r.Get("/artists", h.listArtists)
		r.Get("/artists/{id}", h.getArtist)
		r.Get("/tracks", h.listTracks)
		r.Get("/playlists", h.listPlaylists)
		r.Get("/playlists/{id}", h.getPlaylist)
		r.Get("/search", h.search)
	})
}

// ── Albums ────────────────────────────────────────────────────────────────

func (h *Handler) listAlbums(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)
	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "date"
	}

	ctx := r.Context()
	q := queries.New(h.db.Read())

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
	default: // date
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
		dbErr(w, "failed to list albums", err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, paginatedResponse(albums, page, limit, int(total)))
}

func (h *Handler) getAlbum(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	rel, err := q.GetReleaseByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "album not found")
			return
		}
		dbErr(w, "failed to get album", err)
		return
	}

	tracks, err := q.ListReleaseTracks(ctx, queries.ListReleaseTracksParams{
		LibraryID: libraryID, ReleaseID: id,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list tracks", err)
		return
	}
	total, err := q.CountReleaseTracks(ctx, id)
	if err != nil {
		dbErr(w, "failed to count tracks", err)
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
		"id":               rel.ID,
		"title":            rel.Title,
		"edition_notes":    rel.EditionNotes,
		"release_date":     rel.ReleaseDate,
		"label_name":       rel.LabelName,
		"catalog_number":   rel.CatalogNumber,
		"total_discs":      rel.TotalDiscs,
		"release_type":     rel.PrimaryType,
		"release_group_id": rel.ReleaseGroupID,
		"cover_art_hash":   rel.CoverArtUri,
		"is_compilation":   false,
		"tracks":           trackList,
		"pagination":       pagMeta(page, limit, int(total)),
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ── Artists ───────────────────────────────────────────────────────────────

func (h *Handler) listArtists(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	rows, err := q.ListArtists(ctx, queries.ListArtistsParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list artists", err)
		return
	}
	total, err := q.CountArtists(ctx, libraryID)
	if err != nil {
		dbErr(w, "failed to count artists", err)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, a := range rows {
		out = append(out, map[string]any{
			"id":            a.ID,
			"name":          a.Name,
			"sort_name":     a.SortName,
			"mbid":          a.Mbid,
			"country":       a.Country,
			"release_count": a.ReleaseCount,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

func (h *Handler) getArtist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	a, err := q.GetArtistByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "artist not found")
			return
		}
		dbErr(w, "failed to get artist", err)
		return
	}

	releases, err := q.ListReleasesByArtistByDate(ctx, queries.ListReleasesByArtistByDateParams{
		ArtistID: id, LibraryID: libraryID,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list releases", err)
		return
	}
	total, err := q.CountReleasesByArtist(ctx, queries.CountReleasesByArtistParams{
		ArtistID: id, LibraryID: libraryID,
	})
	if err != nil {
		dbErr(w, "failed to count releases", err)
		return
	}

	rels := make([]map[string]any, 0, len(releases))
	for _, row := range releases {
		rels = append(rels, releaseRowToMap(row.ID, row.Title, row.ReleaseDate, row.EditionNotes,
			row.LabelName, row.PrimaryType, row.ReleaseGroupID,
			row.PrimaryArtistID, row.PrimaryArtistName, row.IsVariousArtists, row.CoverArtUri))
	}

	out := map[string]any{
		"id":         a.ID,
		"name":       a.Name,
		"sort_name":  a.SortName,
		"mbid":       a.Mbid,
		"country":    a.Country,
		"bio":        a.Bio,
		"begin_year": a.BeginYear,
		"end_year":   a.EndYear,
		"releases":   rels,
		"pagination": pagMeta(page, limit, int(total)),
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ── Tracks ────────────────────────────────────────────────────────────────

func (h *Handler) listTracks(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	albumID := r.URL.Query().Get("album_id")
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	if albumID != "" {
		rows, err := q.ListReleaseTracks(ctx, queries.ListReleaseTracksParams{
			LibraryID: libraryID, ReleaseID: albumID,
			Limit: int64(limit), Offset: offset,
		})
		if err != nil {
			dbErr(w, "failed to list tracks", err)
			return
		}
		total, err := q.CountReleaseTracks(ctx, albumID)
		if err != nil {
			dbErr(w, "failed to count tracks", err)
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
		httpx.WriteJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
		return
	}

	rows, err := q.ListTracks(ctx, queries.ListTracksParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list tracks", err)
		return
	}
	total, err := q.CountTracks(ctx, libraryID)
	if err != nil {
		dbErr(w, "failed to count tracks", err)
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
	httpx.WriteJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

// ── Playlists ─────────────────────────────────────────────────────────────

func (h *Handler) listPlaylists(w http.ResponseWriter, r *http.Request) {
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	rows, err := q.ListPlaylists(ctx, queries.ListPlaylistsParams{
		LibraryID: libraryID, Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list playlists", err)
		return
	}
	total, err := q.CountPlaylists(ctx, libraryID)
	if err != nil {
		dbErr(w, "failed to count playlists", err)
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, p := range rows {
		out = append(out, map[string]any{
			"id": p.ID, "name": p.Name, "description": p.Description,
			"track_count": p.TrackCount, "is_public": p.IsPublic,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, paginatedResponse(out, page, limit, int(total)))
}

func (h *Handler) getPlaylist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	ctx := r.Context()
	q := queries.New(h.db.Read())

	pl, err := q.GetPlaylistByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "playlist not found")
			return
		}
		dbErr(w, "failed to get playlist", err)
		return
	}

	tracks, err := q.ListPlaylistTracks(ctx, queries.ListPlaylistTracksParams{
		LibraryID: libraryID, PlaylistID: id,
		Limit: int64(limit), Offset: offset,
	})
	if err != nil {
		dbErr(w, "failed to list playlist tracks", err)
		return
	}
	total, err := q.CountPlaylistTracks(ctx, id)
	if err != nil {
		dbErr(w, "failed to count playlist tracks", err)
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
	httpx.WriteJSON(w, http.StatusOK, out)
}

// ── Search ────────────────────────────────────────────────────────────────

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "q is required")
		return
	}
	libraryID := r.URL.Query().Get("library_id")
	if libraryID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_param", "library_id is required")
		return
	}
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	slog.Debug("search", "q", q, "library_id", libraryID, "page", page, "limit", limit)

	ctx := r.Context()
	dbConn := h.db.Read()

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

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
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

// ── Helpers ───────────────────────────────────────────────────────────────

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

func parsePage(r *http.Request) (page, limit int) {
	page, limit = 1, 50
	if v := r.URL.Query().Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			page = n
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 200 {
			limit = n
		}
	}
	return page, limit
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

func dbErr(w http.ResponseWriter, msg string, err error) {
	slog.Error(msg, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, "db_error", msg)
}
