package library

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// CoverArtManager caches artwork to <data-dir>/art/ using SHA-256
// content-addressed filenames. One file per unique image regardless of how
// many tracks or releases share it.
type CoverArtManager struct {
	artDir string
	http   *http.Client
}

// NewCoverArtManager creates a manager that caches art under artDir.
func NewCoverArtManager(artDir string) (*CoverArtManager, error) {
	if err := os.MkdirAll(artDir, 0o750); err != nil {
		return nil, fmt.Errorf("coverart: create dir %q: %w", artDir, err)
	}
	return &CoverArtManager{
		artDir: artDir,
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// ProcessEmbedded caches art bytes embedded in a tag and returns the hex hash.
// Returns ("", nil) when data is empty.
func (m *CoverArtManager) ProcessEmbedded(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	return m.cache(data)
}

// FindFolderArt looks for common cover art filenames in dir.
// Returns the hex hash of the first file found, or ("", nil) if none exist.
func (m *CoverArtManager) FindFolderArt(dir string) (string, error) {
	candidates := []string{
		"cover.jpg", "cover.jpeg", "cover.png", "cover.webp",
		"Cover.jpg", "Cover.jpeg", "Cover.png",
		"folder.jpg", "folder.jpeg", "folder.png",
		"front.jpg", "front.jpeg", "front.png",
		"AlbumArtSmall.jpg", "Folder.jpg",
	}
	for _, name := range candidates {
		path := filepath.Join(dir, name)
		data, err := os.ReadFile(path) //nolint:gosec // scanner-validated path
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		return m.cache(data)
	}
	return "", nil
}

// ProcessResult caches an ArtworkResult — either inline bytes (e.g. from
// Cover Art Archive) or a remote URL (e.g. TheAudioDB, Fanart.tv).
// Returns the hex hash, or ("", nil) when nothing is available.
func (m *CoverArtManager) ProcessResult(ctx context.Context, r ArtworkResult) (string, error) {
	if len(r.Bytes) > 0 {
		return m.cache(r.Bytes)
	}
	if r.URL != "" {
		return m.fetchAndCache(ctx, r.URL)
	}
	return "", nil
}

// Exists reports whether art with the given hex hash is already on disk.
func (m *CoverArtManager) Exists(hash string) bool {
	_, err := os.Stat(filepath.Join(m.artDir, hash))
	return err == nil
}

// Path returns the absolute filesystem path for the given hash.
func (m *CoverArtManager) Path(hash string) string {
	return filepath.Join(m.artDir, hash)
}

// cache writes data to the art directory if not already present and returns
// its SHA-256 hex hash.
func (m *CoverArtManager) cache(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	dest := filepath.Join(m.artDir, hash)

	// dest is <artDir>/<sha256hex> — only safe hex characters, not user-controlled.
	if _, err := os.Stat(dest); err == nil { //nolint:gosec
		return hash, nil // already cached
	}

	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:gosec
		return "", fmt.Errorf("coverart: write %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, dest); err != nil { //nolint:gosec
		_ = os.Remove(tmp) //nolint:errcheck,gosec // best-effort cleanup; tmp is <artDir>/<sha256hex>.tmp
		return "", fmt.Errorf("coverart: rename %q: %w", tmp, err)
	}
	return hash, nil
}

// fetchAndCache downloads a remote URL and caches it. Returns ("", nil) on 404.
func (m *CoverArtManager) fetchAndCache(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "sarga/dev ( https://github.com/sarga-app/sarga )")

	resp, err := m.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("coverart: fetch %q: HTTP %d", rawURL, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10 MB cap
	if err != nil {
		return "", err
	}
	return m.cache(data)
}
