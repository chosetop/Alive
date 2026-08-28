package site

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	settings SiteSettings
}

func (s *fakeStore) Get(context.Context) (SiteSettings, error) {
	return s.settings, nil
}

func (s *fakeStore) UpdateTheme(_ context.Context, theme string, expectedRevision int64) (SiteSettings, error) {
	if s.settings.Revision != expectedRevision {
		return SiteSettings{}, ErrVersionConflict
	}
	s.settings.DefaultTheme = theme
	s.settings.Revision++
	s.settings.UpdatedAt = s.settings.UpdatedAt.Add(time.Second)
	return s.settings, nil
}

func TestServiceGetReturnsStoredDefault(t *testing.T) {
	want := SiteSettings{DefaultTheme: "lamp", Revision: 4, UpdatedAt: time.Unix(42, 0)}
	service := NewService(&fakeStore{settings: want})

	got, err := service.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != want {
		t.Fatalf("Get() = %+v, want %+v", got, want)
	}
}

func TestServiceUpdateAcceptsEveryThemeAndIncrementsRevision(t *testing.T) {
	for _, theme := range []string{"ink", "lamp", "codex-lavender", "night-ink"} {
		t.Run(theme, func(t *testing.T) {
			store := &fakeStore{settings: SiteSettings{DefaultTheme: "ink", Revision: 1}}
			service := NewService(store)

			got, err := service.UpdateTheme(context.Background(), theme, 1)
			if err != nil {
				t.Fatalf("UpdateTheme() error = %v", err)
			}
			if got.DefaultTheme != theme || got.Revision != 2 {
				t.Fatalf("UpdateTheme() = %+v, want theme %q revision 2", got, theme)
			}
		})
	}
}

func TestServiceUpdateRejectsUnknownThemeBeforeStore(t *testing.T) {
	store := &fakeStore{settings: SiteSettings{DefaultTheme: "ink", Revision: 1}}
	service := NewService(store)

	_, err := service.UpdateTheme(context.Background(), "bogus", 1)
	if !errors.Is(err, ErrInvalidTheme) {
		t.Fatalf("UpdateTheme() error = %v, want ErrInvalidTheme", err)
	}
	if store.settings.DefaultTheme != "ink" || store.settings.Revision != 1 {
		t.Fatalf("store changed after invalid theme: %+v", store.settings)
	}
}

func TestServiceUpdateReturnsVersionConflict(t *testing.T) {
	store := &fakeStore{settings: SiteSettings{DefaultTheme: "ink", Revision: 2}}
	service := NewService(store)

	_, err := service.UpdateTheme(context.Background(), "lamp", 1)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("UpdateTheme() error = %v, want ErrVersionConflict", err)
	}
}
