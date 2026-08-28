package site

import (
	"context"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/dbtest"
)

func TestRepositoryPersistsNightInk(t *testing.T) {
	pool := dbtest.Pool(t)
	repository := NewRepository(pool)
	ctx := context.Background()

	before, err := repository.Get(ctx)
	if err != nil {
		t.Fatalf("Get() before update: %v", err)
	}

	updated, err := repository.UpdateTheme(ctx, "night-ink", before.Revision)
	if err != nil {
		t.Fatalf("UpdateTheme(night-ink): %v", err)
	}
	t.Cleanup(func() {
		if _, err := repository.UpdateTheme(ctx, before.DefaultTheme, updated.Revision); err != nil {
			t.Errorf("restore site default theme: %v", err)
		}
	})

	got, err := repository.Get(ctx)
	if err != nil {
		t.Fatalf("Get() after update: %v", err)
	}
	if got.DefaultTheme != "night-ink" {
		t.Fatalf("DefaultTheme = %q, want night-ink", got.DefaultTheme)
	}
}
