//nolint:testpackage // covers the unexported byte-length constant indirectly via behaviour.
package auth

import (
	"bytes"
	"testing"
)

func TestIssueRefreshToken_Shape(t *testing.T) {
	t.Parallel()
	rt, err := IssueRefreshToken()
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if len(rt.Hash) != 32 {
		t.Errorf("Hash length = %d, want 32 (sha256)", len(rt.Hash))
	}
	if rt.Plaintext == "" {
		t.Error("Plaintext is empty")
	}
}

func TestIssueRefreshToken_Random(t *testing.T) {
	t.Parallel()
	a, _ := IssueRefreshToken()
	b, _ := IssueRefreshToken()
	if a.Plaintext == b.Plaintext {
		t.Error("two refresh tokens should differ")
	}
	if bytes.Equal(a.Hash, b.Hash) {
		t.Error("two refresh hashes should differ")
	}
}

func TestHashRefreshToken_RoundTrip(t *testing.T) {
	t.Parallel()
	rt, _ := IssueRefreshToken()
	h, err := HashRefreshToken(rt.Plaintext)
	if err != nil {
		t.Fatalf("HashRefreshToken: %v", err)
	}
	if !bytes.Equal(h, rt.Hash) {
		t.Error("hash of plaintext does not match issued hash")
	}
}

func TestHashRefreshToken_RejectsMalformed(t *testing.T) {
	t.Parallel()
	if _, err := HashRefreshToken("not-base64!!!"); err == nil {
		t.Error("expected error for non-base64 input")
	}
	// Wrong length (16 bytes encoded).
	if _, err := HashRefreshToken("YWFhYWFhYWFhYWFhYWFhYQ"); err == nil {
		t.Error("expected error for wrong-length token")
	}
}
