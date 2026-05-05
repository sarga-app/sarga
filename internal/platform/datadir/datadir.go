// Package datadir owns the layout of sarga's persistent runtime data
// directory. It centralises path resolution + lazy subdir creation with
// correct permissions, so feature packages never compute data subpaths
// themselves.
//
// The root path is fixed at DefaultRoot ("./data"). Operators don't
// override this in config — they choose where it lives via Docker volume
// mount, systemd WorkingDirectory, or the binary's CWD.
//
// Layout:
//
//	<root>/secrets/         0700  long-lived secrets (e.g. jwt.key)
//	<root>/certs/           0700  TLS cert cache (autocert)
//	<root>/cache/<kind>/    0755  regenerable caches (transcode, coverart)
//	<root>/tmp/             0700  atomic-write staging
//
// Backup guidance: secrets/ + certs/ are critical (losing secrets/jwt.key
// invalidates all sessions; losing certs/ may hit Let's Encrypt rate limits).
// cache/ and tmp/ are throwaway.
package datadir

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

// DefaultRoot is the only place the data dir path is named.
// Operators relocate it via Docker volume mount, systemd WorkingDirectory,
// or the binary's CWD — not via config.
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

// New validates (or creates) the root data dir and returns a Dirs handle.
// Subdirs are created lazily on first access.
func New(root string) (*Dirs, error) {
	if root == "" {
		return nil, errors.New("state: root path is empty")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("state: resolve root: %w", err)
	}
	if err := ensureDir(abs, rootMode, true); err != nil {
		return nil, fmt.Errorf("state: root %s: %w", abs, err)
	}
	return &Dirs{root: abs}, nil
}

func (d *Dirs) Root() string { return d.root }

// SecretFile ensures <root>/secrets exists (0700) and returns the full path
// to <root>/secrets/<name>. Caller is responsible for file perms (use 0600).
func (d *Dirs) SecretFile(name string) (string, error) {
	dir, err := d.ensureSubdir(SecretsSubdir, secretsMode)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// CertsDir ensures <root>/certs exists (0700) and returns its path.
// Suitable for autocert.DirCache.
func (d *Dirs) CertsDir() (string, error) {
	return d.ensureSubdir(CertsSubdir, certsMode)
}

// CacheDir ensures <root>/cache/<kind> exists (0755) and returns its path.
// kind must be a single path component (e.g. "transcode", "coverart").
func (d *Dirs) CacheDir(kind string) (string, error) {
	if kind == "" || kind != filepath.Base(kind) || kind == "." || kind == ".." {
		return "", fmt.Errorf("state: invalid cache kind %q", kind)
	}
	if _, err := d.ensureSubdir(CacheSubdir, cacheMode); err != nil {
		return "", err
	}
	full := filepath.Join(d.root, CacheSubdir, kind)
	if err := ensureDir(full, cacheMode, false); err != nil {
		return "", fmt.Errorf("state: cache/%s: %w", kind, err)
	}
	return full, nil
}

// TmpDir ensures <root>/tmp exists (0700) and returns its path.
// Use for atomic-write staging on the same filesystem as the final target.
func (d *Dirs) TmpDir() (string, error) {
	return d.ensureSubdir(TmpSubdir, tmpMode)
}

func (d *Dirs) ensureSubdir(name string, mode os.FileMode) (string, error) {
	full := filepath.Join(d.root, name)
	if err := ensureDir(full, mode, false); err != nil {
		return "", fmt.Errorf("state: %s: %w", name, err)
	}
	return full, nil
}

// ensureDir creates dir with mode if missing. If warnLoosePerms is true,
// logs a warning when an existing dir is world/group accessible.
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
			slog.Warn("state dir is world/group readable", "dir", dir, "perm", perm)
		}
	}
	return nil
}
