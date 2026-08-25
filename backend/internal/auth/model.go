// Package auth owns owner identity and login sessions.
//
// The types and rules here are plain Go. Nothing in this file imports a database
// driver or an HTTP framework, so the same logic backs the API and the CLI. The
// HTTP adapter lives beside it and depends on it, never the reverse.
package auth

import (
	"errors"
	"net/netip"
	"time"
)

// Sentinel errors. Callers compare with errors.Is rather than matching strings.
//
// These exist so the driver's errors stop at the repository. pgx.ErrNoRows means
// "no row came back", which is a storage fact; ErrUserNotFound is the domain
// fact. Letting the former through would make every layer above depend on which
// driver is in use.
var (
	// ErrUserNotFound reports that no user matches the given identifier.
	ErrUserNotFound = errors.New("auth: user not found")

	// ErrSessionNotFound reports that no session matches the given token.
	ErrSessionNotFound = errors.New("auth: session not found")

	// ErrSessionExpired reports that a session exists but its lifetime ended.
	// Kept apart from ErrSessionNotFound on purpose: the two are different
	// events, and only one of them suggests the user was logged in recently.
	// Both still produce the same 401 at the HTTP boundary.
	ErrSessionExpired = errors.New("auth: session expired")

	// ErrInvalidCredentials reports a failed login.
	//
	// One error covers both "no such user" and "wrong password". The caller
	// cannot tell them apart, so a response built from it cannot leak whether an
	// account exists.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrUsernameTaken reports that the username already exists.
	ErrUsernameTaken = errors.New("auth: username already taken")
)

// User is the owner account as the domain sees it.
//
// There is no password hash field. A hash is needed at exactly one moment, when
// a login is verified, and Credentials carries it there. Keeping it out of this
// type means a User value cannot leak one by being logged or serialised.
type User struct {
	ID          int64
	Username    string
	DisplayName string
	// Role is stored and reported but never consulted. Authentication asks only
	// whether a session is valid. Code that branches on this field would be
	// role-based access control, which this project has not adopted.
	Role      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Credentials pairs a user with the stored hash needed to verify a password.
//
// Separate from User so that a password hash travels only on the login path.
type Credentials struct {
	User         User
	PasswordHash string
}

// Session is a login session as the domain sees it.
//
// TokenHash is present; the plaintext token is not. The plaintext exists once,
// in the response that sets the cookie, and is never stored or returned again.
type Session struct {
	ID        int64
	UserID    int64
	TokenHash []byte
	ExpiresAt time.Time
	CreatedAt time.Time

	// UserAgent and IP are recorded for review only. Authentication never
	// compares them: pinning a session to an IP ends the login when a phone
	// changes network, and pinning to a user agent ends it when the browser
	// updates itself. Both misfire far more often than they stop an attacker who
	// already holds the token.
	UserAgent string
	IP        netip.Addr
}

// IsExpired reports whether the session's lifetime has ended at the given time.
//
// The clock is a parameter rather than a call to time.Now inside, so tests can
// check the boundary without sleeping.
func (s Session) IsExpired(now time.Time) bool {
	return !s.ExpiresAt.After(now)
}

// NeedsRenewal reports whether less than half of lifetime remains.
//
// Renewing on every request would turn each read of a protected endpoint into a
// database write. Renewing at the halfway point keeps an active session alive
// while writing on a small fraction of requests.
func (s Session) NeedsRenewal(now time.Time, lifetime time.Duration) bool {
	if s.IsExpired(now) {
		return false
	}
	return s.ExpiresAt.Sub(now) < lifetime/2
}

// Authenticated is what the HTTP layer receives for a valid request: the user,
// plus the session it arrived on.
type Authenticated struct {
	User    User
	Session Session

	// Renewed reports that this request extended the session's expiry.
	//
	// The HTTP adapter needs it to decide whether to write a fresh cookie, and it
	// cannot work that out from ExpiresAt alone: a session renewed a moment ago
	// and one issued a moment ago look identical. Without this the cookie would
	// keep its original Max-Age while the row slid forward, and the browser would
	// stop sending a token the server still accepts.
	//
	// Set by the service, not the repository, because renewal is a decision.
	Renewed bool
}
