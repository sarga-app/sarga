package datadir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sarga-app/sarga/internal/platform/datadir"
)

func TestNew_CreatesMissingRoot(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "nested", "data")

	d, err := datadir.New(root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if d.Root() != root {
		// Root() returns absolute, but t.TempDir() is already absolute on darwin/linux,
		// so direct equality should hold. If a future platform breaks this, switch to filepath.EvalSymlinks compare.
		abs, _ := filepath.Abs(root)
		if d.Root() != abs {
			t.Errorf("Root() = %q, want %q", d.Root(), abs)
		}
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected root dir, got err=%v info=%v", err, info)
	}
}

func TestNew_RejectsEmpty(t *testing.T) {
	t.Parallel()
	if _, err := datadir.New(""); err == nil {
		t.Error("expected error for empty root")
	}
}

func TestSecretFile_LazyCreatesSecretsDirWith0700(t *testing.T) {
	t.Parallel()
	d, _ := datadir.New(t.TempDir())

	path, err := d.SecretFile("jwt.key")
	if err != nil {
		t.Fatalf("SecretFile: %v", err)
	}
	if filepath.Base(filepath.Dir(path)) != datadir.SecretsSubdir {
		t.Errorf("path %q not under %s/", path, datadir.SecretsSubdir)
	}
	info, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("secrets/ perm = %#o, want 0700", perm)
	}
}

func TestCacheDir_RejectsTraversal(t *testing.T) {
	t.Parallel()
	d, _ := datadir.New(t.TempDir())

	for _, kind := range []string{"", ".", "..", "foo/bar", "../etc"} {
		if _, err := d.CacheDir(kind); err == nil {
			t.Errorf("CacheDir(%q): expected error", kind)
		}
	}
}

func TestCacheDir_CreatesNestedKindDir(t *testing.T) {
	t.Parallel()
	d, _ := datadir.New(t.TempDir())

	path, err := d.CacheDir("transcode")
	if err != nil {
		t.Fatalf("CacheDir: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected dir, got err=%v info=%v", err, info)
	}
}
