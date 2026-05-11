-- Scanner snapshot (for reconciler change detection) --------------------

-- name: ListTrackFileSnapshots :many
SELECT id, relative_path, mtime, file_size_bytes
FROM track_files
WHERE library_id = ?;

-- name: ListTrackFilesNullDuration :many
SELECT tf.track_id, tf.relative_path
FROM track_files tf
JOIN tracks t ON t.id = tf.track_id
WHERE tf.library_id = ? AND t.duration_ms IS NULL;

-- Releases (albums) -------------------------------------------------------

-- name: ListReleasesByDate :many
SELECT
    r.id, r.title, r.release_date, r.edition_notes, r.label_name,
    rg.id AS release_group_id, rg.primary_type, rg.first_release_year,
    (SELECT a.id FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_id,
    (SELECT a.name FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_name,
    (SELECT a.is_various_artists FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS is_various_artists,
    (SELECT uri FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_uri
FROM releases r
JOIN release_groups rg ON rg.id = r.release_group_id
WHERE EXISTS (
    SELECT 1 FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE rt.release_id = r.id AND tf.library_id = ?
)
ORDER BY rg.first_release_year DESC, r.title
LIMIT ? OFFSET ?;

-- name: ListReleasesByTitle :many
SELECT
    r.id, r.title, r.release_date, r.edition_notes, r.label_name,
    rg.id AS release_group_id, rg.primary_type, rg.first_release_year,
    (SELECT a.id FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_id,
    (SELECT a.name FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_name,
    (SELECT a.is_various_artists FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS is_various_artists,
    (SELECT uri FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_uri
FROM releases r
JOIN release_groups rg ON rg.id = r.release_group_id
WHERE EXISTS (
    SELECT 1 FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE rt.release_id = r.id AND tf.library_id = ?
)
ORDER BY r.title, rg.first_release_year
LIMIT ? OFFSET ?;

-- name: ListReleasesByAdded :many
SELECT
    r.id, r.title, r.release_date, r.edition_notes, r.label_name,
    rg.id AS release_group_id, rg.primary_type, rg.first_release_year,
    (SELECT a.id FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_id,
    (SELECT a.name FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS primary_artist_name,
    (SELECT a.is_various_artists FROM artists a JOIN release_artists ra ON ra.artist_id = a.id WHERE ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1 LIMIT 1) AS is_various_artists,
    (SELECT uri FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_uri
FROM releases r
JOIN release_groups rg ON rg.id = r.release_group_id
JOIN (
    SELECT rt.release_id, MAX(tf.added_at) AS max_added
    FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE tf.library_id = ?
    GROUP BY rt.release_id
) latest ON latest.release_id = r.id
ORDER BY latest.max_added DESC, r.title
LIMIT ? OFFSET ?;

-- name: CountReleases :one
SELECT COUNT(DISTINCT r.id)
FROM releases r
WHERE EXISTS (
    SELECT 1 FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE rt.release_id = r.id AND tf.library_id = ?
);

-- name: ListReleasesByArtistByDate :many
SELECT
    r.id, r.title, r.release_date, r.edition_notes, r.label_name,
    rg.id AS release_group_id, rg.primary_type, rg.first_release_year,
    COALESCE(a.id, '') AS primary_artist_id, COALESCE(a.name, '') AS primary_artist_name, COALESCE(a.is_various_artists, 0) AS is_various_artists,
    (SELECT COALESCE(uri, '') FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_uri
FROM releases r
JOIN release_groups rg ON rg.id = r.release_group_id
JOIN release_artists ra_filter ON ra_filter.release_id = r.id AND ra_filter.artist_id = ?
LEFT JOIN release_artists ra ON ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1
LEFT JOIN artists a ON a.id = ra.artist_id
WHERE EXISTS (
    SELECT 1 FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE rt.release_id = r.id AND tf.library_id = ?
)
ORDER BY rg.first_release_year DESC, r.title
LIMIT ? OFFSET ?;

-- name: CountReleasesByArtist :one
SELECT COUNT(DISTINCT r.id)
FROM releases r
JOIN release_artists ra ON ra.release_id = r.id AND ra.artist_id = ?
WHERE EXISTS (
    SELECT 1 FROM release_tracks rt
    JOIN track_files tf ON tf.track_id = rt.track_id
    WHERE rt.release_id = r.id AND tf.library_id = ?
);

-- name: GetReleaseByID :one
SELECT
    r.id, r.mbid, r.title, r.release_date, r.edition_notes, r.label_name,
    r.barcode, r.catalog_number, r.total_discs,
    rg.id AS release_group_id, rg.mbid AS release_group_mbid,
    rg.primary_type, rg.first_release_year,
    (SELECT uri FROM cover_art WHERE release_id = r.id AND is_preferred = 1 LIMIT 1) AS cover_art_uri
FROM releases r
JOIN release_groups rg ON rg.id = r.release_group_id
WHERE r.id = ?;

-- name: ListReleaseArtists :many
SELECT a.id, a.name, a.sort_name, a.is_various_artists, ra.role, ra.position
FROM release_artists ra
JOIN artists a ON a.id = ra.artist_id
WHERE ra.release_id = ?
ORDER BY ra.position;

-- name: ListReleaseTracks :many
SELECT
    rt.disc_number, rt.position,
    t.id AS track_id, t.mbid AS track_mbid, t.title, t.duration_ms, t.isrc,
    t.bpm, t.key_sig,
    tf.id AS track_file_id, tf.format, tf.codec, tf.bitrate_kbps,
    tf.sample_rate_hz, tf.bit_depth, tf.is_lossless, tf.is_preferred,
    tf.match_confidence, tf.match_source, tf.missing_since
FROM release_tracks rt
JOIN tracks t ON t.id = rt.track_id
LEFT JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ? AND tf.is_preferred = 1
WHERE rt.release_id = ?
ORDER BY rt.disc_number, rt.position
LIMIT ? OFFSET ?;

-- name: CountReleaseTracks :one
SELECT COUNT(*) FROM release_tracks WHERE release_id = ?;

-- name: ListTrackFilesForTrack :many
SELECT
    id, format, codec, bitrate_kbps, sample_rate_hz, bit_depth,
    is_lossless, is_preferred, match_confidence, match_source, missing_since
FROM track_files
WHERE track_id = ? AND library_id = ?
ORDER BY is_preferred DESC, is_lossless DESC, bitrate_kbps DESC;

-- Artists -----------------------------------------------------------------

-- name: ListArtists :many
SELECT
    a.id, a.mbid, a.name, a.sort_name, a.country, a.bio,
    a.begin_year, a.end_year, a.is_various_artists,
    COALESCE((SELECT uri FROM cover_art WHERE artist_id = a.id AND is_preferred = 1 LIMIT 1), '') AS image_uri,
    COUNT(DISTINCT ra.release_id) AS release_count
FROM artists a
LEFT JOIN release_artists ra ON ra.artist_id = a.id
WHERE a.is_various_artists = 0
  AND EXISTS (
      SELECT 1 FROM release_artists ra2
      JOIN release_tracks rt ON rt.release_id = ra2.release_id
      JOIN track_files tf ON tf.track_id = rt.track_id
      WHERE ra2.artist_id = a.id AND tf.library_id = ?
  )
GROUP BY a.id
ORDER BY a.sort_name
LIMIT ? OFFSET ?;

-- name: CountArtists :one
SELECT COUNT(DISTINCT a.id)
FROM artists a
WHERE a.is_various_artists = 0
  AND EXISTS (
      SELECT 1 FROM release_artists ra2
      JOIN release_tracks rt ON rt.release_id = ra2.release_id
      JOIN track_files tf ON tf.track_id = rt.track_id
      WHERE ra2.artist_id = a.id AND tf.library_id = ?
  );

-- name: GetArtistByID :one
SELECT
    a.id, a.mbid, a.name, a.sort_name, a.country, a.bio,
    a.begin_year, a.end_year, a.is_various_artists, a.discogs_id, a.tadb_id,
    COALESCE((SELECT uri FROM cover_art WHERE artist_id = a.id AND is_preferred = 1 LIMIT 1), '') AS image_uri
FROM artists a
WHERE a.id = ?;

-- Tracks ------------------------------------------------------------------

-- name: ListTracks :many
SELECT
    t.id, t.mbid, t.title, t.duration_ms, t.isrc, t.bpm, t.key_sig,
    tf.id AS track_file_id, tf.library_id, tf.format, tf.codec,
    tf.bitrate_kbps, tf.is_lossless, tf.is_preferred,
    tf.match_confidence, tf.match_source, tf.missing_since
FROM tracks t
JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ? AND tf.is_preferred = 1
ORDER BY t.title
LIMIT ? OFFSET ?;

-- name: CountTracks :one
SELECT COUNT(DISTINCT t.id)
FROM tracks t
JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ?;

-- name: ListTracksByRelease :many
SELECT
    rt.disc_number, rt.position,
    t.id, t.mbid, t.title, t.duration_ms, t.isrc, t.bpm, t.key_sig,
    tf.id AS track_file_id, tf.format, tf.codec, tf.bitrate_kbps,
    tf.is_lossless, tf.is_preferred, tf.match_confidence, tf.match_source
FROM release_tracks rt
JOIN tracks t ON t.id = rt.track_id
LEFT JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ? AND tf.is_preferred = 1
WHERE rt.release_id = ?
ORDER BY rt.disc_number, rt.position;

-- Playlists ---------------------------------------------------------------

-- name: ListPlaylists :many
SELECT
    p.id, p.name, p.description, p.is_public, p.owner_id,
    p.created_at, p.updated_at,
    COUNT(pt.track_id) AS track_count
FROM playlists p
LEFT JOIN playlist_tracks pt ON pt.playlist_id = p.id
WHERE EXISTS (
    SELECT 1 FROM playlist_tracks pt2
    JOIN track_files tf ON tf.track_id = pt2.track_id
    WHERE pt2.playlist_id = p.id AND tf.library_id = ?
)
GROUP BY p.id
ORDER BY p.name
LIMIT ? OFFSET ?;

-- name: CountPlaylists :one
SELECT COUNT(DISTINCT p.id)
FROM playlists p
WHERE EXISTS (
    SELECT 1 FROM playlist_tracks pt
    JOIN track_files tf ON tf.track_id = pt.track_id
    WHERE pt.playlist_id = p.id AND tf.library_id = ?
);

-- name: GetPlaylistByID :one
SELECT
    p.id, p.name, p.description, p.is_public, p.owner_id,
    p.created_at, p.updated_at,
    COUNT(pt.track_id) AS track_count
FROM playlists p
LEFT JOIN playlist_tracks pt ON pt.playlist_id = p.id
WHERE p.id = ?
GROUP BY p.id;

-- name: ListPlaylistTracks :many
SELECT
    pt.position,
    t.id AS track_id, t.title, t.duration_ms,
    tf.format, tf.codec, tf.is_lossless, tf.is_preferred
FROM playlist_tracks pt
JOIN tracks t ON t.id = pt.track_id
LEFT JOIN track_files tf ON tf.track_id = t.id AND tf.library_id = ? AND tf.is_preferred = 1
WHERE pt.playlist_id = ?
ORDER BY pt.position
LIMIT ? OFFSET ?;

-- name: CountPlaylistTracks :one
SELECT COUNT(*) FROM playlist_tracks WHERE playlist_id = ?;

-- Enricher lookups -------------------------------------------------------

-- name: GetTrackFileByID :one
SELECT
    id, track_id, library_id, relative_path, format, codec,
    bitrate_kbps, sample_rate_hz, bit_depth, channels,
    file_size_bytes, mtime, content_hash, source_kind,
    is_lossless, is_preferred, match_confidence, match_source,
    missing_since, rg_track_gain, rg_track_peak, rg_album_gain, rg_album_peak,
    added_at, updated_at
FROM track_files
WHERE id = ?;

-- name: GetEnrichmentTrackInfo :one
-- Returns the denormalized info needed to run the enrichment pipeline
-- for a single track file: track metadata + primary artist + primary release.
SELECT
    tf.id        AS track_file_id,
    tf.relative_path,
    tf.library_id,
    tf.match_source,
    t.id         AS track_id,
    t.title      AS track_title,
    t.duration_ms,
    t.mbid       AS track_mbid,
    t.isrc,
    a.id         AS artist_id,
    a.name       AS artist_name,
    a.mbid       AS artist_mbid,
    r.id         AS release_id,
    r.title      AS release_title,
    r.mbid       AS release_mbid,
    rg.id        AS release_group_id,
    rg.mbid      AS release_group_mbid
FROM track_files tf
JOIN tracks t ON t.id = tf.track_id
LEFT JOIN release_tracks rt ON rt.track_id = t.id
LEFT JOIN releases r ON r.id = rt.release_id
LEFT JOIN release_groups rg ON rg.id = r.release_group_id
LEFT JOIN release_artists ra ON ra.release_id = r.id AND ra.role = 'primary' AND ra.position = 1
LEFT JOIN artists a ON a.id = ra.artist_id
WHERE tf.id = ?
LIMIT 1;

-- Library stats (for sidebar) ---------------------------------------------
-- NOTE: FTS5 search queries (SearchReleases, SearchArtists, SearchTracks)
-- are executed as raw SQL in the catalog handler — sqlc cannot parse FTS5
-- virtual table syntax (rank column, table MATCH expression).

-- name: GetLibraryStats :one
SELECT
    COUNT(DISTINCT r.id)  AS release_count,
    COUNT(DISTINCT a.id)  AS artist_count,
    COUNT(DISTINCT t.id)  AS track_count
FROM track_files tf
JOIN tracks t ON t.id = tf.track_id
JOIN release_tracks rt ON rt.track_id = t.id
JOIN releases r ON r.id = rt.release_id
JOIN release_artists ra ON ra.release_id = r.id AND ra.role = 'primary'
JOIN artists a ON a.id = ra.artist_id
WHERE tf.library_id = ?;
