package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/postgres"
)

// The repository's job is translating storage facts into domain facts, and that
// translation only exists against a real database: a fake would return whatever
// errors the fake was written to return.
//
// Skipped unless TEST_DATABASE_URL is set, and refused outright unless it names a
// database ending in _test. Both rules live in dbtest.
func newTestRepository(t *testing.T) (*auth.Repository, *postgres.Pool) {
	t.Helper()

	pool := dbtest.Pool(t)

	// Removes only the accounts this process created, rather than truncating the
	// table. `go test ./...` runs packages in parallel, and a truncate here would
	// delete rows another package is using: see the note in dbtest.
	dbtest.CleanupUsers(t, pool)

	return auth.NewRepository(pool), pool
}

// seedOwner creates one account with a generated name and returns it.
//
// The name is generated rather than passed in: every account in this database has
// to be unique, and a literal would collide with a package running in parallel.
// Tests that care about the name read it from the returned user.
func seedOwner(t *testing.T, repo *auth.Repository, label string) auth.User {
	t.Helper()

	username := dbtest.Username(t, label)

	hash, err := auth.HashPassword("owner-password")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	user, err := repo.CreateUser(context.Background(), auth.CreateUserParams{
		Username:     username,
		PasswordHash: hash,
		DisplayName:  "The Owner",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user
}

func TestRepositoryCreateUser(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	user := seedOwner(t, repo, "owner")

	if user.ID == 0 {
		t.Error("CreateUser returned id 0")
	}
	if user.Role != "owner" {
		t.Errorf("role = %q, want %q (the default)", user.Role, "owner")
	}
	if user.DisplayName != "The Owner" {
		t.Errorf("display_name = %q, want %q", user.DisplayName, "The Owner")
	}

	t.Run("duplicate username becomes ErrUsernameTaken", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, auth.CreateUserParams{
			Username:     user.Username,
			PasswordHash: "x",
		})
		// Without translation this would be a *pgconn.PgError with code 23505,
		// and every caller would need to know that.
		if !errors.Is(err, auth.ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("username differing only in case is also taken", func(t *testing.T) {
		_, err := repo.CreateUser(ctx, auth.CreateUserParams{
			Username:     strings.ToUpper(user.Username),
			PasswordHash: "x",
		})
		if !errors.Is(err, auth.ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	t.Run("empty display name is stored as NULL and read back as empty", func(t *testing.T) {
		created, err := repo.CreateUser(ctx, auth.CreateUserParams{
			Username:     dbtest.Username(t, "nodisplayname"),
			PasswordHash: "x",
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if created.DisplayName != "" {
			t.Errorf("display_name = %q, want empty", created.DisplayName)
		}
	})
}

func TestRepositoryGetCredentialsByUsername(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	user := seedOwner(t, repo, "owner")

	t.Run("lookup is case-insensitive", func(t *testing.T) {
		// Stored lowercase, looked up uppercase. citext makes this match without any
		// lowercasing in Go, which is the reason the column has that type.
		creds, err := repo.GetCredentialsByUsername(ctx, strings.ToUpper(user.Username))
		if err != nil {
			t.Fatalf("GetCredentialsByUsername: %v", err)
		}
		if creds.User.ID != user.ID {
			t.Errorf("id = %d, want %d", creds.User.ID, user.ID)
		}
		if creds.PasswordHash == "" {
			t.Error("PasswordHash is empty; the login path needs it")
		}
	})

	t.Run("the returned hash verifies the real password", func(t *testing.T) {
		creds, err := repo.GetCredentialsByUsername(ctx, strings.ToUpper(user.Username))
		if err != nil {
			t.Fatalf("GetCredentialsByUsername: %v", err)
		}

		ok, err := auth.VerifyPassword("owner-password", creds.PasswordHash)
		if err != nil {
			t.Fatalf("VerifyPassword: %v", err)
		}
		if !ok {
			t.Error("the stored hash did not verify the password it was made from")
		}
	})

	t.Run("missing user becomes ErrUserNotFound", func(t *testing.T) {
		_, err := repo.GetCredentialsByUsername(ctx, "nobody")
		// pgx.ErrNoRows must not reach this far.
		if !errors.Is(err, auth.ErrUserNotFound) {
			t.Errorf("error = %v, want ErrUserNotFound", err)
		}
	})
}

func TestRepositoryGetUserByID(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	user := seedOwner(t, repo, "owner")

	got, err := repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if got.Username != user.Username {
		t.Errorf("username = %q, want %q", got.Username, user.Username)
	}

	t.Run("missing id becomes ErrUserNotFound", func(t *testing.T) {
		_, err := repo.GetUserByID(ctx, 999999)
		if !errors.Is(err, auth.ErrUserNotFound) {
			t.Errorf("error = %v, want ErrUserNotFound", err)
		}
	})
}
