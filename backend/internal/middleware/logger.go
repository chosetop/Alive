package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// Logger writes one structured line per completed request.
//
// The logger is injected rather than taken from slog.Default() so tests can
// capture output and so main stays the only place that decides log format.
func Logger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		status := c.Writer.Status()

		attrs := []any{
			slog.String("method", c.Request.Method),
			slog.String("path", path),
			slog.Int("status", status),
			slog.Duration("latency", time.Since(start)),
			slog.String("ip", c.ClientIP()),
			slog.String("request_id", httpx.RequestIDFromContext(c.Request.Context())),
		}
		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}

		// Errors collected by handlers via c.Error are logged here. This is the
		// only place a cause is written out; it never enters a response body.
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}

		msg := "request handled"
		switch {
		case status >= 500:
			logger.Error(msg, attrs...)
		case status >= 400:
			logger.Warn(msg, attrs...)
		default:
			logger.Info(msg, attrs...)
		}
	}
}
