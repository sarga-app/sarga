package scanner

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

type gcCounts struct {
	releases      int64
	releaseGroups int64
	artists       int64
}

func pruneOrphans(ctx context.Context, db *sql.DB) (gcCounts, error) {
	var c gcCounts
	start := time.Now()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return c, err
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.ExecContext(ctx, `
		DELETE FROM releases
		WHERE NOT EXISTS (SELECT 1 FROM release_tracks WHERE release_id = releases.id)`)
	if err != nil {
		return c, err
	}
	c.releases, _ = res.RowsAffected() //nolint:errcheck // RowsAffected never errors for sqlite

	res, err = tx.ExecContext(ctx, `
		DELETE FROM release_groups
		WHERE NOT EXISTS (SELECT 1 FROM releases WHERE release_group_id = release_groups.id)`)
	if err != nil {
		return c, err
	}
	c.releaseGroups, _ = res.RowsAffected() //nolint:errcheck

	res, err = tx.ExecContext(ctx, `
		DELETE FROM artists
		WHERE NOT EXISTS (SELECT 1 FROM release_artists WHERE artist_id = artists.id)
		  AND NOT EXISTS (SELECT 1 FROM track_artists   WHERE artist_id = artists.id)`)
	if err != nil {
		return c, err
	}
	c.artists, _ = res.RowsAffected() //nolint:errcheck

	if err := tx.Commit(); err != nil {
		return c, err
	}

	slog.Debug("scan: gc",
		"releases", c.releases,
		"release_groups", c.releaseGroups,
		"artists", c.artists,
		"elapsed", time.Since(start).Round(time.Millisecond))
	return c, nil
}
