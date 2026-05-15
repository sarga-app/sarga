package scanner

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type pruneCounts struct {
	releases      int64
	releaseGroups int64
	artists       int64
}

func pruneOrphans(ctx context.Context, db *sql.DB) (pruneCounts, error) {
	var c pruneCounts
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

	elapsed := time.Since(start).Round(time.Millisecond)
	if c.releases+c.releaseGroups+c.artists == 0 {
		slog.Debug("scan: prune no orphans", "elapsed", elapsed)
	} else {
		slog.Info("scan: pruned orphans",
			"releases", c.releases,
			"release_groups", c.releaseGroups,
			"artists", c.artists,
			"elapsed", elapsed)
	}
	return c, nil
}

func markMissingTrackFiles(ctx context.Context, sqlDB *sql.DB, libraryID string, foundPaths []string) (int64, error) {
	if len(foundPaths) == 0 {
		res, err := sqlDB.ExecContext(ctx,
			`UPDATE track_files SET missing_since = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
             WHERE library_id = ? AND missing_since IS NULL`, libraryID)
		if err != nil {
			return 0, err
		}
		n, _ := res.RowsAffected() //nolint:errcheck // RowsAffected never errors for sqlite
		return n, nil
	}
	placeholders := strings.Repeat("?,", len(foundPaths))
	placeholders = placeholders[:len(placeholders)-1]
	query := fmt.Sprintf( //nolint:gosec // placeholders is "?,?,?" — no user data in the format verb
		`UPDATE track_files SET missing_since = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
         WHERE library_id = ? AND relative_path NOT IN (%s) AND missing_since IS NULL`,
		placeholders,
	)
	args := make([]any, 0, len(foundPaths)+1)
	args = append(args, libraryID)
	for _, p := range foundPaths {
		args = append(args, p)
	}
	res, err := sqlDB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected() //nolint:errcheck
	return n, nil
}
