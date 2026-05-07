package crypto_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sarga-app/sarga/internal/support/crypto"
)

func TestLoadOrGenerate_createsFileOnFirstCall(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")

	k, err := crypto.LoadOrGenerate(path)
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

func TestLoadOrGenerate_roundtrip(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")

	k1, err := crypto.LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	k2, err := crypto.LoadOrGenerate(path)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if *k1 != *k2 {
		t.Error("keys differ across loads")
	}
}

func TestLoadOrGenerate_rejectsBadLength(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	if err := os.WriteFile(path, []byte("tooshort"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := crypto.LoadOrGenerate(path); err == nil {
		t.Fatal("expected error for wrong-length key file")
	}
}

func TestMasterKey_Derive_deterministic(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	k, _ := crypto.LoadOrGenerate(path)

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
	k, _ := crypto.LoadOrGenerate(path)

	a, _ := k.Derive("purpose-a")
	b, _ := k.Derive("purpose-b")
	if a == b {
		t.Error("different purposes produced identical subkeys")
	}
}

func TestMasterKey_Derive_emptyPurposeErrors(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "master.key")
	k, _ := crypto.LoadOrGenerate(path)

	if _, err := k.Derive(""); err == nil {
		t.Error("expected error for empty purpose")
	}
}
