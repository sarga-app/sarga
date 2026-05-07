-- +goose Up

CREATE TABLE library_paths (
    id         TEXT NOT NULL PRIMARY KEY,
    library_id TEXT NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    path       TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX library_paths_path_key       ON library_paths (path);
CREATE INDEX        library_paths_library_id_idx ON library_paths (library_id);

-- Migrate existing libraries.path values into library_paths rows.
-- IDs are 32-char lowercase hex strings — unique, sufficient as a row id.
INSERT INTO library_paths (id, library_id, path)
SELECT lower(hex(randomblob(16))), id, path
FROM libraries
WHERE path IS NOT NULL AND path != '';

-- A library's scan locations live in library_paths now, not on the libraries
-- row itself. SQLite ≥ 3.35 supports ALTER TABLE DROP COLUMN.
ALTER TABLE libraries DROP COLUMN path;


-- +goose Down

ALTER TABLE libraries ADD COLUMN path TEXT NOT NULL DEFAULT '';
UPDATE libraries SET path = COALESCE(
    (SELECT path FROM library_paths WHERE library_paths.library_id = libraries.id ORDER BY created_at LIMIT 1),
    ''
);
DROP TABLE library_paths;
