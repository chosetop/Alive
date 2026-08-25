package authhttp_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/auth/authtest"
	"github.com/p30huiwei/alive/backend/internal/authhttp"
)

const (
	testUsername = "Owner"
	testPassword = "correct-horse-battery-staple"
)

// Argon2id costs roughly 200 ms, so the account's hash is computed once for the
// whole package.
var (
	hashOnce sync.Once
	hashed   string
)

func ownerHash(t *testing.T) string {
	t.Helper()
	hashOnce.Do(func() {
		h, err := auth.HashPassword(testPassword)
		if err != nil {
			t.Fatalf("HashPassword: %v", err)
		}
		hashed = h
	})
	return hashed
}

// newTestServer builds the real routes over a fake store.
//
// The whole chain is exercised: Gin binding, the handler, the cookie, the
// middleware. Only storage is faked, because these tests are about what the HTTP
// surface does, not about SQL.
func newTestServer(t *testing.T, opts ...auth.ServiceOption) (http.Handler, *authtest.Store) {
	t.Helper()

	store := authtest.NewStore()
	if _, err := store.CreateUser(context.Background(), auth.CreateUserParams{
		Username:     testUsername,
		PasswordHash: ownerHash(t),
		DisplayName:  "The Owner",
	}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	service := auth.NewService(store, opts...)
	handler := authhttp.NewHandler(service, testSessionConfig(), slog.New(slog.NewTextHandler(io.Discard, nil)))

	engine := gin.New()
	handler.Register(engine.Group("/api/v1"))

	return engine, store
}

// login posts credentials and returns the response.
func login(t *testing.T, srv http.Handler, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec
}

// sessionCookie returns the session cookie from a response, or nil.
func sessionCookie(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == "alive_session" {
			return c
		}
	}
	return nil
}

// decodeData unwraps the "data" envelope.
func decodeData(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body.Data
}

// decodeError unwraps the "error" envelope.
func decodeError(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body struct {
		Error map[string]any `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body.Error
}

func TestLoginSuccess(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
	}

	data := decodeData(t, rec)
	if data["username"] != "Owner" {
		t.Errorf("username = %v, want Owner", data["username"])
	}
	// Confirmed field list: id, username, role, display_name.
	if data["role"] != "owner" {
		t.Errorf("role = %v, want owner", data["role"])
	}
	if data["display_name"] != "The Owner" {
		t.Errorf("display_name = %v, want The Owner", data["display_name"])
	}
	if _, ok := data["id"]; !ok {
		t.Error("id is missing from the response")
	}

	// The token must not be in the body. In JSON it is readable by any script that
	// can parse the response, which is the whole thing HttpOnly prevents.
	for _, forbidden := range []string{"token", "password", "password_hash", "session"} {
		if _, present := data[forbidden]; present {
			t.Errorf("response carries %q", forbidden)
		}
	}

	cookie := sessionCookie(rec)
	if cookie == nil {
		t.Fatal("no session cookie was set")
	}
	if err := auth.ValidateTokenFormat(auth.Token(cookie.Value)); err != nil {
		t.Errorf("the cookie does not carry a well-formed token: %v", err)
	}
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	srv, _ := newTestServer(t)

	// Both must be identical, body and status. Any difference is a way to learn
	// which usernames exist.
	wrongPassword := login(t, srv, `{"username":"Owner","password":"wrong-password"}`)
	unknownUser := login(t, srv, `{"username":"nobody","password":"`+testPassword+`"}`)

	for name, rec := range map[string]*httptest.ResponseRecorder{
		"wrong password": wrongPassword,
		"unknown user":   unknownUser,
	} {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
		if got := decodeError(t, rec)["code"]; got != "INVALID_CREDENTIALS" {
			t.Errorf("%s: code = %v, want INVALID_CREDENTIALS", name, got)
		}
		if sessionCookie(rec) != nil {
			t.Errorf("%s: a session cookie was set on a failed login", name)
		}
	}

	if wrongPassword.Body.String() != unknownUser.Body.String() {
		t.Errorf("the two failures differ:\n  %s\n  %s", wrongPassword.Body, unknownUser.Body)
	}
}

// TestLoginRejectsMalformedRequests covers the 400/401 split you confirmed: a
// body the server cannot read is a bad request, a credential it will not accept
// is a refused one.
func TestLoginRejectsMalformedRequests(t *testing.T) {
	srv, _ := newTestServer(t)

	tests := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ``},
		{name: "not json", body: `nonsense`},
		{name: "missing password", body: `{"username":"Owner"}`},
		{name: "missing username", body: `{"password":"` + testPassword + `"}`},
		{name: "empty username", body: `{"username":"","password":"` + testPassword + `"}`},
		{name: "empty password", body: `{"username":"Owner","password":""}`},
		{name: "wrong types", body: `{"username":123,"password":true}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := login(t, srv, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400; body %s", rec.Code, rec.Body)
			}
			if got := decodeError(t, rec)["code"]; got != "INVALID_INPUT" {
				t.Errorf("code = %v, want INVALID_INPUT", got)
			}
		})
	}
}

// TestLoginDoesNotLeakTheHash guards against the response ever carrying password
// material, whatever the shape of the JSON.
func TestLoginDoesNotLeakTheHash(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`)

	if strings.Contains(rec.Body.String(), "argon2id") {
		t.Errorf("the response body carries the password hash: %s", rec.Body)
	}
	if strings.Contains(rec.Body.String(), testPassword) {
		t.Errorf("the response body carries the password: %s", rec.Body)
	}
}

func TestMe(t *testing.T) {
	srv, _ := newTestServer(t)

	t.Run("a valid session returns the account", func(t *testing.T) {
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))
		if cookie == nil {
			t.Fatal("login set no cookie")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body %s", rec.Code, rec.Body)
		}

		data := decodeData(t, rec)
		for field, want := range map[string]any{
			"username":     "Owner",
			"role":         "owner",
			"display_name": "The Owner",
		} {
			if data[field] != want {
				t.Errorf("%s = %v, want %v", field, data[field], want)
			}
		}
		if len(data) != 4 {
			t.Errorf("response has %d fields, want exactly 4: %v", len(data), data)
		}
	})

	t.Run("no cookie gives 401", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
		if got := decodeError(t, rec)["code"]; got != "UNAUTHORIZED" {
			t.Errorf("code = %v, want UNAUTHORIZED", got)
		}
	})
}

// TestMeRejectsBadSessions checks that every reason gives the same answer. A
// response that said "expired" rather than "unknown" would confirm the token had
// once been issued.
func TestMeRejectsBadSessions(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	unknown, _, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	tests := []struct {
		name  string
		value string
	}{
		{name: "malformed token", value: "not-a-token"},
		{name: "well-formed but unknown", value: string(unknown)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, _ := newTestServer(t, auth.WithClock(func() time.Time { return now }))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
			req.AddCookie(&http.Cookie{Name: "alive_session", Value: tc.value})
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			if got := decodeError(t, rec)["code"]; got != "UNAUTHORIZED" {
				t.Errorf("code = %v, want UNAUTHORIZED", got)
			}

			// The browser is holding something that will never work. Clearing it
			// stops every later request from carrying it.
			cookie := sessionCookie(rec)
			if cookie == nil {
				t.Fatal("the dead cookie was not cleared")
			}
			if cookie.Value != "" || cookie.MaxAge > 0 {
				t.Errorf("cookie = %q with max-age %d, want cleared", cookie.Value, cookie.MaxAge)
			}
		})
	}
}

func TestMeRejectsExpiredSession(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	clock := now
	srv, _ := newTestServer(t, auth.WithClock(func() time.Time { return clock }))

	cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))
	if cookie == nil {
		t.Fatal("login set no cookie")
	}

	clock = now.Add(auth.DefaultSessionLifetime + time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	// Same code as an unknown token. The difference belongs in the log.
	if got := decodeError(t, rec)["code"]; got != "UNAUTHORIZED" {
		t.Errorf("code = %v, want UNAUTHORIZED", got)
	}
}

// logout posts to the logout endpoint, carrying cookie when it is not nil.
func logout(t *testing.T, srv http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec
}

func TestLogout(t *testing.T) {
	t.Run("ends the session and clears the cookie", func(t *testing.T) {
		srv, _ := newTestServer(t)
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))
		if cookie == nil {
			t.Fatal("login set no cookie")
		}

		rec := logout(t, srv, cookie)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204; body %s", rec.Code, rec.Body)
		}

		cleared := sessionCookie(rec)
		if cleared == nil {
			t.Fatal("logout set no cookie to clear the session")
		}
		if cleared.Value != "" {
			t.Errorf("cleared cookie value = %q, want empty", cleared.Value)
		}
		if cleared.MaxAge > 0 {
			t.Errorf("cleared cookie max-age = %d, want 0 or negative", cleared.MaxAge)
		}

		// The row is gone, so the old cookie cannot work even if a client keeps it.
		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(cookie)
		after := httptest.NewRecorder()
		srv.ServeHTTP(after, req)

		if after.Code != http.StatusUnauthorized {
			t.Errorf("status on the old cookie = %d, want 401", after.Code)
		}
	})

	// Idempotent. A client's goal is to be logged out; when it already is, that
	// goal is met and an error would invite a retry of nothing.
	t.Run("without a cookie it still succeeds", func(t *testing.T) {
		srv, _ := newTestServer(t)

		if rec := logout(t, srv, nil); rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})

	t.Run("twice it still succeeds", func(t *testing.T) {
		srv, _ := newTestServer(t)
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))

		if rec := logout(t, srv, cookie); rec.Code != http.StatusNoContent {
			t.Fatalf("first logout: status = %d, want 204", rec.Code)
		}
		if rec := logout(t, srv, cookie); rec.Code != http.StatusNoContent {
			t.Errorf("second logout: status = %d, want 204", rec.Code)
		}
	})

	t.Run("a malformed cookie still succeeds", func(t *testing.T) {
		srv, _ := newTestServer(t)

		rec := logout(t, srv, &http.Cookie{Name: "alive_session", Value: "not-a-token"})
		if rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})

	// Logout must not require a session. A client whose session already ended
	// would otherwise be stuck holding a cookie it has no way to drop.
	t.Run("an expired session can still log out", func(t *testing.T) {
		clock := time.Now()
		srv, _ := newTestServer(t, auth.WithClock(func() time.Time { return clock }))
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))

		clock = clock.Add(auth.DefaultSessionLifetime + time.Hour)

		if rec := logout(t, srv, cookie); rec.Code != http.StatusNoContent {
			t.Errorf("status = %d, want 204", rec.Code)
		}
	})
}

// TestMeRefreshesCookieOnRenewal is the property that keeps a sliding session
// usable. The row's expiry moves forward on use; if the cookie kept its original
// Max-Age, the browser would stop sending a token the server still accepts and
// the owner would be logged out mid-use.
func TestMeRefreshesCookieOnRenewal(t *testing.T) {
	const lifetime = auth.DefaultSessionLifetime

	t.Run("no Set-Cookie while the session is fresh", func(t *testing.T) {
		clock := time.Now()
		srv, store := newTestServer(t, auth.WithClock(func() time.Time { return clock }))
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))

		clock = clock.Add(lifetime / 4)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if store.TouchCalls != 0 {
			t.Errorf("TouchSession called %d times, want 0", store.TouchCalls)
		}
		// A Set-Cookie on every request would be noise.
		if sessionCookie(rec) != nil {
			t.Error("a cookie was rewritten although the session was not renewed")
		}
	})

	t.Run("a fresh cookie once past halfway", func(t *testing.T) {
		clock := time.Now()
		srv, store := newTestServer(t, auth.WithClock(func() time.Time { return clock }))
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))

		clock = clock.Add(lifetime/2 + time.Minute)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if store.TouchCalls != 1 {
			t.Errorf("TouchSession called %d times, want 1", store.TouchCalls)
		}

		refreshed := sessionCookie(rec)
		if refreshed == nil {
			t.Fatal("the session was renewed but no cookie was written")
		}
		// The same token, a new Max-Age. Renewal extends a session; it does not
		// issue a different one.
		if refreshed.Value != cookie.Value {
			t.Error("the refreshed cookie carries a different token")
		}
		if refreshed.MaxAge != int(lifetime.Seconds()) {
			t.Errorf("max-age = %d, want %d", refreshed.MaxAge, int(lifetime.Seconds()))
		}
	})

	// A failed renewal must not fail the request: the token is valid and the caller
	// is who they say they are.
	t.Run("a failed renewal leaves the request working", func(t *testing.T) {
		clock := time.Now()
		srv, store := newTestServer(t, auth.WithClock(func() time.Time { return clock }))
		cookie := sessionCookie(login(t, srv, `{"username":"Owner","password":"`+testPassword+`"}`))

		clock = clock.Add(lifetime/2 + time.Minute)
		store.FailTouchSession = errors.New("write failed")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		// No cookie either: promising a lifetime the row does not have would keep
		// the browser sending a token past the server's expiry.
		if sessionCookie(rec) != nil {
			t.Error("a cookie was refreshed although the renewal write failed")
		}
	})
}

// TestMeRequiresTheMiddleware guards the wiring. Without RequireAuth the handler
// would answer 500 rather than serve an empty user, and this asserts the route as
// registered has the middleware.
func TestMeRequiresAuthentication(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body %s", rec.Code, rec.Body)
	}
	if got := decodeError(t, rec)["code"]; got != "UNAUTHORIZED" {
		t.Errorf("code = %v, want UNAUTHORIZED", got)
	}
}
