-- name: EnqueueTrackFile :exec
INSERT INTO enrichment_queue (id, track_file_id, library_id, priority)
VALUES (?, ?, ?, ?)
ON CONFLICT (track_file_id) WHERE status IN ('pending','in_progress')
DO NOTHING;

-- name: ClaimEnrichmentBatch :many
UPDATE enrichment_queue
SET
    status            = 'in_progress',
    last_attempted_at = CURRENT_TIMESTAMP,
    attempts          = attempts + 1
WHERE id IN (
    SELECT id FROM enrichment_queue
    WHERE status = 'pending'
    ORDER BY priority DESC, created_at
    LIMIT ?
)
RETURNING id, track_file_id, library_id, priority, attempts, last_attempted_at, status, last_error, created_at;

-- name: MarkEnrichmentDone :exec
DELETE FROM enrichment_queue WHERE id = ?;

-- name: MarkEnrichmentFailed :exec
UPDATE enrichment_queue SET
    status     = CASE WHEN attempts >= 3 THEN 'failed' ELSE 'pending' END,
    last_error = ?
WHERE id = ?;

-- name: ResetStuckEnrichments :exec
UPDATE enrichment_queue SET status = 'pending'
WHERE status = 'in_progress'
  AND last_attempted_at < datetime('now', '-10 minutes');

-- name: CountPendingEnrichments :one
SELECT COUNT(*) FROM enrichment_queue
WHERE library_id = ? AND status IN ('pending','in_progress');

-- name: CountFailedEnrichments :one
SELECT COUNT(*) FROM enrichment_queue
WHERE library_id = ? AND status = 'failed';
