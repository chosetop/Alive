package auth_test

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/auth/authtest"
)

const testPassword = "correct-horse-battery-staple"

// Argon2id costs roughly 200 ms per call, so the hash for the test account is
// computed once for the whole package rather than once per test.
var (
	testHashOnce sync.Once
	testHash     string
)

func ownerHash(t *testing.T) string {
	t.Helper()
	testHashOnce.Do(func() {
		h, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		testHash = h
	})
	return testHash
}

// newTestService returns a service over a fake store, with one account already
// present and the clock fixed. A fixed clock is what makes the expiry and renewal
// boundaries testable without sleeping.
func newTestService(t *testing.T, now time.Time, opts ...auth.ServiceOption) (*auth.Service, *authtest.Store, auth.User) {
	t.Helper()

	store := authtest.NewStore()
	user, err := store.CreateUser(context.Background(), auth.CreateUserParams{
		Username:     "Owner",
		PasswordHash: ownerHash(t),
		DisplayName:  "The Owner",
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	opts = append([]auth.ServiceOption{
		auth.WithClock(func() time.Time { return now }),
	}, opts...)

	return auth.NewService(store, opts...), store, user
}

func TestServiceLogin(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("correct password opens a session", func(t *testing.T) {
		svc, store, user := newTestService(t, now)

		result, err := svc.Login(ctx, auth.LoginParams{
			Username:  "Owner",
			Password:  testPassword,
			UserAgent: "curl/8.4.0",
			IP:        netip.MustParseAddr("203.0.113.7"),
		})
		if err != nil {
			t.Fatalf("Login: %v", err)
		}

		if result.User.ID != user.ID {
			t.Errorf("user id = %d, want %d", result.User.ID, user.ID)
		}
		if err := auth.ValidateTokenFormat(result.Token); err != nil {
			t.Errorf("returned token is malformed: %v", err)
		}
		// The stored digest must be the digest of the returned token, or the
		// cookie the client receives will never match a row.
		if !auth.EqualTokenHash(result.Session.TokenHash, auth.HashToken(result.Token)) {
			t.Error("the stored digest is not the digest of the returned token")
		}
		if want := now.Add(auth.DefaultSessionLifetime); !result.Session.ExpiresAt.Equal(want) {
			t.Errorf("expires_at = %v, want %v", result.Session.ExpiresAt, want)
		}
		// Recorded for review, not consulted.
		if result.Session.UserAgent != "curl/8.4.0" {
			t.Errorf("user_agent = %q, want %q", result.Session.UserAgent, "curl/8.4.0")
		}
		if store.CreateCalls != 1 {
			t.Errorf("CreateSession called %d times, want 1", store.CreateCalls)
		}
	})

	t.Run("username is matched case-insensitively", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)

		if _, err := svc.Login(ctx, auth.LoginParams{Username: "OWNER", Password: testPassword}); err != nil {
			t.Errorf("Login with a different case = %v, want nil", err)
		}
	})

	// The property that matters most here: the two failures must be
	// indistinguishable. If they ever diverge, the login endpoint becomes a way to
	// enumerate usernames.
	t.Run("wrong password and unknown user give the same error", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)

		_, wrongPassword := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: "not-the-password"})
		_, noSuchUser := svc.Login(ctx, auth.LoginParams{Username: "nobody", Password: testPassword})

		for name, err := range map[string]error{"wrong password": wrongPassword, "unknown user": noSuchUser} {
			if !errors.Is(err, auth.ErrInvalidCredentials) {
				t.Errorf("%s: error = %v, want ErrInvalidCredentials", name, err)
			}
		}
		if wrongPassword.Error() != noSuchUser.Error() {
			t.Errorf("the two failures carry different messages:\n  %q\n  %q", wrongPassword, noSuchUser)
		}
	})

	t.Run("a failed login opens no session", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)

		if _, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: "wrong"}); err == nil {
			t.Fatal("Login with a wrong password returned nil error")
		}
		if store.CreateCalls != 0 {
			t.Errorf("CreateSession called %d times after a failed login, want 0", store.CreateCalls)
		}
	})

	// A stored hash that cannot be parsed is a broken row. Reporting it as
	// ErrInvalidCredentials would leave the owner retyping a correct password
	// forever while the real fault stayed invisible.
	t.Run("a corrupt stored hash is not a credential failure", func(t *testing.T) {
		store := authtest.NewStore()
		if _, err := store.CreateUser(ctx, auth.CreateUserParams{
			Username:     "Owner",
			PasswordHash: "$argon2id$broken",
		}); err != nil {
			t.Fatalf("seed user: %v", err)
		}
		svc := auth.NewService(store)

		_, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: testPassword})
		if errors.Is(err, auth.ErrInvalidCredentials) {
			t.Error("a corrupt hash was reported as ErrInvalidCredentials")
		}
		if !errors.Is(err, auth.ErrPasswordHashInvalid) {
			t.Errorf("error = %v, want ErrPasswordHashInvalid", err)
		}
	})

	// An infrastructure fault must not be flattened into a credential failure:
	// a database outage would otherwise look like the owner mistyping.
	t.Run("a storage failure is reported as itself", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)
		outage := errors.New("connection refused")
		store.FailGetCredentials = outage

		_, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: testPassword})
		if !errors.Is(err, outage) {
			t.Errorf("error = %v, want the underlying storage error", err)
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			t.Error("a storage outage was reported as ErrInvalidCredentials")
		}
	})

	t.Run("a failure to store the session fails the login", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)
		store.FailCreateSession = errors.New("disk full")

		// The alternative would be handing out a token with no row behind it: a
		// client that believes it is logged in and is refused on the next request.
		if _, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: testPassword}); err == nil {
			t.Error("Login returned nil after CreateSession failed")
		}
	})
}

func TestServiceLoginTokensAreDistinct(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	svc, _, _ := newTestService(t, now)
	ctx := context.Background()

	first, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: testPassword})
	if err != nil {
		t.Fatalf("first Login: %v", err)
	}
	second, err := svc.Login(ctx, auth.LoginParams{Username: "Owner", Password: testPassword})
	if err != nil {
		t.Fatalf("second Login: %v", err)
	}

	if first.Token == second.Token {
		t.Error("two logins produced the same token")
	}
	// Two concurrent sessions are allowed: a phone and a laptop should not evict
	// each other.
	if first.Session.ID == second.Session.ID {
		t.Error("two logins produced the same session")
	}
}

// loginAt opens a session against a service whose clock reads at.
func loginAt(t *testing.T, svc *auth.Service) auth.Token {
	t.Helper()

	result, err := svc.Login(context.Background(), auth.LoginParams{
		Username: "Owner",
		Password: testPassword,
	})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	return result.Token
}

func TestServiceAuthenticate(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("a fresh token resolves to its user", func(t *testing.T) {
		svc, _, user := newTestService(t, now)
		token := loginAt(t, svc)

		got, err := svc.Authenticate(ctx, token)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if got.User.ID != user.ID {
			t.Errorf("user id = %d, want %d", got.User.ID, user.ID)
		}
		if got.User.Username != "Owner" {
			t.Errorf("username = %q, want %q", got.User.Username, "Owner")
		}
	})

	// Rejected on length alone, before any query. The assertion on the call count
	// is the point: a malformed cookie must not reach the database.
	t.Run("a malformed token is refused without a lookup", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)

		_, err := svc.Authenticate(ctx, "not-a-token")
		if !errors.Is(err, auth.ErrInvalidToken) {
			t.Errorf("error = %v, want ErrInvalidToken", err)
		}
		if store.GetSessionCalls != 0 {
			t.Errorf("GetSessionByHash called %d times, want 0", store.GetSessionCalls)
		}
	})

	t.Run("an unknown token gives ErrSessionNotFound", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)

		unknown, _, err := auth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		if _, err := svc.Authenticate(ctx, unknown); !errors.Is(err, auth.ErrSessionNotFound) {
			t.Errorf("error = %v, want ErrSessionNotFound", err)
		}
	})

	// ErrSessionExpired is kept apart from ErrSessionNotFound so the logs can tell
	// "this person was logged in until yesterday" from "this token was never
	// issued". Both still answer 401.
	t.Run("an expired session gives ErrSessionExpired", func(t *testing.T) {
		clock := now
		svc, _, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(auth.DefaultSessionLifetime + time.Second)

		_, err := svc.Authenticate(ctx, token)
		if !errors.Is(err, auth.ErrSessionExpired) {
			t.Errorf("error = %v, want ErrSessionExpired", err)
		}
		if errors.Is(err, auth.ErrSessionNotFound) {
			t.Error("an expired session was reported as not found")
		}
	})

	t.Run("expiry exactly at now counts as expired", func(t *testing.T) {
		clock := now
		svc, _, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(auth.DefaultSessionLifetime)

		if _, err := svc.Authenticate(ctx, token); !errors.Is(err, auth.ErrSessionExpired) {
			t.Errorf("error = %v, want ErrSessionExpired", err)
		}
	})
}

func TestServiceAuthenticateRenewal(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()
	const lifetime = auth.DefaultSessionLifetime

	// Renewing on every request would make each read of a protected endpoint a
	// write. These two cases pin the halfway rule that avoids it.
	t.Run("no write while more than half the lifetime remains", func(t *testing.T) {
		clock := now
		svc, store, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(lifetime / 4)

		if _, err := svc.Authenticate(ctx, token); err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if store.TouchCalls != 0 {
			t.Errorf("TouchSession called %d times, want 0", store.TouchCalls)
		}
	})

	// Renewed is what the HTTP adapter reads to decide whether to write a fresh
	// cookie. It cannot work that out from ExpiresAt: a session renewed a moment
	// ago and one issued a moment ago look the same.
	t.Run("Renewed is false when nothing was renewed", func(t *testing.T) {
		clock := now
		svc, _, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(lifetime / 4)

		got, err := svc.Authenticate(ctx, token)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if got.Renewed {
			t.Error("Renewed = true although the session was not renewed")
		}
	})

	t.Run("a write once past halfway", func(t *testing.T) {
		clock := now
		svc, store, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(lifetime/2 + time.Minute)

		got, err := svc.Authenticate(ctx, token)
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if store.TouchCalls != 1 {
			t.Errorf("TouchSession called %d times, want 1", store.TouchCalls)
		}
		// The returned session must carry the new expiry, not the old one: the
		// adapter reads it to refresh the cookie.
		if want := clock.Add(lifetime); !got.Session.ExpiresAt.Equal(want) {
			t.Errorf("expires_at = %v, want %v", got.Session.ExpiresAt, want)
		}
		if !got.Renewed {
			t.Error("Renewed = false although the session was extended")
		}
	})

	// The renewal write is bookkeeping. The token is valid and the caller is who
	// they say they are, so a failed write must not turn into a 401.
	t.Run("a failed renewal does not fail the request", func(t *testing.T) {
		clock := now
		svc, store, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		clock = now.Add(lifetime/2 + time.Minute)
		store.FailTouchSession = errors.New("write failed")

		got, err := svc.Authenticate(ctx, token)
		if err != nil {
			t.Fatalf("Authenticate after a failed renewal = %v, want nil", err)
		}
		// The old expiry stands, since the write did not land.
		if want := now.Add(lifetime); !got.Session.ExpiresAt.Equal(want) {
			t.Errorf("expires_at = %v, want the original %v", got.Session.ExpiresAt, want)
		}
		// Renewed must stay false. A cookie promising a lifetime the row does not
		// have would keep the browser sending a token past the server's expiry.
		if got.Renewed {
			t.Error("Renewed = true although the renewal write failed")
		}
	})

	t.Run("renewal extends a session past its original expiry", func(t *testing.T) {
		clock := now
		svc, _, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))
		token := loginAt(t, svc)

		// One request late in the lifetime, then a check after the original expiry
		// would have passed. This is what "sliding" has to mean.
		clock = now.Add(lifetime - time.Hour)
		if _, err := svc.Authenticate(ctx, token); err != nil {
			t.Fatalf("Authenticate: %v", err)
		}

		clock = now.Add(lifetime + time.Hour)
		if _, err := svc.Authenticate(ctx, token); err != nil {
			t.Errorf("Authenticate after renewal = %v, want nil", err)
		}
	})
}

func TestServiceLogout(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	t.Run("the session stops working", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)
		token := loginAt(t, svc)

		if err := svc.Logout(ctx, token); err != nil {
			t.Fatalf("Logout: %v", err)
		}

		if _, err := svc.Authenticate(ctx, token); !errors.Is(err, auth.ErrSessionNotFound) {
			t.Errorf("error after logout = %v, want ErrSessionNotFound", err)
		}
	})

	// A client that retries a logout, or sends a stale cookie, is not reporting a
	// problem. Answering with an error would invite a retry of something that has
	// nothing left to do.
	t.Run("logging out twice succeeds", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)
		token := loginAt(t, svc)

		if err := svc.Logout(ctx, token); err != nil {
			t.Fatalf("first Logout: %v", err)
		}
		if err := svc.Logout(ctx, token); err != nil {
			t.Errorf("second Logout = %v, want nil", err)
		}
	})

	t.Run("a malformed token is accepted without a delete", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)

		if err := svc.Logout(ctx, "garbage"); err != nil {
			t.Errorf("Logout with a malformed token = %v, want nil", err)
		}
		if store.DeleteCalls != 0 {
			t.Errorf("DeleteSessionByHash called %d times, want 0", store.DeleteCalls)
		}
	})

	t.Run("logging out ends only the session used", func(t *testing.T) {
		svc, _, _ := newTestService(t, now)
		phone := loginAt(t, svc)
		laptop := loginAt(t, svc)

		if err := svc.Logout(ctx, phone); err != nil {
			t.Fatalf("Logout: %v", err)
		}

		if _, err := svc.Authenticate(ctx, laptop); err != nil {
			t.Errorf("the other session was ended too: %v", err)
		}
	})

	// A real delete failure is worth reporting: the session is still usable, and
	// the caller believes it is not.
	t.Run("a storage failure is reported", func(t *testing.T) {
		svc, store, _ := newTestService(t, now)
		token := loginAt(t, svc)
		store.FailDeleteByHash = errors.New("delete failed")

		if err := svc.Logout(ctx, token); err == nil {
			t.Error("Logout returned nil after a failed delete")
		}
	})
}

func TestServiceCreateUser(t *testing.T) {
	ctx := context.Background()

	t.Run("the created account can log in", func(t *testing.T) {
		svc := auth.NewService(authtest.NewStore())

		user, err := svc.CreateUser(ctx, auth.CreateUserInput{
			Username:    "owner",
			Password:    testPassword,
			DisplayName: "The Owner",
		})
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if user.Role != "owner" {
			t.Errorf("role = %q, want %q", user.Role, "owner")
		}

		// The only assertion that matters: hashing and verification agree.
		if _, err := svc.Login(ctx, auth.LoginParams{Username: "owner", Password: testPassword}); err != nil {
			t.Errorf("Login with the created account = %v, want nil", err)
		}
	})

	t.Run("a duplicate username is refused", func(t *testing.T) {
		svc := auth.NewService(authtest.NewStore())

		if _, err := svc.CreateUser(ctx, auth.CreateUserInput{Username: "owner", Password: testPassword}); err != nil {
			t.Fatalf("CreateUser: %v", err)
		}

		_, err := svc.CreateUser(ctx, auth.CreateUserInput{Username: "OWNER", Password: testPassword})
		if !errors.Is(err, auth.ErrUsernameTaken) {
			t.Errorf("error = %v, want ErrUsernameTaken", err)
		}
	})

	// Validation runs before hashing, so a rejected input does not cost 200 ms.
	t.Run("input is validated before any hashing", func(t *testing.T) {
		svc := auth.NewService(authtest.NewStore())

		tests := []struct {
			name     string
			input    auth.CreateUserInput
			wantErrs error
		}{
			{
				name:     "username too short",
				input:    auth.CreateUserInput{Username: "ab", Password: testPassword},
				wantErrs: auth.ErrInvalidUsername,
			},
			{
				name:     "username too long",
				input:    auth.CreateUserInput{Username: strings.Repeat("a", 65), Password: testPassword},
				wantErrs: auth.ErrInvalidUsername,
			},
			{
				name:     "password too short",
				input:    auth.CreateUserInput{Username: "owner", Password: "short"},
				wantErrs: auth.ErrInvalidPassword,
			},
			{
				name:     "password empty",
				input:    auth.CreateUserInput{Username: "owner", Password: ""},
				wantErrs: auth.ErrInvalidPassword,
			},
			{
				name:     "password too long",
				input:    auth.CreateUserInput{Username: "owner", Password: strings.Repeat("x", 1025)},
				wantErrs: auth.ErrInvalidPassword,
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				_, err := svc.CreateUser(ctx, tc.input)
				if !errors.Is(err, tc.wantErrs) {
					t.Errorf("error = %v, want %v", err, tc.wantErrs)
				}
			})
		}
	})

	// The column's CHECK counts characters, so the check here has to as well. A
	// byte-based check would reject a name the database accepts.
	t.Run("username length counts characters, not bytes", func(t *testing.T) {
		svc := auth.NewService(authtest.NewStore())

		// Three characters, nine bytes.
		if _, err := svc.CreateUser(ctx, auth.CreateUserInput{
			Username: "日志者",
			Password: testPassword,
		}); err != nil {
			t.Errorf("CreateUser with a 3-character name = %v, want nil", err)
		}
	})
}

func TestServicePruneExpiredSessions(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	clock := now
	svc, _, _ := newTestService(t, now, auth.WithClock(func() time.Time { return clock }))

	live := loginAt(t, svc)

	// A second session opened far enough in the past that it is already expired.
	clock = now.Add(-2 * auth.DefaultSessionLifetime)
	stale := loginAt(t, svc)
	clock = now

	removed, err := svc.PruneExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("PruneExpiredSessions: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1", removed)
	}

	if _, err := svc.Authenticate(ctx, live); err != nil {
		t.Errorf("the live session was pruned: %v", err)
	}
	if _, err := svc.Authenticate(ctx, stale); !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("error for the pruned session = %v, want ErrSessionNotFound", err)
	}
}

func TestServiceSessionLifetime(t *testing.T) {
	// The cookie's Max-Age is derived from this, so the override has to take.
	svc := auth.NewService(authtest.NewStore(), auth.WithSessionLifetime(2*time.Hour))
	if got := svc.SessionLifetime(); got != 2*time.Hour {
		t.Errorf("SessionLifetime = %v, want 2h", got)
	}

	// A nonsense value falls back rather than producing a session that expires
	// the moment it is created.
	svc = auth.NewService(authtest.NewStore(), auth.WithSessionLifetime(0))
	if got := svc.SessionLifetime(); got != auth.DefaultSessionLifetime {
		t.Errorf("SessionLifetime = %v, want the default %v", got, auth.DefaultSessionLifetime)
	}
}
