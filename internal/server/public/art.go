package public

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/image/draw"
	"golang.org/x/sync/singleflight"
)

var validHash = regexp.MustCompile(`^[0-9a-f]{64}$`)

const (
	thumbDirName = "thumb"
	jpegQuality  = 82
)

var allowedSizes = map[int]struct{}{160: {}, 320: {}, 480: {}, 640: {}}

type Public struct {
	artDir  string
	limitMB atomic.Int64
	sf      singleflight.Group
	evictMu sync.Mutex
	log     *slog.Logger
}

func New(artDir string, initialLimitMB int64) *Public {
	p := &Public{
		artDir: artDir,
		log:    slog.Default(),
	}
	p.limitMB.Store(initialLimitMB)
	return p
}

func (p *Public) SetCacheLimit(mb int64) { p.limitMB.Store(mb) }

func (p *Public) ThumbUsedBytes() int64 {
	var total int64
	root := filepath.Join(p.artDir, thumbDirName)
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr
		}
		info, ierr := d.Info()
		if ierr == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

func (p *Public) Mount(r chi.Router) {
	r.Get("/public/art/{hash}", p.serveArt)
}

func (p *Public) serveArt(w http.ResponseWriter, r *http.Request) {
	hash := chi.URLParam(r, "hash")
	if !validHash.MatchString(hash) {
		writePublicError(w, http.StatusNotFound, "not_found", "art not found")
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	wStr := r.URL.Query().Get("w")
	if wStr == "" {
		http.ServeFile(w, r, filepath.Join(p.artDir, hash))
		return
	}

	size, err := strconv.Atoi(wStr)
	if err != nil {
		writePublicError(w, http.StatusBadRequest, "invalid_size", "w must be an integer")
		return
	}
	if _, ok := allowedSizes[size]; !ok {
		writePublicError(w, http.StatusBadRequest, "invalid_size", "w must be one of 160, 320, 480, 640")
		return
	}

	thumbPath := p.thumbPath(hash, size)
	if _, statErr := os.Stat(thumbPath); statErr == nil {
		now := time.Now()
		_ = os.Chtimes(thumbPath, now, now)
		http.ServeFile(w, r, thumbPath)
		return
	}

	_, err, _ = p.sf.Do(thumbPath, func() (any, error) {
		if _, statErr := os.Stat(thumbPath); statErr == nil {
			return nil, nil
		}
		return nil, p.generate(filepath.Join(p.artDir, hash), thumbPath, size)
	})
	if err != nil {
		p.log.Error("art: generate thumb", "hash", hash, "w", size, "err", err)
		writePublicError(w, http.StatusInternalServerError, "thumb_failed", "failed to generate thumbnail")
		return
	}

	http.ServeFile(w, r, thumbPath)
	go p.evictIfOver()
}

func (p *Public) thumbPath(hash string, size int) string {
	return filepath.Join(p.artDir, thumbDirName, hash[:2], fmt.Sprintf("%s_w%d.jpg", hash, size))
}

func (p *Public) generate(srcPath, dstPath string, size int) error {
	src, err := os.Open(srcPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer src.Close() //nolint:errcheck

	srcImg, _, err := image.Decode(src)
	if err != nil {
		return fmt.Errorf("decode source: %w", err)
	}

	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dstPath), ".tmp-*.jpg")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) //nolint:errcheck

	if err := jpeg.Encode(tmp, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		tmp.Close() //nolint:errcheck
		return fmt.Errorf("encode: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dstPath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}

type fileEntry struct {
	path  string
	size  int64
	mtime int64
}

func (p *Public) evictIfOver() {
	if !p.evictMu.TryLock() {
		return
	}
	defer p.evictMu.Unlock()

	limitMB := int(p.limitMB.Load())
	if limitMB <= 0 {
		return
	}
	limitBytes := int64(limitMB) * 1024 * 1024

	root := filepath.Join(p.artDir, thumbDirName)
	var entries []fileEntry
	var total int64

	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		total += info.Size()
		entries = append(entries, fileEntry{
			path:  path,
			size:  info.Size(),
			mtime: info.ModTime().UnixNano(),
		})
		return nil
	}); err != nil {
		if !os.IsNotExist(err) {
			p.log.Error("art: evict walk", "err", err)
		}
		return
	}

	if total <= limitBytes {
		return
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].mtime < entries[j].mtime })

	removed := 0
	for _, e := range entries {
		if total <= limitBytes {
			break
		}
		if err := os.Remove(e.path); err != nil {
			p.log.Error("art: evict remove", "path", e.path, "err", err)
			continue
		}
		total -= e.size
		removed++
	}

	p.log.Info("art: evicted thumbnails", "removed", removed, "remaining_bytes", total, "limit_mb", limitMB)
}

func writePublicError(w http.ResponseWriter, status int, code, message string) {
	type payload struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type envelope struct {
		Error payload `json:"error"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(envelope{Error: payload{Code: code, Message: message}})
}
