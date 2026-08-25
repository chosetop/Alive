package auth

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

// Repository reads and writes users and sessions.
//
// It reports facts and translates storage errors into this package's sentinel
// errors. It makes no decisions: an expired session is returned like any other,
// and whether that expiry matters is the service's call.
//
// This file is the only place in the package that imports pgx. Everything above
// it sees domain types and domain errors, so swapping the driver or putting a
// cache in front of it touches nothing else.
type Repository struct {
	q *sqlcgen.Queries
}

// NewRepository wires a repository to a connection pool.
func NewRepository(pool *postgres.Pool) *Repository {
	return &Repository{q: sqlcgen.New(pool)}
}

// uniqueViolation is the PostgreSQL error code for a broken unique constraint.
const uniqueViolation = "23505"

// CreateUserParams carries what is needed to create the owner account.
type CreateUserParams struct {
	Username     string
	PasswordHash string
	Role         string
	DisplayName  string
}

// CreateUser inserts a user and returns it.
//
// A duplicate username becomes ErrUsernameTaken. That distinction has to be made
// here: above this line nobody should have to know that 23505 means a unique
// constraint failed.
func (r *Repository) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	role := params.Role
	if role == "" {
		role = "owner"
	}

	row, err := r.q.CreateUser(ctx, sqlcgen.CreateUserParams{
		Username:     params.Username,
		PasswordHash: params.PasswordHash,
		Role:         role,
		DisplayName:  optionalString(params.DisplayName),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return User{}, fmt.Errorf("%w: %s", ErrUsernameTaken, params.Username)
		}
		return User{}, fmt.Errorf("auth: create user: %w", err)
	}

	return User{
		ID:          row.ID,
		Username:    row.Username,
		DisplayName: derefString(row.DisplayName),
		Role:        row.Role,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// GetCredentialsByUsername returns the user together with the stored password
// hash. This is the only method that exposes a hash, and only the login path
// calls it.
//
// The lookup is case-insensitive because the column is citext, not because
// anything here lowercases the input.
func (r *Repository) GetCredentialsByUsername(ctx context.Context, username string) (Credentials, error) {
	row, err := r.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Credentials{}, fmt.Errorf("%w: %s", ErrUserNotFound, username)
		}
		return Credentials{}, fmt.Errorf("auth: get user by username: %w", err)
	}

	return Credentials{
		User: User{
			ID:          row.ID,
			Username:    row.Username,
			DisplayName: derefString(row.DisplayName),
			Role:        row.Role,
			CreatedAt:   row.CreatedAt,
			UpdatedAt:   row.UpdatedAt,
		},
		PasswordHash: row.PasswordHash,
	}, nil
}

// GetUserByID returns a user without any password material.
func (r *Repository) GetUserByID(ctx context.Context, id int64) (User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, fmt.Errorf("%w: id %d", ErrUserNotFound, id)
		}
		return User{}, fmt.Errorf("auth: get user by id: %w", err)
	}

	return User{
		ID:          row.ID,
		Username:    row.Username,
		DisplayName: derefString(row.DisplayName),
		Role:        row.Role,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}, nil
}

// CreateSessionParams carries what is needed to open a session.
//
// TokenHash is a digest. The plaintext token never reaches this layer, which is
// enforced by the Token type living outside it.
type CreateSessionParams struct {
	UserID    int64
	TokenHash []byte
	ExpiresAt time.Time
	UserAgent string
	IP        netip.Addr
}

// CreateSession inserts a session and returns it.
func (r *Repository) CreateSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	row, err := r.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		UserID:    params.UserID,
		TokenHash: params.TokenHash,
		ExpiresAt: params.ExpiresAt,
		UserAgent: optionalString(params.UserAgent),
		Ip:        optionalAddr(params.IP),
	})
	if err != nil {
		return Session{}, fmt.Errorf("auth: create session: %w", err)
	}

	return Session{
		ID:        row.ID,
		UserID:    row.UserID,
		TokenHash: params.TokenHash,
		ExpiresAt: row.ExpiresAt,
		CreatedAt: row.CreatedAt,
		UserAgent: params.UserAgent,
		IP:        params.IP,
	}, nil
}

// GetSessionByHash returns a session and its user in one round trip.
//
// An expired session is returned, not rejected. The service decides what expiry
// means; if this method filtered on it, an unknown token and a known expired one
// would arrive as the same error and the two events could not be told apart.
func (r *Repository) GetSessionByHash(ctx context.Context, tokenHash []byte) (Authenticated, error) {
	row, err := r.q.GetSessionByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Authenticated{}, ErrSessionNotFound
		}
		return Authenticated{}, fmt.Errorf("auth: get session by hash: %w", err)
	}

	return Authenticated{
		User: User{
			ID:          row.UserIDRef,
			Username:    row.UserUsername,
			DisplayName: derefString(row.UserDisplayName),
			Role:        row.UserRole,
			CreatedAt:   row.UserCreatedAt,
			UpdatedAt:   row.UserUpdatedAt,
		},
		Session: Session{
			ID:        row.ID,
			UserID:    row.UserID,
			TokenHash: row.TokenHash,
			ExpiresAt: row.ExpiresAt,
			CreatedAt: row.CreatedAt,
			UserAgent: derefString(row.UserAgent),
			IP:        derefAddr(row.Ip),
		},
	}, nil
}

// TouchSession moves a session's expiry. Used by sliding renewal.
func (r *Repository) TouchSession(ctx context.Context, sessionID int64, expiresAt time.Time) error {
	err := r.q.TouchSession(ctx, sqlcgen.TouchSessionParams{
		ID:        sessionID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return fmt.Errorf("auth: touch session: %w", err)
	}
	return nil
}

// DeleteSessionByHash ends a session, given the digest from the cookie.
//
// Deleting a row that is not there is not an error. The caller wants the session
// gone, and it is; reporting a failure would make a repeated logout look broken.
func (r *Repository) DeleteSessionByHash(ctx context.Context, tokenHash []byte) error {
	if err := r.q.DeleteSessionByHash(ctx, tokenHash); err != nil {
		return fmt.Errorf("auth: delete session by hash: %w", err)
	}
	return nil
}

// DeleteSession ends a session by id, for a caller that already holds one.
func (r *Repository) DeleteSession(ctx context.Context, sessionID int64) error {
	if err := r.q.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes every session past its expiry and reports how
// many were removed.
func (r *Repository) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	removed, err := r.q.DeleteExpiredSessions(ctx)
	if err != nil {
		return 0, fmt.Errorf("auth: delete expired sessions: %w", err)
	}
	return removed, nil
}

// Conversions between domain zero values and SQL NULL.
//
// The domain uses "" and an invalid netip.Addr for absent values, which keeps
// pointers out of the types that most code handles. The database uses NULL.
// These four functions are the boundary.

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func optionalAddr(addr netip.Addr) *netip.Addr {
	if !addr.IsValid() {
		return nil
	}
	return &addr
}

func derefAddr(addr *netip.Addr) netip.Addr {
	if addr == nil {
		return netip.Addr{}
	}
	return *addr
}
