package scanner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sarga-app/sarga/internal/db/queries"
)

const progressFlushEvery = 500

type progress struct {
	mu    sync.Mutex
	runID string
	found []string

	total, added, updated, unchanged, errored, removed, moved int64

	walkDur, pipelineDur, missingDur, gcDur time.Duration

	flushed struct {
		total, added, updated, unchanged, errored, removed int64
	}
}

func (p *progress) addWalk(d time.Duration)     { p.mu.Lock(); p.walkDur += d; p.mu.Unlock() }
func (p *progress) addPipeline(d time.Duration) { p.mu.Lock(); p.pipelineDur += d; p.mu.Unlock() }
func (p *progress) setMissingDur(d time.Duration) {
	p.mu.Lock()
	p.missingDur = d
	p.mu.Unlock()
}
func (p *progress) setGCDur(d time.Duration) { p.mu.Lock(); p.gcDur = d; p.mu.Unlock() }

func (p *progress) timings() (walk, pipeline, missing, gc time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.walkDur, p.pipelineDur, p.missingDur, p.gcDur
}

func (p *progress) recordAdded()     { p.bump(&p.added, &p.total) }
func (p *progress) recordUpdated()   { p.bump(&p.updated, &p.total) }
func (p *progress) recordUnchanged() { p.bump(&p.unchanged, &p.total) }
func (p *progress) recordMoved()     { p.bump(&p.moved, &p.total) }
func (p *progress) recordErrored()   { p.bump(&p.errored) }
func (p *progress) recordRemoved(n int64) {
	p.mu.Lock()
	p.removed += n
	p.mu.Unlock()
}

func (p *progress) recordFound(path string) {
	p.mu.Lock()
	p.found = append(p.found, path)
	p.mu.Unlock()
}

func (p *progress) bump(fields ...*int64) {
	p.mu.Lock()
	for _, f := range fields {
		*f++
	}
	p.mu.Unlock()
}

func (p *progress) snapshotFound() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.found))
	copy(out, p.found)
	return out
}

func (p *progress) flushIfDue(ctx context.Context, wq *queries.Queries) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.total-p.flushed.total < progressFlushEvery {
		return
	}
	p.flushLocked(ctx, wq)
}

func (p *progress) flushFinal(ctx context.Context, wq *queries.Queries) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.flushLocked(ctx, wq)
}

func (p *progress) flushLocked(ctx context.Context, wq *queries.Queries) {
	if err := wq.UpdateScanRunProgress(ctx, queries.UpdateScanRunProgressParams{
		FilesScanned:   p.total - p.flushed.total,
		FilesAdded:     p.added - p.flushed.added,
		FilesUpdated:   p.updated - p.flushed.updated,
		FilesRemoved:   p.removed - p.flushed.removed,
		FilesUnchanged: p.unchanged - p.flushed.unchanged,
		FilesErrored:   p.errored - p.flushed.errored,
		ID:             p.runID,
	}); err != nil {
		slog.Error("scan: flush progress", "run_id", p.runID, "err", err)
	}
	p.flushed.total = p.total
	p.flushed.added = p.added
	p.flushed.updated = p.updated
	p.flushed.unchanged = p.unchanged
	p.flushed.errored = p.errored
	p.flushed.removed = p.removed
}

type progressTotals struct {
	Total, Added, Updated, Unchanged, Moved, Removed, Errored int64
}

func (p *progress) totals() progressTotals {
	p.mu.Lock()
	defer p.mu.Unlock()
	return progressTotals{
		Total: p.total, Added: p.added, Updated: p.updated,
		Unchanged: p.unchanged, Moved: p.moved,
		Removed: p.removed, Errored: p.errored,
	}
}

func optFloat(f float64) *float64 {
	if f == 0 {
		return nil
	}
	return &f
}

func optInt64(n int) *int64 {
	if n <= 0 {
		return nil
	}
	v := int64(n)
	return &v
}
