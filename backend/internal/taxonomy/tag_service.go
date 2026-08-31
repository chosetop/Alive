package taxonomy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

type TagService struct {
	store TagStore
	log   *slog.Logger
}

type TagServiceOption func(*TagService)

func WithTagLogger(l *slog.Logger) TagServiceOption {
	return func(s *TagService) {
		if l != nil {
			s.log = l
		}
	}
}

func NewTagService(store TagStore, opts ...TagServiceOption) *TagService {
	s := &TagService{store: store}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *TagService) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}

func (s *TagService) Create(ctx context.Context, in CreateTagInput) (Tag, error) {
	name := NormalizeTagName(in.Name)
	if err := ValidateTagName(name); err != nil {
		return Tag{}, err
	}

	slug := NormalizeTagSlug(in.Slug)
	if slug == "" {
		slug = NormalizeTagSlug(name)
	}
	if err := ValidateTagSlug(slug); err != nil {
		return Tag{}, err
	}

	exists, err := s.store.NameExists(ctx, name)
	if err != nil {
		return Tag{}, err
	}
	if exists {
		return Tag{}, fmt.Errorf("%w: %s", ErrTagNameTaken, name)
	}

	exists, err = s.store.SlugExists(ctx, slug)
	if err != nil {
		return Tag{}, err
	}
	if exists {
		return Tag{}, fmt.Errorf("%w: %s", ErrTagSlugTaken, slug)
	}

	created, err := s.store.Create(ctx, CreateTagParams{Name: name, Slug: slug})
	if err != nil {
		if errors.Is(err, ErrTagNameTaken) || errors.Is(err, ErrTagSlugTaken) {
			return Tag{}, err
		}
		return Tag{}, err
	}
	return created, nil
}

func (s *TagService) Update(ctx context.Context, id int64, in UpdateTagInput) (Tag, error) {
	if id <= 0 {
		return Tag{}, fmt.Errorf("%w: id %d", ErrTagNotFound, id)
	}
	if in.Name == nil && in.Slug == nil {
		return Tag{}, ErrNoUpdateFields
	}

	current, err := s.store.GetByID(ctx, id)
	if err != nil {
		return Tag{}, err
	}

	params := UpdateTagParams{ID: id}

	if in.Name != nil {
		name := NormalizeTagName(*in.Name)
		if err := ValidateTagName(name); err != nil {
			return Tag{}, err
		}
		if !strings.EqualFold(current.Name, name) {
			exists, err := s.store.NameExists(ctx, name)
			if err != nil {
				return Tag{}, err
			}
			if exists {
				return Tag{}, fmt.Errorf("%w: %s", ErrTagNameTaken, name)
			}
			params.SetName = true
			params.Name = name
		}
	}

	if in.Slug != nil {
		slug := NormalizeTagSlug(*in.Slug)
		if err := ValidateTagSlug(slug); err != nil {
			return Tag{}, err
		}
		if !strings.EqualFold(current.Slug, slug) {
			exists, err := s.store.SlugExists(ctx, slug)
			if err != nil {
				return Tag{}, err
			}
			if exists {
				return Tag{}, fmt.Errorf("%w: %s", ErrTagSlugTaken, slug)
			}
			params.SetSlug = true
			params.Slug = slug
		}
	}

	if !params.SetName && !params.SetSlug {
		return Tag{}, ErrNoUpdateFields
	}

	return s.store.Update(ctx, params)
}

func (s *TagService) Delete(ctx context.Context, id int64) error {
	deleted, err := s.store.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return ErrTagNotFound
	}
	return nil
}

func (s *TagService) List(ctx context.Context, query string, limit int) ([]Tag, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 50 {
		limit = 50
	}
	return s.store.List(ctx, strings.TrimSpace(query), limit)
}
