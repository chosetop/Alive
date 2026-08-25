package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// RequestID assigns an id to every request and puts it on the request context
// and the response header.
//
// It must run before Logger and Recovery so their output can be correlated. An
// incoming X-Request-ID is trusted only for correlation, never for any decision.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(httpx.HeaderRequestID)
		if !isSafeRequestID(id) {
			id = newRequestID()
		}

		// Store on the request context, not only on the gin context, so code
		// below the HTTP layer can read it from a plain context.Context.
		c.Request = c.Request.WithContext(
			httpx.ContextWithRequestID(c.Request.Context(), id),
		)
		c.Header(httpx.HeaderRequestID, id)

		c.Next()
	}
}

// newRequestID returns 16 random bytes in hex.
func newRequestID() string {
	var b [16]byte
	// rand.Read from crypto/rand never returns an error as of Go 1.24.
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// isSafeRequestID accepts a bounded, printable-ASCII id. A client-supplied
// value ends up in logs and a response header, so control characters and
// unbounded length are rejected rather than sanitised.
func isSafeRequestID(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if c := id[i]; c < 0x20 || c > 0x7e {
			return false
		}
	}
	return true
}
