package authhttp

import (
	"errors"
	"log/slog"
	"net/netip"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// Handler serves the auth endpoints.
//
// It parses requests, sets cookies and maps domain errors onto statuses. Every
// decision about whether a login succeeds belongs to auth.Service; this type
// only translates.
type Handler struct {
	service *auth.Service
	cookies *CookieManager
	logger  *slog.Logger
}

// NewHandler wires a handler to the service and the session configuration.
func NewHandler(service *auth.Service, sessionCfg config.SessionConfig, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		service: service,
		cookies: NewCookieManager(sessionCfg),
		logger:  logger,
	}
}

// Register adds the auth routes to the API group.
//
// login and logout are public; logout has to be, because a request whose session
// already ended must still be able to clear its cookie. Requiring auth there
// would leave a client with a dead cookie and no way to drop it.
func (h *Handler) Register(api *gin.RouterGroup, loginMiddleware ...gin.HandlerFunc) {
	group := api.Group("/auth")
	group.POST("/login", append(loginMiddleware, h.Login)...)
	group.POST("/logout", h.Logout)

	api.GET("/me", h.RequireAuth(), h.Me)
}

// Login verifies a password, opens a session and sets the cookie.
//
// The token is never in the body. It travels in a HttpOnly cookie, so script on
// the page cannot read it; returning it in JSON would hand it to anything that
// can parse the response.
func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 400, not 401. A body missing a field is a malformed request, and saying
		// so is not a hint about credentials: the client never got as far as
		// presenting any.
		httpx.Error(c, apperr.InvalidInput("username and password are required").WithCause(err))
		return
	}

	result, err := h.service.Login(c.Request.Context(), auth.LoginParams{
		Username:  req.Username,
		Password:  req.Password,
		UserAgent: c.Request.UserAgent(),
		IP:        clientIP(c),
	})
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			// One response for a wrong password and for an account that does not
			// exist. The service already merged them; keeping them merged here is
			// what stops the endpoint from confirming which usernames are real.
			h.logger.WarnContext(c.Request.Context(), "login rejected",
				slog.String("username", req.Username),
				slog.String("ip", c.ClientIP()),
			)
			httpx.Error(c, apperr.InvalidCredentials("invalid username or password"))
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "login failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	h.cookies.Set(c, result.Token)

	h.logger.InfoContext(c.Request.Context(), "login succeeded",
		slog.Int64("user_id", result.User.ID),
		slog.Int64("session_id", result.Session.ID),
	)

	httpx.OK(c, newUserResponse(result.User))
}

// Logout ends the session and clears the cookie.
//
// Idempotent, and public on purpose. A client's goal is to end up logged out; if
// it already is, that goal is met and an error would invite a retry of nothing.
// The cookie is cleared whatever the state of the session.
func (h *Handler) Logout(c *gin.Context) {
	token, ok := h.cookies.Read(c)
	if !ok {
		// No cookie. Already in the desired state.
		httpx.NoContent(c)
		return
	}

	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		// The row may still be there, so the session could still work. Leaving the
		// cookie in place keeps the client's view and the server's in agreement
		// rather than showing a logged-out UI backed by a live session.
		h.logger.ErrorContext(c.Request.Context(), "logout failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	h.cookies.Clear(c)
	httpx.NoContent(c)
}

// Me returns the account behind the current session.
//
// Reached only through RequireAuth, so the identity is already on the context.
func (h *Handler) Me(c *gin.Context) {
	authenticated, ok := Authenticated(c)
	if !ok {
		// Unreachable while the route keeps its middleware. Answering 500 rather
		// than an empty user is deliberate: a missing identity here is a wiring
		// mistake, and returning a blank body would hide it.
		h.logger.ErrorContext(c.Request.Context(), "authenticated identity missing from context")
		httpx.Error(c, apperr.Internal("authentication context missing"))
		return
	}

	httpx.OK(c, newUserResponse(authenticated.User))
}

// clientIP returns the caller's address for the session record.
//
// Recorded for review only, never compared. A failure to parse yields the zero
// Addr, which the repository stores as NULL: an unparseable address is worth
// nothing, and refusing the login over it would be absurd.
//
// gin.ClientIP honours proxy headers only for trusted proxies, and the router
// currently trusts none, so this is the socket's own address.
func clientIP(c *gin.Context) netip.Addr {
	addr, err := netip.ParseAddr(c.ClientIP())
	if err != nil {
		return netip.Addr{}
	}
	return addr
}
