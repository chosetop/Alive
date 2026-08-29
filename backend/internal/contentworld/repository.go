package contentworld

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (r *Repository) ListOpen(ctx context.Context) ([]Setting, error) {
	rows, err := r.q.ListOpenWorlds(ctx)
	if err != nil {
		return nil, fmt.Errorf("contentworld: list open worlds: %w", err)
	}

	settings := make([]Setting, 0, len(rows))
	for _, row := range rows {
		settings = append(settings, settingFromRow(settingRow{
			World:       row.World,
			Status:      row.Status,
			NavLabel:    row.NavLabel,
			SortOrder:   row.SortOrder,
			DefaultView: row.DefaultView,
			Revision:    row.Revision,
			UpdatedAt:   row.UpdatedAt,
		}))
	}
	return settings, nil
}

func (r *Repository) ListAll(ctx context.Context) ([]Setting, error) {
	rows, err := r.q.ListAllWorlds(ctx)
	if err != nil {
		return nil, fmt.Errorf("contentworld: list worlds: %w", err)
	}

	settings := make([]Setting, 0, len(rows))
	for _, row := range rows {
		settings = append(settings, settingFromRow(settingRow{
			World:       row.World,
			Status:      row.Status,
			NavLabel:    row.NavLabel,
			SortOrder:   row.SortOrder,
			DefaultView: row.DefaultView,
			Revision:    row.Revision,
			UpdatedAt:   row.UpdatedAt,
		}))
	}
	return settings, nil
}

func (r *Repository) Get(ctx context.Context, key Key) (Setting, error) {
	row, err := r.q.GetWorld(ctx, string(key))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Setting{}, fmt.Errorf("%w: %s", ErrWorldNotFound, key)
		}
		return Setting{}, fmt.Errorf("contentworld: get world %s: %w", key, err)
	}

	return settingFromRow(settingRow{
		World:       row.World,
		Status:      row.Status,
		NavLabel:    row.NavLabel,
		SortOrder:   row.SortOrder,
		DefaultView: row.DefaultView,
		Revision:    row.Revision,
		UpdatedAt:   row.UpdatedAt,
	}), nil
}

func (r *Repository) Update(ctx context.Context, key Key, input UpdateInput) (Setting, error) {
	row, err := r.q.UpdateWorld(ctx, sqlcgen.UpdateWorldParams{
		World:            string(key),
		ExpectedRevision: input.ExpectedRevision,
		SetStatus:        input.Status != nil,
		Status:           statusValue(input.Status),
		SetNavLabel:      input.NavLabel != nil,
		NavLabel:         stringValue(input.NavLabel),
		SetDefaultView:   input.DefaultView != nil,
		DefaultView:      viewValue(input.DefaultView),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Setting{}, fmt.Errorf("%w: expected revision %d", ErrVersionConflict, input.ExpectedRevision)
		}
		return Setting{}, fmt.Errorf("contentworld: update world %s: %w", key, err)
	}

	return settingFromRow(settingRow{
		World:       row.World,
		Status:      row.Status,
		NavLabel:    row.NavLabel,
		SortOrder:   row.SortOrder,
		DefaultView: row.DefaultView,
		Revision:    row.Revision,
		UpdatedAt:   row.UpdatedAt,
	}), nil
}

type settingRow struct {
	World       string
	Status      string
	NavLabel    string
	SortOrder   int32
	DefaultView string
	Revision    int64
	UpdatedAt   time.Time
}

func settingFromRow(row settingRow) Setting {
	return Setting{
		World:       Key(row.World),
		Status:      Status(row.Status),
		NavLabel:    row.NavLabel,
		SortOrder:   int(row.SortOrder),
		DefaultView: ViewMode(row.DefaultView),
		Revision:    row.Revision,
		UpdatedAt:   row.UpdatedAt,
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func statusValue(value *Status) string {
	if value == nil {
		return ""
	}
	return string(*value)
}

func viewValue(value *ViewMode) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
