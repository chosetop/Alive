package taxonomy_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

type fakeTagStore struct {
	tags map[int64]taxonomy.Tag
	next int64

	failCreate     error
	failGetByID    error
	failGetBySlug  error
	failList       error
	failUpdate     error
	failDelete     error
	failSlugExists error
	failNameExists error

	slugExists map[string]bool
	nameExists map[string]bool

	lastCreate    taxonomy.CreateTagParams
	lastUpdate    taxonomy.UpdateTagParams
	lastListQuery string
	lastListLimit int
	deleteCalls   int
}

func newFakeTagStore() *fakeTagStore {
	return &fakeTagStore{
		tags:       make(map[int64]taxonomy.Tag),
		next:       1,
		slugExists: make(map[string]bool),
		nameExists: make(map[string]bool),
	}
}

func (s *fakeTagStore) Create(_ context.Context, params taxonomy.CreateTagParams) (taxonomy.Tag, error) {
	s.lastCreate = params
	if s.failCreate != nil {
		return taxonomy.Tag{}, s.failCreate
	}
	for _, tag := range s.tags {
		if strings.EqualFold(tag.Name, params.Name) {
			return taxonomy.Tag{}, taxonomy.ErrTagNameTaken
		}
		if tag.Slug == params.Slug {
			return taxonomy.Tag{}, taxonomy.ErrTagSlugTaken
		}
	}
	tag := taxonomy.Tag{ID: s.next, Name: params.Name, Slug: params.Slug}
	s.tags[s.next] = tag
	s.next++
	return tag, nil
}

func (s *fakeTagStore) GetByID(_ context.Context, id int64) (taxonomy.Tag, error) {
	if s.failGetByID != nil {
		return taxonomy.Tag{}, s.failGetByID
	}
	tag, ok := s.tags[id]
	if !ok {
		return taxonomy.Tag{}, taxonomy.ErrTagNotFound
	}
	return tag, nil
}

func (s *fakeTagStore) GetBySlug(_ context.Context, slug string) (taxonomy.Tag, error) {
	if s.failGetBySlug != nil {
		return taxonomy.Tag{}, s.failGetBySlug
	}
	for _, tag := range s.tags {
		if tag.Slug == slug {
			return tag, nil
		}
	}
	return taxonomy.Tag{}, taxonomy.ErrTagNotFound
}

func (s *fakeTagStore) List(_ context.Context, query string, limit int) ([]taxonomy.Tag, error) {
	if s.failList != nil {
		return nil, s.failList
	}
	s.lastListQuery = query
	s.lastListLimit = limit
	return nil, nil
}

func (s *fakeTagStore) Update(_ context.Context, params taxonomy.UpdateTagParams) (taxonomy.Tag, error) {
	s.lastUpdate = params
	if s.failUpdate != nil {
		return taxonomy.Tag{}, s.failUpdate
	}
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

func (s *fakeTagStore) Delete(_ context.Context, id int64) (bool, error) {
	s.deleteCalls++
	if s.failDelete != nil {
		return false, s.failDelete
	}
	if _, ok := s.tags[id]; !ok {
		return false, nil
	}
	delete(s.tags, id)
	return true, nil
}

func (s *fakeTagStore) SlugExists(_ context.Context, slug string) (bool, error) {
	if s.failSlugExists != nil {
		return false, s.failSlugExists
	}
	if s.slugExists[slug] {
		return true, nil
	}
	for _, tag := range s.tags {
		if tag.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

func (s *fakeTagStore) NameExists(_ context.Context, name string) (bool, error) {
	if s.failNameExists != nil {
		return false, s.failNameExists
	}
	if s.nameExists[strings.ToLower(name)] {
		return true, nil
	}
	for _, tag := range s.tags {
		if strings.EqualFold(tag.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func newTagService() (*taxonomy.TagService, *fakeTagStore) {
	store := newFakeTagStore()
	return taxonomy.NewTagService(store, taxonomy.WithTagLogger(slog.New(slog.NewTextHandler(io.Discard, nil)))), store
}

func TestCreateTrimsNameAndGeneratesSlug(t *testing.T) {
	service, store := newTagService()

	created, err := service.Create(context.Background(), taxonomy.CreateTagInput{
		Name: "  Mixed Case  ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Name != "Mixed Case" {
		t.Fatalf("name = %q, want Mixed Case", created.Name)
	}
	if created.Slug != "mixed-case" {
		t.Fatalf("slug = %q, want mixed-case", created.Slug)
	}
	if store.lastCreate.Slug != "mixed-case" {
		t.Fatalf("stored slug = %q, want mixed-case", store.lastCreate.Slug)
	}
}

func TestCreateUsesAnExplicitSlug(t *testing.T) {
	service, _ := newTagService()

	created, err := service.Create(context.Background(), taxonomy.CreateTagInput{
		Name: "素材",
		Slug: "  Video Clips  ",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Slug != "video-clips" {
		t.Fatalf("slug = %q, want video-clips", created.Slug)
	}
}

func TestCreateRejectsAnEmptySlugResult(t *testing.T) {
	service, store := newTagService()

	_, err := service.Create(context.Background(), taxonomy.CreateTagInput{Name: "   "})
	if !errors.Is(err, taxonomy.ErrInvalidTagName) && !errors.Is(err, taxonomy.ErrInvalidTagSlug) {
		t.Fatalf("Create = %v, want invalid name or slug", err)
	}
	if store.lastCreate.Name != "" || store.lastCreate.Slug != "" {
		t.Fatalf("store was called: %+v", store.lastCreate)
	}
}

func TestCreateRejectsCaseInsensitiveDuplicateName(t *testing.T) {
	service, store := newTagService()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel"}

	_, err := service.Create(context.Background(), taxonomy.CreateTagInput{
		Name: "travel",
		Slug: "journey",
	})
	if !errors.Is(err, taxonomy.ErrTagNameTaken) {
		t.Fatalf("Create = %v, want ErrTagNameTaken", err)
	}
}

func TestCreateRejectsDuplicateSlug(t *testing.T) {
	service, store := newTagService()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel"}

	_, err := service.Create(context.Background(), taxonomy.CreateTagInput{
		Name: "Journey",
		Slug: "travel",
	})
	if !errors.Is(err, taxonomy.ErrTagSlugTaken) {
		t.Fatalf("Create = %v, want ErrTagSlugTaken", err)
	}
}

func TestUpdateRejectsNoFields(t *testing.T) {
	service, store := newTagService()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel"}

	_, err := service.Update(context.Background(), 1, taxonomy.UpdateTagInput{})
	if !errors.Is(err, taxonomy.ErrNoUpdateFields) {
		t.Fatalf("Update = %v, want ErrNoUpdateFields", err)
	}
}

func TestUpdateRejectsDuplicateSlug(t *testing.T) {
	service, store := newTagService()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel"}
	store.tags[2] = taxonomy.Tag{ID: 2, Name: "Journey", Slug: "journey"}

	_, err := service.Update(context.Background(), 1, taxonomy.UpdateTagInput{
		Slug: ptr("journey"),
	})
	if !errors.Is(err, taxonomy.ErrTagSlugTaken) {
		t.Fatalf("Update = %v, want ErrTagSlugTaken", err)
	}
}

func TestListTrimsQueryAndCapsLimit(t *testing.T) {
	service, store := newTagService()

	if _, err := service.List(context.Background(), "  kyoto  ", 500); err != nil {
		t.Fatalf("List: %v", err)
	}
	if store.lastListQuery != "kyoto" {
		t.Fatalf("query = %q, want kyoto", store.lastListQuery)
	}
	if store.lastListLimit != 50 {
		t.Fatalf("limit = %d, want 50", store.lastListLimit)
	}
}

func TestDeletePropagatesTagInUse(t *testing.T) {
	service, store := newTagService()
	store.tags[1] = taxonomy.Tag{ID: 1, Name: "Travel", Slug: "travel"}
	store.failDelete = taxonomy.ErrTagInUse

	if err := service.Delete(context.Background(), 1); !errors.Is(err, taxonomy.ErrTagInUse) {
		t.Fatalf("Delete = %v, want ErrTagInUse", err)
	}
	if store.deleteCalls != 1 {
		t.Fatalf("delete calls = %d, want 1", store.deleteCalls)
	}
}

func ptr[T any](v T) *T { return &v }
