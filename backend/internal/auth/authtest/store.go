// Package authtest provides an in-memory auth.Store for tests.
//
// It lives in its own package because two test packages need it: internal/auth
// tests the rules against it, and internal/authhttp tests the HTTP surface over
// the real service. A copy in each would drift, and a field added to the domain
// would be handled by one fake and not the other.
//
// Under internal/, so nothing outside this module can depend on it.
package authtest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
)

// fakeStore is an in-memory Store.
//
// The service's rules are decisions, not queries: whether a wrong password is
// distinguishable from a missing user, whether an expired session is refused,
// whether renewal happens at the right moment. Testing those against a real
// database would mean waiting on I/O to observe logic that never touches it.
//
// The repository's own translation of driver errors is tested against a real
// database in repository_test.go, which is where that belongs.
type Store struct {
	users    map[int64]auth.User
	hashes   map[int64]string
	sessions map[string]auth.Session

	nextUserID    int64
	nextSessionID int64

	// Injected failures, one per method that has a distinct failure path.
	FailGetCredentials error
	FailCreateSession  error
	FailTouchSession   error
	FailDeleteByHash   error

	// Call counts, for asserting that something did or did not happen.
	GetSessionCalls int
	TouchCalls      int
	CreateCalls     int
	DeleteCalls     int
}

// Checked at compile time. If auth.Store gains a method, this fails to build
// rather than every test that uses the fake failing with a confusing message.
var _ auth.Store = (*Store)(nil)

func NewStore() *Store {
	return &Store{
		users:         make(map[int64]auth.User),
		hashes:        make(map[int64]string),
		sessions:      make(map[string]auth.Session),
		nextUserID:    1,
		nextSessionID: 1,
	}
}

// key indexes sessions by digest. A map needs a comparable key and []byte is not.
func key(tokenHash []byte) string {
	return fmt.Sprintf("%x", tokenHash)
}

func (f *Store) CreateUser(_ context.Context, params auth.CreateUserParams) (auth.User, error) {
	for _, u := range f.users {
		// citext in the real column; case-insensitive here for the same reason.
		if strings.EqualFold(u.Username, params.Username) {
			return auth.User{}, fmt.Errorf("%w: %s", auth.ErrUsernameTaken, params.Username)
		}
	}

	role := params.Role
	if role == "" {
		role = "owner"
	}

	now := time.Now()
	user := auth.User{
		ID:          f.nextUserID,
		Username:    params.Username,
		DisplayName: params.DisplayName,
		Role:        role,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	f.users[user.ID] = user
	f.hashes[user.ID] = params.PasswordHash
	f.nextUserID++

	return user, nil
}

func (f *Store) GetCredentialsByUsername(_ context.Context, username string) (auth.Credentials, error) {
	if f.FailGetCredentials != nil {
		return auth.Credentials{}, f.FailGetCredentials
	}
	for id, u := range f.users {
		if strings.EqualFold(u.Username, username) {
			return auth.Credentials{User: u, PasswordHash: f.hashes[id]}, nil
		}
	}
	return auth.Credentials{}, fmt.Errorf("%w: %s", auth.ErrUserNotFound, username)
}

func (f *Store) GetUserByID(_ context.Context, id int64) (auth.User, error) {
	u, ok := f.users[id]
	if !ok {
		return auth.User{}, fmt.Errorf("%w: id %d", auth.ErrUserNotFound, id)
	}
	return u, nil
}

func (f *Store) CreateSession(_ context.Context, params auth.CreateSessionParams) (auth.Session, error) {
	f.CreateCalls++
	if f.FailCreateSession != nil {
		return auth.Session{}, f.FailCreateSession
	}

	session := auth.Session{
		ID:        f.nextSessionID,
		UserID:    params.UserID,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt,
		CreatedAt: time.Now(),
		UserAgent: params.UserAgent,
		IP:        params.IP,
	}
	f.sessions[key(params.TokenHash)] = session
	f.nextSessionID++

	return session, nil
}

func (f *Store) GetSessionByHash(_ context.Context, tokenHash []byte) (auth.Authenticated, error) {
	f.GetSessionCalls++
	session, ok := f.sessions[key(tokenHash)]
	if !ok {
		return auth.Authenticated{}, auth.ErrSessionNotFound
	}
	// Expired sessions come back, matching the real query.
	user, ok := f.users[session.UserID]
	if !ok {
		return auth.Authenticated{}, auth.ErrSessionNotFound
	}
	return auth.Authenticated{User: user, Session: session}, nil
}

func (f *Store) TouchSession(_ context.Context, sessionID int64, expiresAt time.Time) error {
	f.TouchCalls++
	if f.FailTouchSession != nil {
		return f.FailTouchSession
	}
	for k, s := range f.sessions {
		if s.ID == sessionID {
			s.ExpiresAt = expiresAt
			f.sessions[k] = s
			return nil
		}
	}
	return nil
}

func (f *Store) DeleteSessionByHash(_ context.Context, tokenHash []byte) error {
	f.DeleteCalls++
	if f.FailDeleteByHash != nil {
		return f.FailDeleteByHash
	}
	delete(f.sessions, key(tokenHash))
	return nil
}

func (f *Store) DeleteExpiredSessions(_ context.Context) (int64, error) {
	var removed int64
	now := time.Now()
	for k, s := range f.sessions {
		if s.IsExpired(now) {
			delete(f.sessions, k)
			removed++
		}
	}
	return removed, nil
}
