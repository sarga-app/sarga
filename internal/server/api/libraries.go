package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/auth"
	"github.com/sarga-app/sarga/internal/db/queries"
	"github.com/sarga-app/sarga/internal/scanner"
	"github.com/sarga-app/sarga/openapi"
)

const libBodyLimit = 64 * 1024

func (a *API) mountLibrariesRoutes(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(requireAuth(a.jwt))
		r.Use(bodyLimit(libBodyLimit))

		r.Get("/libraries", a.listLibraries)
		r.Get("/libraries/{libraryId}", withLibraryID(a.getLibrary))

		r.Post("/libraries/{libraryId}/scan", withLibraryID(a.triggerScan))
		r.Post("/libraries/{libraryId}/scan/cancel", withLibraryID(a.cancelScan))
		r.Get("/libraries/{libraryId}/scan-runs", withLibraryID(a.listScanRuns))
		r.Get("/libraries/{libraryId}/scan-runs/{runId}", withLibraryAndRunID(a.getScanRun))
		r.Get("/libraries/{libraryId}/settings", withLibraryID(a.getLibrarySettings))

		r.Group(func(r chi.Router) {
			r.Use(requireAdmin)
			r.Post("/libraries", a.createLibrary)
			r.Put("/libraries/{libraryId}", withLibraryID(a.updateLibrary))
			r.Delete("/libraries/{libraryId}", withLibraryID(a.deleteLibrary))
			r.Get("/libraries/{libraryId}/access", withLibraryID(a.listLibraryMembers))
			r.Post("/libraries/{libraryId}/access", withLibraryID(a.grantLibraryAccess))
			r.Delete("/libraries/{libraryId}/access/{userId}", withLibraryAndUserID(a.revokeLibraryAccess))
			r.Put("/libraries/{libraryId}/settings", withLibraryID(a.putLibrarySettings))
		})
	})
}

func (a *API) listLibraries(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := queries.New(a.db.Read())

	var libs []queries.Library
	var err error
	if auth.IsAdminFrom(ctx) {
		libs, err = q.ListLibraries(ctx)
	} else {
		uid, _ := auth.UserIDFrom(ctx)
		libs, err = q.ListLibrariesForUser(ctx, uid.String())
	}
	if err != nil {
		slog.Error("list libraries", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	pathsByLib, err := pathsForLibraries(ctx, q)
	if err != nil {
		slog.Error("list libraries: load paths", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	out := make([]openapi.Library, 0, len(libs))
	for _, l := range libs {
		out = append(out, toGenLibrary(l, pathsByLib[l.ID]))
	}
	writeJSON(w, http.StatusOK, map[string]any{"libraries": out})
}

func (a *API) createLibrary(w http.ResponseWriter, r *http.Request) {
	var req openapi.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	cleanPaths, err := validateLibraryWrite(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := r.Context()
	libID := uuid.NewString()

	lib, err := a.libTx(ctx, func(q *queries.Queries) (queries.Library, error) {
		l, e := q.CreateLibrary(ctx, queries.CreateLibraryParams{
			ID:   libID,
			Name: strings.TrimSpace(req.Name),
		})
		if e != nil {
			return queries.Library{}, e
		}
		for _, p := range cleanPaths {
			if _, e := q.AddLibraryPath(ctx, queries.AddLibraryPathParams{
				ID:        uuid.NewString(),
				LibraryID: libID,
				Path:      p,
			}); e != nil {
				return queries.Library{}, e
			}
		}
		return l, nil
	})
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "library name or path already exists")
			return
		}
		slog.Error("create library", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	slog.Info("library: created", "library_id", lib.ID, "name", lib.Name)
	writeJSON(w, http.StatusCreated, toGenLibrary(lib, cleanPaths))

	if a.scanner != nil {
		settings, err := queries.New(a.db.Read()).GetLibrarySettings(ctx, lib.ID)
		if err == nil && settings.ScanOnAdd {
			go func() { //nolint:contextcheck,gosec
				if err := a.scanner.Scan(context.Background(), lib.ID); err != nil &&
					!errors.Is(err, scanner.ErrScanAlreadyRunning) {
					slog.Error("scan_on_create: async error", "library_id", lib.ID, "err", err)
				}
			}()
		}
	}
}

func (a *API) getLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(a.db.Read())

	lib, err := q.GetLibraryByID(ctx, libraryID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("get library", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	if !auth.IsAdminFrom(ctx) {
		uid, _ := auth.UserIDFrom(ctx)
		if _, accessErr := q.GetLibraryAccess(ctx, queries.GetLibraryAccessParams{
			LibraryID: lib.ID,
			UserID:    uid.String(),
		}); accessErr != nil {
			writeError(w, http.StatusForbidden, "forbidden", "no access to this library")
			return
		}
	}

	paths, err := loadPaths(ctx, q, lib.ID)
	if err != nil {
		slog.Error("get library: load paths", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	writeJSON(w, http.StatusOK, toGenLibrary(lib, paths))
}

func (a *API) updateLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	var req openapi.LibraryWrite
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	cleanPaths, err := validateLibraryWrite(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	ctx := r.Context()
	lib, err := a.libTx(ctx, func(q *queries.Queries) (queries.Library, error) {
		l, e := q.UpdateLibrary(ctx, queries.UpdateLibraryParams{
			ID:   libraryID.String(),
			Name: strings.TrimSpace(req.Name),
		})
		if e != nil {
			return queries.Library{}, e
		}
		if e := q.ClearLibraryPaths(ctx, l.ID); e != nil {
			return queries.Library{}, e
		}
		for _, p := range cleanPaths {
			if _, e := q.AddLibraryPath(ctx, queries.AddLibraryPathParams{
				ID:        uuid.NewString(),
				LibraryID: l.ID,
				Path:      p,
			}); e != nil {
				return queries.Library{}, e
			}
		}
		return l, nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "conflict", "library name or path already exists")
			return
		}
		slog.Error("update library", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}

	slog.Info("library: updated", "library_id", libraryID)
	writeJSON(w, http.StatusOK, toGenLibrary(lib, cleanPaths))
}

func (a *API) deleteLibrary(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	q := queries.New(a.db.Write())
	if err := q.DeleteLibrary(r.Context(), libraryID.String()); err != nil {
		slog.Error("delete library", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	slog.Info("library: deleted", "library_id", libraryID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listLibraryMembers(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(a.db.Read())
	if _, err := q.GetLibraryByID(ctx, libraryID.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("list library members: get library", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	rows, err := q.ListLibraryMembers(ctx, libraryID.String())
	if err != nil {
		slog.Error("list library members", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	out := make([]openapi.LibraryMember, 0, len(rows))
	for _, m := range rows {
		member := openapi.LibraryMember{
			UserId:    uuid.MustParse(m.UserID),
			Username:  m.Username,
			Role:      openapi.LibraryMemberRole(m.Role),
			GrantedAt: libParseTime(m.GrantedAt),
		}
		member.DisplayName = m.DisplayName
		out = append(out, member)
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": out})
}

func (a *API) grantLibraryAccess(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(a.db.Write())
	if _, err := q.GetLibraryByID(ctx, libraryID.String()); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "library not found")
			return
		}
		slog.Error("grant library access: get library", "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	var req openapi.GrantAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "malformed request body")
		return
	}
	if req.UserId == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "user_id is required")
		return
	}
	role := string(req.Role)
	if role != "viewer" && role != "contributor" && role != "admin" {
		writeError(w, http.StatusBadRequest, "invalid_request", "role must be viewer, contributor, or admin")
		return
	}
	if err := q.GrantLibraryAccess(ctx, queries.GrantLibraryAccessParams{
		LibraryID: libraryID.String(),
		UserID:    req.UserId.String(),
		Role:      role,
	}); err != nil {
		slog.Error("grant library access", "library_id", libraryID, "user_id", req.UserId, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) revokeLibraryAccess(w http.ResponseWriter, r *http.Request, libraryID, userID uuid.UUID) {
	q := queries.New(a.db.Write())
	if err := q.RevokeLibraryAccess(r.Context(), queries.RevokeLibraryAccessParams{
		LibraryID: libraryID.String(),
		UserID:    userID.String(),
	}); err != nil {
		slog.Error("revoke library access", "library_id", libraryID, "user_id", userID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) triggerScan(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if a.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "scanner_not_ready", "scanner not initialised")
		return
	}
	ctx := r.Context()
	q := queries.New(a.db.Read())

	if _, err := q.GetActiveScanRun(ctx, libraryID.String()); err == nil {
		writeError(w, http.StatusConflict, "scan_running", "a scan is already running for this library")
		return
	}

	slog.Info("scan: triggered", "library_id", libraryID)
	go func() { //nolint:contextcheck,gosec
		bgCtx := context.Background()
		if err := a.scanner.Scan(bgCtx, libraryID.String()); err != nil &&
			!errors.Is(err, scanner.ErrScanAlreadyRunning) {
			slog.Error("scan: async error", "library_id", libraryID, "err", err)
		}
	}()

	writeJSON(w, http.StatusAccepted, map[string]any{})
}

func (a *API) cancelScan(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if a.scanner == nil {
		writeError(w, http.StatusServiceUnavailable, "scanner_not_ready", "scanner not initialised")
		return
	}
	if err := a.scanner.Cancel(r.Context(), libraryID.String()); err != nil {
		if errors.Is(err, scanner.ErrNoActiveScan) {
			writeError(w, http.StatusNotFound, "no_active_scan", "no scan is currently running for this library")
			return
		}
		writeError(w, http.StatusInternalServerError, "cancel_failed", "failed to cancel scan")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listScanRuns(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	q := queries.New(a.db.Read())
	ctx := r.Context()

	rows, err := q.ListScanRuns(ctx, queries.ListScanRunsParams{
		LibraryID: libraryID.String(),
		Limit:     int64(limit),
		Offset:    offset,
	})
	if err != nil {
		slog.Error("list scan runs", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "failed to list scan runs")
		return
	}
	total, err := q.CountScanRuns(ctx, libraryID.String())
	if err != nil {
		slog.Error("count scan runs", "library_id", libraryID, "err", err)
		writeError(w, http.StatusInternalServerError, "db_error", "failed to count scan runs")
		return
	}

	data := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		data = append(data, scanRunToMap(row.ID, row.LibraryID, row.StartedAt, row.FinishedAt,
			row.Status, row.FilesScanned, row.FilesAdded, row.FilesUpdated, row.FilesRemoved,
			row.FilesUnchanged, row.FilesErrored, row.TotalFiles, row.Error))
	}

	totalPages := int(total) / limit
	if int(total)%limit != 0 || total == 0 {
		totalPages++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":        page,
			"limit":       limit,
			"total":       int(total),
			"total_pages": totalPages,
		},
	})
}

func (a *API) getScanRun(w http.ResponseWriter, r *http.Request, libraryID, runID uuid.UUID) {
	q := queries.New(a.db.Read())
	row, err := q.GetScanRun(r.Context(), runID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "not_found", "scan run not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "failed to get scan run")
		return
	}
	if row.LibraryID != libraryID.String() {
		writeError(w, http.StatusNotFound, "not_found", "scan run not found")
		return
	}
	resp := scanRunToMap(row.ID, row.LibraryID, row.StartedAt, row.FinishedAt,
		row.Status, row.FilesScanned, row.FilesAdded, row.FilesUpdated, row.FilesRemoved,
		row.FilesUnchanged, row.FilesErrored, row.TotalFiles, row.Error)
	if row.Status == "running" && a.scanner != nil {
		if live := a.scanner.LiveState(row.ID); live != nil {
			resp["current_folder"] = live.CurrentFolder
			resp["files_scanned"] = live.Total
			resp["files_added"] = live.Added
			resp["files_updated"] = live.Updated
			resp["files_unchanged"] = live.Unchanged
			resp["files_removed"] = live.Removed
			resp["files_errored"] = live.Errored
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *API) getLibrarySettings(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	ctx := r.Context()
	q := queries.New(a.db.Read())
	s, err := q.GetLibrarySettings(ctx, libraryID.String())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to get settings")
			return
		}
		if _, execErr := a.db.Write().ExecContext(ctx,
			"INSERT OR IGNORE INTO library_settings (library_id) VALUES (?)",
			libraryID.String(),
		); execErr != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to initialize settings")
			return
		}
		s, err = q.GetLibrarySettings(ctx, libraryID.String())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to get settings after init")
			return
		}
	}
	writeJSON(w, http.StatusOK, settingsToMap(s))
}

func (a *API) putLibrarySettings(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	var body struct {
		ScanOnCreate             *bool    `json:"scan_on_create"`
		WatchForChanges          *bool    `json:"watch_for_changes"`
		ScanSchedule             *string  `json:"scan_schedule"`
		FollowSymlinks           *bool    `json:"follow_symlinks"`
		ScanPlaylistFiles        *bool    `json:"scan_playlist_files"`
		Extensions               []string `json:"extensions"`
		EnableMusicbrainz        *bool    `json:"enable_musicbrainz"`
		EnableAcoustid           *bool    `json:"enable_acoustid"`
		EnableDiscogs            *bool    `json:"enable_discogs"`
		MatchConfidenceThreshold *float64 `json:"match_confidence_threshold"`
		MetadataLanguage         *string  `json:"metadata_language"`
		AlbumArtPriority         []string `json:"album_art_priority"`
		ArtistImagePriority      []string `json:"artist_image_priority"`
		FormatPreference         []string `json:"format_preference"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON")
		return
	}

	ctx := r.Context()
	q := queries.New(a.db.Read())

	cur, err := q.GetLibrarySettings(ctx, libraryID.String())
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to get settings")
			return
		}
		if _, execErr := a.db.Write().ExecContext(ctx,
			"INSERT OR IGNORE INTO library_settings (library_id) VALUES (?)",
			libraryID.String(),
		); execErr != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to initialize settings")
			return
		}
		cur, err = q.GetLibrarySettings(ctx, libraryID.String())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "db_error", "failed to get settings after init")
			return
		}
	}

	p := queries.UpdateLibrarySettingsParams{
		ScanOnAdd:                 cur.ScanOnAdd,
		FsnotifyEnabled:           cur.FsnotifyEnabled,
		ScanSchedule:              cur.ScanSchedule,
		FollowSymlinks:            cur.FollowSymlinks,
		ScanPlaylistFiles:         cur.ScanPlaylistFiles,
		FileExtensions:            cur.FileExtensions,
		MbEnrichmentEnabled:       cur.MbEnrichmentEnabled,
		AcoustidEnabled:           cur.AcoustidEnabled,
		DiscogsEnabled:            cur.DiscogsEnabled,
		MatchThreshold:            cur.MatchThreshold,
		MetadataLanguage:          cur.MetadataLanguage,
		ArtistProviderPriority:    cur.ArtistProviderPriority,
		AlbumProviderPriority:     cur.AlbumProviderPriority,
		AlbumArtSourcePriority:    cur.AlbumArtSourcePriority,
		ArtistImageSourcePriority: cur.ArtistImageSourcePriority,
		FormatPreferenceOrder:     cur.FormatPreferenceOrder,
		LibraryID:                 libraryID.String(),
	}
	if body.ScanOnCreate != nil {
		p.ScanOnAdd = *body.ScanOnCreate
	}
	if body.WatchForChanges != nil {
		p.FsnotifyEnabled = *body.WatchForChanges
	}
	if body.ScanSchedule != nil {
		p.ScanSchedule = nilIfEmpty(*body.ScanSchedule)
	}
	if body.FollowSymlinks != nil {
		p.FollowSymlinks = *body.FollowSymlinks
	}
	if body.ScanPlaylistFiles != nil {
		p.ScanPlaylistFiles = *body.ScanPlaylistFiles
	}
	if body.Extensions != nil {
		p.FileExtensions = strings.Join(body.Extensions, ",")
	}
	if body.EnableMusicbrainz != nil {
		p.MbEnrichmentEnabled = *body.EnableMusicbrainz
	}
	if body.EnableAcoustid != nil {
		p.AcoustidEnabled = *body.EnableAcoustid
	}
	if body.EnableDiscogs != nil {
		p.DiscogsEnabled = *body.EnableDiscogs
	}
	if body.MatchConfidenceThreshold != nil {
		v := *body.MatchConfidenceThreshold
		if v < 0 || v > 1 {
			writeError(w, http.StatusUnprocessableEntity, "invalid_value", "match_confidence_threshold must be 0–1")
			return
		}
		p.MatchThreshold = v
	}
	if body.MetadataLanguage != nil {
		p.MetadataLanguage = *body.MetadataLanguage
	}
	if body.AlbumArtPriority != nil {
		p.AlbumArtSourcePriority = strings.Join(body.AlbumArtPriority, ",")
	}
	if body.ArtistImagePriority != nil {
		p.ArtistImageSourcePriority = strings.Join(body.ArtistImagePriority, ",")
	}
	if body.FormatPreference != nil {
		p.FormatPreferenceOrder = strings.Join(body.FormatPreference, ",")
	}

	wq := queries.New(a.db.Write())
	updated, err := wq.UpdateLibrarySettings(ctx, p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "failed to update settings")
		return
	}
	writeJSON(w, http.StatusOK, settingsToMap(updated))
}

func (a *API) libTx(ctx context.Context, fn func(*queries.Queries) (queries.Library, error)) (queries.Library, error) {
	tx, err := a.db.Write().BeginTx(ctx, nil)
	if err != nil {
		return queries.Library{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	lib, err := fn(queries.New(tx))
	if err != nil {
		return queries.Library{}, err
	}
	if err := tx.Commit(); err != nil {
		return queries.Library{}, fmt.Errorf("commit: %w", err)
	}
	return lib, nil
}

type (
	libraryHandler     func(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID)
	libraryUserHandler func(w http.ResponseWriter, r *http.Request, libraryID, userID uuid.UUID)
	libraryRunHandler  func(w http.ResponseWriter, r *http.Request, libraryID, runID uuid.UUID)
)

func withLibraryID(fn libraryHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "libraryId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "libraryId must be a valid UUID")
			return
		}
		fn(w, r, id)
	}
}

func withLibraryAndRunID(fn libraryRunHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libID, err := uuid.Parse(chi.URLParam(r, "libraryId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "libraryId must be a valid UUID")
			return
		}
		runID, err := uuid.Parse(chi.URLParam(r, "runId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "runId must be a valid UUID")
			return
		}
		fn(w, r, libID, runID)
	}
}

func withLibraryAndUserID(fn libraryUserHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libID, err := uuid.Parse(chi.URLParam(r, "libraryId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "libraryId must be a valid UUID")
			return
		}
		userID, err := uuid.Parse(chi.URLParam(r, "userId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_id", "userId must be a valid UUID")
			return
		}
		fn(w, r, libID, userID)
	}
}

func loadPaths(ctx context.Context, q *queries.Queries, libraryID string) ([]string, error) {
	rows, err := q.ListLibraryPaths(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	ps := make([]string, 0, len(rows))
	for _, r := range rows {
		ps = append(ps, r.Path)
	}
	return ps, nil
}

func pathsForLibraries(ctx context.Context, q *queries.Queries) (map[string][]string, error) {
	all, err := q.ListAllLibraryPaths(ctx)
	if err != nil {
		return nil, err
	}
	byLib := make(map[string][]string, len(all))
	for _, p := range all {
		byLib[p.LibraryID] = append(byLib[p.LibraryID], p.Path)
	}
	return byLib, nil
}

func validateLibraryWrite(req openapi.LibraryWrite) ([]string, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, errors.New("name must not be empty")
	}
	if len(req.Paths) == 0 {
		return nil, errors.New("at least one path is required")
	}
	seen := make(map[string]struct{}, len(req.Paths))
	out := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		clean := filepath.Clean(strings.TrimSpace(p))
		if clean == "" || clean == "." {
			return nil, errors.New("paths must not be empty")
		}
		if _, dup := seen[clean]; dup {
			return nil, fmt.Errorf("duplicate path %q", clean)
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out, nil
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

func toGenLibrary(l queries.Library, paths []string) openapi.Library {
	if paths == nil {
		paths = []string{}
	}
	return openapi.Library{
		Id:        uuid.MustParse(l.ID),
		Name:      l.Name,
		Paths:     paths,
		CreatedAt: libParseTime(l.CreatedAt),
		UpdatedAt: libParseTime(l.UpdatedAt),
	}
}

func libParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func scanRunToMap(
	id, libraryID, startedAt string,
	finishedAt *string,
	status string,
	scanned, added, updated, removed, unchanged, errored, total int64,
	errMsg *string,
) map[string]any {
	var pct *float64
	if total > 0 {
		v := float64(scanned) / float64(total) * 100
		pct = &v
	}
	return map[string]any{
		"id":              id,
		"library_id":      libraryID,
		"started_at":      startedAt,
		"finished_at":     finishedAt,
		"status":          status,
		"files_scanned":   scanned,
		"files_added":     added,
		"files_updated":   updated,
		"files_removed":   removed,
		"files_unchanged": unchanged,
		"files_errored":   errored,
		"total_files":     total,
		"pct_complete":    pct,
		"error":           errMsg,
	}
}

func parsePage(r *http.Request) (page, limit int) {
	page = 1
	limit = 50
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

func settingsToMap(s queries.LibrarySetting) map[string]any {
	return map[string]any{
		"scan_on_create":             s.ScanOnAdd,
		"watch_for_changes":          s.FsnotifyEnabled,
		"scan_schedule":              s.ScanSchedule,
		"follow_symlinks":            s.FollowSymlinks,
		"scan_playlist_files":        s.ScanPlaylistFiles,
		"extensions":                 splitCSV(s.FileExtensions),
		"enable_musicbrainz":         s.MbEnrichmentEnabled,
		"enable_acoustid":            s.AcoustidEnabled,
		"enable_discogs":             s.DiscogsEnabled,
		"match_confidence_threshold": s.MatchThreshold,
		"metadata_language":          s.MetadataLanguage,
		"album_art_priority":         splitCSV(s.AlbumArtSourcePriority),
		"artist_image_priority":      splitCSV(s.ArtistImageSourcePriority),
		"format_preference":          splitCSV(s.FormatPreferenceOrder),
	}
}

func splitCSV(s string) []string {
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
