-- Artists -----------------------------------------------------------------

-- name: GetArtistByMBID :one
SELECT id, mbid, name, sort_name, country, bio, begin_year, end_year,
       is_various_artists, discogs_id, tadb_id, created_at, updated_at
FROM artists WHERE mbid = ? LIMIT 1;

-- name: GetArtistByName :one
SELECT id, mbid, name, sort_name, country, bio, begin_year, end_year,
       is_various_artists, discogs_id, tadb_id, created_at, updated_at
FROM artists WHERE name = ? LIMIT 1;

-- name: InsertArtist :one
INSERT INTO artists (id, mbid, name, sort_name, is_various_artists)
VALUES (?, ?, ?, ?, ?)
RETURNING id, mbid, name, sort_name, country, bio, begin_year, end_year,
          is_various_artists, discogs_id, tadb_id, created_at, updated_at;

-- name: UpdateArtistCore :exec
UPDATE artists SET
    name       = ?,
    sort_name  = ?,
    mbid       = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateArtistEnrichment :exec
UPDATE artists SET
    country    = ?,
    bio        = ?,
    begin_year = ?,
    end_year   = ?,
    discogs_id = ?,
    tadb_id    = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- Release groups ----------------------------------------------------------

-- name: GetReleaseGroupByMBID :one
SELECT id, mbid, title, primary_type, first_release_year, created_at, updated_at
FROM release_groups WHERE mbid = ? LIMIT 1;

-- name: GetReleaseGroupByTitle :one
SELECT id, mbid, title, primary_type, first_release_year, created_at, updated_at
FROM release_groups WHERE title = ? LIMIT 1;

-- name: InsertReleaseGroup :one
INSERT INTO release_groups (id, mbid, title, primary_type, first_release_year)
VALUES (?, ?, ?, ?, ?)
RETURNING id, mbid, title, primary_type, first_release_year, created_at, updated_at;

-- name: UpdateReleaseGroupCore :exec
UPDATE release_groups SET
    mbid               = ?,
    title              = ?,
    primary_type       = ?,
    first_release_year = ?,
    updated_at         = CURRENT_TIMESTAMP
WHERE id = ?;

-- Releases ----------------------------------------------------------------

-- name: GetReleaseByMBID :one
SELECT id, mbid, release_group_id, title, country, release_date, label_name,
       barcode, edition_notes, catalog_number, discogs_id, total_discs,
       created_at, updated_at
FROM releases WHERE mbid = ? LIMIT 1;

-- name: GetReleaseByGroupAndTitle :one
SELECT id, mbid, release_group_id, title, country, release_date, label_name,
       barcode, edition_notes, catalog_number, discogs_id, total_discs,
       created_at, updated_at
FROM releases WHERE release_group_id = ? AND title = ? LIMIT 1;

-- name: InsertRelease :one
INSERT INTO releases (id, mbid, release_group_id, title, country, release_date,
                      label_name, barcode, edition_notes, catalog_number, total_discs)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING id, mbid, release_group_id, title, country, release_date, label_name,
          barcode, edition_notes, catalog_number, discogs_id, total_discs,
          created_at, updated_at;

-- name: UpdateReleaseCore :exec
UPDATE releases SET
    mbid             = ?,
    title            = ?,
    country          = ?,
    release_date     = ?,
    label_name       = ?,
    barcode          = ?,
    edition_notes    = ?,
    catalog_number   = ?,
    total_discs      = ?,
    updated_at       = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateReleaseEnrichment :exec
UPDATE releases SET
    discogs_id = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- Tracks ------------------------------------------------------------------

-- name: GetTrackByMBID :one
SELECT id, mbid, title, duration_ms, isrc, bpm, key_sig, acoustid_id,
       created_at, updated_at
FROM tracks WHERE mbid = ? LIMIT 1;

-- name: InsertTrack :one
INSERT INTO tracks (id, mbid, title, duration_ms, isrc, bpm, key_sig)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, mbid, title, duration_ms, isrc, bpm, key_sig, acoustid_id,
          created_at, updated_at;

-- name: UpdateTrackCore :exec
UPDATE tracks SET
    mbid        = ?,
    title       = ?,
    duration_ms = ?,
    isrc        = ?,
    bpm         = ?,
    key_sig     = ?,
    acoustid_id = ?,
    updated_at  = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateTrackDurationIfNull :exec
UPDATE tracks SET duration_ms = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND duration_ms IS NULL;

-- Relation links ----------------------------------------------------------

-- name: LinkReleaseArtist :exec
INSERT INTO release_artists (release_id, artist_id, position, role)
VALUES (?, ?, ?, ?)
ON CONFLICT (release_id, artist_id, role) DO NOTHING;

-- name: LinkTrackArtist :exec
INSERT INTO track_artists (track_id, artist_id, position, role)
VALUES (?, ?, ?, ?)
ON CONFLICT (track_id, artist_id, role) DO NOTHING;

-- name: UpsertReleaseTrack :exec
INSERT INTO release_tracks (release_id, track_id, disc_number, position)
VALUES (?, ?, ?, ?)
ON CONFLICT (release_id, disc_number, position) DO UPDATE SET
    track_id = excluded.track_id;

-- Track files -------------------------------------------------------------

-- name: GetTrackFileByPath :one
SELECT
    id, track_id, library_id, relative_path, format, codec,
    bitrate_kbps, sample_rate_hz, bit_depth, channels,
    file_size_bytes, mtime, content_hash, source_kind,
    is_lossless, is_preferred, match_confidence, match_source,
    missing_since, rg_track_gain, rg_track_peak, rg_album_gain, rg_album_peak,
    added_at, updated_at
FROM track_files
WHERE library_id = ? AND relative_path = ?;

-- name: InsertTrackFile :one
INSERT INTO track_files (
    id, track_id, library_id, relative_path, format, codec,
    bitrate_kbps, sample_rate_hz, bit_depth, channels,
    file_size_bytes, mtime, content_hash, source_kind,
    is_lossless, is_preferred, match_confidence, match_source,
    rg_track_gain, rg_track_peak, rg_album_gain, rg_album_peak
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING
    id, track_id, library_id, relative_path, format, codec,
    bitrate_kbps, sample_rate_hz, bit_depth, channels,
    file_size_bytes, mtime, content_hash, source_kind,
    is_lossless, is_preferred, match_confidence, match_source,
    missing_since, rg_track_gain, rg_track_peak, rg_album_gain, rg_album_peak,
    added_at, updated_at;

-- name: FindMissingTrackFileByHash :one
SELECT id, track_id, relative_path
FROM track_files
WHERE library_id = ? AND content_hash = ? AND missing_since IS NOT NULL
LIMIT 1;

-- name: RepathTrackFile :exec
UPDATE track_files SET
    relative_path   = ?,
    file_size_bytes = ?,
    mtime           = ?,
    missing_since   = NULL,
    updated_at      = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateTrackFileCore :exec
UPDATE track_files SET
    track_id        = ?,
    format          = ?,
    codec           = ?,
    bitrate_kbps    = ?,
    sample_rate_hz  = ?,
    bit_depth       = ?,
    channels        = ?,
    file_size_bytes = ?,
    mtime           = ?,
    content_hash    = ?,
    is_lossless     = ?,
    rg_track_gain   = ?,
    rg_track_peak   = ?,
    rg_album_gain   = ?,
    rg_album_peak   = ?,
    missing_since   = NULL,
    updated_at      = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateTrackFileMatch :exec
UPDATE track_files SET
    match_confidence = ?,
    match_source     = ?,
    updated_at       = CURRENT_TIMESTAMP
WHERE id = ?;

-- NOTE: MarkTrackFilesMissing is executed as raw SQL in the scanner
-- (dynamic IN clause for variable-length ID list; sqlc cannot expand slices for SQLite).

-- name: RestoreTrackFile :exec
UPDATE track_files SET
    missing_since = NULL,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?;

-- Cover art + lyrics ------------------------------------------------------

-- name: UpsertCoverArt :one
INSERT INTO cover_art (id, release_id, release_group_id, artist_id,
                       source_kind, uri, width, height, content_hash, is_preferred)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    source_kind  = excluded.source_kind,
    uri          = excluded.uri,
    width        = excluded.width,
    height       = excluded.height,
    content_hash = excluded.content_hash,
    is_preferred = excluded.is_preferred,
    fetched_at   = CURRENT_TIMESTAMP
RETURNING id, release_id, release_group_id, artist_id,
          source_kind, uri, width, height, content_hash, is_preferred, fetched_at;

-- name: UpsertLyrics :exec
INSERT INTO lyrics (id, track_id, source, is_synced, language, content)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (track_id, source) DO UPDATE SET
    is_synced  = excluded.is_synced,
    language   = excluded.language,
    content    = excluded.content,
    fetched_at = CURRENT_TIMESTAMP;
