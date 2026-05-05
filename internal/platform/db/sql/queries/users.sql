-- name: GetUserByID :one
-- Lookup by primary key. Returns disabled users too — caller decides.
SELECT
    id, username, email, password_hash, display_name,
    is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
-- Caller must lowercase the username before calling.
SELECT
    id, username, email, password_hash, display_name,
    is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users
WHERE username = $1;

-- name: GetUserByEmail :one
-- Caller must lowercase the email before calling.
SELECT
    id, username, email, password_hash, display_name,
    is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users
WHERE email = sqlc.arg('email')::text;

-- name: CreateUser :one
INSERT INTO users (
    username, email, password_hash, display_name, is_admin, password_changed_at
) VALUES (
    $1, $2, $3, $4, $5, CASE WHEN $3::text IS NOT NULL THEN now() ELSE NULL END
)
RETURNING id, username, email, password_hash, display_name,
          is_admin, disabled_at, password_changed_at, created_at, updated_at;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash       = $2,
    password_changed_at = now()
WHERE id = $1;
