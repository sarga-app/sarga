-- +goose Up

CREATE TABLE playlists (
    id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT        NOT NULL,
    description TEXT,
    is_public   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX        playlists_owner_idx ON playlists (owner_id);
CREATE UNIQUE INDEX playlists_owner_name_key ON playlists (owner_id, lower(name));
CREATE TRIGGER playlists_updated_at BEFORE UPDATE ON playlists FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE playlist_tracks (
    playlist_id UUID        NOT NULL REFERENCES playlists(id) ON DELETE CASCADE,
    position    INT         NOT NULL,
    track_id    UUID        NOT NULL REFERENCES tracks(id)    ON DELETE CASCADE,
    added_by_id UUID                 REFERENCES users(id)     ON DELETE SET NULL,
    added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (playlist_id, position)
);
CREATE INDEX playlist_tracks_track_idx ON playlist_tracks (track_id);

CREATE TABLE listening_events (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id              UUID        NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    track_id             UUID        NOT NULL REFERENCES tracks(id)      ON DELETE RESTRICT,
    track_file_id        UUID                 REFERENCES track_files(id) ON DELETE SET NULL,
    played_at            TIMESTAMPTZ NOT NULL,
    duration_listened_ms INT         NOT NULL,
    completed            BOOLEAN     NOT NULL DEFAULT FALSE,
    source               TEXT        NOT NULL,
    client_name          TEXT,
    client_version       TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT listening_events_source_check CHECK (
        source IN ('web','subsonic','spotify_import','apple_music_import','manual_import')
    ),
    CONSTRAINT listening_events_duration_nonneg CHECK (duration_listened_ms >= 0)
);
CREATE INDEX listening_events_user_played_idx  ON listening_events (user_id, played_at DESC);
CREATE INDEX listening_events_track_idx        ON listening_events (track_id);
CREATE INDEX listening_events_played_at_idx    ON listening_events (played_at DESC);

CREATE TABLE scrobble_targets (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind          TEXT        NOT NULL,
    session_key   TEXT        NOT NULL,
    enabled       BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT scrobble_targets_kind_check CHECK (kind IN ('lastfm','listenbrainz'))
);
CREATE UNIQUE INDEX scrobble_targets_user_kind_key ON scrobble_targets (user_id, kind);
CREATE TRIGGER scrobble_targets_updated_at BEFORE UPDATE ON scrobble_targets FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE scrobble_outbox (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    listening_event_id   UUID        NOT NULL REFERENCES listening_events(id) ON DELETE CASCADE,
    scrobble_target_id   UUID        NOT NULL REFERENCES scrobble_targets(id) ON DELETE CASCADE,
    status               TEXT        NOT NULL DEFAULT 'pending',
    attempt_count        INT         NOT NULL DEFAULT 0,
    last_attempted_at    TIMESTAMPTZ,
    last_error           TEXT,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at              TIMESTAMPTZ,
    CONSTRAINT scrobble_outbox_status_check CHECK (status IN ('pending','sent','failed','permanent_failure'))
);
CREATE UNIQUE INDEX scrobble_outbox_event_target_key ON scrobble_outbox (listening_event_id, scrobble_target_id);
CREATE INDEX        scrobble_outbox_pending_idx
    ON scrobble_outbox (created_at) WHERE status = 'pending';

CREATE TABLE scan_runs (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    library_id      UUID        NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    started_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at     TIMESTAMPTZ,
    status          TEXT        NOT NULL DEFAULT 'running',
    files_scanned   INT         NOT NULL DEFAULT 0,
    files_added     INT         NOT NULL DEFAULT 0,
    files_updated   INT         NOT NULL DEFAULT 0,
    files_removed   INT         NOT NULL DEFAULT 0,
    error           TEXT,
    CONSTRAINT scan_runs_status_check CHECK (status IN ('running','completed','failed','cancelled'))
);
CREATE INDEX scan_runs_library_started_idx ON scan_runs (library_id, started_at DESC);
CREATE INDEX scan_runs_running_idx         ON scan_runs (library_id) WHERE status = 'running';


-- +goose Down

DROP TABLE IF EXISTS scan_runs;
DROP TABLE IF EXISTS scrobble_outbox;
DROP TABLE IF EXISTS scrobble_targets;
DROP TABLE IF EXISTS listening_events;
DROP TABLE IF EXISTS playlist_tracks;
DROP TABLE IF EXISTS playlists;
