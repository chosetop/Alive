package config_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/config"
)

// setMinimalEnv provides the one value that has no default.
func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://alive:alive@127.0.0.1:5432/alive?sslmode=disable")
}

// TestCORSDefaultsPerEnvironment covers the same-origin deployment decision:
// admin is served from /admin on the site's own domain, so production needs no
// allowed origins, while development needs the dev server ports.
func TestCORSDefaultsPerEnvironment(t *testing.T) {
	t.Run("development defaults to the dev server ports", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "development")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load() = %v, want no error", err)
		}
		if len(cfg.CORS.AllowedOrigins) != 2 {
			t.Errorf("AllowedOrigins = %v, want 2 entries", cfg.CORS.AllowedOrigins)
		}
	})

	t.Run("production defaults to no origins", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load() = %v, want no error", err)
		}
		if len(cfg.CORS.AllowedOrigins) != 0 {
			t.Errorf("AllowedOrigins = %v, want none: production is same-origin", cfg.CORS.AllowedOrigins)
		}
	})

	t.Run("development rejects an empty list", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "development")
		t.Setenv("CORS_ALLOWED_ORIGINS", "")

		if _, err := config.Load(); err == nil {
			t.Error("Load() = nil error, want a failure: dev servers are cross-origin")
		}
	})
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantMsg string
	}{
		{
			name:    "wildcard origin",
			env:     map[string]string{"CORS_ALLOWED_ORIGINS": "*"},
			wantMsg: "must not be",
		},
		{
			name:    "origin without a scheme",
			env:     map[string]string{"CORS_ALLOWED_ORIGINS": "localhost:3000"},
			wantMsg: "absolute origin",
		},
		{
			name:    "port out of range",
			env:     map[string]string{"SERVER_PORT": "99999"},
			wantMsg: "SERVER_PORT",
		},
		{
			name:    "idle pool larger than open pool",
			env:     map[string]string{"DB_MAX_IDLE_CONNS": "50", "DB_MAX_OPEN_CONNS": "10"},
			wantMsg: "must not exceed",
		},
		{
			name:    "unknown environment name",
			env:     map[string]string{"APP_ENV": "staging"},
			wantMsg: "APP_ENV",
		},
		{
			name:    "duration without a unit",
			env:     map[string]string{"SERVER_READ_TIMEOUT": "15"},
			wantMsg: "duration",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setMinimalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatal("Load() = nil error, want a failure")
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wantMsg)
			}
		})
	}
}

// TestLoadRequiresADatabase guards the one value with no usable default.
func TestLoadRequiresADatabase(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_NAME", "")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() = nil error, want a failure")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Errorf("error = %q, want it to mention DATABASE_URL", err)
	}
}

// TestLoadReportsEveryProblemAtOnce matters for usability: fixing one variable
// per run makes a misconfigured deployment a slow guessing game.
func TestLoadReportsEveryProblemAtOnce(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("SERVER_PORT", "99999")
	t.Setenv("DB_MAX_IDLE_CONNS", "50")
	t.Setenv("DB_MAX_OPEN_CONNS", "10")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() = nil error, want a failure")
	}

	msg := err.Error()
	if !strings.Contains(msg, "SERVER_PORT") || !strings.Contains(msg, "must not exceed") {
		t.Errorf("error = %q, want it to report both problems", msg)
	}
}

func TestSessionDefaults(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if want := 7 * 24 * time.Hour; cfg.Session.Lifetime != want {
		t.Errorf("Lifetime = %v, want %v", cfg.Session.Lifetime, want)
	}
	if want := 30 * 24 * time.Hour; cfg.Session.AbsoluteLifetime != want {
		t.Errorf("AbsoluteLifetime = %v, want %v", cfg.Session.AbsoluteLifetime, want)
	}
	if cfg.Session.CookieName != "alive_session" {
		t.Errorf("CookieName = %q, want %q", cfg.Session.CookieName, "alive_session")
	}
	if cfg.Session.CookiePath != "/api/v1" {
		t.Errorf("CookiePath = %q, want %q", cfg.Session.CookiePath, "/api/v1")
	}
	// Secure defaults to on, so a deployment has to opt out explicitly rather than
	// forget to opt in.
	if !cfg.Session.CookieSecure {
		t.Error("CookieSecure = false, want the default to be true")
	}
	if len(cfg.TrustedProxies) != 0 {
		t.Errorf("TrustedProxies = %v, want none in development", cfg.TrustedProxies)
	}
}

func TestProductionRequiresTrustedProxyConfiguration(t *testing.T) {
	t.Run("production rejects an omitted proxy list", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_COOKIE_SECURE", "true")

		_, err := config.Load()
		if err == nil {
			t.Fatal("Load() = nil error, want a refusal")
		}
		if !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
			t.Errorf("error = %q, want it to name TRUSTED_PROXIES", err)
		}
	})

	t.Run("production accepts explicit proxy CIDRs", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_COOKIE_SECURE", "true")
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32, 10.0.0.0/8")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got, want := cfg.TrustedProxies, []string{"127.0.0.1/32", "10.0.0.0/8"}; !reflect.DeepEqual(got, want) {
			t.Errorf("TrustedProxies = %v, want %v", got, want)
		}
	})
}

// TestProductionRequiresSecureCookie covers the trap the plan called out: if
// production boots with Secure off, login still works, the cookie is still set,
// and the session token travels in the clear with nothing in the response to show
// it. Refusing to start is the only place that failure becomes visible.
func TestProductionRequiresSecureCookie(t *testing.T) {
	t.Run("production rejects an insecure cookie", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_COOKIE_SECURE", "false")
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

		_, err := config.Load()
		if err == nil {
			t.Fatal("Load() = nil error, want a refusal")
		}
		if !strings.Contains(err.Error(), "SESSION_COOKIE_SECURE") {
			t.Errorf("error = %q, want it to name SESSION_COOKIE_SECURE", err)
		}
	})

	// Development is the reason the setting exists at all: http://localhost never
	// receives a Secure cookie, so requiring it everywhere would break local login.
	t.Run("development allows an insecure cookie", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "development")
		t.Setenv("SESSION_COOKIE_SECURE", "false")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.Session.CookieSecure {
			t.Error("CookieSecure = true, want false")
		}
	})

	t.Run("production accepts a secure cookie", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_COOKIE_SECURE", "true")
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

		if _, err := config.Load(); err != nil {
			t.Errorf("Load: %v", err)
		}
	})
}

func TestSessionRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "unparsable boolean",
			env:  map[string]string{"SESSION_COOKIE_SECURE": "yes-please"},
			want: "SESSION_COOKIE_SECURE",
		},
		{
			name: "unparsable duration",
			env:  map[string]string{"SESSION_LIFETIME": "one week"},
			want: "SESSION_LIFETIME",
		},
		{
			// A zero lifetime would issue sessions that are expired on arrival.
			name: "zero lifetime",
			env:  map[string]string{"SESSION_LIFETIME": "0s"},
			want: "SESSION_LIFETIME",
		},
		{
			name: "negative lifetime",
			env:  map[string]string{"SESSION_LIFETIME": "-1h"},
			want: "SESSION_LIFETIME",
		},
		{
			name: "zero absolute lifetime",
			env:  map[string]string{"SESSION_ABSOLUTE_LIFETIME": "0s"},
			want: "SESSION_ABSOLUTE_LIFETIME",
		},
		{
			name: "invalid proxy list",
			env:  map[string]string{"TRUSTED_PROXIES": "not-a-network"},
			want: "TRUSTED_PROXIES",
		},
		{
			name: "public proxy network",
			env:  map[string]string{"TRUSTED_PROXIES": "0.0.0.0/0"},
			want: "TRUSTED_PROXIES",
		},
		{
			name: "empty cookie name",
			env:  map[string]string{"SESSION_COOKIE_NAME": ""},
			want: "SESSION_COOKIE_NAME",
		},
		{
			// Set-but-empty, like the name above. Both are caught only because these
			// two variables are read with os.LookupEnv; the shared lookup helper
			// substitutes its fallback for an empty value and would hide this.
			name: "empty cookie path",
			env:  map[string]string{"SESSION_COOKIE_PATH": ""},
			want: "SESSION_COOKIE_PATH",
		},
		{
			// A relative path is resolved against the request's directory, so the
			// cookie would come back on some paths and not others.
			name: "relative cookie path",
			env:  map[string]string{"SESSION_COOKIE_PATH": "api/v1"},
			want: "SESSION_COOKIE_PATH",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setMinimalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatal("Load() = nil error, want a failure")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %s", err, tc.want)
			}
		})
	}
}

func TestRateLimitDefaults(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.RateLimit.LoginAttempts != 5 {
		t.Errorf("LoginAttempts = %d, want 5", cfg.RateLimit.LoginAttempts)
	}
	if cfg.RateLimit.LoginWindow != time.Minute {
		t.Errorf("LoginWindow = %v, want 1m", cfg.RateLimit.LoginWindow)
	}
	// On by default, like SESSION_COOKIE_SECURE: a deployment has to say so to
	// leave the login endpoint unlimited.
	if !cfg.RateLimit.Enabled {
		t.Error("Enabled = false, want the default to be true")
	}
}

// TestProductionRequiresRateLimit mirrors the Secure-cookie guard. A production
// process with the limiter off looks entirely healthy: logins work, nothing is
// logged, and the endpoint will answer password guesses as fast as it can hash
// them.
func TestProductionRequiresRateLimit(t *testing.T) {
	t.Run("production rejects a disabled limiter", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "production")
		t.Setenv("SESSION_COOKIE_SECURE", "true")
		t.Setenv("RATE_LIMIT_ENABLED", "false")
		t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")

		_, err := config.Load()
		if err == nil {
			t.Fatal("Load() = nil error, want a refusal")
		}
		if !strings.Contains(err.Error(), "RATE_LIMIT_ENABLED") {
			t.Errorf("error = %q, want it to name RATE_LIMIT_ENABLED", err)
		}
	})

	// Development is why the switch exists: a script that logs in repeatedly, or a
	// test run, should not have to wait out a window.
	t.Run("development allows a disabled limiter", func(t *testing.T) {
		setMinimalEnv(t)
		t.Setenv("APP_ENV", "development")
		t.Setenv("RATE_LIMIT_ENABLED", "false")

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.RateLimit.Enabled {
			t.Error("Enabled = true, want false")
		}
	})
}

func TestRateLimitRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "unparsable attempts",
			env:  map[string]string{"RATE_LIMIT_LOGIN_ATTEMPTS": "a few"},
			want: "RATE_LIMIT_LOGIN_ATTEMPTS",
		},
		{
			// Zero would refuse every login, including the owner's. That is a
			// locked-out site, not a protected one.
			name: "zero attempts",
			env:  map[string]string{"RATE_LIMIT_LOGIN_ATTEMPTS": "0"},
			want: "RATE_LIMIT_LOGIN_ATTEMPTS",
		},
		{
			name: "negative attempts",
			env:  map[string]string{"RATE_LIMIT_LOGIN_ATTEMPTS": "-5"},
			want: "RATE_LIMIT_LOGIN_ATTEMPTS",
		},
		{
			name: "unparsable window",
			env:  map[string]string{"RATE_LIMIT_LOGIN_WINDOW": "a minute"},
			want: "RATE_LIMIT_LOGIN_WINDOW",
		},
		{
			name: "zero window",
			env:  map[string]string{"RATE_LIMIT_LOGIN_WINDOW": "0s"},
			want: "RATE_LIMIT_LOGIN_WINDOW",
		},
		{
			name: "unparsable enabled",
			env:  map[string]string{"RATE_LIMIT_ENABLED": "sometimes"},
			want: "RATE_LIMIT_ENABLED",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setMinimalEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}

			_, err := config.Load()
			if err == nil {
				t.Fatal("Load() = nil error, want a failure")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %s", err, tc.want)
			}
		})
	}
}

// TestRateLimitValidatedWhileDisabled: a bad value must fail at startup even
// when the limiter is off, so that switching it on later is not the moment the
// value first takes effect.
func TestRateLimitValidatedWhileDisabled(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("RATE_LIMIT_ENABLED", "false")
	t.Setenv("RATE_LIMIT_LOGIN_ATTEMPTS", "0")

	_, err := config.Load()
	if err == nil {
		t.Fatal("Load() = nil error, want a failure")
	}
	if !strings.Contains(err.Error(), "RATE_LIMIT_LOGIN_ATTEMPTS") {
		t.Errorf("error = %q, want it to name RATE_LIMIT_LOGIN_ATTEMPTS", err)
	}
}

func TestLoadOSSConfigDefaultsDisabled(t *testing.T) {
	setMinimalEnv(t)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OSS.Enabled {
		t.Fatal("OSS should be disabled by default")
	}
}
