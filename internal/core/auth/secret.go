package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// Issuer / Audience are not user-configurable. Tuning them changes the
// security model — code change + audit, not a config knob.
const (
	DefaultIssuer   = "sarga"
	DefaultAudience = "sarga-web"
)

const (
	SecretFileName = "secret.key"

	secretBytes    = 64
	secretFileMode = 0o600
	stateDirMode   = 0o700
	minSecretBytes = 32
)

// LoadOrCreateSecretFile returns the JWT signing secret stored at <stateDir>/secret.key,
// generating a fresh 64-byte secret on first run. Race-safe across concurrent
// boots and crash-safe (fsync + atomic create-only via hardlink).
func LoadOrCreateSecretFile(stateDir string) ([]byte, error) {
	if err := ensureStateDir(stateDir); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	path := filepath.Join(stateDir, SecretFileName)

	if data, err := readSecret(path); err == nil {
		return data, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	fresh := make([]byte, secretBytes)
	if _, err := rand.Read(fresh); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	if err := writeSecretAtomic(path, fresh); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Concurrent boot won the race — re-read what they wrote.
			return readSecret(path)
		}
		return nil, fmt.Errorf("write secret: %w", err)
	}
	slog.Info("jwt secret: generated and persisted", "path", path)
	return fresh, nil
}

func ensureStateDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, stateDirMode)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", dir)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		slog.Warn("state dir is world/group readable", "dir", dir, "perm", perm)
	}
	return nil
}

func readSecret(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("secret file %s has unsafe permissions %#o (want %#o); refusing to load", path, perm, secretFileMode)
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is from validated state dir, not user input
	if err != nil {
		return nil, err
	}
	if len(data) < minSecretBytes {
		return nil, fmt.Errorf("secret file %s shorter than %d bytes (corrupt?)", path, minSecretBytes)
	}
	return data, nil
}

func writeSecretAtomic(path string, secret []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "secret-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) } //nolint:errcheck // best-effort cleanup of orphaned temp

	if err := os.Chmod(tmpPath, secretFileMode); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if _, err := tmp.Write(secret); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}

	// Link is atomic create-only; Rename would clobber a competing winner.
	if err := os.Link(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	cleanup()

	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // dir is the state dir, not user input
	if err != nil {
		return
	}
	_ = d.Sync()  //nolint:errcheck // best-effort: not all filesystems support directory sync
	_ = d.Close() //nolint:errcheck // best-effort
}
