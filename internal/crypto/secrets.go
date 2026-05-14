package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/hkdf"
)

const (
	keySize = 32

	jwtSecretGenBytes = 64
	jwtSecretMinBytes = 32

	keyFileMode fs.FileMode = 0o600

	JWTSecretFileName = "jwt.key"
)

type MasterKey [keySize]byte

// ParseMasterKey accepts hex, base64 (padded), or base64 (raw).
func ParseMasterKey(encoded string) (*MasterKey, error) {
	s := strings.TrimSpace(encoded)
	if s == "" {
		return nil, errors.New("crypto: master key is empty")
	}
	raw, err := decodeMasterKey(s)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode master key: %w", err)
	}
	if len(raw) != keySize {
		return nil, fmt.Errorf("crypto: master key: want %d bytes, got %d", keySize, len(raw))
	}
	var k MasterKey
	copy(k[:], raw)
	return &k, nil
}

func decodeMasterKey(s string) ([]byte, error) {
	if b, err := hex.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

func LoadOrGenerateMasterKey(path string) (*MasterKey, error) {
	data, err := readKeyFile(path)
	switch {
	case err == nil:
		if len(data) != keySize {
			return nil, fmt.Errorf("crypto: master key at %s: want %d bytes, got %d", path, keySize, len(data))
		}
		slog.Info("master key: loaded", "path", path)
	case errors.Is(err, fs.ErrNotExist):
		data = make([]byte, keySize)
		if _, err := io.ReadFull(rand.Reader, data); err != nil {
			return nil, fmt.Errorf("crypto: generate master key: %w", err)
		}
		if err := writeKeyFile(path, data); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return LoadOrGenerateMasterKey(path)
			}
			return nil, fmt.Errorf("crypto: write master key: %w", err)
		}
		slog.Info("master key: generated and persisted", "path", path)
	default:
		return nil, err
	}
	var k MasterKey
	copy(k[:], data)
	return &k, nil
}

func LoadOrGenerateJWTSecret(path string) ([]byte, error) {
	data, err := readKeyFile(path)
	switch {
	case err == nil:
		if len(data) < jwtSecretMinBytes {
			return nil, fmt.Errorf("crypto: jwt secret at %s: want at least %d bytes, got %d", path, jwtSecretMinBytes, len(data))
		}
		slog.Info("jwt secret: loaded", "path", path)
		return data, nil
	case errors.Is(err, fs.ErrNotExist):
		fresh := make([]byte, jwtSecretGenBytes)
		if _, err := rand.Read(fresh); err != nil {
			return nil, fmt.Errorf("crypto: generate jwt secret: %w", err)
		}
		if err := writeKeyFile(path, fresh); err != nil {
			if errors.Is(err, fs.ErrExist) {
				return LoadOrGenerateJWTSecret(path)
			}
			return nil, fmt.Errorf("crypto: write jwt secret: %w", err)
		}
		slog.Info("jwt secret: generated and persisted", "path", path)
		return fresh, nil
	default:
		return nil, err
	}
}

// Derive produces a 32-byte subkey via HKDF-SHA256 with info "sarga/v1/<purpose>".
func (k *MasterKey) Derive(purpose string) ([keySize]byte, error) {
	if purpose == "" {
		return [keySize]byte{}, errors.New("crypto: derive: purpose must not be empty")
	}
	r := hkdf.New(sha256.New, k[:], nil, []byte("sarga/v1/"+purpose))
	var out [keySize]byte
	if _, err := io.ReadFull(r, out[:]); err != nil {
		return [keySize]byte{}, fmt.Errorf("crypto: derive %q: %w", purpose, err)
	}
	return out, nil
}

// --- on-disk key file helpers (0600, atomic create-only) ---

func readKeyFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("crypto: key file %s has unsafe permissions %#o (want %#o); refusing to load", path, perm, keyFileMode)
	}
	return os.ReadFile(path) //nolint:gosec // path comes from validated state dir
}

// writeKeyFile atomically creates path holding data at mode 0600. Returns
// fs.ErrExist if a concurrent boot already created the file — caller re-reads.
func writeKeyFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "key-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) } //nolint:errcheck

	if err := os.Chmod(tmpPath, keyFileMode); err != nil {
		_ = tmp.Close() //nolint:errcheck
		cleanup()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close() //nolint:errcheck
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}

	// Link is atomic create-only; Rename would clobber a concurrent winner.
	if err := os.Link(tmpPath, path); err != nil {
		cleanup()
		if errors.Is(err, fs.ErrExist) {
			return fs.ErrExist
		}
		return err
	}
	cleanup()
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec
	if err != nil {
		return
	}
	_ = d.Sync()  //nolint:errcheck
	_ = d.Close() //nolint:errcheck
}
