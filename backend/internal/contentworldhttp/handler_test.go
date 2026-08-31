package contentworldhttp

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/contentworld/contentworldtest"
)

func newTestEngine(store *contentworldtest.Store) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(contentworld.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil)))
	api := engine.Group("/api/v1")
	handler.Register(api, func(c *gin.Context) { c.Next() })
	return engine
}

func TestGetWorldsIsPublicAndOmitsUnopenedAndHiddenWorlds(t *testing.T) {
	store := contentworldtest.NewStore()
	store.Settings[contentworld.Saying] = contentworld.Setting{
		World:       contentworld.Saying,
		Status:      contentworld.Hidden,
		NavLabel:    "片语",
		SortOrder:   20,
		DefaultView: contentworld.ViewStream,
		Revision:    1,
		UpdatedAt:   store.Settings[contentworld.Saying].UpdatedAt,
	}
	engine := newTestEngine(store)

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/worlds", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"world":"journal"`) {
		t.Fatalf("body = %s, want journal world", body)
	}
	if strings.Contains(body, `"world":"saying"`) || strings.Contains(body, `"world":"video"`) {
		t.Fatalf("body = %s, want hidden and unopened worlds omitted", body)
	}
	if strings.Contains(body, `"status"`) || strings.Contains(body, `"revision"`) {
		t.Fatalf("body = %s, public response must omit admin fields", body)
	}
}

func TestGetAdminWorldsReturnsLifecycleFields(t *testing.T) {
	engine := newTestEngine(contentworldtest.NewStore())

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/worlds", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"open"`) || !strings.Contains(body, `"revision":1`) || !strings.Contains(body, `"updated_at"`) {
		t.Fatalf("body = %s, want lifecycle fields", body)
	}
}

func TestPatchAdminWorldUpdatesLifecycleSetting(t *testing.T) {
	engine := newTestEngine(contentworldtest.NewStore())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/worlds/saying", strings.NewReader(`{"status":"open","nav_label":"片语","default_view":"wall","revision":1}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"open"`) || !strings.Contains(body, `"default_view":"wall"`) || !strings.Contains(body, `"revision":2`) {
		t.Fatalf("body = %s, want updated lifecycle world", body)
	}
}

func TestPatchAdminWorldMapsLifecycleErrors(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		body   string
		field  string
		status int
	}{
		{name: "invalid status", path: "/api/v1/admin/worlds/saying", body: `{"status":"published","revision":1}`, field: "status", status: http.StatusBadRequest},
		{name: "invalid label", path: "/api/v1/admin/worlds/saying", body: `{"nav_label":"","revision":1}`, field: "nav_label", status: http.StatusBadRequest},
		{name: "invalid view", path: "/api/v1/admin/worlds/journal", body: `{"default_view":"wall","revision":1}`, field: "default_view", status: http.StatusBadRequest},
		{name: "stale revision", path: "/api/v1/admin/worlds/saying", body: `{"status":"open","revision":99}`, field: "revision", status: http.StatusConflict},
		{name: "unknown world", path: "/api/v1/admin/worlds/book", body: `{"status":"open","revision":1}`, field: "", status: http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngine(contentworldtest.NewStore())
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if tc.field != "" && !strings.Contains(rec.Body.String(), `"`+tc.field+`"`) {
				t.Fatalf("body = %s, want field %q", rec.Body.String(), tc.field)
			}
		})
	}
}
