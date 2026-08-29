package router_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/router"
	"github.com/p30huiwei/alive/backend/internal/site"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// testConfig is a valid development configuration with the rate limiter left at
// its zero value, which is disabled. Tests that need the limiter set it.
func testConfig() *config.Config {
	return &config.Config{
		Env: config.EnvDevelopment,
		Server: config.ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		CORS: config.CORSConfig{
			AllowedOrigins:   []string{"http://localhost:3000"},
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           time.Hour,
		},
		Session: config.SessionConfig{
			Lifetime:   7 * 24 * time.Hour,
			CookieName: "alive_session",
			CookiePath: "/api/v1",
		},
	}
}

// newTestRouter builds the real router with a nil pool.
//
// A nil pool is safe for every route asserted here, and it keeps these tests
// free of a live database. Readiness, the one route that needs the pool, is left
// to an integration test in the stage that introduces one.
func newTestRouter() http.Handler {
	return newRouterWith(testConfig())
}

func newRouterWith(cfg *config.Config) http.Handler {
	// A service over a nil repository. The routes asserted here never reach
	// storage: an unauthenticated request is refused on the missing cookie, and a
	// malformed login body on the binding step, both before any query.
	authService := auth.NewService(auth.NewRepository(nil))
	entryService := entry.NewService(entry.NewRepository(nil))
	taxonomyService := taxonomy.NewService(taxonomy.NewRepository(nil))
	siteService := site.NewService(site.NewRepository(nil))
	worldService := contentworld.NewService(contentworld.NewRepository(nil))

	return router.New(router.Dependencies{
		Config:          cfg,
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		Pool:            nil,
		AuthService:     authService,
		EntryService:    entryService,
		TaxonomyService: taxonomyService,
		SiteService:     siteService,
		WorldService:    worldService,
	})
}

// TestNewRequiresAuthService covers the startup guard. Without it the router
// builds an auth handler over a nil service and the process serves traffic until
// the first login panics.
func TestNewRequiresAuthService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("router.New accepted a nil AuthService")
		}
	}()

	router.New(router.Dependencies{
		Config: &config.Config{Env: config.EnvDevelopment},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
}

// TestNewRequiresEntryService covers the same guard for entries. AuthService is
// supplied so that the panic can only come from the missing entry service.
func TestNewRequiresEntryService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("router.New accepted a nil EntryService")
		}
	}()

	router.New(router.Dependencies{
		Config:      &config.Config{Env: config.EnvDevelopment},
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService: auth.NewService(auth.NewRepository(nil)),
	})
}

// TestNewRequiresTaxonomyService covers the guard for categories.
//
// Worth its own test rather than being folded into the entry one, because this is
// the dependency it would be tempting to make optional: the category endpoints
// could be left unregistered, but the entry list's ?category= filter resolves
// through this service. A nil here means that filter answers with every entry on
// the site, and a client cannot tell.
func TestNewRequiresTaxonomyService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("router.New accepted a nil TaxonomyService")
		}
	}()

	router.New(router.Dependencies{
		Config:       &config.Config{Env: config.EnvDevelopment},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:  auth.NewService(auth.NewRepository(nil)),
		EntryService: entry.NewService(entry.NewRepository(nil)),
	})
}

func TestNewRequiresSiteService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("router.New accepted a nil SiteService")
		}
	}()

	router.New(router.Dependencies{
		Config:          &config.Config{Env: config.EnvDevelopment},
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:     auth.NewService(auth.NewRepository(nil)),
		EntryService:    entry.NewService(entry.NewRepository(nil)),
		TaxonomyService: taxonomy.NewService(taxonomy.NewRepository(nil)),
	})
}

func TestNewRequiresWorldService(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("router.New accepted a nil WorldService")
		}
	}()

	router.New(router.Dependencies{
		Config:          &config.Config{Env: config.EnvDevelopment},
		Logger:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		AuthService:     auth.NewService(auth.NewRepository(nil)),
		EntryService:    entry.NewService(entry.NewRepository(nil)),
		TaxonomyService: taxonomy.NewService(taxonomy.NewRepository(nil)),
		SiteService:     site.NewService(site.NewRepository(nil)),
	})
}

// TestHealthThroughFullMiddlewareChain is the check the README documents:
// GET /health returns 200 with {"data":{"status":"ok"}}.
func TestHealthThroughFullMiddlewareChain(t *testing.T) {
	handler := newTestRouter()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	const want = `{"data":{"status":"ok"}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}

	// Every response carries a correlation id, health included.
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header is missing")
	}
}

func TestSiteRoutesAreMountedWithTheRightGuards(t *testing.T) {
	handler := newTestRouter()

	for _, tc := range []struct {
		method string
		path   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/v1/site", want: http.StatusInternalServerError},
		{method: http.MethodPatch, path: "/api/v1/admin/site", want: http.StatusUnauthorized},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{"default_theme":"lamp","revision":1}`)))
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

func TestWorldRoutesAreMountedWithTheRightGuards(t *testing.T) {
	handler := newTestRouter()

	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   int
	}{
		{method: http.MethodGet, path: "/api/v1/worlds", want: http.StatusInternalServerError},
		{method: http.MethodGet, path: "/api/v1/admin/worlds", want: http.StatusUnauthorized},
		{method: http.MethodPatch, path: "/api/v1/admin/worlds/journal", body: `{"status":"open","revision":1}`, want: http.StatusUnauthorized},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var body io.Reader
			if tc.body != "" {
				body = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, body)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

// TestEntryRoutesAreMountedWithTheRightGuards asserts on the real wiring, not on
// a handler built with a permissive guard.
//
// The entryhttp tests run their routes behind a middleware that passes
// everything, because what makes a session valid is authhttp's business. That
// leaves one thing only this file can prove: that the create route is actually
// registered behind the real RequireAuth, and the two reads are not.
//
// The nil pool is why the reads assert 500 rather than 200. Reaching a status at
// all is the point: 404 would mean the route was never registered, and 401 would
// mean a reader needs an account to see the front page.
func TestEntryRoutesAreMountedWithTheRightGuards(t *testing.T) {
	handler := newTestRouter()

	// Every route that writes, plus both admin reads. The admin ones matter most:
	// they are the only routes in the service that can return a draft, so a missing
	// guard there publishes unfinished work rather than merely allowing an edit.
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/entries", `{"title":"t","slug":"s","content_md":"b"}`},
		{http.MethodPatch, "/api/v1/entries/1", `{"title":"t"}`},
		{http.MethodDelete, "/api/v1/entries/1", ""},
		{http.MethodPost, "/api/v1/entries/1/publish", ""},
		{http.MethodPost, "/api/v1/entries/1/unpublish", ""},
		{http.MethodPost, "/api/v1/entries/1/archive", ""},
		{http.MethodGet, "/api/v1/admin/entries", ""},
		{http.MethodGet, "/api/v1/admin/entries/1", ""},
	} {
		t.Run(tc.method+" "+tc.path+" requires a session", func(t *testing.T) {
			var reader io.Reader
			if tc.body != "" {
				reader = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, reader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			// 401 from the guard, before the body is read and before the nil pool could
			// be touched. A 500 here would mean the request reached storage, which is
			// what "the guard is missing" looks like, and a 404 would mean the route was
			// never registered at all.
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401\nbody: %s", rec.Code, rec.Body.String())
			}
			if body := rec.Body.String(); !strings.Contains(body, `"code":"UNAUTHORIZED"`) {
				t.Errorf("body = %s, want UNAUTHORIZED", body)
			}
		})
	}

	// Journal reads are public under their world-scoped paths.
	for _, path := range []string{"/api/v1/journals", "/api/v1/journals/kyoto-spring"} {
		t.Run("public read: "+path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s answered 401; this route must be public", path)
			}
			if rec.Code == http.StatusNotFound {
				t.Fatalf("%s answered 404; the route is not registered", path)
			}
		})
	}

	for _, path := range []string{"/api/v1/entries", "/api/v1/entries/kyoto-spring"} {
		t.Run("legacy public read removed: "+path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

			if rec.Code == http.StatusUnauthorized {
				t.Fatalf("%s answered 401; GET should not be an authenticated read route", path)
			}
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s status = %d, want 405 once public GET is removed", path, rec.Code)
			}
		})
	}
}

// TestCategoryRoutesAreMountedWithTheRightGuards is the same check for categories.
// The public list is the one asymmetry worth stating: unlike entries, no query here
// withholds rows, so the guard is the only thing separating a reader from an editor.
func TestCategoryRoutesAreMountedWithTheRightGuards(t *testing.T) {
	handler := newTestRouter()

	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/v1/categories", `{"name":"n","slug":"s"}`},
		{http.MethodPatch, "/api/v1/categories/1", `{"name":"n"}`},
		{http.MethodDelete, "/api/v1/categories/1", ""},
		{http.MethodGet, "/api/v1/admin/categories", ""},
		{http.MethodGet, "/api/v1/admin/categories/1", ""},
	} {
		t.Run(tc.method+" "+tc.path+" requires a session", func(t *testing.T) {
			var reader io.Reader
			if tc.body != "" {
				reader = strings.NewReader(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, reader)
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401\nbody: %s", rec.Code, rec.Body.String())
			}
		})
	}

	t.Run("the public list is public", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/categories", nil))

		if rec.Code == http.StatusUnauthorized {
			t.Fatal("GET /api/v1/categories answered 401; the site navigation must be public")
		}
		if rec.Code == http.StatusNotFound {
			t.Fatal("GET /api/v1/categories answered 404; the route is not registered")
		}
	})
}

// TestTheCategoryFilterIsWired covers the one thing neither adapter's own tests can
// see: that entryhttp's CategoryResolver is actually connected to the taxonomy
// service. Each package tests its own half against a fake, so an unwired resolver
// would pass both suites and answer every filtered request with the unfiltered list.
//
// Two assertions, because the pool is nil here and a resolver that reached storage
// and one that was never wired both end in a 500:
//
//   - a malformed slug proves the resolver ran. The service refuses it as not-found
//     without a query, so a 404 can only come from the resolver having been called.
//     An unwired one would answer 500 and a dropped filter 200.
//   - a well-formed slug then proves it reaches storage rather than being answered
//     from nothing.
func TestTheCategoryFilterIsWired(t *testing.T) {
	handler := newTestRouter()

	get := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}

	t.Run("a malformed slug is refused by the resolver", func(t *testing.T) {
		// "Not A Slug" cannot match a row, and taxonomy answers that without a query.
		rec := get("/api/v1/journals?category=Not%20A%20Slug")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
		}
		// Not the router's own no-route 404: that would mean the path never matched.
		if strings.Contains(rec.Body.String(), "no route") {
			t.Fatalf("the entry list route is not registered: %s", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "no category matches this slug") {
			t.Errorf("the 404 did not come from the category resolver: %s", rec.Body.String())
		}
	})

	t.Run("a well-formed slug reaches storage", func(t *testing.T) {
		rec := get("/api/v1/journals?category=travel")

		// 200 would mean the filter was dropped and the unfiltered list served, which
		// hands a client every entry on the site with no way to tell.
		if rec.Code == http.StatusOK {
			t.Fatalf("a filtered request was answered without resolving the category: %s",
				rec.Body.String())
		}
	})

	t.Run("no filter still serves the list", func(t *testing.T) {
		// The unfiltered list must not resolve anything, or the front page would need a
		// working categories table to render at all.
		if rec := get("/api/v1/journals"); rec.Code == http.StatusNotFound {
			t.Fatalf("the unfiltered list answered 404: %s", rec.Body.String())
		}
	})
}

func TestUnmatchedRequestsUseTheErrorEnvelope(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "unknown path",
			method:     http.MethodGet,
			path:       "/does-not-exist",
			wantStatus: http.StatusNotFound,
			wantCode:   `"code":"NOT_FOUND"`,
		},
		{
			// Without HandleMethodNotAllowed this would be a misleading 404.
			name:       "known path wrong method",
			method:     http.MethodPost,
			path:       "/health",
			wantStatus: http.StatusMethodNotAllowed,
			wantCode:   `"code":"INVALID_INPUT"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := newTestRouter()

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			body := rec.Body.String()
			if !strings.Contains(body, tc.wantCode) {
				t.Errorf("body = %s, want it to contain %s", body, tc.wantCode)
			}
			if !strings.Contains(body, `"error"`) {
				t.Errorf("body = %s, want an error envelope", body)
			}
		})
	}
}

// TestLoginRateLimitIsMounted asserts the limiter guards the real login route as
// registered. The middleware's own behaviour is covered in internal/middleware;
// what is checked here is the wiring, which no unit test of the limiter can
// prove.
//
// The bodies are deliberately malformed, so each request is refused at the JSON
// binding step and never reaches storage. That is what lets this run against a
// nil pool: the limiter sits before the handler, so it counts the request either
// way.
func TestLoginRateLimitIsMounted(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimit = config.RateLimitConfig{
		LoginAttempts: 3,
		LoginWindow:   time.Minute,
		Enabled:       true,
	}
	handler := newRouterWith(cfg)

	post := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.7:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	for i := 1; i <= 3; i++ {
		if rec := post(); rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400 from the handler", i, rec.Code)
		}
	}

	rec := post()
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 4: status = %d, want 429", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"code":"RATE_LIMITED"`) {
		t.Errorf("body = %s, want RATE_LIMITED", rec.Body)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After header on the refusal")
	}
	// The limiter answers through httpx like every other error, so the request id
	// is present and a user can quote one value to find the log line.
	if !strings.Contains(rec.Body.String(), `"request_id"`) {
		t.Errorf("body = %s, want a request_id", rec.Body)
	}
}

// TestLoginRateLimitOffByConfig covers the other half of the switch: with the
// limiter disabled, nothing is added to the chain and repeated attempts keep
// reaching the handler.
func TestLoginRateLimitOffByConfig(t *testing.T) {
	cfg := testConfig()
	cfg.RateLimit = config.RateLimitConfig{
		LoginAttempts: 1,
		LoginWindow:   time.Minute,
		Enabled:       false,
	}
	handler := newRouterWith(cfg)

	for i := 1; i <= 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.8:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("attempt %d: status = %d, want 400; the disabled limiter interfered", i, rec.Code)
		}
	}
}

// TestLoginRateLimitIsPerProcess guards a mistake that would be invisible in
// production: building the limiter inside the request path instead of once at
// startup. The counts would reset on every attempt and the limit would never be
// reached. Two separate routers must not share counts, but one router must keep
// them across requests.
func TestLoginRateLimitIsPerProcess(t *testing.T) {
	newLimited := func() http.Handler {
		cfg := testConfig()
		cfg.RateLimit = config.RateLimitConfig{
			LoginAttempts: 1,
			LoginWindow:   time.Minute,
			Enabled:       true,
		}
		return newRouterWith(cfg)
	}

	post := func(handler http.Handler) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.9:54321"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}

	first := newLimited()
	if got := post(first); got != http.StatusBadRequest {
		t.Fatalf("first request: status = %d, want 400", got)
	}
	if got := post(first); got != http.StatusTooManyRequests {
		t.Errorf("second request to the same router: status = %d, want 429", got)
	}

	// A second process starts with its own counts.
	if got := post(newLimited()); got != http.StatusBadRequest {
		t.Errorf("first request to a fresh router: status = %d, want 400", got)
	}
}
