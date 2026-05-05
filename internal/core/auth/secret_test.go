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
	dir := t.TempDir()

	secret, err := LoadOrCreateSecretFile(dir)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(secret) != secretBytes {
		t.Errorf("secret len = %d, want %d", len(secret), secretBytes)
	}

	info, err := os.Stat(filepath.Join(dir, SecretFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != secretFileMode {
		t.Errorf("file perm = %#o, want %#o", perm, secretFileMode)
	}
}

func TestLoadOrCreateSecretFile_ReusesExisting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	first, _ := LoadOrCreateSecretFile(dir)
	second, _ := LoadOrCreateSecretFile(dir)
	if !bytes.Equal(first, second) {
		t.Error("second call returned a different secret")
	}
}

func TestLoadOrCreateSecretFile_RejectsLoosePerms(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, SecretFileName)
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 64), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSecretFile(dir); err == nil {
		t.Error("expected error for 0644 secret file")
	}
}

func TestLoadOrCreateSecretFile_RejectsTruncated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, SecretFileName)
	if err := os.WriteFile(path, []byte{1, 2, 3}, secretFileMode); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrCreateSecretFile(dir); err == nil {
		t.Error("expected error for truncated secret file")
	}
}

func TestLoadOrCreateSecretFile_CreatesMissingDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "nested", "state")
	if _, err := LoadOrCreateSecretFile(dir); err != nil {
		t.Fatalf("create nested dir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Error("expected directory at nested path")
	}
}
