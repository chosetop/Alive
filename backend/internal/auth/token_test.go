package auth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/auth"
)

func TestGenerateToken(t *testing.T) {
	token, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	t.Run("token decodes to 32 bytes", func(t *testing.T) {
		raw, err := base64.RawURLEncoding.DecodeString(string(token))
		if err != nil {
			t.Fatalf("token is not url-safe base64: %v", err)
		}
		if len(raw) != 32 {
			t.Errorf("token decodes to %d bytes, want 32", len(raw))
		}
	})

	t.Run("token is cookie-safe", func(t *testing.T) {
		// Raw URL encoding must not emit "+", "/" or "=", each of which would
		// need escaping in a cookie value.
		for _, c := range []string{"+", "/", "="} {
			if strings.Contains(string(token), c) {
				t.Errorf("token contains %q, which is not cookie-safe: %q", c, token)
			}
		}
	})

	t.Run("returned hash matches the token", func(t *testing.T) {
		if !auth.EqualTokenHash(hash, auth.HashToken(token)) {
			t.Error("the returned hash is not the hash of the returned token")
		}
	})

	t.Run("hash is 32 bytes", func(t *testing.T) {
		if len(hash) != auth.TokenHashLength {
			t.Errorf("hash is %d bytes, want %d", len(hash), auth.TokenHashLength)
		}
	})
}

// TestGenerateTokenIsUnique is a smoke test for the randomness source. A stuck
// generator is the kind of failure that makes every session share one token.
func TestGenerateTokenIsUnique(t *testing.T) {
	const count = 1000

	seen := make(map[auth.Token]struct{}, count)
	for i := 0; i < count; i++ {
		token, _, err := auth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}
		if _, duplicate := seen[token]; duplicate {
			t.Fatalf("GenerateToken produced a duplicate after %d calls", i)
		}
		seen[token] = struct{}{}
	}
}

// TestHashTokenIsDeterministic is the property that makes lookup by digest work.
// If hashing were salted, the same cookie would produce a different value each
// time and no session could ever be found.
func TestHashTokenIsDeterministic(t *testing.T) {
	const token auth.Token = "a-fixed-token-value"

	first := auth.HashToken(token)
	second := auth.HashToken(token)

	if !auth.EqualTokenHash(first, second) {
		t.Error("HashToken returned different digests for the same token")
	}

	// Pinned against the standard library so the stored format cannot change
	// silently: every existing session row would stop matching.
	want := sha256.Sum256([]byte(token))
	if !auth.EqualTokenHash(first, want[:]) {
		t.Error("HashToken does not agree with crypto/sha256")
	}
}

func TestHashTokenDiffersPerToken(t *testing.T) {
	a := auth.HashToken("token-one")
	b := auth.HashToken("token-two")

	if auth.EqualTokenHash(a, b) {
		t.Error("two different tokens produced the same digest")
	}
}

func TestValidateTokenFormat(t *testing.T) {
	valid, _, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	tests := []struct {
		name    string
		token   auth.Token
		wantErr bool
	}{
		{
			name:    "freshly generated token",
			token:   valid,
			wantErr: false,
		},
		{
			name:    "empty",
			token:   "",
			wantErr: true,
		},
		{
			name:    "not base64",
			token:   "!!!not-base64!!!",
			wantErr: true,
		},
		{
			// 16 bytes encoded: well-formed base64, wrong length. Accepting it
			// would let a caller supply a token with half the intended entropy.
			name:    "too short",
			token:   auth.Token(base64.RawURLEncoding.EncodeToString(make([]byte, 16))),
			wantErr: true,
		},
		{
			name:    "too long",
			token:   auth.Token(base64.RawURLEncoding.EncodeToString(make([]byte, 64))),
			wantErr: true,
		},
		{
			// Standard base64 padding is not part of the raw URL encoding.
			name:    "padded base64",
			token:   auth.Token(base64.StdEncoding.EncodeToString(make([]byte, 32))),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := auth.ValidateTokenFormat(tc.token)

			if tc.wantErr {
				if !errors.Is(err, auth.ErrInvalidToken) {
					t.Errorf("error = %v, want ErrInvalidToken", err)
				}
				return
			}
			if err != nil {
				t.Errorf("error = %v, want nil", err)
			}
		})
	}
}

func TestEqualTokenHash(t *testing.T) {
	a := auth.HashToken("same")
	b := auth.HashToken("same")
	c := auth.HashToken("different")

	if !auth.EqualTokenHash(a, b) {
		t.Error("equal digests reported as unequal")
	}
	if auth.EqualTokenHash(a, c) {
		t.Error("different digests reported as equal")
	}
	if auth.EqualTokenHash(a, nil) {
		t.Error("a digest and nil reported as equal")
	}
	if auth.EqualTokenHash(a, a[:16]) {
		t.Error("digests of different lengths reported as equal")
	}
}
