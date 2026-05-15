package scanner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sarga-app/sarga/internal/tags"
)

type preparedFolder struct {
	dir         string
	files       []preparedFile
	artHash     string
	artFromFile bool
}

type preparedFile struct {
	ev   FileEvent
	hash []byte
	meta tags.Metadata
}

func (s *Scanner) processPath(
	ctx context.Context,
	libraryID, root string,
	cfg ScanSettings,
	cursor string,
	workerN int,
	pr *progress,
) error {
	walkStart := time.Now()
	folders, removed, err := s.rec.ReconcileFolders(ctx, libraryID, root, cfg)
	if err != nil {
		return err
	}
	walkDur := time.Since(walkStart)
	pr.addWalk(walkDur)

	skipped := 0
	if cursor != "" {
		out := folders[:0]
		for _, f := range folders {
			if f.Dir <= cursor {
				skipped++
				continue
			}
			out = append(out, f)
		}
		folders = out
	}

	slog.Debug("scan: walk",
		"elapsed", walkDur.Round(time.Millisecond),
		"folders", len(folders), "removed", len(removed), "skipped_by_cursor", skipped)

	for _, f := range folders {
		for _, ev := range f.Files {
			pr.recordFound(ev.AbsPath)
			if ev.Kind == EventUnchanged {
				pr.recordUnchanged()
			}
		}
	}
	if len(removed) > 0 {
		pr.recordRemoved(int64(len(removed)))
	}

	pipeStart := time.Now()
	in := make(chan FolderEvent, workerN)
	out := make(chan *preparedFolder, workerN)

	go func() {
		defer close(in)
		for _, f := range folders {
			if !f.HasWork() {
				continue
			}
			select {
			case in <- f:
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	for range workerN {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range in {
				pf := s.prepareFolder(f, pr)
				if pf == nil {
					continue
				}
				select {
				case out <- pf:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() { wg.Wait(); close(out) }()

	persistStart := time.Now()
	var persisted int
	for pf := range out {
		if err := s.persistFolder(ctx, libraryID, pf, pr); err != nil {
			slog.Error("scan: persist folder", "dir", pf.dir, "err", err)
		}
		persisted++
	}

	pipelineDur := time.Since(pipeStart)
	pr.addPipeline(pipelineDur)
	slog.Debug("scan: pipeline",
		"folders_with_work", persisted,
		"prepare_wall", persistStart.Sub(pipeStart).Round(time.Millisecond),
		"total_wall", pipelineDur.Round(time.Millisecond),
	)
	return nil
}

func (s *Scanner) prepareFolder(f FolderEvent, pr *progress) *preparedFolder {
	pf := &preparedFolder{dir: f.Dir}
	for _, ev := range f.Files {
		if ev.Kind != EventAdded && ev.Kind != EventChanged {
			continue
		}
		hash, err := HashFile(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: hash failed", "path", ev.AbsPath, "err", err)
			pr.recordErrored()
			continue
		}
		meta, err := tags.Read(ev.AbsPath)
		if err != nil {
			slog.Warn("scan: tag extract failed", "path", ev.AbsPath, "err", err)
			pr.recordErrored()
			continue
		}
		slog.Debug("scan: file",
			"path", ev.AbsPath, "title", meta.Title,
			"duration_ms", meta.DurationMs, "format", meta.Format)
		pf.files = append(pf.files, preparedFile{ev: ev, hash: hash, meta: meta})
	}
	if len(pf.files) == 0 {
		return nil
	}

	for _, pfile := range pf.files {
		if len(pfile.meta.EmbeddedArt) > 0 {
			if h, err := s.art.ProcessEmbedded(pfile.meta.EmbeddedArt); err == nil && h != "" {
				pf.artHash = h
				pf.artFromFile = true
				break
			}
		}
	}
	if pf.artHash == "" {
		if h, err := s.art.FindFolderArt(pf.dir); err == nil && h != "" {
			pf.artHash = h
		}
	}
	return pf
}
