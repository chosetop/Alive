package middleware_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/middleware"
)

// TestRecoveryReturnsJSONAndHidesCause is the reason this package does not use
// gin.Recovery: the panic value must not reach the client, and the response
// must be JSON like every other error in the API.
func TestRecoveryReturnsJSONAndHidesCause(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const secret = "connection to db-primary.internal failed for user alive"

	var logBuf strings.Builder
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.Recovery(logger))
	engine.GET("/boom", func(c *gin.Context) { panic(secret) })

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/boom", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want JSON", ct)
	}

	body := rec.Body.String()
	if strings.Contains(body, secret) {
		t.Errorf("panic detail leaked into response body: %s", body)
	}

	var parsed struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, body)
	}
	if parsed.Error.Code != "INTERNAL" {
		t.Errorf("error.code = %q, want %q", parsed.Error.Code, "INTERNAL")
	}
	if parsed.Error.RequestID == "" {
		t.Error("error.request_id is empty; a 500 must be traceable to a log line")
	}

	// The detail belongs in the log, so the operator can still diagnose it.
	if !strings.Contains(logBuf.String(), secret) {
		t.Error("panic detail is missing from the log")
	}
}

func TestRequestIDIsGeneratedAndReturned(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	engine.Use(middleware.RequestID())
	engine.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })

	t.Run("generated when absent", func(t *testing.T) {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/probe", nil))

		if got := rec.Header().Get("X-Request-ID"); len(got) != 32 {
			t.Errorf("X-Request-ID = %q, want 32 hex chars", got)
		}
	})

	t.Run("incoming value is reused for correlation", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		req.Header.Set("X-Request-ID", "trace-abc-123")
		engine.ServeHTTP(rec, req)

		if got := rec.Header().Get("X-Request-ID"); got != "trace-abc-123" {
			t.Errorf("X-Request-ID = %q, want %q", got, "trace-abc-123")
		}
	})

	t.Run("control characters are replaced", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/probe", nil)
		// A newline in a header would let a client forge extra log lines.
		req.Header["X-Request-Id"] = []string{"bad\nvalue"}
		engine.ServeHTTP(rec, req)

		if got := rec.Header().Get("X-Request-ID"); got == "bad\nvalue" {
			t.Error("unsafe request id was echoed back unchanged")
		}
	})
}
