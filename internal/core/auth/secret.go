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

// JWTSecretFileName is the filename (within state's secrets/ subdir)
// where the auto-generated JWT signing secret is persisted.
const JWTSecretFileName = "jwt.key"

const (
	secretBytes    = 64
	secretFileMode = 0o600
	minSecretBytes = 32
)

// LoadOrCreateSecretFile returns the JWT signing secret stored at path,
// generating a fresh 64-byte secret on first run. Race-safe across concurrent
// boots and crash-safe (fsync + atomic create-only via hardlink).
//
// Caller is responsible for ensuring filepath.Dir(path) exists with safe
// permissions (use platform/state.SecretFile).
func LoadOrCreateSecretFile(path string) ([]byte, error) {
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
