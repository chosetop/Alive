package middleware_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/middleware"
)

// discardLogger keeps refusal warnings out of the test output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRateLimiterAllowsUpToTheLimit(t *testing.T) {
	limiter := middleware.NewRateLimiter(5, time.Minute)

	for i := 1; i <= 5; i++ {
		allowed, retryAfter := limiter.Allow("10.0.0.1")
		if !allowed {
			t.Fatalf("attempt %d was refused, want allowed", i)
		}
		if retryAfter != 0 {
			t.Errorf("attempt %d: retryAfter = %s, want 0", i, retryAfter)
		}
	}

	// The sixth is the first refusal: the limit is "5 per window", so 5 pass.
	allowed, retryAfter := limiter.Allow("10.0.0.1")
	if allowed {
		t.Error("the 6th attempt was allowed, want refused")
	}
	if retryAfter <= 0 || retryAfter > time.Minute {
		t.Errorf("retryAfter = %s, want within (0, 1m]", retryAfter)
	}
}

// TestRateLimiterRefusalDoesNotExtendTheWindow is the property that keeps this a
// fixed window. If a refused attempt bumped the counter's reset time, a client
// that kept retrying would never be let back in.
func TestRateLimiterRefusalDoesNotExtendTheWindow(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	clock := now
	limiter := middleware.NewRateLimiter(2, time.Minute,
		middleware.WithClock(func() time.Time { return clock }))

	limiter.Allow("10.0.0.1")
	limiter.Allow("10.0.0.1")

	// Refused repeatedly through the window.
	for i := 0; i < 10; i++ {
		clock = clock.Add(5 * time.Second)
		if allowed, _ := limiter.Allow("10.0.0.1"); allowed {
			t.Fatalf("attempt at %s was allowed", clock.Sub(now))
		}
	}

	// The original window closes one minute after the first attempt, whatever
	// happened in between.
	clock = now.Add(time.Minute)
	if allowed, _ := limiter.Allow("10.0.0.1"); !allowed {
		t.Error("still refused after the window closed; a refusal extended it")
	}
}

func TestRateLimiterWindowReset(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	clock := now
	limiter := middleware.NewRateLimiter(1, time.Minute,
		middleware.WithClock(func() time.Time { return clock }))

	if allowed, _ := limiter.Allow("10.0.0.1"); !allowed {
		t.Fatal("the first attempt was refused")
	}
	if allowed, _ := limiter.Allow("10.0.0.1"); allowed {
		t.Fatal("the second attempt in the window was allowed")
	}

	tests := []struct {
		name    string
		advance time.Duration
		want    bool
	}{
		{name: "one tick before the boundary", advance: time.Minute - time.Nanosecond, want: false},
		{name: "exactly at the boundary", advance: time.Minute, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Each case starts from a fresh limiter so the cases do not consume each
			// other's budget.
			clock = now
			limiter := middleware.NewRateLimiter(1, time.Minute,
				middleware.WithClock(func() time.Time { return clock }))
			limiter.Allow("10.0.0.1")

			clock = now.Add(tc.advance)
			allowed, _ := limiter.Allow("10.0.0.1")
			if allowed != tc.want {
				t.Errorf("allowed = %v, want %v", allowed, tc.want)
			}
		})
	}
}

// TestRateLimiterKeysAreIndependent guards the point of a per-client limit: one
// address exhausting its budget must not lock anyone else out.
func TestRateLimiterKeysAreIndependent(t *testing.T) {
	limiter := middleware.NewRateLimiter(2, time.Minute)

	limiter.Allow("10.0.0.1")
	limiter.Allow("10.0.0.1")
	if allowed, _ := limiter.Allow("10.0.0.1"); allowed {
		t.Fatal("the first address was not limited")
	}

	if allowed, _ := limiter.Allow("10.0.0.2"); !allowed {
		t.Error("a second address was refused because the first was over its limit")
	}
}

// TestRateLimiterEvictsExpiredEntries covers the leak. Without eviction, every
// address that ever called leaves an entry behind, and an attacker walking
// source addresses grows the map without bound.
func TestRateLimiterEvictsExpiredEntries(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	clock := now
	limiter := middleware.NewRateLimiter(5, time.Minute,
		middleware.WithClock(func() time.Time { return clock }))

	for i := 0; i < 100; i++ {
		limiter.Allow("10.0.0." + strconv.Itoa(i))
	}
	if got := limiter.Size(); got != 100 {
		t.Fatalf("Size = %d, want 100", got)
	}

	// One later call sweeps the closed windows, including its own predecessor.
	clock = now.Add(time.Minute + time.Second)
	limiter.Allow("192.168.1.1")

	if got := limiter.Size(); got != 1 {
		t.Errorf("Size = %d after the window closed, want 1 (only the live entry)", got)
	}
}

func TestRateLimiterIsConcurrencySafe(t *testing.T) {
	limiter := middleware.NewRateLimiter(50, time.Minute)

	const goroutines = 20
	const each = 10

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowedCount := 0

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if allowed, _ := limiter.Allow("10.0.0.1"); allowed {
					mu.Lock()
					allowedCount++
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()

	// 200 attempts against a limit of 50. Exactly 50 must pass: a lost update
	// under the mutex would show up as more.
	if allowedCount != 50 {
		t.Errorf("allowed %d of %d attempts, want exactly 50", allowedCount, goroutines*each)
	}
}

// newLimitedServer mounts the limiter over a handler that records whether it ran.
// A refused request must not reach the handler at all; on the real login route
// that is what stops an attacker from spending 200 ms of CPU per attempt.
func newLimitedServer(t *testing.T, limiter *middleware.RateLimiter, reached *int) http.Handler {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.POST("/api/v1/auth/login", limiter.Middleware(discardLogger()), func(c *gin.Context) {
		*reached++
		c.Status(http.StatusOK)
	})

	return engine
}

func post(srv http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	// gin.ClientIP falls back to RemoteAddr while no proxy is trusted.
	req.RemoteAddr = ip + ":54321"
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	return rec
}

func TestMiddlewareRefusesOverTheLimit(t *testing.T) {
	reached := 0
	limiter := middleware.NewRateLimiter(5, time.Minute)
	srv := newLimitedServer(t, limiter, &reached)

	for i := 1; i <= 5; i++ {
		if rec := post(srv, "10.0.0.1"); rec.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want 200", i, rec.Code)
		}
	}

	rec := post(srv, "10.0.0.1")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("the 6th request: status = %d, want 429", rec.Code)
	}

	// The handler ran five times, not six.
	if reached != 5 {
		t.Errorf("the handler ran %d times, want 5", reached)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if body.Error.Code != "RATE_LIMITED" {
		t.Errorf("code = %q, want RATE_LIMITED", body.Error.Code)
	}
	// The message must not say how many attempts are left or when exactly the
	// window opened; Retry-After carries what a client legitimately needs.
	if body.Error.Message == "" {
		t.Error("the error carries no message")
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("no Retry-After header")
	}
	seconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		t.Fatalf("Retry-After = %q, want an integer count of seconds", retryAfter)
	}
	// Never 0: a client honouring it would retry immediately into another refusal.
	if seconds < 1 || seconds > 60 {
		t.Errorf("Retry-After = %d, want within 1..60", seconds)
	}
}

func TestMiddlewareSeparatesAddresses(t *testing.T) {
	reached := 0
	limiter := middleware.NewRateLimiter(1, time.Minute)
	srv := newLimitedServer(t, limiter, &reached)

	if rec := post(srv, "10.0.0.1"); rec.Code != http.StatusOK {
		t.Fatalf("first address, first request: status = %d, want 200", rec.Code)
	}
	if rec := post(srv, "10.0.0.1"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("first address, second request: status = %d, want 429", rec.Code)
	}
	if rec := post(srv, "10.0.0.2"); rec.Code != http.StatusOK {
		t.Errorf("second address: status = %d, want 200", rec.Code)
	}
}

// TestLoginRateLimitDisabled covers the switch. Returning nil rather than a
// pass-through handler is what keeps a disabled limiter out of the chain.
func TestLoginRateLimitDisabled(t *testing.T) {
	off := middleware.LoginRateLimit(config.RateLimitConfig{
		LoginAttempts: 5,
		LoginWindow:   time.Minute,
		Enabled:       false,
	}, discardLogger())
	if off != nil {
		t.Error("a disabled limiter returned a handler, want nil")
	}

	on := middleware.LoginRateLimit(config.RateLimitConfig{
		LoginAttempts: 5,
		LoginWindow:   time.Minute,
		Enabled:       true,
	}, discardLogger())
	if on == nil {
		t.Error("an enabled limiter returned nil")
	}
}

// TestNewRateLimiterRejectsNonsense checks the fallbacks. A limit of 0 would
// refuse every login, which is a locked-out site rather than a protected one.
func TestNewRateLimiterRejectsNonsense(t *testing.T) {
	tests := []struct {
		name   string
		limit  int
		window time.Duration
	}{
		{name: "zero limit", limit: 0, window: time.Minute},
		{name: "negative limit", limit: -1, window: time.Minute},
		{name: "zero window", limit: 5, window: 0},
		{name: "negative window", limit: 5, window: -time.Minute},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			limiter := middleware.NewRateLimiter(tc.limit, tc.window)
			if allowed, _ := limiter.Allow("10.0.0.1"); !allowed {
				t.Error("the first attempt was refused; the fallback did not apply")
			}
		})
	}
}
