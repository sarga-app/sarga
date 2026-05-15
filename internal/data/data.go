package data

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

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
		return nil, errors.New("data: root path is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("data: resolve root: %w", err)
	}
	if err := ensureDir(abs, rootMode, true); err != nil {
		return nil, fmt.Errorf("data: root %s: %w", abs, err)
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
		return "", fmt.Errorf("data: invalid cache kind %q", kind)
	}
	if _, err := d.ensureSubdir(CacheSubdir, cacheMode); err != nil {
		return "", err
	}
	full := filepath.Join(d.root, CacheSubdir, kind)
	if err := ensureDir(full, cacheMode, false); err != nil {
		return "", fmt.Errorf("data: cache/%s: %w", kind, err)
	}
	return full, nil
}

func (d *Dirs) TmpDir() (string, error) {
	return d.ensureSubdir(TmpSubdir, tmpMode)
}

func (d *Dirs) ensureSubdir(name string, mode os.FileMode) (string, error) {
	full := filepath.Join(d.root, name)
	if err := ensureDir(full, mode, false); err != nil {
		return "", fmt.Errorf("data: %s: %w", name, err)
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
			if err := os.Chmod(dir, mode); err != nil {
				slog.Warn("data: tighten permissions failed",
					"dir", dir, "perm", perm, "err", err,
					"action", "chmod 0700 the data dir manually; sarga refuses to run with loose perms")
			} else {
				slog.Info("data: tightened permissions", "dir", dir, "from", perm, "to", mode)
			}
		}
	}
	return nil
}
