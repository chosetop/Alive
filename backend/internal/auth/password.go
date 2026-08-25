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

// Password hashing with Argon2id.
//
// Argon2id rather than bcrypt for two concrete reasons:
//
//   - bcrypt silently truncates input at 72 bytes. Two passwords sharing their
//     first 72 bytes are the same password to it, and nothing reports this.
//   - bcrypt costs CPU but almost no memory, so purpose-built hardware runs it
//     in parallel cheaply. Argon2id's memory cost is what makes that expensive.
//
// Argon2id, not Argon2i or Argon2d: it is the hybrid, and the variant the RFC
// recommends when the use is password storage.

// Cost parameters. Raising these later is safe: every hash records the values it
// was produced with, so old hashes keep verifying while new ones use the new
// settings.
const (
	// argonMemory is 64 MiB, expressed in KiB as the library expects.
	argonMemory = 64 * 1024
	// argonIterations is the number of passes over memory.
	argonIterations = 3
	// argonParallelism is the number of lanes.
	argonParallelism = 2
	argonSaltLength  = 16
	argonKeyLength   = 32
)

// ErrPasswordHashInvalid reports a stored hash that cannot be parsed.
//
// This is a corrupted or hand-edited row, not a wrong password, and the two must
// not be confused: reporting it as a wrong password would hide real damage.
var ErrPasswordHashInvalid = errors.New("auth: password hash is malformed")

// HashPassword returns a PHC-encoded Argon2id hash of password.
//
// The output is self-describing, in the format shared by every Argon2
// implementation:
//
//	$argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>
//
// Storing the parameters alongside the digest is what lets the cost be raised
// without invalidating existing hashes.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}

	digest := argon2.IDKey(
		[]byte(password),
		salt,
		argonIterations,
		argonMemory,
		argonParallelism,
		argonKeyLength,
	)

	// Raw (unpadded) base64 is what the PHC string format specifies.
	encoding := base64.RawStdEncoding
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory,
		argonIterations,
		argonParallelism,
		encoding.EncodeToString(salt),
		encoding.EncodeToString(digest),
	), nil
}

// VerifyPassword reports whether password matches encodedHash.
//
// It re-derives the hash using the parameters recorded in encodedHash, so a hash
// written under older settings still verifies.
func VerifyPassword(password, encodedHash string) (bool, error) {
	params, salt, want, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	got := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(want)),
	)

	// Constant-time comparison. A byte-by-byte compare returns sooner on an
	// early mismatch, and that timing difference leaks how much of the digest
	// was correct.
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// dummyHash is a valid Argon2id hash of a value nobody uses. It is computed once
// on first use, not at package load, so a process that never logs in does not
// pay for it.
var dummyHash string

// VerifyPasswordDummy performs a hash computation and discards the result.
//
// Login calls this when no such user exists. Without it, a request for a missing
// username returns in microseconds while a request for a real one takes the ~50
// ms that Argon2id costs, and that gap tells an attacker which usernames are
// real. Doing the work anyway makes both paths take the same time.
func VerifyPasswordDummy(password string) {
	if dummyHash == "" {
		h, err := HashPassword("dummy-password-for-timing-equalisation")
		if err != nil {
			// Fall back to a direct derivation. The point is to spend the time,
			// so any Argon2id call of the right cost will do.
			argon2.IDKey([]byte(password), make([]byte, argonSaltLength),
				argonIterations, argonMemory, argonParallelism, argonKeyLength)
			return
		}
		dummyHash = h
	}

	_, _ = VerifyPassword(password, dummyHash)
}

// hashParams holds the cost settings read back from an encoded hash.
type hashParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

// decodeHash parses a PHC-encoded Argon2id string.
func decodeHash(encoded string) (hashParams, []byte, []byte, error) {
	// $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
	// A leading empty field comes from the leading "$".
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: expected 6 fields, got %d", ErrPasswordHashInvalid, len(parts))
	}

	if parts[1] != "argon2id" {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: algorithm is %q, want argon2id", ErrPasswordHashInvalid, parts[1])
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: unreadable version %q", ErrPasswordHashInvalid, parts[2])
	}
	if version != argon2.Version {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: version %d is not supported", ErrPasswordHashInvalid, version)
	}

	var params hashParams
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d",
		&params.memory, &params.iterations, &params.parallelism); err != nil {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: unreadable parameters %q", ErrPasswordHashInvalid, parts[3])
	}

	encoding := base64.RawStdEncoding

	salt, err := encoding.DecodeString(parts[4])
	if err != nil {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: undecodable salt", ErrPasswordHashInvalid)
	}

	digest, err := encoding.DecodeString(parts[5])
	if err != nil {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: undecodable digest", ErrPasswordHashInvalid)
	}
	if len(digest) == 0 {
		return hashParams{}, nil, nil, fmt.Errorf(
			"%w: digest is empty", ErrPasswordHashInvalid)
	}

	return params, salt, digest, nil
}
