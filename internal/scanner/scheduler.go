package scanner

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
)

const jitterRange = 30 * time.Minute

type LibrarySchedule struct {
	LibraryID    string
	CronExpr     string
	ScanOnCreate bool
}

type Scheduler struct {
	scanner *Scanner
	cron    *cron.Cron
}

func NewScheduler(sc *Scanner) *Scheduler {
	return &Scheduler{
		scanner: sc,
		cron:    cron.New(cron.WithSeconds()),
	}
}

func (s *Scheduler) Load(schedules []LibrarySchedule) error {
	n := 0
	for _, ls := range schedules {
		if ls.CronExpr == "" {
			continue
		}
		libraryID := ls.LibraryID
		expr := ls.CronExpr
		_, err := s.cron.AddFunc(expr, func() { //nolint:contextcheck
			jitter := time.Duration(rand.Int64N(int64(jitterRange*2))) - jitterRange //nolint:gosec
			if jitter > 0 {
				time.Sleep(jitter)
			}
			ctx := context.Background()
			if err := s.scanner.Scan(ctx, libraryID); err != nil {
				slog.Error("scheduler: scan failed", "library_id", libraryID, "err", err)
			}
		})
		if err != nil {
			return err
		}
		n++
	}
	slog.Info("scheduler: schedules loaded", "count", n)
	return nil
}

func (s *Scheduler) Start() {
	slog.Info("scheduler: started")
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	slog.Info("scheduler: stopped")
}

func (s *Scheduler) LoadFromDB(ctx context.Context, d *db.DB) error {
	q := queries.New(d.Read())
	libs, err := q.ListLibraries(ctx)
	if err != nil {
		return err
	}
	schedules := make([]LibrarySchedule, 0, len(libs))
	for _, lib := range libs {
		settings, serr := q.GetLibrarySettings(ctx, lib.ID)
		if serr != nil {
			continue
		}
		expr := ""
		if settings.ScanSchedule != nil {
			expr = *settings.ScanSchedule
		}
		schedules = append(schedules, LibrarySchedule{
			LibraryID: lib.ID,
			CronExpr:  expr,
		})
	}
	return s.Load(schedules) //nolint:contextcheck
}
