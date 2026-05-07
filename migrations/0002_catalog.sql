-- +goose Up

CREATE TABLE artists (
    id         TEXT NOT NULL PRIMARY KEY,
    mbid       TEXT,
    name       TEXT NOT NULL,
    sort_name  TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX artists_mbid_key ON artists (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        artists_sort_idx ON artists (sort_name);

CREATE TABLE release_groups (
    id                 TEXT    NOT NULL PRIMARY KEY,
    mbid               TEXT,
    title              TEXT    NOT NULL,
    primary_type       TEXT    NOT NULL DEFAULT 'album',
    first_release_year INTEGER,
    created_at         TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (primary_type IN ('album','ep','single','compilation','soundtrack','live','remix','other'))
) STRICT;
CREATE UNIQUE INDEX release_groups_mbid_key  ON release_groups (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        release_groups_title_idx ON release_groups (title);

CREATE TABLE releases (
    id               TEXT NOT NULL PRIMARY KEY,
    mbid             TEXT,
    release_group_id TEXT NOT NULL REFERENCES release_groups(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    country          TEXT,
    release_date     TEXT,
    label_name       TEXT,
    barcode          TEXT,
    edition_notes    TEXT,
    created_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at       TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX releases_mbid_key          ON releases (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        releases_release_group_idx ON releases (release_group_id);

CREATE TABLE release_artists (
    release_id TEXT    NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    artist_id  TEXT    NOT NULL REFERENCES artists(id)  ON DELETE RESTRICT,
    position   INTEGER NOT NULL,
    role       TEXT    NOT NULL DEFAULT 'primary',
    PRIMARY KEY (release_id, artist_id, role),
    CHECK (role IN ('primary','feature','remixer','producer','other'))
) STRICT, WITHOUT ROWID;
CREATE INDEX release_artists_artist_idx ON release_artists (artist_id);

CREATE TABLE tracks (
    id          TEXT    NOT NULL PRIMARY KEY,
    mbid        TEXT,
    title       TEXT    NOT NULL,
    duration_ms INTEGER,
    isrc        TEXT,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE UNIQUE INDEX tracks_mbid_key  ON tracks (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        tracks_title_idx ON tracks (title);

CREATE TABLE release_tracks (
    release_id  TEXT    NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    track_id    TEXT    NOT NULL REFERENCES tracks(id)   ON DELETE RESTRICT,
    disc_number INTEGER NOT NULL DEFAULT 1,
    position    INTEGER NOT NULL,
    PRIMARY KEY (release_id, disc_number, position)
) STRICT, WITHOUT ROWID;
CREATE INDEX release_tracks_track_idx ON release_tracks (track_id);

CREATE TABLE track_artists (
    track_id  TEXT    NOT NULL REFERENCES tracks(id)  ON DELETE CASCADE,
    artist_id TEXT    NOT NULL REFERENCES artists(id) ON DELETE RESTRICT,
    position  INTEGER NOT NULL,
    role      TEXT    NOT NULL DEFAULT 'primary',
    PRIMARY KEY (track_id, artist_id, role),
    CHECK (role IN ('primary','feature','remixer','producer','other'))
) STRICT, WITHOUT ROWID;
CREATE INDEX track_artists_artist_idx ON track_artists (artist_id);

CREATE TABLE track_files (
    id              TEXT    NOT NULL PRIMARY KEY,
    track_id        TEXT    NOT NULL REFERENCES tracks(id)    ON DELETE CASCADE,
    library_id      TEXT    NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    relative_path   TEXT    NOT NULL,
    format          TEXT    NOT NULL,
    codec           TEXT    NOT NULL,
    bitrate_kbps    INTEGER,
    sample_rate_hz  INTEGER,
    bit_depth       INTEGER,
    channels        INTEGER NOT NULL DEFAULT 2,
    file_size_bytes INTEGER NOT NULL,
    mtime           TEXT    NOT NULL,
    content_hash    BLOB    NOT NULL,
    source_kind     TEXT    NOT NULL DEFAULT 'unknown',
    is_lossless     INTEGER NOT NULL DEFAULT 0,
    is_preferred    INTEGER NOT NULL DEFAULT 0,
    added_at        TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source_kind IN ('unknown','rip','purchase','import','manual')),
    CHECK (length(content_hash) = 16)
) STRICT;
CREATE UNIQUE INDEX track_files_library_path_key ON track_files (library_id, relative_path);
CREATE INDEX        track_files_track_idx        ON track_files (track_id);
CREATE INDEX        track_files_content_hash_idx ON track_files (content_hash);
CREATE UNIQUE INDEX track_files_preferred_per_track_key
    ON track_files (track_id) WHERE is_preferred = 1;

CREATE TABLE lyrics (
    id         TEXT    NOT NULL PRIMARY KEY,
    track_id   TEXT    NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    source     TEXT    NOT NULL,
    is_synced  INTEGER NOT NULL DEFAULT 0,
    language   TEXT,
    content    TEXT    NOT NULL,
    fetched_at TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('lrclib','embedded','manual'))
) STRICT;
CREATE UNIQUE INDEX lyrics_track_source_key ON lyrics (track_id, source);

CREATE TABLE cover_art (
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
CREATE INDEX cover_art_release_idx       ON cover_art (release_id)       WHERE release_id       IS NOT NULL;
CREATE INDEX cover_art_release_group_idx ON cover_art (release_group_id) WHERE release_group_id IS NOT NULL;
CREATE INDEX cover_art_artist_idx        ON cover_art (artist_id)        WHERE artist_id        IS NOT NULL;


-- +goose Down

DROP TABLE IF EXISTS cover_art;
DROP TABLE IF EXISTS lyrics;
DROP TABLE IF EXISTS track_files;
DROP TABLE IF EXISTS track_artists;
DROP TABLE IF EXISTS release_tracks;
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS release_artists;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS release_groups;
DROP TABLE IF EXISTS artists;
