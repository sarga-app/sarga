-- name: InsertScanRun :one
INSERT INTO scan_runs (id, library_id, total_files, status)
VALUES (?, ?, ?, 'running')
RETURNING
    id, library_id, started_at, finished_at, status,
    files_scanned, files_added, files_updated, files_removed,
    files_unchanged, files_errored, total_files, error;

-- name: GetActiveScanRun :one
SELECT
    id, library_id, started_at, finished_at, status,
    files_scanned, files_added, files_updated, files_removed,
    files_unchanged, files_errored, total_files, error
FROM scan_runs
WHERE library_id = ? AND status = 'running'
LIMIT 1;

-- name: UpdateScanRunProgress :exec
UPDATE scan_runs SET
    files_scanned  = files_scanned  + ?,
    files_added    = files_added    + ?,
    files_updated  = files_updated  + ?,
    files_removed  = files_removed  + ?,
    files_unchanged = files_unchanged + ?,
    files_errored  = files_errored  + ?
WHERE id = ?;

-- name: FinishScanRun :exec
UPDATE scan_runs SET
    status      = ?,
    finished_at = CURRENT_TIMESTAMP,
    error       = ?
WHERE id = ?;

-- name: GetScanRun :one
SELECT
    id, library_id, started_at, finished_at, status,
    files_scanned, files_added, files_updated, files_removed,
    files_unchanged, files_errored, total_files, error
FROM scan_runs
WHERE id = ?;

-- name: ListScanRuns :many
SELECT
    id, library_id, started_at, finished_at, status,
    files_scanned, files_added, files_updated, files_removed,
    files_unchanged, files_errored, total_files, error
FROM scan_runs
WHERE library_id = ?
ORDER BY started_at DESC
LIMIT ? OFFSET ?;

-- name: CountScanRuns :one
SELECT COUNT(*) FROM scan_runs WHERE library_id = ?;

-- name: ListInterruptedScanLibraries :many
SELECT DISTINCT library_id FROM scan_runs WHERE status = 'running';

-- name: MarkInterruptedScansAsFailed :exec
UPDATE scan_runs SET status = 'failed', error = 'interrupted by server restart'
WHERE status = 'running';
