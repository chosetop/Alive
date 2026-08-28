package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"time"
	"unicode/utf8"
)

// Store is the storage the service needs.
//
// Declared here, next to the code that calls it, rather than next to the
// implementation. That is what lets the service be tested against a fake without
// a database, and it keeps the list honest: a method appears here only because
// something above it is used. Repository has a DeleteSession by id that the
// service never calls, so it is absent.
type Store interface {
	CreateUser(ctx context.Context, params CreateUserParams) (User, error)
	GetCredentialsByUsername(ctx context.Context, username string) (Credentials, error)
	GetUserByID(ctx context.Context, id int64) (User, error)
	CreateSession(ctx context.Context, params CreateSessionParams) (Session, error)
	GetSessionByHash(ctx context.Context, tokenHash []byte) (Authenticated, error)
	TouchSession(ctx context.Context, sessionID int64, expiresAt time.Time) error
	DeleteSessionByHash(ctx context.Context, tokenHash []byte) error
	DeleteExpiredSessions(ctx context.Context) (int64, error)
}

// Checked at compile time so the real repository cannot drift from the interface.
var _ Store = (*Repository)(nil)

// DefaultSessionLifetime is how long a session lives from its last renewal.
const DefaultSessionLifetime = 7 * 24 * time.Hour
const DefaultSessionAbsoluteLifetime = 30 * 24 * time.Hour

// Password length bounds.
//
// The minimum is a floor, not a policy: this account is created once, by the
// owner, from a shell. Character-class rules would push the owner towards
// "Password1!" and buy nothing.
//
// The maximum is a denial-of-service guard rather than a rule about passwords.
// Argon2id's cost is dominated by its memory parameter, but the input is still
// hashed, and there is no reason to accept a megabyte of it.
const (
	MinPasswordLength = 8
	MaxPasswordLength = 1024
)

// Username length bounds, matching the CHECK constraint on the column. Checking
// here as well turns a constraint violation into a domain error, which is what a
// CLI can print usefully.
const (
	MinUsernameLength = 3
	MaxUsernameLength = 64
)

// Validation errors, raised before any storage work.
//
// Distinct from ErrInvalidCredentials, which reports a failed login. These two
// end up as different HTTP statuses: a password below the minimum length at
// account creation is a bad request, while a wrong password at login is a
// rejected one.
var (
	// ErrInvalidUsername reports a username outside the allowed length.
	ErrInvalidUsername = errors.New("auth: invalid username")

	// ErrInvalidPassword reports a password outside the allowed length.
	ErrInvalidPassword = errors.New("auth: invalid password")
)

// Clock returns the current time. A field rather than a call to time.Now inside
// the service, so a test can place a session exactly at its expiry.
type Clock func() time.Time

// Service holds the login rules.
//
// It knows nothing about HTTP: no cookie, no header, no status code. Login takes
// a username and a password and returns a token; turning that token into a
// Set-Cookie header is the adapter's job. The CLI reaches the same methods
// without a request in sight.
type Service struct {
	store            Store
	lifetime         time.Duration
	absoluteLifetime time.Duration
	now              Clock
	log              *slog.Logger
}

// ServiceOption adjusts a Service at construction.
type ServiceOption func(*Service)

// WithSessionLifetime overrides how long a session lives.
func WithSessionLifetime(d time.Duration) ServiceOption {
	return func(s *Service) {
		if d > 0 {
			s.lifetime = d
		}
	}
}

// WithSessionAbsoluteLifetime overrides the maximum age from login.
func WithSessionAbsoluteLifetime(d time.Duration) ServiceOption {
	return func(s *Service) {
		if d > 0 {
			s.absoluteLifetime = d
		}
	}
}

// WithClock overrides the time source. For tests.
func WithClock(c Clock) ServiceOption {
	return func(s *Service) {
		if c != nil {
			s.now = c
		}
	}
}

// WithLogger sets where non-fatal problems are reported. Without it the service
// falls back to slog.Default.
func WithLogger(l *slog.Logger) ServiceOption {
	return func(s *Service) {
		if l != nil {
			s.log = l
		}
	}
}

// NewService builds a Service over store.
func NewService(store Store, opts ...ServiceOption) *Service {
	s := &Service{
		store:            store,
		lifetime:         DefaultSessionLifetime,
		absoluteLifetime: DefaultSessionAbsoluteLifetime,
		now:              time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// SessionLifetime reports the configured lifetime. The cookie's Max-Age has to
// match it, and the adapter reads it from here rather than keeping its own copy.
func (s *Service) SessionLifetime() time.Duration {
	return s.lifetime
}

// LoginParams is what a login attempt carries.
//
// UserAgent and IP are recorded on the session for later review. They are not
// part of the decision.
type LoginParams struct {
	Username  string
	Password  string
	UserAgent string
	IP        netip.Addr
}

// LoginResult is a successful login: the plaintext token for the cookie, and the
// user and session behind it.
//
// The token appears here and nowhere else. After the adapter writes it into a
// Set-Cookie header, no part of this system can produce it again.
type LoginResult struct {
	Token   Token
	User    User
	Session Session
}

// Login verifies a password and opens a session.
//
// Every failure that is not an infrastructure fault returns
// ErrInvalidCredentials. An unknown username and a wrong password are the same
// error on purpose: any difference between them, in the body or in the timing,
// tells an attacker which usernames exist.
func (s *Service) Login(ctx context.Context, params LoginParams) (LoginResult, error) {
	creds, err := s.store.GetCredentialsByUsername(ctx, params.Username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Hash anyway. Returning here without hashing would make a missing
			// user answer in microseconds and a real one in ~200 ms, and that gap
			// is a username oracle that needs no response body to read.
			VerifyPasswordDummy(params.Password)
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, err
	}

	ok, err := VerifyPassword(params.Password, creds.PasswordHash)
	if err != nil {
		// A stored hash that cannot be parsed is a broken row, not a wrong
		// password. Reporting it as a failed login would hide the real problem
		// behind an owner who believes they are mistyping.
		return LoginResult{}, fmt.Errorf("auth: verify password for %q: %w", params.Username, err)
	}
	if !ok {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, tokenHash, err := GenerateToken()
	if err != nil {
		return LoginResult{}, err
	}

	now := s.now()
	absoluteExpiresAt := now.Add(s.absoluteLifetime)
	expiresAt := now.Add(s.lifetime)
	if expiresAt.After(absoluteExpiresAt) {
		expiresAt = absoluteExpiresAt
	}
	session, err := s.store.CreateSession(ctx, CreateSessionParams{
		UserID:            creds.User.ID,
		TokenHash:         tokenHash,
		ExpiresAt:         expiresAt,
		AbsoluteExpiresAt: absoluteExpiresAt,
		UserAgent:         params.UserAgent,
		IP:                params.IP,
	})
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{Token: token, User: creds.User, Session: session}, nil
}

// Authenticate resolves a token into the user and session behind it.
//
// It returns ErrInvalidToken for a malformed value, ErrSessionNotFound for an
// unknown one, and ErrSessionExpired for a session whose lifetime ended. All
// three become the same 401 at the boundary; they are separate here because the
// logs are where the difference is worth having.
//
// A session past halfway through its lifetime is extended. Renewal failure does
// not fail the request: the token is valid, the caller is who they say they are,
// and refusing a good request because a bookkeeping write failed would be the
// wrong trade. The extension is skipped and the error logged.
func (s *Service) Authenticate(ctx context.Context, token Token) (Authenticated, error) {
	if err := ValidateTokenFormat(token); err != nil {
		// Rejected before any query. A malformed cookie is not worth a round trip.
		return Authenticated{}, err
	}

	authenticated, err := s.store.GetSessionByHash(ctx, HashToken(token))
	if err != nil {
		return Authenticated{}, err
	}

	now := s.now()
	if authenticated.Session.IsExpired(now) {
		// The row is left in place. Deleting it here would turn every request
		// carrying a stale cookie into a write, and the prune job removes it
		// anyway.
		return Authenticated{}, ErrSessionExpired
	}

	if authenticated.Session.NeedsRenewal(now, s.lifetime) {
		extended := now.Add(s.lifetime)
		if !authenticated.Session.AbsoluteExpiresAt.IsZero() && extended.After(authenticated.Session.AbsoluteExpiresAt) {
			extended = authenticated.Session.AbsoluteExpiresAt
		}
		if !extended.After(now) {
			return Authenticated{}, ErrSessionExpired
		}
		if err := s.store.TouchSession(ctx, authenticated.Session.ID, extended); err != nil {
			s.logger().WarnContext(ctx, "session renewal failed",
				slog.Int64("session_id", authenticated.Session.ID),
				slog.String("error", err.Error()),
			)
		} else {
			authenticated.Session.ExpiresAt = extended
			// Reported so the adapter can refresh the cookie. Only set when the
			// write landed: a cookie promising a lifetime the row does not have
			// would keep the browser sending a token past the server's expiry.
			authenticated.Renewed = true
		}
	}

	return authenticated, nil
}

// Logout ends the session the token belongs to.
//
// A token that is malformed or unknown is not an error. The caller asked for the
// session to be gone and it is; a logout that reports failure invites a client to
// retry, and there is nothing to retry.
func (s *Service) Logout(ctx context.Context, token Token) error {
	if err := ValidateTokenFormat(token); err != nil {
		return nil
	}
	return s.store.DeleteSessionByHash(ctx, HashToken(token))
}

// CreateUserInput is a request to create the owner account.
//
// Password is plaintext and lives only for the duration of this call. Nothing
// stores it, logs it, or returns it.
type CreateUserInput struct {
	Username    string
	Password    string
	DisplayName string
	Role        string
}

// CreateUser validates the input, hashes the password and stores the account.
//
// The CLI is the only caller. There is no registration endpoint, and the plan
// says there never will be.
func (s *Service) CreateUser(ctx context.Context, input CreateUserInput) (User, error) {
	if err := ValidateUsername(input.Username); err != nil {
		return User{}, err
	}
	if err := ValidatePassword(input.Password); err != nil {
		return User{}, err
	}

	hash, err := HashPassword(input.Password)
	if err != nil {
		return User{}, err
	}

	return s.store.CreateUser(ctx, CreateUserParams{
		Username:     input.Username,
		PasswordHash: hash,
		Role:         input.Role,
		DisplayName:  input.DisplayName,
	})
}

// PruneExpiredSessions removes every session past its expiry and reports how
// many were removed.
//
// Expired sessions are already refused by Authenticate, so this reclaims space
// rather than closing a hole.
func (s *Service) PruneExpiredSessions(ctx context.Context) (int64, error) {
	return s.store.DeleteExpiredSessions(ctx)
}

// ValidateUsername checks the length in runes.
//
// Runes, not bytes, because the column's CHECK uses length() on citext, which
// counts characters. Measuring bytes here would reject a name that the database
// accepts.
func ValidateUsername(username string) error {
	n := utf8.RuneCountInString(username)
	switch {
	case n < MinUsernameLength:
		return fmt.Errorf("%w: shorter than %d characters", ErrInvalidUsername, MinUsernameLength)
	case n > MaxUsernameLength:
		return fmt.Errorf("%w: longer than %d characters", ErrInvalidUsername, MaxUsernameLength)
	}
	return nil
}

// ValidatePassword checks the length in bytes.
//
// Bytes, not runes, because the limit exists to bound the work Argon2id does and
// Argon2id hashes bytes.
func ValidatePassword(password string) error {
	switch {
	case len(password) < MinPasswordLength:
		return fmt.Errorf("%w: shorter than %d bytes", ErrInvalidPassword, MinPasswordLength)
	case len(password) > MaxPasswordLength:
		return fmt.Errorf("%w: longer than %d bytes", ErrInvalidPassword, MaxPasswordLength)
	}
	return nil
}

func (s *Service) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}
