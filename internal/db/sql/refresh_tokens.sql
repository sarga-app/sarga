-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, parent_id, created_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, user_id, token_hash, expires_at, revoked_at, parent_id, created_at;

-- name: GetRefreshTokenByHash :one
SELECT id, user_id, token_hash, expires_at, revoked_at, parent_id, created_at
FROM refresh_tokens WHERE token_hash = ?;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ? AND revoked_at IS NULL;

-- name: RevokeAllUserRefreshTokens :exec
UPDATE refresh_tokens
SET revoked_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE user_id = ? AND revoked_at IS NULL;
