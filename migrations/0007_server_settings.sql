-- +goose Up
CREATE TABLE server_settings (
    id                 INTEGER PRIMARY KEY CHECK (id = 1),
    art_thumb_cache_mb INTEGER NOT NULL DEFAULT 500 CHECK (art_thumb_cache_mb >= 50),
    lastfm_api_key     BLOB,
    lastfm_secret      BLOB,
    fanart_api_key     BLOB,
    acoustid_api_key   BLOB,
    discogs_token      BLOB,
    updated_at         TEXT    NOT NULL DEFAULT CURRENT_TIMESTAMP
) STRICT;

INSERT INTO server_settings (id) VALUES (1);

-- +goose Down
DROP TABLE server_settings;
