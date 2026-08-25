package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// Recovery turns a panic into a 500 in the standard error envelope.
//
// Gin ships gin.Recovery(), which writes a plain-text 500 and so would be the
// one response in the API that is not JSON. This version logs the stack and
// answers through httpx.Error like every other failure.
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			// A client that disconnects mid-write makes the write fail, not the
			// server. There is no response left to send, so close the connection.
			if isBrokenPipe(recovered) {
				logger.Warn("client closed connection",
					slog.String("path", c.Request.URL.Path),
					slog.String("request_id", httpx.RequestIDFromContext(c.Request.Context())),
				)
				c.Abort()
				return
			}

			err := fmt.Errorf("panic: %v", recovered)

			logger.Error("recovered from panic",
				slog.String("method", c.Request.Method),
				slog.String("path", c.Request.URL.Path),
				slog.String("request_id", httpx.RequestIDFromContext(c.Request.Context())),
				slog.Any("panic", recovered),
				slog.String("stack", string(debug.Stack())),
			)

			// The panic value may name internal state, so it stays in the log
			// as a cause and does not reach the response.
			httpx.Error(c, apperr.Internal("internal server error").WithCause(err))
		}()

		c.Next()
	}
}

// isBrokenPipe reports whether the panic came from writing to a closed peer.
func isBrokenPipe(recovered any) bool {
	err, ok := recovered.(error)
	if !ok {
		return false
	}

	var netErr *net.OpError
	if !errors.As(err, &netErr) {
		return false
	}

	var sysErr *os.SyscallError
	if !errors.As(netErr.Err, &sysErr) {
		return false
	}

	msg := strings.ToLower(sysErr.Error())
	return strings.Contains(msg, "broken pipe") || strings.Contains(msg, "connection reset by peer")
}
