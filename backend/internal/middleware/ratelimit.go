package middleware

import (
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// RateLimiter counts requests per client address inside a fixed window.
//
// In-process and in-memory by decision: this is one server, and a limiter that
// needed Redis would add a network dependency that can fail, taking the login
// endpoint down with it. The cost of that choice is stated plainly: the counts
// are per process, so they reset on restart and would not be shared if the
// service were ever run as more than one instance.
//
// A fixed window, not a sliding one. It admits up to twice the limit across a
// window boundary — five attempts at 0:59 and five more at 1:01. Against
// password guessing that is irrelevant: ten attempts per minute is still four
// orders of magnitude short of useful. A sliding window would have to keep a
// timestamp per request, which is more memory and more code for no gain here.
//
// A RateLimiter is safe for concurrent use.
type RateLimiter struct {
	limit  int
	window time.Duration
	now    func() time.Time

	mu       sync.Mutex
	counters map[string]*counter
}

// counter is one client's attempt count, valid until resetAt.
type counter struct {
	count   int
	resetAt time.Time
}

// RateLimiterOption configures a RateLimiter.
type RateLimiterOption func(*RateLimiter)

// WithClock replaces the time source. Tests use it to cross a window boundary
// without sleeping through one.
func WithClock(now func() time.Time) RateLimiterOption {
	return func(r *RateLimiter) {
		if now != nil {
			r.now = now
		}
	}
}

// NewRateLimiter builds a limiter allowing limit requests per window.
//
// A limit below 1 or a window at or below zero would lock the endpoint or
// divide by nothing; both fall back to the configured defaults rather than
// panicking, because config.validate already rejects them at startup and this
// constructor is not the right place to take a process down.
func NewRateLimiter(limit int, window time.Duration, opts ...RateLimiterOption) *RateLimiter {
	if limit < 1 {
		limit = 5
	}
	if window <= 0 {
		window = time.Minute
	}

	r := &RateLimiter{
		limit:    limit,
		window:   window,
		now:      time.Now,
		counters: make(map[string]*counter),
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

// Allow records an attempt by key and reports whether it is within the limit.
//
// The second return value is how long the caller must wait, and is meaningful
// only when the attempt was refused.
func (r *RateLimiter) Allow(key string) (bool, time.Duration) {
	now := r.now()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Sweep expired entries on the way past. Without this the map keeps one entry
	// per address that ever called, forever, which is a leak an attacker can drive
	// by walking source addresses. Doing it here rather than in a background
	// goroutine removes something whose lifetime would have to be managed, and the
	// cost is bounded by the number of live entries.
	r.sweep(now)

	w, ok := r.counters[key]
	if !ok || now.After(w.resetAt) || now.Equal(w.resetAt) {
		r.counters[key] = &counter{count: 1, resetAt: now.Add(r.window)}
		return true, 0
	}

	if w.count >= r.limit {
		// The count is deliberately not incremented. Counting refused attempts
		// would push resetAt further out on every retry, so a client hammering the
		// endpoint would extend its own block indefinitely and the limit would stop
		// being the fixed window it is documented as.
		return false, w.resetAt.Sub(now)
	}

	w.count++
	return true, 0
}

// sweep deletes entries whose window has closed. Callers must hold r.mu.
func (r *RateLimiter) sweep(now time.Time) {
	for key, w := range r.counters {
		if now.After(w.resetAt) || now.Equal(w.resetAt) {
			delete(r.counters, key)
		}
	}
}

// Size reports how many addresses are currently tracked. Exposed for tests,
// which assert that expired entries are actually released.
func (r *RateLimiter) Size() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.counters)
}

// Middleware rejects a request over the limit with 429.
//
// The key is the client address. gin.ClientIP honours proxy headers only for
// proxies the engine trusts, and the router trusts none, so today this is the
// socket's own address.
//
// Before deployment behind Caddy, SetTrustedProxies must name the proxy.
// Otherwise every request arrives from the proxy's single address, all callers
// land in one bucket, and this stops being a per-client limit: it becomes a
// site-wide cap of limit logins per window, where one attacker locks out the
// owner. See docs/progress.md.
func (r *RateLimiter) Middleware(logger *slog.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = slog.Default()
	}

	return func(c *gin.Context) {
		allowed, retryAfter := r.Allow(c.ClientIP())
		if allowed {
			c.Next()
			return
		}

		// Rounded up: a Retry-After of 0 invites an immediate retry that is certain
		// to be refused.
		seconds := int(retryAfter.Seconds())
		if retryAfter > 0 && retryAfter%time.Second != 0 {
			seconds++
		}
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))

		logger.WarnContext(c.Request.Context(), "rate limit exceeded",
			slog.String("ip", c.ClientIP()),
			slog.String("path", c.FullPath()),
			slog.Int("retry_after_seconds", seconds),
		)

		// httpx.Error aborts, so the handler never runs and no password is hashed.
		httpx.Error(c, apperr.RateLimited("too many attempts, please try again later"))
	}
}

// LoginRateLimit returns the middleware to put in front of the login route, or
// nil when the limiter is disabled.
//
// Returning nil rather than a pass-through handler means a disabled limiter adds
// nothing to the chain at all: the caller appends the result only when it is
// non-nil, so there is no middleware frame to wonder about while reading a
// stack trace.
func LoginRateLimit(cfg config.RateLimitConfig, logger *slog.Logger) gin.HandlerFunc {
	if !cfg.Enabled {
		return nil
	}
	return NewRateLimiter(cfg.LoginAttempts, cfg.LoginWindow).Middleware(logger)
}
