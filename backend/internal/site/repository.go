package site

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

type Repository struct {
	q *sqlcgen.Queries
}

func NewRepository(pool *postgres.Pool) *Repository {
	return &Repository{q: sqlcgen.New(pool)}
}

func (r *Repository) Get(ctx context.Context) (SiteSettings, error) {
	row, err := r.q.GetSiteSettings(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiteSettings{}, fmt.Errorf("site: settings row missing: %w", err)
		}
		return SiteSettings{}, fmt.Errorf("site: get settings: %w", err)
	}
	return SiteSettings{DefaultTheme: row.DefaultTheme, Revision: row.Revision, UpdatedAt: row.UpdatedAt}, nil
}

func (r *Repository) UpdateTheme(ctx context.Context, theme string, expectedRevision int64) (SiteSettings, error) {
	row, err := r.q.UpdateSiteTheme(ctx, sqlcgen.UpdateSiteThemeParams{
		DefaultTheme:     theme,
		ExpectedRevision: expectedRevision,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return SiteSettings{}, fmt.Errorf("%w: expected revision %d", ErrVersionConflict, expectedRevision)
		}
		return SiteSettings{}, fmt.Errorf("site: update theme: %w", err)
	}
	return SiteSettings{DefaultTheme: row.DefaultTheme, Revision: row.Revision, UpdatedAt: row.UpdatedAt}, nil
}
