//nolint:testpackage // exercises file perm helpers indirectly via behaviour.
package auth

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateSecretFile_GeneratesOnFirstRun(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), JWTSecretFileName)

	secret, err := LoadOrCreateSecretFile(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(secret) != secretBytes {
		t.Errorf("secret len = %d, want %d", len(secret), secretBytes)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != secretFileMode {
		t.Errorf("file perm = %#o, want %#o", perm, secretFileMode)
	}
}

func TestLoadOrCreateSecretFile_ReusesExisting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), JWTSecretFileName)

	first, _ := LoadOrCreateSecretFile(path)
	second, _ := LoadOrCreateSecretFile(path)
	if !bytes.Equal(first, second) {
		t.Error("second call returned a different secret")
	}
}

func TestLoadOrCreateSecretFile_RejectsLoosePerms(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), JWTSecretFileName)
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 64), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSecretFile(path); err == nil {
		t.Error("expected error for 0644 secret file")
	}
}

func TestLoadOrCreateSecretFile_RejectsTruncated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), JWTSecretFileName)
	if err := os.WriteFile(path, []byte{1, 2, 3}, secretFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSecretFile(path); err == nil {
		t.Error("expected error for truncated secret file")
	}
}
