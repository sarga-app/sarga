package library

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type EventKind uint8

const (
	EventAdded EventKind = iota
	EventChanged
	EventRemoved
	EventUnchanged
)

type FileEvent struct {
	Kind    EventKind
	AbsPath string
	DBEntry *DBEntry
	Info    fs.FileInfo
}

type DBEntry struct {
	ID        string
	MTime     string
	SizeBytes int64
}

type ScanSettings struct {
	Extensions     map[string]struct{}
	FollowSymlinks bool
}

func ParseExtensions(s string) map[string]struct{} {
	parts := strings.Split(s, ",")
	m := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			m[strings.ToLower(p)] = struct{}{}
		}
	}
	return m
}

type Reconciler struct {
	store Store
}

func NewReconciler(store Store) *Reconciler {
	return &Reconciler{store: store}
}

func (r *Reconciler) Reconcile(
	ctx context.Context,
	libraryID, root string,
	settings ScanSettings,
) (events <-chan FileEvent, errc <-chan error) {
	evCh := make(chan FileEvent, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(evCh)
		defer close(errCh)

		if err := r.reconcile(ctx, libraryID, root, settings, evCh); err != nil {
			errCh <- err
		}
	}()
	return evCh, errCh
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	libraryID, root string,
	settings ScanSettings,
	out chan<- FileEvent,
) error {
	snapshots, err := r.store.ListTrackFileSnapshots(ctx, libraryID)
	if err != nil {
		return err
	}
	stored := make(map[string]DBEntry, len(snapshots))
	for _, s := range snapshots {
		stored[s.RelativePath] = DBEntry{
			ID:        s.ID,
			MTime:     s.Mtime,
			SizeBytes: s.FileSizeBytes,
		}
	}

	found := make(map[string]struct{}, len(snapshots))

	walkFn := func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip unreadable dirs; don't abort the whole walk
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			if !settings.FollowSymlinks {
				return nil
			}
		}

		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := settings.Extensions[ext]; !ok {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil //nolint:nilerr // stat failed; skip this file, don't abort walk
		}

		found[path] = struct{}{}

		stored, ok := stored[path]
		if !ok {
			select {
			case out <- FileEvent{Kind: EventAdded, AbsPath: path, Info: info}:
			case <-ctx.Done():
				return ctx.Err()
			}
			return nil
		}

		// Compare mtime and size (tier-1 change detection).
		storedTime := stored.MTime
		diskMTime := info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
		entry := stored
		kind := EventUnchanged
		if diskMTime != storedTime || info.Size() != stored.SizeBytes {
			kind = EventChanged
		}
		select {
		case out <- FileEvent{Kind: kind, AbsPath: path, DBEntry: &entry, Info: info}:
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	walkRoot := os.DirFS(root)
	if settings.FollowSymlinks {
		if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			return walkFn(path, d, err)
		}); err != nil {
			return err
		}
	} else {
		if err := fs.WalkDir(walkRoot, ".", func(rel string, d fs.DirEntry, err error) error {
			abs := filepath.Join(root, rel)
			return walkFn(abs, d, err)
		}); err != nil {
			return err
		}
	}

	for absPath, entry := range stored {
		if _, seen := found[absPath]; !seen {
			e := entry
			select {
			case out <- FileEvent{Kind: EventRemoved, AbsPath: absPath, DBEntry: &e}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}
