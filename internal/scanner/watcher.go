package scanner

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/sarga-app/sarga/internal/db"
	"github.com/sarga-app/sarga/internal/db/queries"
)

const debounceWindow = 2 * time.Second

type Watcher struct {
	scanner *Scanner
	watcher *fsnotify.Watcher

	mu      sync.Mutex
	pending map[string]time.Time // libraryID → time of last event
}

func NewWatcher(sc *Scanner) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		scanner: sc,
		watcher: fw,
		pending: make(map[string]time.Time),
	}, nil
}

func (w *Watcher) Watch(root string) error {
	return w.watcher.Add(root)
}

func (w *Watcher) LoadPathsFromDB(ctx context.Context, d *db.DB) (map[string]string, error) {
	rows, err := queries.New(d.Read()).ListAllLibraryPaths(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, p := range rows {
		m[p.Path] = p.LibraryID
	}
	return m, nil
}

func (w *Watcher) Run(ctx context.Context, pathToLibrary map[string]string) {
	slog.Info("watcher: started", "paths", len(pathToLibrary))
	ticker := time.NewTicker(debounceWindow / 2)
	defer ticker.Stop()
	defer func() {
		w.watcher.Close() //nolint:errcheck
		slog.Info("watcher: stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			return

		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			libraryID := resolveLibraryID(ev.Name, pathToLibrary)
			if libraryID == "" {
				continue
			}
			w.mu.Lock()
			w.pending[libraryID] = time.Now()
			w.mu.Unlock()

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Warn("watcher: fs event error", "err", err)

		case <-ticker.C:
			w.fireDue(ctx)
		}
	}
}

func (w *Watcher) fireDue(ctx context.Context) {
	now := time.Now()
	w.mu.Lock()
	var due []string
	for id, t := range w.pending {
		if now.Sub(t) >= debounceWindow {
			due = append(due, id)
			delete(w.pending, id)
		}
	}
	w.mu.Unlock()

	for _, id := range due {
		go func(libraryID string) {
			if err := w.scanner.Scan(ctx, libraryID); err != nil {
				slog.Error("watcher: incremental scan failed", "library_id", libraryID, "err", err)
			}
		}(id)
	}
}

func resolveLibraryID(path string, pathToLibrary map[string]string) string {
	for root, id := range pathToLibrary {
		if len(path) >= len(root) && path[:len(root)] == root {
			return id
		}
	}
	return ""
}
