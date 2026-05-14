package coverart

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

	"github.com/sarga-app/sarga/internal/providers"
)

type Manager struct {
	artDir string
	http   *http.Client
}

func NewManager(artDir string) (*Manager, error) {
	if err := os.MkdirAll(artDir, 0o750); err != nil {
		return nil, fmt.Errorf("coverart: create dir %q: %w", artDir, err)
	}
	return &Manager{
		artDir: artDir,
		http:   &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (m *Manager) ProcessEmbedded(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	return m.cache(data)
}

func (m *Manager) FindFolderArt(dir string) (string, error) {
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

// ProcessResult dispatches to inline bytes (CAA) or URL fetch (TheAudioDB, Fanart.tv).
func (m *Manager) ProcessResult(ctx context.Context, r providers.ArtworkResult) (string, error) {
	if len(r.Bytes) > 0 {
		return m.cache(r.Bytes)
	}
	if r.URL != "" {
		return m.fetchAndCache(ctx, r.URL)
	}
	return "", nil
}

func (m *Manager) Exists(hash string) bool {
	_, err := os.Stat(filepath.Join(m.artDir, hash))
	return err == nil
}

func (m *Manager) Path(hash string) string {
	return filepath.Join(m.artDir, hash)
}

func (m *Manager) cache(data []byte) (string, error) {
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	dest := filepath.Join(m.artDir, hash)

	// dest is <artDir>/<sha256hex> — only safe hex characters, not user-controlled.
	if _, err := os.Stat(dest); err == nil { //nolint:gosec
		return hash, nil
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

// fetchAndCache returns ("", nil) on 404 — a missing remote image is not an error.
func (m *Manager) fetchAndCache(ctx context.Context, rawURL string) (string, error) {
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

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", err
	}
	return m.cache(data)
}
