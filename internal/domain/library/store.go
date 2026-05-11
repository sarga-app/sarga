package library

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/sarga-app/sarga/internal/support/db/queries"
)

type Store interface {
	ListLibraryPaths(ctx context.Context, libraryID string) ([]queries.LibraryPath, error)
	GetLibrarySettings(ctx context.Context, libraryID string) (queries.LibrarySetting, error)

	ListTrackFileSnapshots(ctx context.Context, libraryID string) ([]queries.ListTrackFileSnapshotsRow, error)
	ListTrackFilesNullDuration(ctx context.Context, libraryID string) ([]queries.ListTrackFilesNullDurationRow, error)

	GetArtistByMBID(ctx context.Context, mbid *string) (queries.GetArtistByMBIDRow, error)
	GetArtistByName(ctx context.Context, name string) (queries.GetArtistByNameRow, error)
	InsertArtist(ctx context.Context, arg queries.InsertArtistParams) (queries.InsertArtistRow, error)
	UpdateArtistCore(ctx context.Context, arg queries.UpdateArtistCoreParams) error
	UpdateArtistEnrichment(ctx context.Context, arg queries.UpdateArtistEnrichmentParams) error

	GetReleaseGroupByMBID(ctx context.Context, mbid *string) (queries.ReleaseGroup, error)
	GetReleaseGroupByTitle(ctx context.Context, title string) (queries.ReleaseGroup, error)
	InsertReleaseGroup(ctx context.Context, arg queries.InsertReleaseGroupParams) (queries.ReleaseGroup, error)
	UpdateReleaseGroupCore(ctx context.Context, arg queries.UpdateReleaseGroupCoreParams) error

	GetReleaseByMBID(ctx context.Context, mbid *string) (queries.GetReleaseByMBIDRow, error)
	GetReleaseByGroupAndTitle(ctx context.Context, arg queries.GetReleaseByGroupAndTitleParams) (queries.GetReleaseByGroupAndTitleRow, error)
	InsertRelease(ctx context.Context, arg queries.InsertReleaseParams) (queries.InsertReleaseRow, error)
	UpdateReleaseCore(ctx context.Context, arg queries.UpdateReleaseCoreParams) error
	UpdateReleaseEnrichment(ctx context.Context, arg queries.UpdateReleaseEnrichmentParams) error

	GetTrackByMBID(ctx context.Context, mbid *string) (queries.GetTrackByMBIDRow, error)
	InsertTrack(ctx context.Context, arg queries.InsertTrackParams) (queries.InsertTrackRow, error)
	UpdateTrackCore(ctx context.Context, arg queries.UpdateTrackCoreParams) error
	UpdateTrackDurationIfNull(ctx context.Context, arg queries.UpdateTrackDurationIfNullParams) error

	LinkReleaseArtist(ctx context.Context, arg queries.LinkReleaseArtistParams) error
	LinkTrackArtist(ctx context.Context, arg queries.LinkTrackArtistParams) error
	UpsertReleaseTrack(ctx context.Context, arg queries.UpsertReleaseTrackParams) error

	GetTrackFileByPath(ctx context.Context, arg queries.GetTrackFileByPathParams) (queries.GetTrackFileByPathRow, error)
	GetTrackFileByID(ctx context.Context, id string) (queries.GetTrackFileByIDRow, error)
	GetEnrichmentTrackInfo(ctx context.Context, id string) (queries.GetEnrichmentTrackInfoRow, error)
	InsertTrackFile(ctx context.Context, arg queries.InsertTrackFileParams) (queries.InsertTrackFileRow, error)
	UpdateTrackFileCore(ctx context.Context, arg queries.UpdateTrackFileCoreParams) error
	UpdateTrackFileMatch(ctx context.Context, arg queries.UpdateTrackFileMatchParams) error
	RestoreTrackFile(ctx context.Context, id string) error

	UpsertCoverArt(ctx context.Context, arg queries.UpsertCoverArtParams) (queries.CoverArt, error)
	UpsertLyrics(ctx context.Context, arg queries.UpsertLyricsParams) error

	InsertScanRun(ctx context.Context, arg queries.InsertScanRunParams) (queries.InsertScanRunRow, error)
	UpdateScanRunProgress(ctx context.Context, arg queries.UpdateScanRunProgressParams) error
	FinishScanRun(ctx context.Context, arg queries.FinishScanRunParams) error
	GetActiveScanRun(ctx context.Context, libraryID string) (queries.GetActiveScanRunRow, error)

	EnqueueTrackFile(ctx context.Context, arg queries.EnqueueTrackFileParams) error
	ClaimEnrichmentBatch(ctx context.Context, limit int64) ([]queries.EnrichmentQueue, error)
	MarkEnrichmentDone(ctx context.Context, id string) error
	MarkEnrichmentFailed(ctx context.Context, arg queries.MarkEnrichmentFailedParams) error
	ResetStuckEnrichments(ctx context.Context) error
}

// MarkMissingTrackFiles marks track_files for the given library as missing when
// their absolute path is not in foundPaths. Uses raw SQL — sqlc cannot expand a
// variable-length IN clause for SQLite.
func MarkMissingTrackFiles(ctx context.Context, db *sql.DB, libraryID string, foundPaths []string) error {
	if len(foundPaths) == 0 {
		_, err := db.ExecContext(ctx,
			`UPDATE track_files SET missing_since = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
             WHERE library_id = ? AND missing_since IS NULL`, libraryID)
		return err
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
	_, err := db.ExecContext(ctx, query, args...)
	return err
}
