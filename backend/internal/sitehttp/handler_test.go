package sitehttp

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/site"
)

type fakeStore struct {
	settings site.SiteSettings
	err      error
}

func (s *fakeStore) Get(context.Context) (site.SiteSettings, error) {
	return s.settings, nil
}

func (s *fakeStore) UpdateTheme(_ context.Context, theme string, expectedRevision int64) (site.SiteSettings, error) {
	if s.err != nil {
		return site.SiteSettings{}, s.err
	}
	if expectedRevision != s.settings.Revision {
		return site.SiteSettings{}, site.ErrVersionConflict
	}
	s.settings.DefaultTheme = theme
	s.settings.Revision++
	s.settings.UpdatedAt = s.settings.UpdatedAt.Add(time.Second)
	return s.settings, nil
}

func newTestEngine(store *fakeStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	handler := NewHandler(site.NewService(store), slog.New(slog.NewTextHandler(io.Discard, nil)))
	api := engine.Group("/api/v1")
	handler.Register(api, func(c *gin.Context) { c.Next() })
	return engine
}

func TestGetSiteSettingsIsPublicAndReturnsRevision(t *testing.T) {
	engine := newTestEngine(&fakeStore{settings: site.SiteSettings{DefaultTheme: "lamp", Revision: 4, UpdatedAt: time.Unix(42, 0)}})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/site", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"default_theme":"lamp"`) || !strings.Contains(body, `"revision":4`) {
		t.Fatalf("body = %s, want theme and revision", body)
	}
}

func TestPatchSiteSettingsUpdatesTheme(t *testing.T) {
	engine := newTestEngine(&fakeStore{settings: site.SiteSettings{DefaultTheme: "ink", Revision: 1}})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/site", strings.NewReader(`{"default_theme":"codex-lavender","revision":1}`))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"default_theme":"codex-lavender"`) || !strings.Contains(body, `"revision":2`) {
		t.Fatalf("body = %s, want updated theme and revision", body)
	}
}

func TestPatchSiteSettingsMapsDomainErrorsToFields(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		err    error
		field  string
		status int
	}{
		{name: "invalid theme", body: `{"default_theme":"bogus","revision":1}`, field: "default_theme", status: http.StatusBadRequest},
		{name: "stale revision", body: `{"default_theme":"lamp","revision":1}`, err: site.ErrVersionConflict, field: "revision", status: http.StatusConflict},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newTestEngine(&fakeStore{settings: site.SiteSettings{DefaultTheme: "ink", Revision: 2}, err: tc.err})
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/site", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(rec, req)

			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.status, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), `"`+tc.field+`"`) {
				t.Errorf("body = %s, want field %q", rec.Body.String(), tc.field)
			}
		})
	}
}
