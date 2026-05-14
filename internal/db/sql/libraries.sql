-- name: CreateLibrary :one
INSERT INTO libraries (id, name)
VALUES (?, ?)
RETURNING id, name, created_at, updated_at;

-- name: GetLibraryByID :one
SELECT id, name, created_at, updated_at
FROM libraries
WHERE id = ?;

-- name: ListLibraries :many
SELECT id, name, created_at, updated_at
FROM libraries
ORDER BY name;

-- name: ListLibrariesForUser :many
SELECT l.id, l.name, l.created_at, l.updated_at
FROM libraries l
JOIN library_access la ON la.library_id = l.id
WHERE la.user_id = ?
ORDER BY l.name;

-- name: UpdateLibrary :one
UPDATE libraries
SET name = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = ?
RETURNING id, name, created_at, updated_at;

-- name: DeleteLibrary :exec
DELETE FROM libraries WHERE id = ?;

-- name: GrantLibraryAccess :exec
INSERT INTO library_access (library_id, user_id, role)
VALUES (?, ?, ?)
ON CONFLICT (library_id, user_id) DO UPDATE SET role = excluded.role;

-- name: RevokeLibraryAccess :exec
DELETE FROM library_access WHERE library_id = ? AND user_id = ?;

-- name: GetLibraryAccess :one
SELECT library_id, user_id, role, granted_at
FROM library_access
WHERE library_id = ? AND user_id = ?;

-- name: ListLibraryMembers :many
SELECT la.library_id, la.user_id, la.role, la.granted_at,
       u.username, u.display_name
FROM library_access la
JOIN users u ON u.id = la.user_id
WHERE la.library_id = ?
ORDER BY u.username;

-- name: AddLibraryPath :one
INSERT INTO library_paths (id, library_id, path)
VALUES (?, ?, ?)
RETURNING id, library_id, path, created_at;

-- name: ListLibraryPaths :many
SELECT id, library_id, path, created_at
FROM library_paths
WHERE library_id = ?
ORDER BY created_at;

-- name: ListAllLibraryPaths :many
SELECT id, library_id, path, created_at
FROM library_paths
ORDER BY library_id, created_at;

-- name: ClearLibraryPaths :exec
DELETE FROM library_paths WHERE library_id = ?;

-- name: RemoveLibraryPath :exec
DELETE FROM library_paths WHERE id = ? AND library_id = ?;
