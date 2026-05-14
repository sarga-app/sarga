-- name: GetUserByID :one
SELECT id, username, email, password_hash, display_name,
       is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT id, username, email, password_hash, display_name,
       is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users WHERE username = ?;

-- name: GetUserByEmail :one
SELECT id, username, email, password_hash, display_name,
       is_admin, disabled_at, password_changed_at, created_at, updated_at
FROM users WHERE email = sqlc.arg('email');

-- name: CreateUser :one
INSERT INTO users (id, username, email, password_hash, display_name, is_admin, password_changed_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, username, email, password_hash, display_name,
          is_admin, disabled_at, password_changed_at, created_at, updated_at;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash       = ?,
    password_changed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?;
