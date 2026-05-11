package libraries

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/google/uuid"

	"github.com/sarga-app/sarga/internal/domain/library"
	"github.com/sarga-app/sarga/internal/support/db/queries"
	"github.com/sarga-app/sarga/internal/transport/httpx"
)

func (h *Handler) TriggerScan(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	if h.scanner == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "scanner_not_ready", "scanner not initialised")
		return
	}
	ctx := r.Context()
	q := queries.New(h.db.Read())

	if _, err := q.GetActiveScanRun(ctx, libraryID.String()); err == nil {
		httpx.WriteError(w, http.StatusConflict, "scan_running", "a scan is already running for this library")
		return
	}

	slog.Info("scan: triggered", "library_id", libraryID)
	go func() { //nolint:contextcheck,gosec
		bgCtx := context.Background()
		if err := h.scanner.Scan(bgCtx, libraryID.String()); err != nil &&
			!errors.Is(err, library.ErrScanAlreadyRunning) {
			slog.Error("scan: async error", "library_id", libraryID, "err", err)
		}
	}()

	httpx.WriteJSON(w, http.StatusAccepted, map[string]any{})
}

func (h *Handler) ListScanRuns(w http.ResponseWriter, r *http.Request, libraryID uuid.UUID) {
	page, limit := parsePage(r)
	offset := int64((page - 1) * limit)

	q := queries.New(h.db.Read())
	ctx := r.Context()

	rows, err := q.ListScanRuns(ctx, queries.ListScanRunsParams{
		LibraryID: libraryID.String(),
		Limit:     int64(limit),
		Offset:    offset,
	})
	if err != nil {
		slog.Error("list scan runs", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to list scan runs")
		return
	}
	total, err := q.CountScanRuns(ctx, libraryID.String())
	if err != nil {
		slog.Error("count scan runs", "library_id", libraryID, "err", err)
		httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to count scan runs")
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

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data": data,
		"pagination": map[string]int{
			"page":        page,
			"limit":       limit,
			"total":       int(total),
			"total_pages": totalPages,
		},
	})
}

func (h *Handler) GetScanRun(w http.ResponseWriter, r *http.Request, libraryID, runID uuid.UUID) {
	q := queries.New(h.db.Read())
	row, err := q.GetScanRun(r.Context(), runID.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "scan run not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "db_error", "failed to get scan run")
		return
	}
	if row.LibraryID != libraryID.String() {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "scan run not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, scanRunToMap(row.ID, row.LibraryID, row.StartedAt, row.FinishedAt,
		row.Status, row.FilesScanned, row.FilesAdded, row.FilesUpdated, row.FilesRemoved,
		row.FilesUnchanged, row.FilesErrored, row.TotalFiles, row.Error))
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
	m := map[string]any{
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
	return m
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
