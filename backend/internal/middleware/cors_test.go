package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/middleware"
)

func corsConfig() config.CORSConfig {
	return config.CORSConfig{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
}

func newCORSEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.CORS(corsConfig()))
	engine.GET("/probe", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return engine
}

func TestCORS(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		origin          string
		wantStatus      int
		wantAllowOrigin string
	}{
		{
			name:            "allowed origin is echoed back",
			method:          http.MethodGet,
			origin:          "http://localhost:3000",
			wantStatus:      http.StatusOK,
			wantAllowOrigin: "http://localhost:3000",
		},
		{
			// The request still runs; the browser blocks the response because
			// the allow-origin header is absent.
			name:            "unknown origin gets no cors header",
			method:          http.MethodGet,
			origin:          "http://evil.example.com",
			wantStatus:      http.StatusOK,
			wantAllowOrigin: "",
		},
		{
			name:            "preflight from allowed origin is answered 204",
			method:          http.MethodOptions,
			origin:          "http://localhost:3000",
			wantStatus:      http.StatusNoContent,
			wantAllowOrigin: "http://localhost:3000",
		},
		{
			// Rejected loudly so the failure shows up in logs, not only in a
			// browser console.
			name:            "preflight from unknown origin is rejected",
			method:          http.MethodOptions,
			origin:          "http://evil.example.com",
			wantStatus:      http.StatusForbidden,
			wantAllowOrigin: "",
		},
		{
			name:            "request without origin is untouched",
			method:          http.MethodGet,
			origin:          "",
			wantStatus:      http.StatusOK,
			wantAllowOrigin: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newCORSEngine()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/probe", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			engine.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tc.wantAllowOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, tc.wantAllowOrigin)
			}
		})
	}
}

// TestCORSSetsVaryOnOrigin guards a caching bug: without Vary, a shared cache
// can serve a response containing one origin's allow header to another origin.
func TestCORSSetsVaryOnOrigin(t *testing.T) {
	engine := newCORSEngine()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/probe", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q", got, "Origin")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, "true")
	}
}
