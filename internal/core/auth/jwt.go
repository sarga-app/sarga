package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	DefaultAccessTTL  = 30 * time.Minute
	DefaultRefreshTTL = 30 * 24 * time.Hour

	signingMethod = "HS256"
)

var ErrInvalidToken = errors.New("invalid token")

// AccessClaims namespaces custom claims under "sarga." to avoid collisions.
type AccessClaims struct {
	jwt.RegisteredClaims
	IsAdmin bool `json:"sarga.is_admin"`
}

// JWTConfig is constructed once at startup and passed to Issue/Parse.
type JWTConfig struct {
	Secret    []byte
	Issuer    string
	Audience  string
	AccessTTL time.Duration
	Clock     func() time.Time
}

func (c JWTConfig) now() time.Time {
	if c.Clock != nil {
		return c.Clock()
	}
	return time.Now()
}

func IssueAccessToken(cfg JWTConfig, userID uuid.UUID, isAdmin bool) (string, error) {
	if len(cfg.Secret) == 0 {
		return "", errors.New("jwt: empty secret")
	}
	now := cfg.now()
	claims := AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{cfg.Audience},
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.AccessTTL)),
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
		jwt.WithIssuer(cfg.Issuer),
		jwt.WithAudience(cfg.Audience),
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
