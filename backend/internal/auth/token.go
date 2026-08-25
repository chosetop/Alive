package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
)

// Session tokens.
//
// A token is 32 bytes from crypto/rand, handed to the browser in a cookie and
// stored only as its SHA-256 digest.
//
// SHA-256 here, Argon2id for passwords, and the difference is deliberate. A slow
// hash defends a low-entropy secret against guessing; a person's password has
// perhaps 40 bits of entropy, so each guess must be made expensive. This token
// has 256 bits from a cryptographic source, which puts guessing out of reach
// whatever the hash costs. Using Argon2id here would add 200 ms to every
// authenticated request and buy nothing.
//
// Hashing at all is what limits the damage from a leaked database: the rows hold
// digests, and a digest cannot be replayed as a cookie.

const (
	// tokenLength is 32 bytes, 256 bits.
	tokenLength = 32

	// TokenHashLength is the size of a stored digest, in bytes.
	TokenHashLength = sha256.Size
)

// ErrInvalidToken reports a token that cannot have come from GenerateToken.
//
// Rejecting it before any database work means a malformed cookie costs one length
// check rather than a query.
var ErrInvalidToken = errors.New("auth: session token is malformed")

// Token is a plaintext session token.
//
// A distinct type, not a string, so that a plaintext token cannot be passed
// where a digest is expected. The compiler catches the mix-up that would
// otherwise write a usable credential into the database.
type Token string

// GenerateToken returns a new random token and the digest to store.
//
// Returning both together is what stops the plaintext from being written by
// accident: the caller never has to hash anything itself.
func GenerateToken() (Token, []byte, error) {
	raw := make([]byte, tokenLength)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("auth: read random bytes: %w", err)
	}

	// URL-safe, unpadded base64: the value travels in a cookie, and "+" or "/"
	// would need escaping.
	token := Token(base64.RawURLEncoding.EncodeToString(raw))

	return token, HashToken(token), nil
}

// HashToken returns the SHA-256 digest of token.
//
// Deterministic and unsalted, unlike password hashing. Authentication looks a
// session up *by* its digest, so the same token must always produce the same
// value. A per-row salt would make that lookup impossible: finding the right
// salt would require finding the row first.
func HashToken(token Token) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}

// ValidateTokenFormat reports whether token could have come from GenerateToken.
//
// A cheap gate in front of the database. It says nothing about whether the token
// is known or current.
func ValidateTokenFormat(token Token) error {
	if token == "" {
		return fmt.Errorf("%w: empty", ErrInvalidToken)
	}

	raw, err := base64.RawURLEncoding.DecodeString(string(token))
	if err != nil {
		return fmt.Errorf("%w: not url-safe base64", ErrInvalidToken)
	}
	if len(raw) != tokenLength {
		return fmt.Errorf("%w: decodes to %d bytes, want %d", ErrInvalidToken, len(raw), tokenLength)
	}

	return nil
}

// EqualTokenHash compares two digests in constant time.
//
// Session lookup goes through the database index, so this is not on the hot
// path; it exists for the places that compare digests in Go, where a timing
// difference would leak how many leading bytes matched.
func EqualTokenHash(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}
