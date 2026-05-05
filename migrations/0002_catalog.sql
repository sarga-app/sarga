-- +goose Up

CREATE TABLE artists (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    mbid        UUID,
    name        TEXT        NOT NULL,
    sort_name   TEXT        NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX artists_mbid_key   ON artists (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        artists_sort_idx   ON artists (sort_name);
CREATE TRIGGER artists_updated_at BEFORE UPDATE ON artists FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE release_groups (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    mbid                UUID,
    title               TEXT        NOT NULL,
    primary_type        TEXT        NOT NULL DEFAULT 'album',
    first_release_year  INT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT release_groups_primary_type_check CHECK (
        primary_type IN ('album','ep','single','compilation','soundtrack','live','remix','other')
    )
);
CREATE UNIQUE INDEX release_groups_mbid_key ON release_groups (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        release_groups_title_idx ON release_groups (title);
CREATE TRIGGER release_groups_updated_at BEFORE UPDATE ON release_groups FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE releases (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    mbid              UUID,
    release_group_id  UUID        NOT NULL REFERENCES release_groups(id) ON DELETE CASCADE,
    title             TEXT        NOT NULL,
    country           TEXT,
    release_date      DATE,
    label_name        TEXT,
    barcode           TEXT,
    edition_notes     TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX releases_mbid_key             ON releases (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        releases_release_group_idx    ON releases (release_group_id);
CREATE TRIGGER releases_updated_at BEFORE UPDATE ON releases FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE release_artists (
    release_id UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    artist_id  UUID NOT NULL REFERENCES artists(id)  ON DELETE RESTRICT,
    position   INT  NOT NULL,
    role       TEXT NOT NULL DEFAULT 'primary',
    PRIMARY KEY (release_id, artist_id, role),
    CONSTRAINT release_artists_role_check CHECK (role IN ('primary','feature','remixer','producer','other'))
);
CREATE INDEX release_artists_artist_idx ON release_artists (artist_id);

CREATE TABLE tracks (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    mbid        UUID,
    title       TEXT        NOT NULL,
    duration_ms INT,
    isrc        TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tracks_mbid_key ON tracks (mbid) WHERE mbid IS NOT NULL;
CREATE INDEX        tracks_title_idx ON tracks (title);
CREATE TRIGGER tracks_updated_at BEFORE UPDATE ON tracks FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE release_tracks (
    release_id  UUID NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    track_id    UUID NOT NULL REFERENCES tracks(id)   ON DELETE RESTRICT,
    disc_number INT  NOT NULL DEFAULT 1,
    position    INT  NOT NULL,
    PRIMARY KEY (release_id, disc_number, position)
);
CREATE INDEX release_tracks_track_idx ON release_tracks (track_id);

CREATE TABLE track_artists (
    track_id  UUID NOT NULL REFERENCES tracks(id)  ON DELETE CASCADE,
    artist_id UUID NOT NULL REFERENCES artists(id) ON DELETE RESTRICT,
    position  INT  NOT NULL,
    role      TEXT NOT NULL DEFAULT 'primary',
    PRIMARY KEY (track_id, artist_id, role),
    CONSTRAINT track_artists_role_check CHECK (role IN ('primary','feature','remixer','producer','other'))
);
CREATE INDEX track_artists_artist_idx ON track_artists (artist_id);

CREATE TABLE track_files (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id        UUID        NOT NULL REFERENCES tracks(id)    ON DELETE CASCADE,
    library_id      UUID        NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    relative_path   TEXT        NOT NULL,
    format          TEXT        NOT NULL,
    codec           TEXT        NOT NULL,
    bitrate_kbps    INT,
    sample_rate_hz  INT,
    bit_depth       INT,
    channels        INT         NOT NULL DEFAULT 2,
    file_size_bytes BIGINT      NOT NULL,
    mtime           TIMESTAMPTZ NOT NULL,
    content_hash    BYTEA       NOT NULL,
    source_kind     TEXT        NOT NULL DEFAULT 'unknown',
    is_lossless     BOOLEAN     NOT NULL DEFAULT FALSE,
    is_preferred    BOOLEAN     NOT NULL DEFAULT FALSE,
    added_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT track_files_source_kind_check CHECK (
        source_kind IN ('unknown','rip','purchase','import','manual')
    ),
    CONSTRAINT track_files_content_hash_len CHECK (octet_length(content_hash) = 16)
);
CREATE UNIQUE INDEX track_files_library_path_key ON track_files (library_id, relative_path);
CREATE INDEX        track_files_track_idx        ON track_files (track_id);
CREATE INDEX        track_files_content_hash_idx ON track_files (content_hash);
CREATE UNIQUE INDEX track_files_preferred_per_track_key
    ON track_files (track_id) WHERE is_preferred;
CREATE TRIGGER track_files_updated_at BEFORE UPDATE ON track_files FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE lyrics (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    track_id    UUID        NOT NULL REFERENCES tracks(id) ON DELETE CASCADE,
    source      TEXT        NOT NULL,
    is_synced   BOOLEAN     NOT NULL DEFAULT FALSE,
    language    TEXT,
    content     TEXT        NOT NULL,
    fetched_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT lyrics_source_check CHECK (source IN ('lrclib','embedded','manual'))
);
CREATE UNIQUE INDEX lyrics_track_source_key ON lyrics (track_id, source);

CREATE TABLE cover_art (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id        UUID        REFERENCES releases(id)       ON DELETE CASCADE,
    release_group_id  UUID        REFERENCES release_groups(id) ON DELETE CASCADE,
    artist_id         UUID        REFERENCES artists(id)        ON DELETE CASCADE,
    source_kind       TEXT        NOT NULL,
    uri               TEXT        NOT NULL,
    width             INT,
    height            INT,
    content_hash      BYTEA,
    is_preferred      BOOLEAN     NOT NULL DEFAULT FALSE,
    fetched_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT cover_art_scope_check CHECK (
        num_nonnulls(release_id, release_group_id, artist_id) = 1
    ),
    CONSTRAINT cover_art_source_kind_check CHECK (
        source_kind IN ('caa','folder','embedded','manual')
    )
);
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
