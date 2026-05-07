package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"golang.org/x/crypto/hkdf"
)

const keySize = 32

// MasterKey is the root encryption secret for the server. It is never used
// directly — callers derive purpose-specific subkeys via Derive.
type MasterKey [keySize]byte

// LoadOrGenerate reads the 32-byte master key at path, or generates and persists
// one if the file does not exist.
func LoadOrGenerate(path string) (*MasterKey, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is constructed by datadir, not user input
	if errors.Is(err, fs.ErrNotExist) {
		return generate(path)
	}
	if err != nil {
		return nil, fmt.Errorf("crypto: read master key: %w", err)
	}
	if len(data) != keySize {
		return nil, fmt.Errorf("crypto: master key at %s: want %d bytes, got %d", path, keySize, len(data))
	}
	var k MasterKey
	copy(k[:], data)
	return &k, nil
}

func generate(path string) (*MasterKey, error) {
	var k MasterKey
	if _, err := io.ReadFull(rand.Reader, k[:]); err != nil {
		return nil, fmt.Errorf("crypto: generate master key: %w", err)
	}
	if err := os.WriteFile(path, k[:], 0o600); err != nil {
		return nil, fmt.Errorf("crypto: write master key: %w", err)
	}
	return &k, nil
}

// Derive produces a 32-byte subkey for the given purpose using HKDF-SHA256.
// purpose must be non-empty and unique per use-case (e.g. "external-tokens").
// The info label is "sarga/v1/<purpose>".
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
