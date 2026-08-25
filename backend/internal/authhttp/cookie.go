// Package authhttp adapts the auth domain to HTTP.
//
// It is the only place that knows about cookies, headers, status codes and Gin.
// internal/auth holds the rules and imports none of that, which is why the CLI
// can create an account without linking a web framework. The dependency runs one
// way: this package imports auth, never the reverse.
package authhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/config"
)

// CookieManager writes and reads the session cookie.
//
// Every attribute of that cookie is decided here. Spreading them across the
// handlers is how a logout ends up clearing a cookie whose Path does not match
// the one login set, leaving the browser holding a credential nothing can remove.
type CookieManager struct {
	cfg config.SessionConfig
}

// NewCookieManager returns a manager over the session configuration.
func NewCookieManager(cfg config.SessionConfig) *CookieManager {
	return &CookieManager{cfg: cfg}
}

// Name reports the cookie's name.
func (m *CookieManager) Name() string { return m.cfg.CookieName }

// Set writes the session cookie carrying token.
//
// Max-Age comes from the configured lifetime rather than from the session's own
// expiry, because the two must agree: the browser should stop sending the token
// at about the moment the server stops accepting it. A sliding session renewed on
// a later request gets a fresh cookie then.
func (m *CookieManager) Set(c *gin.Context, token auth.Token) {
	http.SetCookie(c.Writer, m.cookie(string(token), int(m.cfg.Lifetime.Seconds())))
}

// Clear expires the session cookie.
//
// MaxAge below zero is how net/http emits "Max-Age=0", which tells the browser to
// drop the cookie now. The value is emptied as well, so a client that ignores the
// instruction is left with nothing usable.
//
// Every other attribute matches Set. A browser matches a cookie for replacement
// by name, domain and path; a Clear with a different Path adds a second cookie
// instead of removing the first.
func (m *CookieManager) Clear(c *gin.Context) {
	http.SetCookie(c.Writer, m.cookie("", -1))
}

// Read returns the token from the request's session cookie.
//
// The second result is false when the cookie is absent or empty. Whether the
// token is well-formed, known or current is not decided here.
func (m *CookieManager) Read(c *gin.Context) (auth.Token, bool) {
	value, err := c.Cookie(m.cfg.CookieName)
	if err != nil || value == "" {
		return "", false
	}
	return auth.Token(value), true
}

// cookie builds the session cookie. One constructor, so Set and Clear cannot
// disagree about the attributes.
func (m *CookieManager) cookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:  m.cfg.CookieName,
		Value: value,

		// Path narrows where the credential is sent. /api/v1 covers the API and
		// keeps the cookie off requests for the site's pages and assets.
		Path: m.cfg.CookiePath,

		// Domain is deliberately unset. An empty Domain produces a host-only
		// cookie, sent to exactly the host that issued it. Naming a domain would
		// widen it to every subdomain, including any that is not this API.
		Domain: "",

		MaxAge: maxAge,

		// HttpOnly keeps the token out of document.cookie, so script injected
		// into the admin page cannot read it. This is the attribute that makes a
		// session cookie a better credential store than localStorage, which is
		// readable by any script on the page by design.
		HttpOnly: true,

		// Secure restricts the cookie to HTTPS. Off in development only, because
		// http://localhost would otherwise never receive it.
		Secure: m.cfg.CookieSecure,

		// Strict means the cookie is absent from any cross-site request, so a
		// form or fetch on another origin cannot act as the logged-in owner. This
		// closes most of the CSRF surface without a token exchange, and it is
		// affordable only because admin is served from the same origin as the
		// API. A subdomain split would force Lax and bring the tokens back.
		SameSite: http.SameSiteStrictMode,
	}
}
