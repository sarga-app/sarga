package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// OWASP 2024 argon2id parameters.
const (
	argonMemoryKiB   uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 4
	argonSaltLen            = 16
	argonKeyLen      uint32 = 32

	// Bound input so an attacker can't DoS the hasher with huge inputs.
	maxPasswordBytes = 1024
)

var ErrInvalidHash = errors.New("invalid argon2 hash")

func Hash(password string) (string, error) {
	if len(password) > maxPasswordBytes {
		return "", fmt.Errorf("password exceeds %d bytes", maxPasswordBytes)
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemoryKiB, argonParallelism, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemoryKiB, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify returns (false, nil) on mismatch, (false, err) only on a malformed hash.
func Verify(password, encodedHash string) (bool, error) {
	if len(password) > maxPasswordBytes {
		return false, fmt.Errorf("password exceeds %d bytes", maxPasswordBytes)
	}
	params, salt, storedKey, err := decode(encodedHash)
	if err != nil {
		return false, err
	}
	derivedKey := argon2.IDKey([]byte(password), salt, params.iterations, params.memoryKiB, params.parallelism, uint32(len(storedKey))) //nolint:gosec // base64-decoded key length fits uint32
	return subtle.ConstantTimeCompare(derivedKey, storedKey) == 1, nil
}

// Call after a successful Verify to silently upgrade the stored hash.
func NeedsRehash(encodedHash string) bool {
	params, _, _, err := decode(encodedHash)
	if err != nil {
		return true
	}
	return params.memoryKiB < argonMemoryKiB ||
		params.iterations < argonIterations ||
		params.parallelism < argonParallelism
}

type hashParams struct {
	memoryKiB   uint32
	iterations  uint32
	parallelism uint8
}

func decode(encodedHash string) (params hashParams, salt, key []byte, err error) {
	// PHC: ["", "argon2id", "v=N", "m=,t=,p=", "<salt>", "<key>"]
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	var version int
	if _, scanErr := fmt.Sscanf(parts[2], "v=%d", &version); scanErr != nil {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return hashParams{}, nil, nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidHash, version)
	}
	if _, scanErr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &params.memoryKiB, &params.iterations, &params.parallelism); scanErr != nil {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return hashParams{}, nil, nil, ErrInvalidHash
	}
	return params, salt, key, nil
}
