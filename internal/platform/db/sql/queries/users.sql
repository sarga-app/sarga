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
