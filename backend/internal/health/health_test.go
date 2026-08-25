package health_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/health"
)

// discardLogger keeps test output readable.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestLiveDoesNotTouchDatabase passes a nil pool on purpose.
//
// If the liveness handler ever starts reading the database it will panic here,
// which is the point: liveness must stay independent of every dependency.
func TestLiveDoesNotTouchDatabase(t *testing.T) {
	gin.SetMode(gin.TestMode)

	engine := gin.New()
	health.NewHandler(nil, discardLogger()).Register(engine)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	// Assert the exact documented shape: {"data":{"status":"ok"}}
	var body struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, rec.Body.String())
	}
	if body.Data.Status != "ok" {
		t.Errorf(`data.status = %q, want "ok"`, body.Data.Status)
	}

	// A success envelope must not carry an error key.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	if _, exists := raw["error"]; exists {
		t.Error(`success response contains an "error" key`)
	}
	if _, exists := raw["data"]; !exists {
		t.Error(`success response is missing the "data" key`)
	}
}
