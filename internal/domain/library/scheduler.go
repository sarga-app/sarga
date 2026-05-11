package library

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/robfig/cron/v3"
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

func NewScheduler(scanner *Scanner) *Scheduler {
	return &Scheduler{
		scanner: scanner,
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
