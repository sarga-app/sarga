package transport

import (
	"context"

	"github.com/sarga-app/sarga/internal/domain/library"
	"github.com/sarga-app/sarga/internal/support/db"
	"github.com/sarga-app/sarga/internal/support/db/queries"
)

type libraryStore struct {
	rq *queries.Queries
	wq *queries.Queries
}

func newLibraryStore(d *db.DB) library.Store {
	return &libraryStore{
		rq: queries.New(d.Read()),
		wq: queries.New(d.Write()),
	}
}

func (s *libraryStore) ListLibraryPaths(ctx context.Context, libraryID string) ([]queries.LibraryPath, error) {
	return s.rq.ListLibraryPaths(ctx, libraryID)
}

func (s *libraryStore) GetLibrarySettings(ctx context.Context, libraryID string) (queries.LibrarySetting, error) {
	return s.rq.GetLibrarySettings(ctx, libraryID)
}

func (s *libraryStore) ListTrackFileSnapshots(ctx context.Context, libraryID string) ([]queries.ListTrackFileSnapshotsRow, error) {
	return s.rq.ListTrackFileSnapshots(ctx, libraryID)
}

func (s *libraryStore) ListTrackFilesNullDuration(ctx context.Context, libraryID string) ([]queries.ListTrackFilesNullDurationRow, error) {
	return s.rq.ListTrackFilesNullDuration(ctx, libraryID)
}

func (s *libraryStore) GetArtistByMBID(ctx context.Context, mbid *string) (queries.GetArtistByMBIDRow, error) {
	return s.rq.GetArtistByMBID(ctx, mbid)
}

func (s *libraryStore) GetArtistByName(ctx context.Context, name string) (queries.GetArtistByNameRow, error) {
	return s.rq.GetArtistByName(ctx, name)
}

func (s *libraryStore) InsertArtist(ctx context.Context, arg queries.InsertArtistParams) (queries.InsertArtistRow, error) {
	return s.wq.InsertArtist(ctx, arg)
}

func (s *libraryStore) UpdateArtistCore(ctx context.Context, arg queries.UpdateArtistCoreParams) error {
	return s.wq.UpdateArtistCore(ctx, arg)
}

func (s *libraryStore) UpdateArtistEnrichment(ctx context.Context, arg queries.UpdateArtistEnrichmentParams) error {
	return s.wq.UpdateArtistEnrichment(ctx, arg)
}

func (s *libraryStore) GetReleaseGroupByMBID(ctx context.Context, mbid *string) (queries.ReleaseGroup, error) {
	return s.rq.GetReleaseGroupByMBID(ctx, mbid)
}

func (s *libraryStore) GetReleaseGroupByTitle(ctx context.Context, title string) (queries.ReleaseGroup, error) {
	return s.rq.GetReleaseGroupByTitle(ctx, title)
}

func (s *libraryStore) InsertReleaseGroup(ctx context.Context, arg queries.InsertReleaseGroupParams) (queries.ReleaseGroup, error) {
	return s.wq.InsertReleaseGroup(ctx, arg)
}

func (s *libraryStore) UpdateReleaseGroupCore(ctx context.Context, arg queries.UpdateReleaseGroupCoreParams) error {
	return s.wq.UpdateReleaseGroupCore(ctx, arg)
}

func (s *libraryStore) GetReleaseByMBID(ctx context.Context, mbid *string) (queries.GetReleaseByMBIDRow, error) {
	return s.rq.GetReleaseByMBID(ctx, mbid)
}

func (s *libraryStore) GetReleaseByGroupAndTitle(ctx context.Context, arg queries.GetReleaseByGroupAndTitleParams) (queries.GetReleaseByGroupAndTitleRow, error) {
	return s.rq.GetReleaseByGroupAndTitle(ctx, arg)
}

func (s *libraryStore) InsertRelease(ctx context.Context, arg queries.InsertReleaseParams) (queries.InsertReleaseRow, error) {
	return s.wq.InsertRelease(ctx, arg)
}

func (s *libraryStore) UpdateReleaseCore(ctx context.Context, arg queries.UpdateReleaseCoreParams) error {
	return s.wq.UpdateReleaseCore(ctx, arg)
}

func (s *libraryStore) UpdateReleaseEnrichment(ctx context.Context, arg queries.UpdateReleaseEnrichmentParams) error {
	return s.wq.UpdateReleaseEnrichment(ctx, arg)
}

func (s *libraryStore) GetTrackByMBID(ctx context.Context, mbid *string) (queries.GetTrackByMBIDRow, error) {
	return s.rq.GetTrackByMBID(ctx, mbid)
}

func (s *libraryStore) InsertTrack(ctx context.Context, arg queries.InsertTrackParams) (queries.InsertTrackRow, error) {
	return s.wq.InsertTrack(ctx, arg)
}

func (s *libraryStore) UpdateTrackCore(ctx context.Context, arg queries.UpdateTrackCoreParams) error {
	return s.wq.UpdateTrackCore(ctx, arg)
}

func (s *libraryStore) UpdateTrackDurationIfNull(ctx context.Context, arg queries.UpdateTrackDurationIfNullParams) error {
	return s.wq.UpdateTrackDurationIfNull(ctx, arg)
}

func (s *libraryStore) LinkReleaseArtist(ctx context.Context, arg queries.LinkReleaseArtistParams) error {
	return s.wq.LinkReleaseArtist(ctx, arg)
}

func (s *libraryStore) LinkTrackArtist(ctx context.Context, arg queries.LinkTrackArtistParams) error {
	return s.wq.LinkTrackArtist(ctx, arg)
}

func (s *libraryStore) UpsertReleaseTrack(ctx context.Context, arg queries.UpsertReleaseTrackParams) error {
	return s.wq.UpsertReleaseTrack(ctx, arg)
}

func (s *libraryStore) GetTrackFileByPath(ctx context.Context, arg queries.GetTrackFileByPathParams) (queries.GetTrackFileByPathRow, error) {
	return s.rq.GetTrackFileByPath(ctx, arg)
}

func (s *libraryStore) GetTrackFileByID(ctx context.Context, id string) (queries.GetTrackFileByIDRow, error) {
	return s.rq.GetTrackFileByID(ctx, id)
}

func (s *libraryStore) GetEnrichmentTrackInfo(ctx context.Context, id string) (queries.GetEnrichmentTrackInfoRow, error) {
	return s.rq.GetEnrichmentTrackInfo(ctx, id)
}

func (s *libraryStore) InsertTrackFile(ctx context.Context, arg queries.InsertTrackFileParams) (queries.InsertTrackFileRow, error) {
	return s.wq.InsertTrackFile(ctx, arg)
}

func (s *libraryStore) UpdateTrackFileCore(ctx context.Context, arg queries.UpdateTrackFileCoreParams) error {
	return s.wq.UpdateTrackFileCore(ctx, arg)
}

func (s *libraryStore) UpdateTrackFileMatch(ctx context.Context, arg queries.UpdateTrackFileMatchParams) error {
	return s.wq.UpdateTrackFileMatch(ctx, arg)
}

func (s *libraryStore) RestoreTrackFile(ctx context.Context, id string) error {
	return s.wq.RestoreTrackFile(ctx, id)
}

func (s *libraryStore) UpsertCoverArt(ctx context.Context, arg queries.UpsertCoverArtParams) (queries.CoverArt, error) {
	return s.wq.UpsertCoverArt(ctx, arg)
}

func (s *libraryStore) UpsertLyrics(ctx context.Context, arg queries.UpsertLyricsParams) error {
	return s.wq.UpsertLyrics(ctx, arg)
}

func (s *libraryStore) InsertScanRun(ctx context.Context, arg queries.InsertScanRunParams) (queries.InsertScanRunRow, error) {
	return s.wq.InsertScanRun(ctx, arg)
}

func (s *libraryStore) UpdateScanRunProgress(ctx context.Context, arg queries.UpdateScanRunProgressParams) error {
	return s.wq.UpdateScanRunProgress(ctx, arg)
}

func (s *libraryStore) FinishScanRun(ctx context.Context, arg queries.FinishScanRunParams) error {
	return s.wq.FinishScanRun(ctx, arg)
}

func (s *libraryStore) GetActiveScanRun(ctx context.Context, libraryID string) (queries.GetActiveScanRunRow, error) {
	return s.rq.GetActiveScanRun(ctx, libraryID)
}

func (s *libraryStore) EnqueueTrackFile(ctx context.Context, arg queries.EnqueueTrackFileParams) error {
	return s.wq.EnqueueTrackFile(ctx, arg)
}

func (s *libraryStore) ClaimEnrichmentBatch(ctx context.Context, limit int64) ([]queries.EnrichmentQueue, error) {
	return s.wq.ClaimEnrichmentBatch(ctx, limit)
}

func (s *libraryStore) MarkEnrichmentDone(ctx context.Context, id string) error {
	return s.wq.MarkEnrichmentDone(ctx, id)
}

func (s *libraryStore) MarkEnrichmentFailed(ctx context.Context, arg queries.MarkEnrichmentFailedParams) error {
	return s.wq.MarkEnrichmentFailed(ctx, arg)
}

func (s *libraryStore) ResetStuckEnrichments(ctx context.Context) error {
	return s.wq.ResetStuckEnrichments(ctx)
}

func loadSchedules(ctx context.Context, d *db.DB, sched *library.Scheduler) error {
	q := queries.New(d.Read())
	libs, err := q.ListLibraries(ctx)
	if err != nil {
		return err
	}
	var schedules []library.LibrarySchedule
	for _, lib := range libs {
		s, serr := q.GetLibrarySettings(ctx, lib.ID)
		if serr != nil {
			continue
		}
		schedules = append(schedules, library.LibrarySchedule{
			LibraryID: lib.ID,
			CronExpr:  ptrOrEmpty(s.ScanSchedule),
		})
	}
	return sched.Load(schedules) //nolint:contextcheck
}

func buildPathMap(ctx context.Context, d *db.DB) (map[string]string, error) {
	paths, err := queries.New(d.Read()).ListAllLibraryPaths(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(paths))
	for _, p := range paths {
		m[p.Path] = p.LibraryID
	}
	return m, nil
}

func ptrOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
