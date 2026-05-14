package crypto_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/sarga-app/sarga/internal/crypto"
)

// --- master key ---

func TestLoadOrGenerateMasterKey_createsFileOnFirstCall(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")

	k, err := crypto.LoadOrGenerateMasterKey(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *k == (crypto.MasterKey{}) {
		t.Fatal("generated key is all-zero")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("key file not created: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("key file perm: want 0600, got %04o", perm)
	}
}

func TestLoadOrGenerateMasterKey_roundtrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")

	k1, err := crypto.LoadOrGenerateMasterKey(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	k2, err := crypto.LoadOrGenerateMasterKey(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if *k1 != *k2 {
		t.Error("keys differ across loads")
	}
}

func TestLoadOrGenerateMasterKey_rejectsBadLength(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, []byte("tooshort"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := crypto.LoadOrGenerateMasterKey(path); err == nil {
		t.Fatal("expected error for wrong-length key file")
	}
}

func TestMasterKey_Derive_deterministic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	k, _ := crypto.LoadOrGenerateMasterKey(path)

	a, err := k.Derive("external-tokens")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := k.Derive("external-tokens")
	if a != b {
		t.Error("Derive is not deterministic for the same purpose")
	}
}

func TestMasterKey_Derive_isolatedByPurpose(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	k, _ := crypto.LoadOrGenerateMasterKey(path)

	a, _ := k.Derive("purpose-a")
	b, _ := k.Derive("purpose-b")
	if a == b {
		t.Error("different purposes produced identical subkeys")
	}
}

func TestMasterKey_Derive_emptyPurposeErrors(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	k, _ := crypto.LoadOrGenerateMasterKey(path)

	if _, err := k.Derive(""); err == nil {
		t.Error("expected error for empty purpose")
	}
}

// --- jwt secret ---

func TestLoadOrGenerateJWTSecret_generatesOnFirstRun(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), crypto.JWTSecretFileName)

	secret, err := crypto.LoadOrGenerateJWTSecret(path)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if len(secret) < 32 {
		t.Errorf("secret len = %d, want >= 32", len(secret))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %#o, want 0600", perm)
	}
}

func TestLoadOrGenerateJWTSecret_reusesExisting(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), crypto.JWTSecretFileName)

	first, _ := crypto.LoadOrGenerateJWTSecret(path)
	second, _ := crypto.LoadOrGenerateJWTSecret(path)
	if !bytes.Equal(first, second) {
		t.Error("second call returned a different secret")
	}
}

func TestLoadOrGenerateJWTSecret_rejectsLoosePerms(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), crypto.JWTSecretFileName)
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 64), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := crypto.LoadOrGenerateJWTSecret(path); err == nil {
		t.Error("expected error for 0644 secret file")
	}
}

func TestLoadOrGenerateJWTSecret_rejectsTruncated(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), crypto.JWTSecretFileName)
	if err := os.WriteFile(path, []byte{1, 2, 3}, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := crypto.LoadOrGenerateJWTSecret(path); err == nil {
		t.Error("expected error for truncated secret file")
	}
}
