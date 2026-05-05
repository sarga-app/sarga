-- name: PingBootstrap :one
-- Smoke query so sqlc generates code before real queries land (Step 4+).
-- Removed when _bootstrap is dropped.
SELECT id FROM _bootstrap LIMIT 1;
