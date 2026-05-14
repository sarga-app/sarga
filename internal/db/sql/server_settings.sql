-- name: GetServerSettings :one
SELECT
    id, art_thumb_cache_mb,
    lastfm_api_key, lastfm_secret, fanart_api_key, acoustid_api_key, discogs_token,
    updated_at
FROM server_settings
WHERE id = 1;

-- name: UpdateServerSettings :one
UPDATE server_settings SET
    art_thumb_cache_mb = ?,
    lastfm_api_key     = ?,
    lastfm_secret      = ?,
    fanart_api_key     = ?,
    acoustid_api_key   = ?,
    discogs_token      = ?,
    updated_at         = CURRENT_TIMESTAMP
WHERE id = 1
RETURNING
    id, art_thumb_cache_mb,
    lastfm_api_key, lastfm_secret, fanart_api_key, acoustid_api_key, discogs_token,
    updated_at;
