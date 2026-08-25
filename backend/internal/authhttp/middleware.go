package authhttp

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// contextKey is unexported so no other package can write this key.
//
// A plain string would let any package overwrite the authenticated identity by
// setting the same value in the Gin context. An unexported type makes that
// impossible outside this file.
type contextKey struct{ name string }

var authenticatedKey = contextKey{name: "authhttp.authenticated"}

// RequireAuth rejects a request that does not carry a valid session.
//
// It is HTTP adaptation and nothing else: read the cookie, ask the service, and
// turn the answer into a status. No rule about what makes a session valid lives
// here, which is why the same decision backs the CLI without a request.
func (h *Handler) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := h.cookies.Read(c)
		if !ok {
			// No cookie at all. Nothing to clear, so the response only says 401.
			httpx.Error(c, apperr.Unauthorized("authentication required"))
			return
		}

		authenticated, err := h.service.Authenticate(c.Request.Context(), token)
		if err != nil {
			h.rejectSession(c, err)
			return
		}

		// A renewed session gets a fresh cookie, so the browser's Max-Age tracks the
		// row. Without this the session would slide server-side while the cookie
		// expired on its original schedule, logging the owner out mid-use.
		//
		// Only on renewal. Rewriting the cookie on every request would add a
		// Set-Cookie header to every response for nothing.
		if authenticated.Renewed {
			h.cookies.Set(c, token)
		}

		c.Set(authenticatedKey.name, authenticated)
		c.Next()
	}
}

// rejectSession answers a failed authentication.
//
// Every reason produces the same 401 and the same message. The reasons are worth
// distinguishing in the log, never in the response: telling a client that a token
// was "expired" rather than "unknown" confirms it was once issued.
func (h *Handler) rejectSession(c *gin.Context, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidToken),
		errors.Is(err, auth.ErrSessionNotFound),
		errors.Is(err, auth.ErrSessionExpired):

		// The browser is holding a credential that will never work again. Clearing
		// it stops every later request from carrying it and costing a lookup.
		h.cookies.Clear(c)

		h.logger.DebugContext(c.Request.Context(), "session rejected",
			slog.String("reason", err.Error()),
		)
		httpx.Error(c, apperr.Unauthorized("authentication required"))

	default:
		// An infrastructure failure. Answering 401 here would tell the owner their
		// session ended when the real problem is that the database is unreachable,
		// and the cookie must survive so the session still works once it recovers.
		h.logger.ErrorContext(c.Request.Context(), "authentication failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
	}
}

// Authenticated returns the identity RequireAuth stored on the context.
//
// The second result is false when the request did not pass through RequireAuth.
// Handlers behind it can ignore that case; the type assertion is what keeps this
// honest if the middleware is ever left off a route.
func Authenticated(c *gin.Context) (auth.Authenticated, bool) {
	value, exists := c.Get(authenticatedKey.name)
	if !exists {
		return auth.Authenticated{}, false
	}
	authenticated, ok := value.(auth.Authenticated)
	return authenticated, ok
}
