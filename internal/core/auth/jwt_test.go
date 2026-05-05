//nolint:testpackage // exercises unexported AccessClaims fields.
package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func testCfg(t *testing.T) JWTConfig {
	t.Helper()
	return JWTConfig{
		Secret:   []byte("test-secret-at-least-32-bytes-long!!"),
		Issuer:   "sarga-test",
		Audience: "sarga-web",
		Clock:    func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) },
	}
}

func TestIssueAndParse_RoundTrip(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	uid := uuid.MustParse("12345678-1234-1234-1234-123456789abc")

	tok, err := IssueAccessToken(cfg, uid, true)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Errorf("token %q does not have 3 JWT segments", tok)
	}

	claims, err := ParseAccessToken(cfg, tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, _ := claims.UserID(); got != uid {
		t.Errorf("UserID = %v, want %v", got, uid)
	}
	if !claims.IsAdmin {
		t.Error("IsAdmin = false, want true")
	}
	if claims.Issuer != cfg.Issuer {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, cfg.Issuer)
	}
	if claims.ID == "" {
		t.Error("jti claim is empty")
	}
}

func TestParse_ExpiredRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	tok, _ := IssueAccessToken(cfg, uuid.New(), false)

	// Move clock past expiry.
	expired := cfg
	expired.Clock = func() time.Time { return cfg.Clock().Add(31 * time.Minute) }

	_, err := ParseAccessToken(expired, tok)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestParse_WrongIssuerRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	tok, _ := IssueAccessToken(cfg, uuid.New(), false)

	other := cfg
	other.Issuer = "evil"
	if _, err := ParseAccessToken(other, tok); err == nil {
		t.Error("expected error for wrong issuer")
	}
}

func TestParse_WrongAudienceRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	tok, _ := IssueAccessToken(cfg, uuid.New(), false)

	other := cfg
	other.Audience = "wrong"
	if _, err := ParseAccessToken(other, tok); err == nil {
		t.Error("expected error for wrong audience")
	}
}

func TestParse_WrongSecretRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	tok, _ := IssueAccessToken(cfg, uuid.New(), false)

	other := cfg
	other.Secret = []byte("different-secret-32-bytes-long!!!!!!")
	if _, err := ParseAccessToken(other, tok); err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestParse_AlgNoneRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	// Forge a token signed with alg=none — the classic attack.
	claims := jwt.RegisteredClaims{
		Issuer:    cfg.Issuer,
		Subject:   uuid.New().String(),
		Audience:  jwt.ClaimStrings{cfg.Audience},
		ExpiresAt: jwt.NewNumericDate(cfg.now().Add(time.Hour)),
		IssuedAt:  jwt.NewNumericDate(cfg.now()),
	}
	noneTok := jwt.NewWithClaims(jwt.SigningMethodNone, claims)
	signed, err := noneTok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("forge none-token: %v", err)
	}

	if _, err := ParseAccessToken(cfg, signed); err == nil {
		t.Error("alg=none token should be rejected")
	}
}

func TestParse_TamperedRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	tok, _ := IssueAccessToken(cfg, uuid.New(), false)
	pos := len(tok) - 5
	tampered := tok[:pos] + flipChar(tok[pos]) + tok[pos+1:]
	if _, err := ParseAccessToken(cfg, tampered); err == nil {
		t.Error("tampered token should be rejected")
	}
}

func TestIssue_EmptySecretRejected(t *testing.T) {
	t.Parallel()
	cfg := testCfg(t)
	cfg.Secret = nil
	if _, err := IssueAccessToken(cfg, uuid.New(), false); err == nil {
		t.Error("Issue with empty secret should error")
	}
}

// Regression: production server constructs JWTConfig with Secret only and
// relies on issuer()/audience() defaulting on both Issue AND Parse paths.
// Earlier, Parse used the raw cfg.Issuer/cfg.Audience (empty strings) and
// rejected every token.
func TestIssueAndParse_DefaultIssuerAudience(t *testing.T) {
	t.Parallel()
	cfg := JWTConfig{Secret: []byte("test-secret-at-least-32-bytes-long!!")}
	uid := uuid.New()

	tok, err := IssueAccessToken(cfg, uid, true)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	claims, err := ParseAccessToken(cfg, tok)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if claims.Issuer != DefaultIssuer {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, DefaultIssuer)
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != DefaultAudience {
		t.Errorf("Audience = %v, want [%q]", claims.Audience, DefaultAudience)
	}
}
