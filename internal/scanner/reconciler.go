package scanner

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sarga-app/sarga/internal/db/queries"
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

// FolderEvent groups every audio file the reconciler saw under one directory.
// Files appear in deterministic order (lexical by path). Removed files do not
// belong to any folder and are returned separately by ReconcileFolders.
type FolderEvent struct {
	Dir   string
	Files []FileEvent
}

// HasWork reports whether the folder contains any file that needs processing
// (i.e. a file that's new or changed since the last scan).
func (f FolderEvent) HasWork() bool {
	for _, e := range f.Files {
		if e.Kind == EventAdded || e.Kind == EventChanged {
			return true
		}
	}
	return false
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
	rq *queries.Queries
}

func NewReconciler(rq *queries.Queries) *Reconciler {
	return &Reconciler{rq: rq}
}

// ReconcileFolders walks the library root, groups every audio file by its
// parent directory, and returns one FolderEvent per directory that contained
// at least one audio file. Files in the DB whose path wasn't seen on disk are
// returned in `removed`.
//
// This is synchronous on purpose: the parallelism we want lives in the
// scanner pipeline (parallel hash/tag-extract → serial per-folder tx).
// Streaming events during the walk added contention without buying anything,
// and the entire reconciliation footprint at 100k files is ~15MB of paths.
func (r *Reconciler) ReconcileFolders(
	ctx context.Context,
	libraryID, root string,
	settings ScanSettings,
) (folders []FolderEvent, removed []FileEvent, err error) {
	snapshots, err := r.rq.ListTrackFileSnapshots(ctx, libraryID)
	if err != nil {
		return nil, nil, err
	}
	stored := make(map[string]DBEntry, len(snapshots))
	for _, s := range snapshots {
		stored[s.RelativePath] = DBEntry{
			ID:        s.ID,
			MTime:     s.Mtime,
			SizeBytes: s.FileSizeBytes,
		}
	}

	byDir := make(map[string][]FileEvent)
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
		if d.Type()&fs.ModeSymlink != 0 && !settings.FollowSymlinks {
			return nil
		}
		if _, ok := settings.Extensions[strings.ToLower(filepath.Ext(path))]; !ok {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil //nolint:nilerr // stat failed; skip this file, don't abort walk
		}

		found[path] = struct{}{}
		dir := filepath.Dir(path)

		ev := FileEvent{AbsPath: path, Info: info}
		if existing, ok := stored[path]; ok {
			entry := existing
			ev.DBEntry = &entry
			diskMTime := info.ModTime().UTC().Format("2006-01-02T15:04:05Z")
			if diskMTime != existing.MTime || info.Size() != existing.SizeBytes {
				ev.Kind = EventChanged
			} else {
				ev.Kind = EventUnchanged
			}
		} else {
			ev.Kind = EventAdded
		}

		byDir[dir] = append(byDir[dir], ev)
		return nil
	}

	walkRoot := os.DirFS(root)
	if settings.FollowSymlinks {
		if werr := filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
			return walkFn(path, d, e)
		}); werr != nil {
			return nil, nil, werr
		}
	} else {
		if werr := fs.WalkDir(walkRoot, ".", func(rel string, d fs.DirEntry, e error) error {
			abs := filepath.Join(root, rel)
			return walkFn(abs, d, e)
		}); werr != nil {
			return nil, nil, werr
		}
	}

	folders = make([]FolderEvent, 0, len(byDir))
	for dir, files := range byDir {
		sort.Slice(files, func(i, j int) bool { return files[i].AbsPath < files[j].AbsPath })
		folders = append(folders, FolderEvent{Dir: dir, Files: files})
	}
	sort.Slice(folders, func(i, j int) bool { return folders[i].Dir < folders[j].Dir })

	for absPath, entry := range stored {
		if _, seen := found[absPath]; !seen {
			e := entry
			removed = append(removed, FileEvent{Kind: EventRemoved, AbsPath: absPath, DBEntry: &e})
		}
	}

	return folders, removed, nil
}
