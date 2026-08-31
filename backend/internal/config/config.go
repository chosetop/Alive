// Package config loads and validates configuration from the environment.
//
// Configuration is read exactly once, at startup, into an immutable value that
// is passed explicitly to whatever needs it. No package-level config variable
// exists, so no code can reach configuration it was not handed.
//
// A missing or malformed value fails startup. A server that boots with a
// half-valid configuration fails later, under load, somewhere unrelated.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Env names the deployment environment.
type Env string

const (
	EnvDevelopment Env = "development"
	EnvProduction  Env = "production"
)

// Config is the fully validated configuration for one process.
type Config struct {
	Env            Env
	Server         ServerConfig
	Database       DatabaseConfig
	CORS           CORSConfig
	Session        SessionConfig
	RateLimit      RateLimitConfig
	OSS            OSSConfig
	TrustedProxies []string
}

type OSSConfig struct {
	Enabled         bool
	Region          string
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PublicBaseURL   string
	PresignTTL      time.Duration
	MaxUploadBytes  int64
}

// IsDevelopment reports whether verbose, human-oriented behaviour is allowed.
type ServerConfig struct {
	Host string
	Port int

	// ReadTimeout bounds reading the request, headers and body together.
	ReadTimeout time.Duration
	// WriteTimeout bounds writing the response.
	WriteTimeout time.Duration
	// IdleTimeout bounds how long a keep-alive connection may stay unused.
	IdleTimeout time.Duration
	// ShutdownTimeout bounds how long in-flight requests may finish during a
	// graceful shutdown before they are dropped.
	ShutdownTimeout time.Duration
}

// Addr returns the host:port the HTTP server listens on.
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// DatabaseConfig describes the PostgreSQL connection and its pool limits.
type DatabaseConfig struct {
	// DSN is a libpq-style URL: postgres://user:pass@host:port/dbname?sslmode=disable
	DSN string

	MaxOpenConns int
	MaxIdleConns int
	// ConnMaxLifetime caps how long a single connection is reused. Bounded
	// lifetime lets a restarted or failed-over database drain old connections.
	ConnMaxLifetime time.Duration
	// ConnectTimeout bounds the initial connectivity check at startup.
	ConnectTimeout time.Duration
}

// CORSConfig lists the browser origins allowed to call this API.
type CORSConfig struct {
	// AllowedOrigins holds exact origins, scheme included. Wildcards are
	// rejected: credentialed requests cannot use "*", and this API will serve
	// a cookie-authenticated admin.
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	// AllowCredentials permits cookies on cross-origin requests.
	AllowCredentials bool
	// MaxAge is how long a browser may cache the preflight result.
	MaxAge time.Duration
}

// SessionConfig describes the login session and the cookie that carries it.
type SessionConfig struct {
	// Lifetime is how long a session lives from its last renewal. It is also the
	// cookie's Max-Age, so the browser stops sending a token at about the moment
	// the server stops accepting it.
	Lifetime         time.Duration
	AbsoluteLifetime time.Duration

	// CookieName is the name of the session cookie.
	CookieName string

	// CookiePath limits which request paths carry the cookie. Narrower is better:
	// it decides where the credential is sent at all.
	CookiePath string

	// CookieSecure restricts the cookie to HTTPS.
	//
	// Environment-dependent out of necessity, not preference. Development runs on
	// http://localhost, where a Secure cookie is never sent and login appears
	// broken. validate() requires it in production, so a deployment that forgets
	// APP_ENV fails to start rather than sending a credential in the clear.
	CookieSecure bool
}

// RateLimitConfig bounds how often one client may attempt to log in.
//
// It applies to the login endpoint only. Every other route is either cheap or
// already requires a session, and a limit on them would cost the site owner
// normal use to slow down an attacker who is not attacking there.
type RateLimitConfig struct {
	// LoginAttempts is how many attempts one address may make per window.
	LoginAttempts int

	// LoginWindow is the length of the fixed counting window.
	LoginWindow time.Duration

	// Enabled turns the limiter off entirely.
	//
	// Present for one real case: tests and local scripts that log in repeatedly.
	// validate() refuses to let production disable it, because a login endpoint
	// backed by a 200 ms password hash is both a brute-force target and a way to
	// spend the server's CPU.
	Enabled bool
}

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	var errs []error
	collect := func(err error) {
		if err != nil {
			errs = append(errs, err)
		}
	}

	env, err := parseEnv(lookup("APP_ENV", string(EnvDevelopment)))
	collect(err)

	port, err := parseInt("SERVER_PORT", lookup("SERVER_PORT", "8080"))
	collect(err)

	readTimeout, err := parseDuration("SERVER_READ_TIMEOUT", lookup("SERVER_READ_TIMEOUT", "15s"))
	collect(err)

	writeTimeout, err := parseDuration("SERVER_WRITE_TIMEOUT", lookup("SERVER_WRITE_TIMEOUT", "15s"))
	collect(err)

	idleTimeout, err := parseDuration("SERVER_IDLE_TIMEOUT", lookup("SERVER_IDLE_TIMEOUT", "60s"))
	collect(err)

	shutdownTimeout, err := parseDuration("SERVER_SHUTDOWN_TIMEOUT", lookup("SERVER_SHUTDOWN_TIMEOUT", "10s"))
	collect(err)

	dsn, err := resolveDSN()
	collect(err)

	maxOpen, err := parseInt("DB_MAX_OPEN_CONNS", lookup("DB_MAX_OPEN_CONNS", "25"))
	collect(err)

	maxIdle, err := parseInt("DB_MAX_IDLE_CONNS", lookup("DB_MAX_IDLE_CONNS", "5"))
	collect(err)

	connLifetime, err := parseDuration("DB_CONN_MAX_LIFETIME", lookup("DB_CONN_MAX_LIFETIME", "30m"))
	collect(err)

	connectTimeout, err := parseDuration("DB_CONNECT_TIMEOUT", lookup("DB_CONNECT_TIMEOUT", "5s"))
	collect(err)

	// CORS_ALLOWED_ORIGINS does not go through lookup, which treats an empty
	// value as absent and substitutes the fallback. Here the two must differ:
	// setting the variable to empty is how production turns CORS off, and
	// silently replacing that with a localhost default would leave a policy in
	// place that cannot be switched off.
	rawOrigins, originsSet := os.LookupEnv("CORS_ALLOWED_ORIGINS")
	if !originsSet && env == EnvDevelopment {
		// Nuxt (3000) and Vite (5173) dev servers.
		rawOrigins = "http://localhost:3000,http://localhost:5173"
	}
	origins, err := parseOrigins(rawOrigins)
	collect(err)

	corsMaxAge, err := parseDuration("CORS_MAX_AGE", lookup("CORS_MAX_AGE", "12h"))
	collect(err)

	sessionLifetime, err := parseDuration("SESSION_LIFETIME", lookup("SESSION_LIFETIME", "168h"))
	collect(err)

	absoluteSessionLifetime, err := parseDuration("SESSION_ABSOLUTE_LIFETIME", lookup("SESSION_ABSOLUTE_LIFETIME", "720h"))
	collect(err)

	trustedProxies, err := parseTrustedProxies(lookup("TRUSTED_PROXIES", ""))
	collect(err)

	// Defaults to on. A deployment has to say so explicitly to send a session
	// cookie over plain HTTP, and validate() then refuses to let production do it.
	cookieSecure, err := parseBool("SESSION_COOKIE_SECURE", lookup("SESSION_COOKIE_SECURE", "true"))
	collect(err)

	// Read through os.LookupEnv rather than lookup, which cannot tell an absent
	// variable from one set to empty and substitutes the fallback for both. The
	// difference matters: absent means "use the default", while empty is a broken
	// deploy template. Substituting there would leave a config file and a running
	// server disagreeing about which cookie carries the session, and validate()
	// below is what turns that into a refusal to start.
	cookieName, ok := os.LookupEnv("SESSION_COOKIE_NAME")
	if !ok {
		cookieName = "alive_session"
	}

	cookiePath, ok := os.LookupEnv("SESSION_COOKIE_PATH")
	if !ok {
		cookiePath = "/api/v1"
	}

	loginAttempts, err := parseInt("RATE_LIMIT_LOGIN_ATTEMPTS", lookup("RATE_LIMIT_LOGIN_ATTEMPTS", "5"))
	collect(err)

	loginWindow, err := parseDuration("RATE_LIMIT_LOGIN_WINDOW", lookup("RATE_LIMIT_LOGIN_WINDOW", "1m"))
	collect(err)

	rateLimitEnabled, err := parseBool("RATE_LIMIT_ENABLED", lookup("RATE_LIMIT_ENABLED", "true"))
	collect(err)
	ossEnabled, err := parseBool("OSS_ENABLED", lookup("OSS_ENABLED", "false"))
	collect(err)
	ossTTL, err := parseDuration("OSS_PRESIGN_TTL", lookup("OSS_PRESIGN_TTL", "10m"))
	collect(err)
	maxUpload, err := parseInt("MEDIA_MAX_UPLOAD_BYTES", lookup("MEDIA_MAX_UPLOAD_BYTES", "104857600"))
	collect(err)

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}

	cfg := &Config{
		Env: env,
		Server: ServerConfig{
			Host:            lookup("SERVER_HOST", "127.0.0.1"),
			Port:            port,
			ReadTimeout:     readTimeout,
			WriteTimeout:    writeTimeout,
			IdleTimeout:     idleTimeout,
			ShutdownTimeout: shutdownTimeout,
		},
		Database: DatabaseConfig{
			DSN:             dsn,
			MaxOpenConns:    maxOpen,
			MaxIdleConns:    maxIdle,
			ConnMaxLifetime: connLifetime,
			ConnectTimeout:  connectTimeout,
		},
		CORS: CORSConfig{
			AllowedOrigins:   origins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
			ExposedHeaders:   []string{"X-Request-ID"},
			AllowCredentials: true,
			MaxAge:           corsMaxAge,
		},
		Session: SessionConfig{
			Lifetime:         sessionLifetime,
			AbsoluteLifetime: absoluteSessionLifetime,
			CookieName:       cookieName,
			CookiePath:       cookiePath,
			CookieSecure:     cookieSecure,
		},
		RateLimit: RateLimitConfig{
			LoginAttempts: loginAttempts,
			LoginWindow:   loginWindow,
			Enabled:       rateLimitEnabled,
		},
		OSS:            OSSConfig{Enabled: ossEnabled, Region: lookup("OSS_REGION", ""), Endpoint: lookup("OSS_ENDPOINT", ""), Bucket: lookup("OSS_BUCKET", ""), AccessKeyID: lookup("OSS_ACCESS_KEY_ID", ""), AccessKeySecret: lookup("OSS_ACCESS_KEY_SECRET", ""), PublicBaseURL: lookup("OSS_PUBLIC_BASE_URL", ""), PresignTTL: ossTTL, MaxUploadBytes: int64(maxUpload)},
		TrustedProxies: trustedProxies,
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// IsDevelopment reports whether this process runs in the development env.
func (c *Config) IsDevelopment() bool { return c.Env == EnvDevelopment }

func (c *Config) validate() error {
	var errs []error

	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Errorf("SERVER_PORT must be in 1..65535, got %d", c.Server.Port))
	}
	if c.Database.MaxOpenConns < 1 {
		errs = append(errs, fmt.Errorf("DB_MAX_OPEN_CONNS must be at least 1, got %d", c.Database.MaxOpenConns))
	}
	if c.Database.MaxIdleConns < 0 {
		errs = append(errs, fmt.Errorf("DB_MAX_IDLE_CONNS must not be negative, got %d", c.Database.MaxIdleConns))
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, fmt.Errorf(
			"DB_MAX_IDLE_CONNS (%d) must not exceed DB_MAX_OPEN_CONNS (%d)",
			c.Database.MaxIdleConns, c.Database.MaxOpenConns))
	}
	// An empty origin list is valid and is the correct production setting.
	// admin is served from /admin on the site's own domain, so browser calls to
	// /api are same-origin and no CORS exchange happens at all. Development is
	// the only case that needs origins, because the dev servers run on their own
	// ports.
	if c.Env == EnvDevelopment && len(c.CORS.AllowedOrigins) == 0 {
		errs = append(errs, errors.New(
			"CORS_ALLOWED_ORIGINS must list at least one origin in development, "+
				"where admin and frontend run on their own ports"))
	}

	if c.Session.Lifetime <= 0 {
		errs = append(errs, fmt.Errorf("SESSION_LIFETIME must be positive, got %s", c.Session.Lifetime))
	}
	if c.Session.AbsoluteLifetime <= 0 {
		errs = append(errs, fmt.Errorf("SESSION_ABSOLUTE_LIFETIME must be positive, got %s", c.Session.AbsoluteLifetime))
	}
	if c.Session.AbsoluteLifetime < c.Session.Lifetime {
		errs = append(errs, fmt.Errorf("SESSION_ABSOLUTE_LIFETIME (%s) must not be shorter than SESSION_LIFETIME (%s)", c.Session.AbsoluteLifetime, c.Session.Lifetime))
	}
	if c.Session.CookieName == "" {
		errs = append(errs, errors.New("SESSION_COOKIE_NAME must not be empty"))
	}
	// A relative path would be resolved against the request's directory, so the
	// cookie would come back on some paths and not others.
	if !strings.HasPrefix(c.Session.CookiePath, "/") {
		errs = append(errs, fmt.Errorf("SESSION_COOKIE_PATH must start with /, got %q", c.Session.CookiePath))
	}
	// Refusing to start is the whole point. A production process that boots with
	// this off sends the session credential over plain HTTP, and nothing in the
	// response would show it: login works, the cookie is set, and the token is
	// readable by anything on the path.
	if c.Env == EnvProduction && !c.Session.CookieSecure {
		errs = append(errs, errors.New(
			"SESSION_COOKIE_SECURE must be true in production; "+
				"a session cookie without Secure travels over plain HTTP"))
	}

	// Validated even when the limiter is off, so switching it on cannot be the
	// moment a nonsense value first takes effect.
	if c.RateLimit.LoginAttempts < 1 {
		errs = append(errs, fmt.Errorf(
			"RATE_LIMIT_LOGIN_ATTEMPTS must be at least 1, got %d; "+
				"use RATE_LIMIT_ENABLED=false to turn the limiter off",
			c.RateLimit.LoginAttempts))
	}
	if c.RateLimit.LoginWindow <= 0 {
		errs = append(errs, fmt.Errorf("RATE_LIMIT_LOGIN_WINDOW must be positive, got %s", c.RateLimit.LoginWindow))
	}
	if c.Env == EnvProduction && !c.RateLimit.Enabled {
		errs = append(errs, errors.New(
			"RATE_LIMIT_ENABLED must be true in production; "+
				"an unlimited login endpoint can be brute-forced, and each attempt "+
				"costs the server one password hash"))
	}
	if c.Env == EnvProduction && len(c.TrustedProxies) == 0 {
		errs = append(errs, errors.New(
			"TRUSTED_PROXIES must list at least one proxy network in production; refusing to trust forwarded client addresses implicitly"))
	}
	if c.OSS.PresignTTL <= 0 {
		errs = append(errs, errors.New("OSS_PRESIGN_TTL must be positive"))
	}
	if c.OSS.MaxUploadBytes < 1 || c.OSS.MaxUploadBytes > 104857600 {
		errs = append(errs, fmt.Errorf("MEDIA_MAX_UPLOAD_BYTES must be in 1..104857600, got %d", c.OSS.MaxUploadBytes))
	}
	if c.OSS.Enabled {
		if c.OSS.Region == "" || c.OSS.Bucket == "" || c.OSS.AccessKeyID == "" || c.OSS.AccessKeySecret == "" || c.OSS.PublicBaseURL == "" {
			errs = append(errs, errors.New("OSS_REGION, OSS_BUCKET, OSS_ACCESS_KEY_ID, OSS_ACCESS_KEY_SECRET and OSS_PUBLIC_BASE_URL are required when OSS_ENABLED=true"))
		}
		if u, err := url.Parse(c.OSS.PublicBaseURL); err != nil || u.Scheme != "https" || u.Host == "" {
			errs = append(errs, errors.New("OSS_PUBLIC_BASE_URL must be an https URL when OSS_ENABLED=true"))
		}
	}

	return errors.Join(errs...)
}

func parseTrustedProxies(raw string) ([]string, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	var proxies []string
	for _, item := range strings.Split(raw, ",") {
		proxy := strings.TrimSpace(item)
		if proxy == "" {
			return nil, errors.New("TRUSTED_PROXIES contains an empty entry")
		}
		if ip := net.ParseIP(proxy); ip != nil {
			if ip.To4() != nil {
				proxy += "/32"
			} else {
				proxy += "/128"
			}
		} else if _, network, err := net.ParseCIDR(proxy); err != nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is not an IP or CIDR: %w", proxy, err)
		} else if ones, _ := network.Mask.Size(); ones == 0 {
			return nil, fmt.Errorf("TRUSTED_PROXIES entry %q is too broad; use the proxy's IP or private network", proxy)
		}
		proxies = append(proxies, proxy)
	}
	return proxies, nil
}

// resolveDSN prefers DATABASE_URL and otherwise assembles a DSN from parts.
// Both forms are useful: hosting providers hand out one URL, while local
// development is easier to read as separate values.
func resolveDSN() (string, error) {
	if dsn, ok := os.LookupEnv("DATABASE_URL"); ok && strings.TrimSpace(dsn) != "" {
		if _, err := url.Parse(dsn); err != nil {
			return "", fmt.Errorf("DATABASE_URL is not a valid URL: %w", err)
		}
		return dsn, nil
	}

	name := lookup("DB_NAME", "")
	if name == "" {
		return "", errors.New("either DATABASE_URL or DB_NAME must be set")
	}

	u := &url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%s", lookup("DB_HOST", "127.0.0.1"), lookup("DB_PORT", "5432")),
		Path:   "/" + name,
		User:   url.UserPassword(lookup("DB_USER", "postgres"), lookup("DB_PASSWORD", "")),
	}
	u.RawQuery = url.Values{"sslmode": {lookup("DB_SSLMODE", "disable")}}.Encode()

	return u.String(), nil
}

func lookup(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func parseEnv(raw string) (Env, error) {
	switch Env(raw) {
	case EnvDevelopment, EnvProduction:
		return Env(raw), nil
	default:
		return "", fmt.Errorf("APP_ENV must be %q or %q, got %q", EnvDevelopment, EnvProduction, raw)
	}
}

func parseBool(key, raw string) (bool, error) {
	value, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean, got %q", key, raw)
	}
	return value, nil
}

func parseInt(key, raw string) (int, error) {
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, raw)
	}
	return v, nil
}

func parseDuration(key, raw string) (time.Duration, error) {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration such as 15s or 30m, got %q", key, raw)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %q", key, raw)
	}
	return d, nil
}

// parseOrigins splits a comma-separated origin list and rejects "*".
func parseOrigins(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))

	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if origin == "*" {
			return nil, errors.New(
				`CORS_ALLOWED_ORIGINS must not be "*": this API allows credentials, ` +
					"which browsers refuse to send to a wildcard origin")
		}
		u, err := url.Parse(origin)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS entry %q must be an absolute origin such as https://example.com", origin)
		}
		origins = append(origins, strings.TrimSuffix(origin, "/"))
	}

	return origins, nil
}
