-- +goose Up

CREATE TABLE playlists (
    id          TEXT    NOT NULL PRIMARY KEY,
    owner_id    TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    description TEXT,
    is_public   INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;
CREATE INDEX        playlists_owner_idx      ON playlists (owner_id);
CREATE UNIQUE INDEX playlists_owner_name_key ON playlists (owner_id, lower(name));

CREATE TABLE playlist_tracks (
    playlist_id TEXT    NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    track_id    TEXT    NOT NULL REFERENCES tracks(id)    ON DELETE CASCADE,
    added_by_id TEXT             REFERENCES users(id)     ON DELETE SET NULL,
    added_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (playlist_id, position)
) STRICT, WITHOUT ROWID;
CREATE INDEX playlist_tracks_track_idx ON playlist_tracks (track_id);

CREATE TABLE listening_events (
    id                   TEXT    NOT NULL PRIMARY KEY,
    user_id              TEXT    NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    track_id             TEXT    NOT NULL REFERENCES tracks(id)      ON DELETE RESTRICT,
    track_file_id        TEXT             REFERENCES track_files(id) ON DELETE SET NULL,
    played_at            TEXT    NOT NULL,
    duration_listened_ms INTEGER NOT NULL,
    completed            INTEGER NOT NULL DEFAULT 0,
    source               TEXT    NOT NULL,
    client_name          TEXT,
    client_version       TEXT,
    created_at           TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (source IN ('web','subsonic','spotify_import','apple_music_import','manual_import')),
    CHECK (duration_listened_ms >= 0)
) STRICT;
CREATE INDEX listening_events_user_played_idx ON listening_events (user_id, played_at DESC);
CREATE INDEX listening_events_track_idx       ON listening_events (track_id);
CREATE INDEX listening_events_played_at_idx   ON listening_events (played_at DESC);

CREATE TABLE scrobble_targets (
    id          TEXT    NOT NULL PRIMARY KEY,
    user_id     TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind        TEXT    NOT NULL,
    session_key TEXT    NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (kind IN ('lastfm','listenbrainz'))
) STRICT;
CREATE UNIQUE INDEX scrobble_targets_user_kind_key ON scrobble_targets (user_id, kind);

CREATE TABLE scrobble_outbox (
    id                 TEXT    NOT NULL PRIMARY KEY,
    listening_event_id TEXT    NOT NULL REFERENCES listening_events(id) ON DELETE CASCADE,
    scrobble_target_id TEXT    NOT NULL REFERENCES scrobble_targets(id) ON DELETE CASCADE,
    status             TEXT    NOT NULL DEFAULT 'pending',
    attempt_count      INTEGER NOT NULL DEFAULT 0,
    last_attempted_at  TEXT,
    last_error         TEXT,
    created_at         TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at            TEXT,
    CHECK (status IN ('pending','sent','failed','permanent_failure'))
) STRICT;
CREATE UNIQUE INDEX scrobble_outbox_event_target_key ON scrobble_outbox (listening_event_id, scrobble_target_id);
CREATE INDEX        scrobble_outbox_pending_idx
    ON scrobble_outbox (created_at) WHERE status = 'pending';

CREATE TABLE scan_runs (
    id            TEXT    NOT NULL PRIMARY KEY,
    library_id    TEXT    NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    started_at    TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at   TEXT,
    status        TEXT    NOT NULL DEFAULT 'running',
    files_scanned INTEGER NOT NULL DEFAULT 0,
    files_added   INTEGER NOT NULL DEFAULT 0,
    files_updated INTEGER NOT NULL DEFAULT 0,
    files_removed INTEGER NOT NULL DEFAULT 0,
    error         TEXT,
    CHECK (status IN ('running','completed','failed','cancelled'))
) STRICT;
CREATE INDEX scan_runs_library_started_idx ON scan_runs (library_id, started_at DESC);
CREATE INDEX scan_runs_running_idx         ON scan_runs (library_id) WHERE status = 'running';


-- +goose Down

DROP TABLE IF EXISTS scan_runs;
DROP TABLE IF EXISTS scrobble_outbox;
DROP TABLE IF EXISTS scrobble_targets;
DROP TABLE IF EXISTS listening_events;
DROP TABLE IF EXISTS playlist_tracks;
DROP TABLE IF EXISTS playlists;
