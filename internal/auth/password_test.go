//nolint:testpackage // tests cover unexported decode + private params.
package auth

import (
	"strings"
	"testing"
)

func TestHash_PHCFormat(t *testing.T) {
	t.Parallel()
	h, err := Hash("hunter2")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !strings.HasPrefix(h, "$argon2id$") {
		t.Errorf("hash %q missing $argon2id$ prefix", h)
	}
	if strings.Count(h, "$") != 5 {
		t.Errorf("hash %q does not have 5 `$` separators (PHC format)", h)
	}
}

func TestHash_RandomSalt(t *testing.T) {
	t.Parallel()
	a, _ := Hash("hunter2")
	b, _ := Hash("hunter2")
	if a == b {
		t.Error("two hashes of the same password should differ (random salt)")
	}
}

func TestVerify_CorrectPassword(t *testing.T) {
	t.Parallel()
	h, _ := Hash("correct horse battery staple")
	ok, err := Verify("correct horse battery staple", h)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !ok {
		t.Error("Verify returned false for correct password")
	}
}

func TestVerify_WrongPassword(t *testing.T) {
	t.Parallel()
	h, _ := Hash("hunter2")
	ok, err := Verify("hunter3", h)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Error("Verify returned true for wrong password")
	}
}

func TestVerify_MalformedHash(t *testing.T) {
	t.Parallel()
	cases := []string{
		"",
		"not-a-hash",
		"$argon2id$",
		"$argon2id$v=19$m=garbage,t=3,p=4$abc$def",
		"$bcrypt$v=2$m=64$abc$def",
		"$argon2id$v=99$m=65536,t=3,p=4$YWFhYWFhYWFhYWFhYWFhYQ$YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE",
	}
	for _, h := range cases {
		t.Run(h, func(t *testing.T) {
			t.Parallel()
			ok, err := Verify("any", h)
			if ok {
				t.Errorf("Verify returned true for malformed hash %q", h)
			}
			if err == nil {
				t.Errorf("Verify returned no error for malformed hash %q", h)
			}
		})
	}
}

func TestVerify_TamperedHash(t *testing.T) {
	t.Parallel()
	h, _ := Hash("hunter2")
	// Flip a char 5 positions from the end — well inside the encoded key,
	// avoiding the trailing 2 padding bits of base64 RawStdEncoding.
	pos := len(h) - 5
	tampered := h[:pos] + flipChar(h[pos]) + h[pos+1:]
	ok, _ := Verify("hunter2", tampered)
	if ok {
		t.Error("tampered hash should not verify")
	}
}

func TestNeedsRehash_CurrentParams(t *testing.T) {
	t.Parallel()
	h, _ := Hash("hunter2")
	if NeedsRehash(h) {
		t.Error("freshly-issued hash should not need rehashing")
	}
}

func TestNeedsRehash_OldParams(t *testing.T) {
	t.Parallel()
	old := "$argon2id$v=19$m=4096,t=1,p=1$YWFhYWFhYWFhYWFhYWFhYQ$YWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWE"
	if !NeedsRehash(old) {
		t.Error("hash with weaker params should need rehashing")
	}
}

func TestNeedsRehash_MalformedHash(t *testing.T) {
	t.Parallel()
	if !NeedsRehash("garbage") {
		t.Error("malformed hash should need rehashing (treated as upgrade-required)")
	}
}

func TestHash_OversizedPasswordRejected(t *testing.T) {
	t.Parallel()
	huge := strings.Repeat("a", maxPasswordBytes+1)
	if _, err := Hash(huge); err == nil {
		t.Error("Hash should reject password larger than max")
	}
}

func flipChar(c byte) string {
	if c == 'A' {
		return "B"
	}
	return "A"
}
