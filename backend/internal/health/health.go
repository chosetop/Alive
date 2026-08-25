// Package health reports whether the process and its dependencies are usable.
//
// It is infrastructure, not a business domain, but it is shaped like one
// (handler plus the type it depends on) so it follows the same vertical layout
// the business packages will use.
//
// Two endpoints, because they answer different questions:
//
//	GET /health        is this process alive?   never touches the database
//	GET /health/ready  can it serve traffic?    pings the database
//
// Collapsing them into one is a common mistake: a liveness probe that fails on
// a database blip makes an orchestrator restart a perfectly healthy process,
// which does not bring the database back.
package health

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/postgres"
)

// pingTimeout bounds the readiness database check. A probe must answer quickly
// or it is useless; waiting on a hung database is itself the failure.
const pingTimeout = 2 * time.Second

// Handler serves the health endpoints.
type Handler struct {
	pool   *postgres.Pool
	logger *slog.Logger
}

// NewHandler wires the handler with its dependencies. The pool is passed in
// rather than reached for, so a test can supply its own.
func NewHandler(pool *postgres.Pool, logger *slog.Logger) *Handler {
	return &Handler{pool: pool, logger: logger}
}

// Register mounts the health routes on r.
func (h *Handler) Register(r gin.IRouter) {
	r.GET("/health", h.Live)
	r.GET("/health/ready", h.Ready)
}

// liveResponse is the body of a successful liveness check.
type liveResponse struct {
	Status string `json:"status"`
}

// readyResponse is the body of a successful readiness check.
type readyResponse struct {
	Status   string             `json:"status"`
	Database databaseStatus     `json:"database"`
	Pool     postgres.PoolStats `json:"pool"`
}

type databaseStatus struct {
	Status  string `json:"status"`
	Latency string `json:"latency"`
}

// Live reports that the process is running and can serve HTTP.
//
// GET /health -> 200 {"data":{"status":"ok"}}
func (h *Handler) Live(c *gin.Context) {
	httpx.OK(c, liveResponse{Status: "ok"})
}

// Ready reports whether every dependency needed to serve a request is usable.
//
// GET /health/ready -> 200 when the database answers, 503 when it does not.
func (h *Handler) Ready(c *gin.Context) {
	ctx := c.Request.Context()

	start := time.Now()
	if err := h.pool.Ping(ctx, pingTimeout); err != nil {
		// The driver error can name host, database and user, so it goes to the
		// log as a cause and the client gets a generic message.
		h.logger.Error("readiness check failed",
			slog.String("dependency", "postgres"),
			slog.String("request_id", httpx.RequestIDFromContext(ctx)),
			slog.Any("error", err),
		)
		httpx.Error(c, apperr.Unavailable("database is not reachable").WithCause(err))
		return
	}
	latency := time.Since(start)

	httpx.OK(c, readyResponse{
		Status: "ok",
		Database: databaseStatus{
			Status:  "ok",
			Latency: latency.String(),
		},
		Pool: h.pool.Stats(),
	})
}
