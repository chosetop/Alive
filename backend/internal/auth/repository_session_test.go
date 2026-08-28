package auth_test

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
)

// openSession creates a session for user and returns the plaintext token with it.
func openSession(t *testing.T, repo *auth.Repository, userID int64, expiresAt time.Time) (auth.Token, auth.Session) {
	t.Helper()

	token, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	session, err := repo.CreateSession(context.Background(), auth.CreateSessionParams{
		UserID:            userID,
		TokenHash:         hash,
		ExpiresAt:         expiresAt,
		AbsoluteExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		UserAgent:         "curl/8.4.0",
		IP:                netip.MustParseAddr("127.0.0.1"),
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	return token, session
}

func TestRepositoryCreateSession(t *testing.T) {
	repo, _ := newTestRepository(t)
	user := seedOwner(t, repo, "owner")

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, session := openSession(t, repo, user.ID, expiresAt)

	if session.ID == 0 {
		t.Error("CreateSession returned id 0")
	}
	if session.UserID != user.ID {
		t.Errorf("user_id = %d, want %d", session.UserID, user.ID)
	}
	// Postgres stores microseconds, so an exact comparison would fail on the
	// nanosecond part.
	if diff := session.ExpiresAt.Sub(expiresAt); diff > time.Millisecond || diff < -time.Millisecond {
		t.Errorf("expires_at is off by %v", diff)
	}
}

func TestRepositoryGetSessionByHash(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	user := seedOwner(t, repo, "owner")

	token, session := openSession(t, repo, user.ID, time.Now().Add(time.Hour))

	t.Run("returns session and user together", func(t *testing.T) {
		got, err := repo.GetSessionByHash(ctx, auth.HashToken(token))
		if err != nil {
			t.Fatalf("GetSessionByHash: %v", err)
		}
		if got.Session.ID != session.ID {
			t.Errorf("session id = %d, want %d", got.Session.ID, session.ID)
		}
		if got.User.ID != user.ID {
			t.Errorf("user id = %d, want %d", got.User.ID, user.ID)
		}
		if got.User.Username != user.Username {
			t.Errorf("username = %q, want %q", got.User.Username, user.Username)
		}
		// Recorded for review. Authentication does not compare either value.
		if got.Session.UserAgent != "curl/8.4.0" {
			t.Errorf("user_agent = %q, want %q", got.Session.UserAgent, "curl/8.4.0")
		}
		if got.Session.IP.String() != "127.0.0.1" {
			t.Errorf("ip = %q, want %q", got.Session.IP, "127.0.0.1")
		}
	})

	t.Run("unknown token becomes ErrSessionNotFound", func(t *testing.T) {
		other, _, err := auth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		_, err = repo.GetSessionByHash(ctx, auth.HashToken(other))
		if !errors.Is(err, auth.ErrSessionNotFound) {
			t.Errorf("error = %v, want ErrSessionNotFound", err)
		}
	})

	// The property the SQL was written for. An expired session must come back so
	// the service can tell "your session ended" from "no such session".
	t.Run("an expired session is still returned", func(t *testing.T) {
		expiredToken, _ := openSession(t, repo, user.ID, time.Now().Add(-time.Hour))

		got, err := repo.GetSessionByHash(ctx, auth.HashToken(expiredToken))
		if err != nil {
			t.Fatalf("GetSessionByHash on an expired session: %v", err)
		}
		if !got.Session.IsExpired(time.Now()) {
			t.Error("the returned session is not reported as expired")
		}
	})
}

func TestRepositoryTouchSession(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	user := seedOwner(t, repo, "owner")

	original := time.Now().Add(time.Hour)
	token, session := openSession(t, repo, user.ID, original)

	extended := original.Add(24 * time.Hour)
	if err := repo.TouchSession(ctx, session.ID, extended); err != nil {
		t.Fatalf("TouchSession: %v", err)
	}

	got, err := repo.GetSessionByHash(ctx, auth.HashToken(token))
	if err != nil {
		t.Fatalf("GetSessionByHash: %v", err)
	}
	if !got.Session.ExpiresAt.After(original) {
		t.Errorf("expires_at = %v, want later than %v", got.Session.ExpiresAt, original)
	}
}

func TestRepositoryDeleteSession(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	user := seedOwner(t, repo, "owner")

	t.Run("delete by hash ends the session", func(t *testing.T) {
		token, _ := openSession(t, repo, user.ID, time.Now().Add(time.Hour))

		if err := repo.DeleteSessionByHash(ctx, auth.HashToken(token)); err != nil {
			t.Fatalf("DeleteSessionByHash: %v", err)
		}

		_, err := repo.GetSessionByHash(ctx, auth.HashToken(token))
		if !errors.Is(err, auth.ErrSessionNotFound) {
			t.Errorf("error = %v, want ErrSessionNotFound after delete", err)
		}
	})

	t.Run("delete by id ends the session", func(t *testing.T) {
		token, session := openSession(t, repo, user.ID, time.Now().Add(time.Hour))

		if err := repo.DeleteSession(ctx, session.ID); err != nil {
			t.Fatalf("DeleteSession: %v", err)
		}

		_, err := repo.GetSessionByHash(ctx, auth.HashToken(token))
		if !errors.Is(err, auth.ErrSessionNotFound) {
			t.Errorf("error = %v, want ErrSessionNotFound after delete", err)
		}
	})

	// Logging out twice is not an error. The caller wants the session gone, and
	// after the first call it is.
	t.Run("deleting an absent session succeeds", func(t *testing.T) {
		absent, _, err := auth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		if err := repo.DeleteSessionByHash(ctx, auth.HashToken(absent)); err != nil {
			t.Errorf("DeleteSessionByHash on an absent session = %v, want nil", err)
		}
		if err := repo.DeleteSession(ctx, 999999); err != nil {
			t.Errorf("DeleteSession on an absent id = %v, want nil", err)
		}
	})
}

func TestRepositoryDeleteExpiredSessions(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()
	user := seedOwner(t, repo, "owner")

	liveToken, _ := openSession(t, repo, user.ID, time.Now().Add(time.Hour))
	openSession(t, repo, user.ID, time.Now().Add(-time.Hour))
	openSession(t, repo, user.ID, time.Now().Add(-48*time.Hour))

	removed, err := repo.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("DeleteExpiredSessions: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}

	// The live session must survive.
	if _, err := repo.GetSessionByHash(ctx, auth.HashToken(liveToken)); err != nil {
		t.Errorf("the live session was removed: %v", err)
	}
}

// TestRepositoryDeleteUserCascadesSessions checks the foreign key's ON DELETE
// CASCADE, so a removed account cannot leave usable credentials behind.
func TestRepositoryDeleteUserCascadesSessions(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()
	user := seedOwner(t, repo, "owner")

	token, _ := openSession(t, repo, user.ID, time.Now().Add(time.Hour))

	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	_, err := repo.GetSessionByHash(ctx, auth.HashToken(token))
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("error = %v, want ErrSessionNotFound; the cascade did not fire", err)
	}
}
