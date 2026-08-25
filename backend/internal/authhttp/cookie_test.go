package authhttp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/authhttp"
	"github.com/p30huiwei/alive/backend/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func testSessionConfig() config.SessionConfig {
	return config.SessionConfig{
		Lifetime:     7 * 24 * time.Hour,
		CookieName:   "alive_session",
		CookiePath:   "/api/v1",
		CookieSecure: true,
	}
}

// setCookie runs a handler that sets the session cookie and returns the cookie
// the response carries. Going through a real ResponseRecorder means the
// assertions are made against the header a browser would receive, not against a
// struct this package built.
func setCookie(t *testing.T, cfg config.SessionConfig, token auth.Token) *http.Cookie {
	t.Helper()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)

	authhttp.NewCookieManager(cfg).Set(c, token)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response carries %d cookies, want 1", len(cookies))
	}
	return cookies[0]
}

func TestCookieManagerSet(t *testing.T) {
	cfg := testSessionConfig()
	const token auth.Token = "a-token-value"

	got := setCookie(t, cfg, token)

	if got.Name != "alive_session" {
		t.Errorf("name = %q, want %q", got.Name, "alive_session")
	}
	if got.Value != string(token) {
		t.Errorf("value = %q, want %q", got.Value, token)
	}
	if got.Path != "/api/v1" {
		t.Errorf("path = %q, want %q", got.Path, "/api/v1")
	}
	// The attribute that keeps the token out of document.cookie. Losing it makes
	// injected script able to read the credential.
	if !got.HttpOnly {
		t.Error("HttpOnly is not set")
	}
	if !got.Secure {
		t.Error("Secure is not set")
	}
	if got.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", got.SameSite)
	}
	// Host-only. A Domain here would widen the cookie to every subdomain.
	if got.Domain != "" {
		t.Errorf("domain = %q, want empty", got.Domain)
	}
	// Max-Age has to match the session lifetime, or the browser stops sending a
	// token the server still accepts, or keeps sending one it does not.
	if want := int(cfg.Lifetime.Seconds()); got.MaxAge != want {
		t.Errorf("max-age = %d, want %d", got.MaxAge, want)
	}
}

func TestCookieManagerSetSecureFollowsConfig(t *testing.T) {
	// Development runs on http://localhost, where a Secure cookie is never sent
	// and login looks broken for no visible reason. config.validate refuses this
	// combination in production.
	cfg := testSessionConfig()
	cfg.CookieSecure = false

	if got := setCookie(t, cfg, "token"); got.Secure {
		t.Error("Secure is set although the configuration disables it")
	}
}

func TestCookieManagerClear(t *testing.T) {
	cfg := testSessionConfig()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

	authhttp.NewCookieManager(cfg).Clear(c)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("response carries %d cookies, want 1", len(cookies))
	}
	got := cookies[0]

	if got.Value != "" {
		t.Errorf("value = %q, want empty", got.Value)
	}
	if got.MaxAge != 0 && got.MaxAge > 0 {
		t.Errorf("max-age = %d, want 0 or negative", got.MaxAge)
	}

	// A browser matches a cookie for replacement by name, domain and path. If
	// Clear differed from Set on any of them, the response would add a second
	// cookie and leave the original in place, so logout would not log anybody out.
	set := setCookie(t, cfg, "token")
	if got.Name != set.Name {
		t.Errorf("clear name = %q, set name = %q", got.Name, set.Name)
	}
	if got.Path != set.Path {
		t.Errorf("clear path = %q, set path = %q", got.Path, set.Path)
	}
	if got.Domain != set.Domain {
		t.Errorf("clear domain = %q, set domain = %q", got.Domain, set.Domain)
	}
	if got.Secure != set.Secure {
		t.Errorf("clear Secure = %v, set Secure = %v", got.Secure, set.Secure)
	}
	if got.HttpOnly != set.HttpOnly {
		t.Errorf("clear HttpOnly = %v, set HttpOnly = %v", got.HttpOnly, set.HttpOnly)
	}
	if got.SameSite != set.SameSite {
		t.Errorf("clear SameSite = %v, set SameSite = %v", got.SameSite, set.SameSite)
	}
}

// TestCookieManagerClearEmitsMaxAgeZero checks the wire format rather than the
// parsed struct. net/http turns MaxAge<0 into "Max-Age=0", and that literal is
// what tells a browser to drop the cookie now.
func TestCookieManagerClearHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)

	authhttp.NewCookieManager(testSessionConfig()).Clear(c)

	header := rec.Header().Get("Set-Cookie")
	if !strings.Contains(header, "Max-Age=0") {
		t.Errorf("Set-Cookie = %q, want it to contain Max-Age=0", header)
	}
}

func TestCookieManagerRead(t *testing.T) {
	manager := authhttp.NewCookieManager(testSessionConfig())

	t.Run("returns the token from the cookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Request.AddCookie(&http.Cookie{Name: "alive_session", Value: "the-token"})

		token, ok := manager.Read(c)
		if !ok {
			t.Fatal("Read reported no cookie")
		}
		if token != "the-token" {
			t.Errorf("token = %q, want %q", token, "the-token")
		}
	})

	t.Run("reports absence when there is no cookie", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)

		if _, ok := manager.Read(c); ok {
			t.Error("Read reported a cookie on a request without one")
		}
	})

	// An empty value is treated as absent. Otherwise a cleared cookie that the
	// browser still sends would be handed on as a token, costing a lookup to
	// discover it is worthless.
	t.Run("an empty value counts as absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Request.Header.Set("Cookie", "alive_session=")

		if _, ok := manager.Read(c); ok {
			t.Error("Read reported a cookie for an empty value")
		}
	})

	t.Run("a differently named cookie is ignored", func(t *testing.T) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Request.AddCookie(&http.Cookie{Name: "other_session", Value: "the-token"})

		if _, ok := manager.Read(c); ok {
			t.Error("Read picked up a cookie with another name")
		}
	})
}

// TestCookieRoundTrip is the property the two halves have to satisfy together:
// what Set writes, Read reads back unchanged. A generated token is used because
// its alphabet is the one that has to survive the trip.
func TestCookieRoundTrip(t *testing.T) {
	cfg := testSessionConfig()
	manager := authhttp.NewCookieManager(cfg)

	for i := 0; i < 50; i++ {
		token, _, err := auth.GenerateToken()
		if err != nil {
			t.Fatalf("GenerateToken: %v", err)
		}

		written := setCookie(t, cfg, token)

		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
		c.Request.AddCookie(written)

		got, ok := manager.Read(c)
		if !ok {
			t.Fatalf("Read reported no cookie for token %q", token)
		}
		if got != token {
			t.Fatalf("round trip changed the token: wrote %q, read %q", token, got)
		}
	}
}

// TestCookieValueNeedsNoQuoting checks that a generated token survives without
// net/http quoting it. Raw URL base64 avoids the characters that would force
// quotes, and a quoted value would come back with them attached.
func TestCookieValueNeedsNoQuoting(t *testing.T) {
	cfg := testSessionConfig()

	token, _, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	authhttp.NewCookieManager(cfg).Set(c, token)

	header := rec.Header().Get("Set-Cookie")
	if strings.Contains(header, `"`) {
		t.Errorf("Set-Cookie = %q, want no quoting", header)
	}
	if !strings.Contains(header, "alive_session="+string(token)) {
		t.Errorf("Set-Cookie = %q, want it to carry the token verbatim", header)
	}
}
