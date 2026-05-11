-- +goose Up

-- artists: enrichment fields
ALTER TABLE artists ADD COLUMN country           TEXT;
ALTER TABLE artists ADD COLUMN bio              TEXT;
ALTER TABLE artists ADD COLUMN begin_year       INTEGER;
ALTER TABLE artists ADD COLUMN end_year         INTEGER;
ALTER TABLE artists ADD COLUMN is_various_artists INTEGER NOT NULL DEFAULT 0;
ALTER TABLE artists ADD COLUMN discogs_id       TEXT;
ALTER TABLE artists ADD COLUMN tadb_id          TEXT;

-- releases: enrichment + edition fields
ALTER TABLE releases ADD COLUMN catalog_number TEXT;
ALTER TABLE releases ADD COLUMN discogs_id     TEXT;
ALTER TABLE releases ADD COLUMN total_discs    INTEGER NOT NULL DEFAULT 1;

-- tracks: audio analysis + external IDs
ALTER TABLE tracks ADD COLUMN bpm         INTEGER;
ALTER TABLE tracks ADD COLUMN key_sig     TEXT;
ALTER TABLE tracks ADD COLUMN acoustid_id TEXT;

-- track_files: match metadata + ReplayGain
ALTER TABLE track_files ADD COLUMN match_confidence REAL    NOT NULL DEFAULT 0;
ALTER TABLE track_files ADD COLUMN match_source     TEXT    NOT NULL DEFAULT 'raw';
ALTER TABLE track_files ADD COLUMN missing_since    TEXT;
ALTER TABLE track_files ADD COLUMN rg_track_gain    REAL;
ALTER TABLE track_files ADD COLUMN rg_track_peak    REAL;
ALTER TABLE track_files ADD COLUMN rg_album_gain    REAL;
ALTER TABLE track_files ADD COLUMN rg_album_peak    REAL;

-- scan_runs: progress counters
ALTER TABLE scan_runs ADD COLUMN total_files     INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scan_runs ADD COLUMN files_unchanged INTEGER NOT NULL DEFAULT 0;
ALTER TABLE scan_runs ADD COLUMN files_errored   INTEGER NOT NULL DEFAULT 0;

-- lyrics: recreate with expanded source CHECK (adds 'sidecar')
CREATE TABLE lyrics_new (
    id         TEXT    NOT NULL PRIMARY KEY,
    track_id   TEXT    NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    source     TEXT    NOT NULL,
    is_synced  INTEGER NOT NULL DEFAULT 0,
    language   TEXT,
    content    TEXT    NOT NULL,
    fetched_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('lrclib','embedded','manual','sidecar'))
) STRICT;
INSERT INTO lyrics_new SELECT * FROM lyrics;
DROP TABLE lyrics;
ALTER TABLE lyrics_new RENAME TO lyrics;
CREATE UNIQUE INDEX lyrics_track_source_key ON lyrics (track_id, source);

-- cover_art: recreate with expanded source_kind CHECK (adds external providers)
CREATE TABLE cover_art_new (
    id               TEXT    NOT NULL PRIMARY KEY,
    release_id       TEXT    REFERENCES releases(id)       ON DELETE CASCADE,
    release_group_id TEXT    REFERENCES release_groups(id) ON DELETE CASCADE,
    artist_id        TEXT    REFERENCES artists(id)        ON DELETE CASCADE,
    source_kind      TEXT    NOT NULL,
    uri              TEXT    NOT NULL,
    width            INTEGER,
    height           INTEGER,
    content_hash     BLOB,
    is_preferred     INTEGER NOT NULL DEFAULT 0,
    fetched_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (release_id IS NOT NULL AND release_group_id IS NULL     AND artist_id IS NULL) OR
        (release_id IS NULL     AND release_group_id IS NOT NULL AND artist_id IS NULL) OR
        (release_id IS NULL     AND release_group_id IS NULL     AND artist_id IS NOT NULL)
    ),
    CHECK (source_kind IN ('caa','folder','embedded','manual','theaudiodb','discogs','fanart','lastfm'))
) STRICT;
INSERT INTO cover_art_new SELECT * FROM cover_art;
DROP TABLE cover_art;
ALTER TABLE cover_art_new RENAME TO cover_art;
CREATE INDEX cover_art_release_idx       ON cover_art (release_id)       WHERE release_id       IS NOT NULL;
CREATE INDEX cover_art_release_group_idx ON cover_art (release_group_id) WHERE release_group_id IS NOT NULL;
CREATE INDEX cover_art_artist_idx        ON cover_art (artist_id)        WHERE artist_id        IS NOT NULL;

-- per-library configuration
CREATE TABLE library_settings (
    library_id                   TEXT NOT NULL PRIMARY KEY REFERENCES libraries(id) ON DELETE CASCADE,
    scan_on_add                  INTEGER NOT NULL DEFAULT 1,
    fsnotify_enabled             INTEGER NOT NULL DEFAULT 1,
    scan_schedule                TEXT,
    follow_symlinks              INTEGER NOT NULL DEFAULT 1,
    scan_playlist_files          INTEGER NOT NULL DEFAULT 1,
    file_extensions              TEXT    NOT NULL DEFAULT '.flac,.mp3,.m4a,.ogg,.opus,.wav,.aiff,.wv,.ape',
    mb_enrichment_enabled        INTEGER NOT NULL DEFAULT 1,
    acoustid_enabled             INTEGER NOT NULL DEFAULT 1,
    discogs_enabled              INTEGER NOT NULL DEFAULT 1,
    match_threshold              REAL    NOT NULL DEFAULT 0.85,
    metadata_language            TEXT    NOT NULL DEFAULT 'en',
    artist_provider_priority     TEXT    NOT NULL DEFAULT 'theaudiodb,lastfm,musicbrainz',
    album_provider_priority      TEXT    NOT NULL DEFAULT 'musicbrainz,discogs,theaudiodb',
    album_art_source_priority    TEXT    NOT NULL DEFAULT 'embedded,folder,caa,discogs,theaudiodb',
    artist_image_source_priority TEXT    NOT NULL DEFAULT 'theaudiodb,fanart,lastfm',
    format_preference_order      TEXT    NOT NULL DEFAULT 'flac,alac,wav,aiff,opus,ogg,aac,mp3',
    created_at                   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                   TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

-- +goose StatementBegin
CREATE TRIGGER library_settings_auto_create
AFTER INSERT ON libraries
BEGIN
    INSERT INTO library_settings (library_id) VALUES (NEW.id);
END;
-- +goose StatementEnd

-- background enrichment queue (survives restarts)
CREATE TABLE enrichment_queue (
    id                TEXT    NOT NULL PRIMARY KEY,
    track_file_id     TEXT    NOT NULL REFERENCES track_files(id) ON DELETE CASCADE,
    library_id        TEXT    NOT NULL REFERENCES libraries(id)   ON DELETE CASCADE,
    priority          INTEGER NOT NULL DEFAULT 0,
    attempts          INTEGER NOT NULL DEFAULT 0,
    last_attempted_at TEXT,
    status            TEXT    NOT NULL DEFAULT 'pending',
    last_error        TEXT,
    created_at        TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (status IN ('pending','in_progress','done','failed'))
) STRICT;
CREATE INDEX enrichment_queue_pending_idx     ON enrichment_queue (priority DESC, created_at) WHERE status = 'pending';
CREATE INDEX enrichment_queue_in_progress_idx ON enrichment_queue (last_attempted_at)         WHERE status = 'in_progress';
CREATE UNIQUE INDEX enrichment_queue_track_file_key ON enrichment_queue (track_file_id)       WHERE status IN ('pending','in_progress');

-- FTS5 search tables (trigram tokenizer; SQLite ≥ 3.34, bundled 3.53)
CREATE VIRTUAL TABLE tracks_fts USING fts5(
    track_id UNINDEXED,
    title,
    tokenize = 'trigram'
);
CREATE VIRTUAL TABLE releases_fts USING fts5(
    release_id UNINDEXED,
    title,
    tokenize = 'trigram'
);
CREATE VIRTUAL TABLE artists_fts USING fts5(
    artist_id UNINDEXED,
    name,
    tokenize = 'trigram'
);

-- FTS sync triggers: tracks
-- +goose StatementBegin
CREATE TRIGGER tracks_fts_insert AFTER INSERT ON tracks BEGIN
    INSERT INTO tracks_fts (track_id, title) VALUES (NEW.id, NEW.title);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER tracks_fts_update AFTER UPDATE OF title ON tracks BEGIN
    DELETE FROM tracks_fts WHERE track_id = OLD.id;
    INSERT INTO tracks_fts (track_id, title) VALUES (NEW.id, NEW.title);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER tracks_fts_delete AFTER DELETE ON tracks BEGIN
    DELETE FROM tracks_fts WHERE track_id = OLD.id;
END;
-- +goose StatementEnd

-- FTS sync triggers: releases
-- +goose StatementBegin
CREATE TRIGGER releases_fts_insert AFTER INSERT ON releases BEGIN
    INSERT INTO releases_fts (release_id, title) VALUES (NEW.id, NEW.title);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER releases_fts_update AFTER UPDATE OF title ON releases BEGIN
    DELETE FROM releases_fts WHERE release_id = OLD.id;
    INSERT INTO releases_fts (release_id, title) VALUES (NEW.id, NEW.title);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER releases_fts_delete AFTER DELETE ON releases BEGIN
    DELETE FROM releases_fts WHERE release_id = OLD.id;
END;
-- +goose StatementEnd

-- FTS sync triggers: artists
-- +goose StatementBegin
CREATE TRIGGER artists_fts_insert AFTER INSERT ON artists BEGIN
    INSERT INTO artists_fts (artist_id, name) VALUES (NEW.id, NEW.name);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER artists_fts_update AFTER UPDATE OF name ON artists BEGIN
    DELETE FROM artists_fts WHERE artist_id = OLD.id;
    INSERT INTO artists_fts (artist_id, name) VALUES (NEW.id, NEW.name);
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER artists_fts_delete AFTER DELETE ON artists BEGIN
    DELETE FROM artists_fts WHERE artist_id = OLD.id;
END;
-- +goose StatementEnd


-- +goose Down

DROP TRIGGER IF EXISTS artists_fts_delete;
DROP TRIGGER IF EXISTS artists_fts_update;
DROP TRIGGER IF EXISTS artists_fts_insert;
DROP TRIGGER IF EXISTS releases_fts_delete;
DROP TRIGGER IF EXISTS releases_fts_update;
DROP TRIGGER IF EXISTS releases_fts_insert;
DROP TRIGGER IF EXISTS tracks_fts_delete;
DROP TRIGGER IF EXISTS tracks_fts_update;
DROP TRIGGER IF EXISTS tracks_fts_insert;
DROP TABLE IF EXISTS artists_fts;
DROP TABLE IF EXISTS releases_fts;
DROP TABLE IF EXISTS tracks_fts;
DROP TABLE IF EXISTS enrichment_queue;
DROP TRIGGER IF EXISTS library_settings_auto_create;
DROP TABLE IF EXISTS library_settings;

-- restore cover_art to original CHECK (source_kind)
CREATE TABLE cover_art_orig (
    id               TEXT    NOT NULL PRIMARY KEY,
    release_id       TEXT    REFERENCES releases(id)       ON DELETE CASCADE,
    release_group_id TEXT    REFERENCES release_groups(id) ON DELETE CASCADE,
    artist_id        TEXT    REFERENCES artists(id)        ON DELETE CASCADE,
    source_kind      TEXT    NOT NULL,
    uri              TEXT    NOT NULL,
    width            INTEGER,
    height           INTEGER,
    content_hash     BLOB,
    is_preferred     INTEGER NOT NULL DEFAULT 0,
    fetched_at       TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (
        (release_id IS NOT NULL AND release_group_id IS NULL     AND artist_id IS NULL) OR
        (release_id IS NULL     AND release_group_id IS NOT NULL AND artist_id IS NULL) OR
        (release_id IS NULL     AND release_group_id IS NULL     AND artist_id IS NOT NULL)
    ),
    CHECK (source_kind IN ('caa','folder','embedded','manual'))
) STRICT;
INSERT INTO cover_art_orig SELECT id, release_id, release_group_id, artist_id, source_kind, uri, width, height, content_hash, is_preferred, fetched_at FROM cover_art WHERE source_kind IN ('caa','folder','embedded','manual');
DROP TABLE cover_art;
ALTER TABLE cover_art_orig RENAME TO cover_art;
CREATE INDEX cover_art_release_idx       ON cover_art (release_id)       WHERE release_id       IS NOT NULL;
CREATE INDEX cover_art_release_group_idx ON cover_art (release_group_id) WHERE release_group_id IS NOT NULL;
CREATE INDEX cover_art_artist_idx        ON cover_art (artist_id)        WHERE artist_id        IS NOT NULL;

-- restore lyrics to original CHECK (source)
CREATE TABLE lyrics_orig (
    id         TEXT    NOT NULL PRIMARY KEY,
    track_id   TEXT    NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    source     TEXT    NOT NULL,
    is_synced  INTEGER NOT NULL DEFAULT 0,
    language   TEXT,
    content    TEXT    NOT NULL,
    fetched_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('lrclib','embedded','manual'))
) STRICT;
INSERT INTO lyrics_orig SELECT * FROM lyrics WHERE source IN ('lrclib','embedded','manual');
DROP TABLE lyrics;
ALTER TABLE lyrics_orig RENAME TO lyrics;
CREATE UNIQUE INDEX lyrics_track_source_key ON lyrics (track_id, source);

-- remove added columns (SQLite 3.35+)
ALTER TABLE scan_runs   DROP COLUMN files_errored;
ALTER TABLE scan_runs   DROP COLUMN files_unchanged;
ALTER TABLE scan_runs   DROP COLUMN total_files;
ALTER TABLE track_files DROP COLUMN rg_album_peak;
ALTER TABLE track_files DROP COLUMN rg_album_gain;
ALTER TABLE track_files DROP COLUMN rg_track_peak;
ALTER TABLE track_files DROP COLUMN rg_track_gain;
ALTER TABLE track_files DROP COLUMN missing_since;
ALTER TABLE track_files DROP COLUMN match_source;
ALTER TABLE track_files DROP COLUMN match_confidence;
ALTER TABLE tracks      DROP COLUMN acoustid_id;
ALTER TABLE tracks      DROP COLUMN key_sig;
ALTER TABLE tracks      DROP COLUMN bpm;
ALTER TABLE releases    DROP COLUMN total_discs;
ALTER TABLE releases    DROP COLUMN discogs_id;
ALTER TABLE releases    DROP COLUMN catalog_number;
ALTER TABLE artists     DROP COLUMN tadb_id;
ALTER TABLE artists     DROP COLUMN discogs_id;
ALTER TABLE artists     DROP COLUMN is_various_artists;
ALTER TABLE artists     DROP COLUMN end_year;
ALTER TABLE artists     DROP COLUMN begin_year;
ALTER TABLE artists     DROP COLUMN bio;
ALTER TABLE artists     DROP COLUMN country;
