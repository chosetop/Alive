package taxonomyhttp_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/taxonomy"
	"github.com/p30huiwei/alive/backend/internal/taxonomyhttp"
)

type tagHTTPStore struct {
	tags map[int64]taxonomy.Tag
	next int64

	lastCreate taxonomy.CreateTagParams
	lastUpdate taxonomy.UpdateTagParams
	lastQuery  string
	lastLimit  int
}

func newTagHTTPStore() *tagHTTPStore {
	return &tagHTTPStore{
		tags: make(map[int64]taxonomy.Tag),
		next: 1,
	}
}

func (s *tagHTTPStore) Create(_ context.Context, params taxonomy.CreateTagParams) (taxonomy.Tag, error) {
	s.lastCreate = params
	tag := taxonomy.Tag{ID: s.next, Name: params.Name, Slug: params.Slug}
	s.tags[s.next] = tag
	s.next++
	return tag, nil
}

func (s *tagHTTPStore) GetByID(_ context.Context, id int64) (taxonomy.Tag, error) {
	tag, ok := s.tags[id]
	if !ok {
		return taxonomy.Tag{}, taxonomy.ErrTagNotFound
	}
	return tag, nil
}

func (s *tagHTTPStore) GetBySlug(_ context.Context, slug string) (taxonomy.Tag, error) {
	for _, tag := range s.tags {
		if tag.Slug == slug {
			return tag, nil
		}
	}
	return taxonomy.Tag{}, taxonomy.ErrTagNotFound
}

func (s *tagHTTPStore) List(_ context.Context, query string, limit int) ([]taxonomy.Tag, error) {
	s.lastQuery = query
	s.lastLimit = limit
	out := make([]taxonomy.Tag, 0, len(s.tags))
	for _, tag := range s.tags {
		out = append(out, tag)
	}
	return out, nil
}

func (s *tagHTTPStore) Update(_ context.Context, params taxonomy.UpdateTagParams) (taxonomy.Tag, error) {
	s.lastUpdate = params
	tag, ok := s.tags[params.ID]
	if !ok {
		return taxonomy.Tag{}, taxonomy.ErrTagNotFound
	}
	if params.SetName {
		tag.Name = params.Name
	}
	if params.SetSlug {
		tag.Slug = params.Slug
	}
	s.tags[params.ID] = tag
	return tag, nil
}

func (s *tagHTTPStore) Delete(_ context.Context, id int64) (bool, error) {
	if _, ok := s.tags[id]; !ok {
		return false, nil
	}
	delete(s.tags, id)
	return true, nil
}

func (s *tagHTTPStore) SlugExists(_ context.Context, slug string) (bool, error) {
	for _, tag := range s.tags {
		if tag.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

func (s *tagHTTPStore) NameExists(_ context.Context, name string) (bool, error) {
	for _, tag := range s.tags {
		if strings.EqualFold(tag.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func newTagServer(t *testing.T) (http.Handler, *tagHTTPStore) {
	t.Helper()

	store := newTagHTTPStore()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel", UsageCount: 2}
	service := taxonomy.NewTagService(store, taxonomy.WithTagLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	handler := taxonomyhttp.NewTagHandler(service)

	engine := gin.New()
	api := engine.Group("/api/v1")
	pass := func(c *gin.Context) { c.Next() }
	handler.RegisterAdmin(api, pass)

	return engine, store
}

func TestTagAdminRoutesSupportCRUD(t *testing.T) {
	handler, store := newTagServer(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/admin/tags", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	items := dataArray(t, rec)
	if len(items) != 1 {
		t.Fatalf("list count = %d, want 1", len(items))
	}
	if got := stringField(t, items[0], "slug"); got != "travel" {
		t.Fatalf("list slug = %q, want travel", got)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", strings.NewReader(`{"name":"京都","slug":"kyoto"}`)))
	rec.Code = rec.Code
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.lastCreate.Name != "京都" || store.lastCreate.Slug != "kyoto" {
		t.Fatalf("create payload = %+v", store.lastCreate)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/v1/admin/tags/1", strings.NewReader(`{"name":"Trips","slug":"trips"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	updated := dataObject(t, rec)
	if got := stringField(t, updated, "name"); got != "Trips" {
		t.Fatalf("updated name = %q, want Trips", got)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tags/1", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204\nbody: %s", rec.Code, rec.Body.String())
	}
	if _, ok := store.tags[1]; ok {
		t.Fatal("tag still exists after delete")
	}
}

func TestTagCreateRejectsDuplicateName(t *testing.T) {
	handler, _ := newTagServer(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/admin/tags", strings.NewReader(`{"name":"travel","slug":"journey"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code == "" {
		t.Fatal("error response missing code")
	}
}
