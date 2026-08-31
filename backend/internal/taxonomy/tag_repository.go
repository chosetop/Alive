package taxonomy

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

const (
	tagUniqueViolation     = "23505"
	tagForeignKeyViolation = "23503"
	tagDependencyViolation = "23001"
)

type TagRepository struct {
	q *sqlcgen.Queries
}

func NewTagRepository(pool *postgres.Pool) *TagRepository {
	return &TagRepository{q: sqlcgen.New(pool)}
}

func (r *TagRepository) Create(ctx context.Context, params CreateTagParams) (Tag, error) {
	row, err := r.q.CreateTag(ctx, sqlcgen.CreateTagParams{
		Name: params.Name,
		Slug: params.Slug,
	})
	if err != nil {
		return Tag{}, translateTagWriteError("create tag", err, params.Name, params.Slug)
	}

	return tagFromRow(row), nil
}

func (r *TagRepository) GetByID(ctx context.Context, id int64) (Tag, error) {
	row, err := r.q.GetTagByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tag{}, fmt.Errorf("%w: id %d", ErrTagNotFound, id)
		}
		return Tag{}, fmt.Errorf("taxonomy: get tag by id: %w", err)
	}
	return tagFromRow(row), nil
}

func (r *TagRepository) GetBySlug(ctx context.Context, slug string) (Tag, error) {
	row, err := r.q.GetTagBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tag{}, fmt.Errorf("%w: %s", ErrTagNotFound, slug)
		}
		return Tag{}, fmt.Errorf("taxonomy: get tag by slug: %w", err)
	}
	return tagFromRow(row), nil
}

func (r *TagRepository) List(ctx context.Context, query string, limit int) ([]Tag, error) {
	rows, err := r.q.ListTags(ctx, sqlcgen.ListTagsParams{
		Query: query,
		Limit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("taxonomy: list tags: %w", err)
	}

	tags := make([]Tag, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, Tag{
			ID:         row.ID,
			Name:       row.Name,
			Slug:       row.Slug,
			UsageCount: row.UsageCount,
			CreatedAt:  row.CreatedAt,
			UpdatedAt:  row.UpdatedAt,
		})
	}
	return tags, nil
}

func (r *TagRepository) ListPublicEntriesByTag(ctx context.Context, slug string, limit, offset int) ([]PublicTagEntry, int64, error) {
	rows, err := r.q.ListPublicEntriesByTag(ctx, sqlcgen.ListPublicEntriesByTagParams{Slug: slug, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, 0, fmt.Errorf("taxonomy: list public tag entries: %w", err)
	}
	total, err := r.q.CountPublicEntriesByTag(ctx, slug)
	if err != nil {
		return nil, 0, fmt.Errorf("taxonomy: count public tag entries: %w", err)
	}
	out := make([]PublicTagEntry, 0, len(rows))
	for _, row := range rows {
		summary, cover := "", ""
		if row.Summary != nil {
			summary = *row.Summary
		}
		if row.CoverUrl != nil {
			cover = *row.CoverUrl
		}
		out = append(out, PublicTagEntry{World: row.World, Kind: row.Kind, Slug: row.Slug, Title: row.Title, Summary: summary, CoverURL: cover})
	}
	return out, total, nil
}

func (r *TagRepository) Update(ctx context.Context, params UpdateTagParams) (Tag, error) {
	row, err := r.q.UpdateTag(ctx, sqlcgen.UpdateTagParams{
		ID:      params.ID,
		SetName: params.SetName,
		Name:    params.Name,
		SetSlug: params.SetSlug,
		Slug:    params.Slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Tag{}, fmt.Errorf("%w: id %d", ErrTagNotFound, params.ID)
		}
		return Tag{}, translateTagWriteError("update tag", err, params.Name, params.Slug)
	}
	return tagFromRow(row), nil
}

func (r *TagRepository) Delete(ctx context.Context, id int64) (bool, error) {
	_, err := r.q.DeleteTag(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, translateTagDeleteError(err)
	}
	return true, nil
}

func (r *TagRepository) NameExists(ctx context.Context, name string) (bool, error) {
	exists, err := r.q.TagNameExists(ctx, name)
	if err != nil {
		return false, fmt.Errorf("taxonomy: tag name exists: %w", err)
	}
	return exists, nil
}

func (r *TagRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	exists, err := r.q.TagSlugExists(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("taxonomy: tag slug exists: %w", err)
	}
	return exists, nil
}

func tagFromRow(row sqlcgen.Tag) Tag {
	return Tag{
		ID:        row.ID,
		Name:      row.Name,
		Slug:      row.Slug,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func translateTagWriteError(op string, err error, name, slug string) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case tagUniqueViolation:
			switch pgErr.ConstraintName {
			case "tags_name_key":
				return fmt.Errorf("%w: %s", ErrTagNameTaken, name)
			case "tags_slug_key":
				return fmt.Errorf("%w: %s", ErrTagSlugTaken, slug)
			}
		}
	}
	return fmt.Errorf("taxonomy: %s: %w", op, err)
}

func translateTagDeleteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == tagForeignKeyViolation || pgErr.Code == tagDependencyViolation) {
		return ErrTagInUse
	}
	return fmt.Errorf("taxonomy: delete tag: %w", err)
}
