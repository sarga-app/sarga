package auth

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Issuer / Audience are not user-configurable: tuning them changes the
// security model — code change + audit, not a config knob.
const (
	DefaultIssuer   = "sarga"
	DefaultAudience = "sarga-web"
)

const (
	AccessTTL  = 30 * time.Minute
	RefreshTTL = 30 * 24 * time.Hour

	signingMethod = "HS256"
)

const JWTSecretFileName = "jwt.key"

const (
	secretBytes    = 64
	secretFileMode = 0o600
	minSecretBytes = 32
)

var ErrInvalidToken = errors.New("invalid token")

// AccessClaims namespaces custom claims under "sarga." to avoid collisions.
type AccessClaims struct {
	jwt.RegisteredClaims
	IsAdmin bool `json:"sarga.is_admin"`
}

// JWTConfig: Secret is mandatory; Issuer/Audience default if zero. Clock is
// injectable for tests.
type JWTConfig struct {
	Secret   []byte
	Issuer   string
	Audience string
	Clock    func() time.Time
}

func (c JWTConfig) issuer() string {
	if c.Issuer != "" {
		return c.Issuer
	}
	return DefaultIssuer
}

func (c JWTConfig) audience() string {
	if c.Audience != "" {
		return c.Audience
	}
	return DefaultAudience
}

func (c JWTConfig) now() time.Time {
	if c.Clock != nil {
		return c.Clock()
	}
	return time.Now()
}

func IssueAccessToken(cfg JWTConfig, userID string, isAdmin bool) (string, error) {
	if len(cfg.Secret) == 0 {
		return "", errors.New("jwt: empty secret")
	}
	now := cfg.now()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.issuer(),
			Subject:   userID,
			Audience:  jwt.ClaimStrings{cfg.audience()},
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
		IsAdmin: isAdmin,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(cfg.Secret)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}
	return signed, nil
}

// ParseAccessToken wraps any validation failure as ErrInvalidToken so callers
// can match without inspecting library-internal error types.
func ParseAccessToken(cfg JWTConfig, tokenStr string) (*AccessClaims, error) {
	if len(cfg.Secret) == 0 {
		return nil, errors.New("jwt: empty secret")
	}
	parser := jwt.NewParser(
		jwt.WithIssuer(cfg.issuer()),
		jwt.WithAudience(cfg.audience()),
		jwt.WithValidMethods([]string{signingMethod}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(cfg.now),
	)
	var claims AccessClaims
	_, err := parser.ParseWithClaims(tokenStr, &claims, func(_ *jwt.Token) (any, error) {
		return cfg.Secret, nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return &claims, nil
}

func (c *AccessClaims) UserID() (uuid.UUID, error) {
	id, err := uuid.Parse(c.Subject)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse subject %q: %w", c.Subject, err)
	}
	return id, nil
}

func LoadOrCreateSecretFile(path string) ([]byte, error) {
	if data, err := readSecret(path); err == nil {
		slog.Info("jwt secret: loaded", "path", path)
		return data, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	fresh := make([]byte, secretBytes)
	if _, err := rand.Read(fresh); err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	if err := writeSecretAtomic(path, fresh); err != nil {
		if errors.Is(err, fs.ErrExist) {
			// Concurrent boot won the race — re-read what they wrote.
			return readSecret(path)
		}
		return nil, fmt.Errorf("write secret: %w", err)
	}
	slog.Info("jwt secret: generated and persisted", "path", path)
	return fresh, nil
}

func readSecret(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return nil, fmt.Errorf("secret file %s has unsafe permissions %#o (want %#o); refusing to load", path, perm, secretFileMode)
	}
	data, err := os.ReadFile(path) //nolint:gosec // path is from validated state dir, not user input
	if err != nil {
		return nil, err
	}
	if len(data) < minSecretBytes {
		return nil, fmt.Errorf("secret file %s shorter than %d bytes (corrupt?)", path, minSecretBytes)
	}
	return data, nil
}

func writeSecretAtomic(path string, secret []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, "secret-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) } //nolint:errcheck // best-effort cleanup of orphaned temp

	if err := os.Chmod(tmpPath, secretFileMode); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if _, err := tmp.Write(secret); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close() //nolint:errcheck // already failing; close is best-effort
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}

	// Link is atomic create-only; Rename would clobber a competing winner.
	if err := os.Link(tmpPath, path); err != nil {
		cleanup()
		return err
	}
	cleanup()

	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	d, err := os.Open(dir) //nolint:gosec // dir is the state dir, not user input
	if err != nil {
		return
	}
	_ = d.Sync()  //nolint:errcheck // best-effort: not all filesystems support directory sync
	_ = d.Close() //nolint:errcheck // best-effort
}
