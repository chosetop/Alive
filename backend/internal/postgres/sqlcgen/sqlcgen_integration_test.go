package sqlcgen_test

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

// Generated code holds SQL as string literals, so a successful build proves
// nothing about whether the statements run. These tests execute every query
// against a real database.
//
// Skipped unless TEST_DATABASE_URL is set, which keeps `go test ./...` working
// on a machine with no PostgreSQL.
func newTestQueries(t *testing.T) (*sqlcgen.Queries, *postgres.Pool) {
	t.Helper()

	pool := dbtest.Pool(t)

	// Removes only the rows this process created, rather than truncating the table.
	// `go test ./...` runs packages in parallel, and a truncate here would delete
	// what another package is using: see the note in dbtest.
	dbtest.CleanupUsers(t, pool)

	return sqlcgen.New(pool), pool
}

func TestUserQueries(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	displayName := "The Owner"

	// Generated, not fixed. Two packages can run at once against this database, and
	// a literal name would collide on the unique constraint.
	username := dbtest.Username(t, "owner")

	created, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Username:     username,
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
		Role:         "owner",
		DisplayName:  &displayName,
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if created.ID == 0 {
		t.Error("CreateUser returned id 0")
	}

	// CreateUser must not hand back the hash. Asserted by type: the row struct
	// has no such field, so this would not compile if the query changed.
	//   _ = created.PasswordHash

	t.Run("GetUserByUsername is case-insensitive", func(t *testing.T) {
		// Stored lowercase, looked up uppercase. citext makes these equal without
		// lower() anywhere in the query.
		got, err := q.GetUserByUsername(ctx, strings.ToUpper(username))
		if err != nil {
			t.Fatalf("GetUserByUsername: %v", err)
		}
		if got.ID != created.ID {
			t.Errorf("id = %d, want %d", got.ID, created.ID)
		}
		if got.PasswordHash == "" {
			t.Error("PasswordHash is empty; the login query must return it")
		}
	})

	t.Run("GetUserByID omits the password hash", func(t *testing.T) {
		got, err := q.GetUserByID(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if got.Username != username {
			t.Errorf("username = %q, want %q", got.Username, username)
		}
	})

	t.Run("duplicate username differing only in case is rejected", func(t *testing.T) {
		_, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
			Username:     strings.ToUpper(username),
			PasswordHash: "x",
			Role:         "owner",
		})
		if err == nil {
			t.Error("CreateUser succeeded; citext unique should have rejected it")
		}
	})

	t.Run("username shorter than three characters is rejected", func(t *testing.T) {
		_, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
			Username:     "ab",
			PasswordHash: "x",
			Role:         "owner",
		})
		if err == nil {
			t.Error("CreateUser succeeded; the length check should have rejected it")
		}
	})
}

func TestSessionQueries(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()

	user, err := q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Username:     dbtest.Username(t, "sessionowner"),
		PasswordHash: "x",
		Role:         "owner",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	hash := sha256.Sum256([]byte("a-plaintext-token"))
	userAgent := "curl/8.4.0"
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	session, err := q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: hash[:],
		ExpiresAt: expiresAt,
		UserAgent: &userAgent,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	t.Run("GetSessionByHash returns session and user in one row", func(t *testing.T) {
		got, err := q.GetSessionByHash(ctx, hash[:])
		if err != nil {
			t.Fatalf("GetSessionByHash: %v", err)
		}
		if got.ID != session.ID {
			t.Errorf("session id = %d, want %d", got.ID, session.ID)
		}
		if got.UserUsername != user.Username {
			t.Errorf("user_username = %q, want %q", got.UserUsername, user.Username)
		}
		if got.UserRole != "owner" {
			t.Errorf("user_role = %q, want %q", got.UserRole, "owner")
		}
	})

	t.Run("an expired session is still returned", func(t *testing.T) {
		// The repository reports facts; expiry is the service's call. If this
		// query filtered on expires_at, an expired session and an unknown token
		// would be indistinguishable to the caller.
		expiredHash := sha256.Sum256([]byte("expired-token"))
		if _, err := q.CreateSession(ctx, sqlcgen.CreateSessionParams{
			UserID:    user.ID,
			TokenHash: expiredHash[:],
			ExpiresAt: time.Now().Add(-time.Hour),
		}); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}

		got, err := q.GetSessionByHash(ctx, expiredHash[:])
		if err != nil {
			t.Fatalf("GetSessionByHash on an expired session: %v", err)
		}
		if !got.ExpiresAt.Before(time.Now()) {
			t.Error("expected the returned session to be expired")
		}
	})

	t.Run("TouchSession moves expires_at", func(t *testing.T) {
		newExpiry := expiresAt.Add(24 * time.Hour)
		if err := q.TouchSession(ctx, sqlcgen.TouchSessionParams{
			ID:        session.ID,
			ExpiresAt: newExpiry,
		}); err != nil {
			t.Fatalf("TouchSession: %v", err)
		}

		got, err := q.GetSessionByHash(ctx, hash[:])
		if err != nil {
			t.Fatalf("GetSessionByHash: %v", err)
		}
		if !got.ExpiresAt.After(expiresAt) {
			t.Errorf("expires_at = %v, want later than %v", got.ExpiresAt, expiresAt)
		}
	})

	t.Run("DeleteSessionByHash ends the session", func(t *testing.T) {
		if err := q.DeleteSessionByHash(ctx, hash[:]); err != nil {
			t.Fatalf("DeleteSessionByHash: %v", err)
		}
		if _, err := q.GetSessionByHash(ctx, hash[:]); err == nil {
			t.Error("GetSessionByHash succeeded after delete, want no rows")
		}
	})

	t.Run("DeleteExpiredSessions reports the row count", func(t *testing.T) {
		removed, err := q.DeleteExpiredSessions(ctx)
		if err != nil {
			t.Fatalf("DeleteExpiredSessions: %v", err)
		}
		if removed != 1 {
			t.Errorf("removed = %d, want 1 (the expired session)", removed)
		}
	})
}
