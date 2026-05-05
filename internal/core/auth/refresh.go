package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const refreshTokenRawBytes = 32

// RefreshToken bundles the plaintext form sent to the client with the SHA-256
// hash stored in the DB. The plaintext never touches the database.
type RefreshToken struct {
	Plaintext string // base64url, sent to client (HttpOnly cookie)
	Hash      []byte // 32-byte SHA-256, stored in refresh_tokens.token_hash
}

// IssueRefreshToken generates a fresh, cryptographically random refresh token.
// SHA-256 (not argon2) because the plaintext is high-entropy random bytes —
// brute-force resistance is unnecessary; speed matters for the verify path.
func IssueRefreshToken() (RefreshToken, error) {
	raw := make([]byte, refreshTokenRawBytes)
	if _, err := rand.Read(raw); err != nil {
		return RefreshToken{}, fmt.Errorf("read random: %w", err)
	}
	sum := sha256.Sum256(raw)
	return RefreshToken{
		Plaintext: base64.RawURLEncoding.EncodeToString(raw),
		Hash:      sum[:],
	}, nil
}

// HashRefreshToken re-derives the storage hash from a client-supplied
// plaintext, used during the refresh flow to look up the token row.
func HashRefreshToken(plaintext string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(plaintext)
	if err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}
	if len(raw) != refreshTokenRawBytes {
		return nil, fmt.Errorf("token: expected %d bytes, got %d", refreshTokenRawBytes, len(raw))
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}
