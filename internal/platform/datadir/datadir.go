// Package datadir owns the data directory layout: lazy subdir creation
// with correct permissions, so feature packages never compute paths themselves.
package datadir

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

const DefaultRoot = "./data"

const (
	SecretsSubdir = "secrets"
	CertsSubdir   = "certs"
	CacheSubdir   = "cache"
	TmpSubdir     = "tmp"

	rootMode    = 0o700
	secretsMode = 0o700
	certsMode   = 0o700
	cacheMode   = 0o755
	tmpMode     = 0o700
)

type Dirs struct {
	root string
}

func New(root string) (*Dirs, error) {
	if root == "" {
		return nil, errors.New("datadir: root path is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("datadir: resolve root: %w", err)
	}
	if err := ensureDir(abs, rootMode, true); err != nil {
		return nil, fmt.Errorf("datadir: root %s: %w", abs, err)
	}
	return &Dirs{root: abs}, nil
}

func (d *Dirs) Root() string { return d.root }

func (d *Dirs) SecretFile(name string) (string, error) {
	dir, err := d.ensureSubdir(SecretsSubdir, secretsMode)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

func (d *Dirs) CertsDir() (string, error) {
	return d.ensureSubdir(CertsSubdir, certsMode)
}

func (d *Dirs) CacheDir(kind string) (string, error) {
	if kind == "" || kind != filepath.Base(kind) || kind == "." || kind == ".." {
		return "", fmt.Errorf("datadir: invalid cache kind %q", kind)
	}
	if _, err := d.ensureSubdir(CacheSubdir, cacheMode); err != nil {
		return "", err
	}
	full := filepath.Join(d.root, CacheSubdir, kind)
	if err := ensureDir(full, cacheMode, false); err != nil {
		return "", fmt.Errorf("datadir: cache/%s: %w", kind, err)
	}
	return full, nil
}

func (d *Dirs) TmpDir() (string, error) {
	return d.ensureSubdir(TmpSubdir, tmpMode)
}

func (d *Dirs) ensureSubdir(name string, mode os.FileMode) (string, error) {
	full := filepath.Join(d.root, name)
	if err := ensureDir(full, mode, false); err != nil {
		return "", fmt.Errorf("datadir: %s: %w", name, err)
	}
	return full, nil
}

func ensureDir(dir string, mode os.FileMode, warnLoosePerms bool) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return os.MkdirAll(dir, mode)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", dir)
	}
	if warnLoosePerms {
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			slog.Warn("data dir is world/group readable", "dir", dir, "perm", perm)
		}
	}
	return nil
}
