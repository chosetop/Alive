package contentworld

import (
	"context"
	"fmt"
)

type Store interface {
	ListOpen(ctx context.Context) ([]Setting, error)
	ListAll(ctx context.Context) ([]Setting, error)
	Get(ctx context.Context, key Key) (Setting, error)
	Update(ctx context.Context, key Key, input UpdateInput) (Setting, error)
}

var _ Store = (*Repository)(nil)

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListOpen(ctx context.Context) ([]Setting, error) {
	settings, err := s.store.ListOpen(ctx)
	if err != nil {
		return nil, err
	}
	sortSettings(settings)
	return settings, nil
}

func (s *Service) ListAdmin(ctx context.Context) ([]Setting, error) {
	settings, err := s.store.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	sortSettings(settings)
	return settings, nil
}

func (s *Service) Update(ctx context.Context, key Key, input UpdateInput) (Setting, error) {
	if _, ok := Lookup(key); !ok {
		return Setting{}, fmt.Errorf("%w: %s", ErrWorldNotFound, key)
	}
	if input.Status != nil && !validStatus(*input.Status) {
		return Setting{}, fmt.Errorf("%w: %s", ErrInvalidStatus, *input.Status)
	}
	if input.NavLabel != nil && !validNavLabel(*input.NavLabel) {
		return Setting{}, fmt.Errorf("%w: %s", ErrInvalidNavLabel, *input.NavLabel)
	}
	if input.DefaultView != nil && !validViewForWorld(key, *input.DefaultView) {
		return Setting{}, fmt.Errorf("%w: %s", ErrInvalidViewMode, *input.DefaultView)
	}
	return s.store.Update(ctx, key, input)
}

func (s *Service) AllowsPublicPublish(ctx context.Context, key Key) (bool, error) {
	if _, ok := Lookup(key); !ok {
		return false, fmt.Errorf("%w: %s", ErrWorldNotFound, key)
	}

	setting, err := s.store.Get(ctx, key)
	if err != nil {
		return false, err
	}

	return setting.Status == Open || setting.Status == Hidden, nil
}
