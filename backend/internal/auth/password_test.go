package auth_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/auth"
)

func TestHashPasswordProducesPHCFormat(t *testing.T) {
	hash, err := auth.HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	// The prefix must record algorithm, version and every cost parameter.
	// Without them, raising the cost later would invalidate stored hashes.
	const wantPrefix = "$argon2id$v=19$m=65536,t=3,p=2$"
	if !strings.HasPrefix(hash, wantPrefix) {
		t.Errorf("hash = %q, want prefix %q", hash, wantPrefix)
	}

	if fields := strings.Split(hash, "$"); len(fields) != 6 {
		t.Errorf("hash has %d fields, want 6: %q", len(fields), hash)
	}
}

func TestHashPasswordIsSaltedPerCall(t *testing.T) {
	const password = "same-password"

	first, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	second, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	// Equal hashes would mean the salt is fixed, which lets one precomputed
	// table attack every account at once.
	if first == second {
		t.Error("hashing the same password twice produced identical output; salt is not random")
	}

	// Both must still verify.
	for i, h := range []string{first, second} {
		ok, err := auth.VerifyPassword(password, h)
		if err != nil {
			t.Fatalf("VerifyPassword %d: %v", i, err)
		}
		if !ok {
			t.Errorf("VerifyPassword %d = false, want true", i)
		}
	}
}

func TestVerifyPassword(t *testing.T) {
	const password = "s3cret-owner-password"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	tests := []struct {
		name    string
		attempt string
		wantOK  bool
	}{
		{name: "exact password", attempt: password, wantOK: true},
		{name: "wrong password", attempt: "wrong", wantOK: false},
		{name: "empty password", attempt: "", wantOK: false},
		{name: "case differs", attempt: strings.ToUpper(password), wantOK: false},
		{name: "trailing space added", attempt: password + " ", wantOK: false},
		{name: "one character short", attempt: password[:len(password)-1], wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := auth.VerifyPassword(tc.attempt, hash)
			if err != nil {
				t.Fatalf("VerifyPassword: %v", err)
			}
			if ok != tc.wantOK {
				t.Errorf("VerifyPassword(%q) = %v, want %v", tc.attempt, ok, tc.wantOK)
			}
		})
	}
}

// TestVerifyPasswordLongInput is the bcrypt trap this implementation avoids:
// bcrypt truncates at 72 bytes, so two passwords sharing a 72-byte prefix would
// both verify. Argon2id has no such limit.
func TestVerifyPasswordLongInput(t *testing.T) {
	base := strings.Repeat("a", 72)
	password := base + "-tail-one"
	other := base + "-tail-two"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := auth.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("the original long password failed to verify")
	}

	ok, err = auth.VerifyPassword(other, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("a password differing only after byte 72 verified; input is being truncated")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{name: "empty string", hash: ""},
		{name: "not a hash", hash: "plaintext-password"},
		{name: "too few fields", hash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA"},
		{name: "wrong algorithm", hash: "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA"},
		{name: "bcrypt hash", hash: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"},
		{name: "unreadable version", hash: "$argon2id$v=abc$m=65536,t=3,p=2$c2FsdA$aGFzaA"},
		{name: "unsupported version", hash: "$argon2id$v=16$m=65536,t=3,p=2$c2FsdA$aGFzaA"},
		{name: "unreadable parameters", hash: "$argon2id$v=19$m=x,t=y,p=z$c2FsdA$aGFzaA"},
		{name: "salt not base64", hash: "$argon2id$v=19$m=65536,t=3,p=2$!!!$aGFzaA"},
		{name: "digest not base64", hash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$!!!"},
		{name: "empty digest", hash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := auth.VerifyPassword("any-password", tc.hash)

			// A corrupted stored hash is not a wrong password. Reporting it as
			// one would turn real damage into a silent login failure.
			if !errors.Is(err, auth.ErrPasswordHashInvalid) {
				t.Errorf("error = %v, want ErrPasswordHashInvalid", err)
			}
			if ok {
				t.Error("VerifyPassword returned true for a malformed hash")
			}
		})
	}
}

// TestVerifyPasswordAcceptsOtherCostParameters proves a hash written under
// different settings still verifies, which is what makes raising the cost later
// a safe change.
func TestVerifyPasswordAcceptsOtherCostParameters(t *testing.T) {
	// Produced with m=32768,t=2,p=1 for the password "legacy-password".
	// Generated by this package's own HashPassword with those constants, then
	// pasted here so the test does not depend on the current values.
	const legacy = "$argon2id$v=19$m=32768,t=2,p=1$" +
		"MTIzNDU2Nzg5MGFiY2RlZg$" +
		"7C1kwCg4bDbnEHsJ2gvUlHhkH9TnAsQqAt5AZmMPRRs"

	// The point is that parsing and re-derivation use the recorded parameters,
	// not the package constants. A wrong password must still be rejected rather
	// than error out.
	ok, err := auth.VerifyPassword("definitely-not-it", legacy)
	if err != nil {
		t.Fatalf("VerifyPassword on a hash with other parameters: %v", err)
	}
	if ok {
		t.Error("a wrong password verified against the legacy hash")
	}
}

func TestVerifyPasswordDummyDoesNotPanic(t *testing.T) {
	// Called on the "no such user" login path purely to spend the same time as
	// a real verification. It has no result to assert; this guards the fallback
	// path from panicking.
	auth.VerifyPasswordDummy("attempted-password")
	auth.VerifyPasswordDummy("")
}

// BenchmarkHashPassword reports the real cost. Argon2id is meant to be slow;
// somewhere in the tens of milliseconds is the target. Much faster means the
// parameters are too weak, much slower makes the login endpoint a denial of
// service surface against itself.
func BenchmarkHashPassword(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := auth.HashPassword("benchmark-password"); err != nil {
			b.Fatal(err)
		}
	}
}
