-- +goose Up
INSERT INTO tracks_fts (track_id, title) SELECT id, title FROM tracks;
INSERT INTO releases_fts (release_id, title) SELECT id, title FROM releases;
INSERT INTO artists_fts (artist_id, name) SELECT id, name FROM artists;

-- +goose Down
DELETE FROM tracks_fts;
DELETE FROM releases_fts;
DELETE FROM artists_fts;
