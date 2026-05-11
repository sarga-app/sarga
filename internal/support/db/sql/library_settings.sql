-- name: GetLibrarySettings :one
SELECT
    library_id, scan_on_add, fsnotify_enabled, scan_schedule,
    follow_symlinks, scan_playlist_files, file_extensions,
    mb_enrichment_enabled, acoustid_enabled, discogs_enabled,
    match_threshold, metadata_language,
    artist_provider_priority, album_provider_priority,
    album_art_source_priority, artist_image_source_priority,
    format_preference_order, created_at, updated_at
FROM library_settings
WHERE library_id = ?;

-- name: UpdateLibrarySettings :one
UPDATE library_settings SET
    scan_on_add              = ?,
    fsnotify_enabled         = ?,
    scan_schedule            = ?,
    follow_symlinks          = ?,
    scan_playlist_files      = ?,
    file_extensions          = ?,
    mb_enrichment_enabled    = ?,
    acoustid_enabled         = ?,
    discogs_enabled          = ?,
    match_threshold          = ?,
    metadata_language        = ?,
    artist_provider_priority     = ?,
    album_provider_priority      = ?,
    album_art_source_priority    = ?,
    artist_image_source_priority = ?,
    format_preference_order      = ?,
    updated_at               = CURRENT_TIMESTAMP
WHERE library_id = ?
RETURNING
    library_id, scan_on_add, fsnotify_enabled, scan_schedule,
    follow_symlinks, scan_playlist_files, file_extensions,
    mb_enrichment_enabled, acoustid_enabled, discogs_enabled,
    match_threshold, metadata_language,
    artist_provider_priority, album_provider_priority,
    album_art_source_priority, artist_image_source_priority,
    format_preference_order, created_at, updated_at;
