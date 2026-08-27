package site

import (
	"context"
	"fmt"
)

type Store interface {
	Get(ctx context.Context) (SiteSettings, error)
	UpdateTheme(ctx context.Context, theme string, expectedRevision int64) (SiteSettings, error)
}

var _ Store = (*Repository)(nil)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Get(ctx context.Context) (SiteSettings, error) {
	return s.store.Get(ctx)
}

func (s *Service) UpdateTheme(ctx context.Context, theme string, expectedRevision int64) (SiteSettings, error) {
	if !validTheme(theme) {
		return SiteSettings{}, fmt.Errorf("%w: %s", ErrInvalidTheme, theme)
	}
	return s.store.UpdateTheme(ctx, theme, expectedRevision)
}
