// Package postgres owns the PostgreSQL connection pool.
//
// The pool is created in main and passed explicitly to whatever needs it. There
// is no package-level pool variable: a global connection makes it impossible to
// run two configurations in one test binary, and it hides which components
// actually touch the database.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/p30huiwei/alive/backend/internal/config"
)

// Pool wraps *pgxpool.Pool. The wrapper exists so callers depend on this
// package rather than on pgx directly, which keeps the driver replaceable.
type Pool struct {
	*pgxpool.Pool
}

// New parses the DSN, builds the pool and verifies connectivity once.
//
// Verifying at startup is deliberate: pgxpool connects lazily, so without a
// ping a misconfigured database surfaces on the first real request instead of
// at boot.
func New(ctx context.Context, cfg config.DatabaseConfig) (*Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	poolCfg.MaxConnIdleTime = cfg.ConnMaxLifetime / 2

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return &Pool{Pool: pool}, nil
}

// Ping reports whether the database answers within timeout.
//
// Used by the health endpoint. It runs a real round trip rather than reading
// pool statistics, because a pool can hold connections that the server has
// already dropped.
func (p *Pool) Ping(ctx context.Context, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return p.Pool.Ping(ctx)
}

// Stats returns pool counters for the health endpoint.
func (p *Pool) Stats() PoolStats {
	s := p.Pool.Stat()
	return PoolStats{
		TotalConns:    s.TotalConns(),
		IdleConns:     s.IdleConns(),
		AcquiredConns: s.AcquiredConns(),
		MaxConns:      s.MaxConns(),
	}
}

// PoolStats is a driver-independent snapshot of pool usage.
type PoolStats struct {
	TotalConns    int32 `json:"total_conns"`
	IdleConns     int32 `json:"idle_conns"`
	AcquiredConns int32 `json:"acquired_conns"`
	MaxConns      int32 `json:"max_conns"`
}

// Close releases every connection. Safe to call more than once.
func (p *Pool) Close() {
	if p.Pool != nil {
		p.Pool.Close()
	}
}
