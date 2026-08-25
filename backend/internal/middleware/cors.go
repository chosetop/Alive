// Package middleware holds cross-cutting HTTP concerns.
//
// Nothing here contains business logic. Middleware may read and annotate a
// request, reject it, or shape a response; it must not know what an Entry is.
package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/config"
)

// CORS answers preflight requests and adds the response headers a browser needs
// for cross-origin calls.
//
// Written by hand rather than pulled from gin-contrib/cors: the policy is a
// fixed allow-list check, and a dependency here would be more code to audit
// than to write. The origin is echoed only when it matches the allow-list
// exactly, because the API allows credentials and "*" is invalid in that case.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, origin := range cfg.AllowedOrigins {
		allowed[origin] = struct{}{}
	}

	allowMethods := strings.Join(cfg.AllowedMethods, ", ")
	allowHeaders := strings.Join(cfg.AllowedHeaders, ", ")
	exposeHeaders := strings.Join(cfg.ExposedHeaders, ", ")
	maxAge := strconv.Itoa(int(cfg.MaxAge.Seconds()))

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		// Same-origin and server-to-server calls carry no Origin. Nothing to do.
		if origin == "" {
			c.Next()
			return
		}

		if _, ok := allowed[origin]; !ok {
			// Send no CORS headers: the browser then blocks the response. A
			// preflight from an unknown origin is answered 403 so the failure is
			// visible in logs instead of only in the browser console.
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		header := c.Writer.Header()
		header.Set("Access-Control-Allow-Origin", origin)
		// Vary tells caches the response body depends on the request Origin.
		header.Add("Vary", "Origin")

		if cfg.AllowCredentials {
			header.Set("Access-Control-Allow-Credentials", "true")
		}
		if exposeHeaders != "" {
			header.Set("Access-Control-Expose-Headers", exposeHeaders)
		}

		if c.Request.Method == http.MethodOptions {
			header.Set("Access-Control-Allow-Methods", allowMethods)
			header.Set("Access-Control-Allow-Headers", allowHeaders)
			header.Set("Access-Control-Max-Age", maxAge)
			// 204 with no body: the preflight answer is entirely in the headers.
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
