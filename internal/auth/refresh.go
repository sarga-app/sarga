package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

const refreshTokenRawBytes = 32

// RefreshToken: plaintext goes to the client (HttpOnly cookie); hash to the DB.
type RefreshToken struct {
	Plaintext string
	Hash      []byte
}

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
