package taxonomy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

// Repository reads and writes categories.
//
// This file is the only place in the package that imports pgx. Above it,
// everything sees domain types and the sentinel errors from model.go.
type Repository struct {
	q *sqlcgen.Queries
}

// NewRepository wires a repository to a connection pool.
func NewRepository(pool *postgres.Pool) *Repository {
	return &Repository{q: sqlcgen.New(pool)}
}

// uniqueViolation is the only driver code translated here.
//
// entry's repository also handles 23514 and 23503: a check violation and a
// foreign key one. Neither can reach this table. The two CHECK constraints on
// categories are already enforced by ValidateName and ValidateSlug, and nothing
// this package writes references another table.
const uniqueViolation = "23505"
const checkViolation = "23514"

// CreateParams is a validated category ready to be written.
type CreateParams struct {
	World       contentworld.Key
	Name        string
	Slug        string
	Description string
	SortOrder   int
}

// UpdateParams is a validated partial change ready to be written.
//
// Each field is paired with a Set flag, as in entry.UpdateParams. The flag is
// not redundant: description is nullable, so "clear the description" and "leave
// it alone" are both an absent value and only the flag tells them apart.
type UpdateParams struct {
	ID int64

	SetName bool
	Name    string

	SetSlug bool
	Slug    string

	SetDescription bool
	Description    string

	SetSortOrder bool
	SortOrder    int
}

// Create writes one category.
func (r *Repository) Create(ctx context.Context, params CreateParams) (Category, error) {
	row, err := r.q.CreateCategory(ctx, sqlcgen.CreateCategoryParams{
		World:       string(params.World),
		Name:        params.Name,
		Slug:        params.Slug,
		Description: optionalString(params.Description),
		SortOrder:   int32(params.SortOrder),
	})
	if err != nil {
		return Category{}, translateWriteError("create category", err, params.Slug)
	}

	return categoryFromRow(rowFields{
		ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
		Description: row.Description, SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// GetByID returns one category.
func (r *Repository) GetByID(ctx context.Context, id int64) (Category, error) {
	row, err := r.q.GetCategoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, fmt.Errorf("%w: id %d", ErrCategoryNotFound, id)
		}
		return Category{}, fmt.Errorf("taxonomy: get category by id: %w", err)
	}

	return categoryFromRow(rowFields{
		ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
		Description: row.Description, SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// GetBySlug returns the category holding this slug.
//
// This is how a category page resolves its URL segment, and how a filtered entry
// list turns ?category=travel into an id.
func (r *Repository) GetBySlug(ctx context.Context, world contentworld.Key, slug string) (Category, error) {
	row, err := r.q.GetCategoryBySlug(ctx, sqlcgen.GetCategoryBySlugParams{
		World: string(world),
		Slug:  slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Category{}, fmt.Errorf("%w: %s", ErrCategoryNotFound, slug)
		}
		return Category{}, fmt.Errorf("taxonomy: get category by slug: %w", err)
	}

	return categoryFromRow(rowFields{
		ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
		Description: row.Description, SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// List returns every category in display order.
//
// No pagination. This is a navigation structure, and one that needs paging is one
// nobody can navigate.
func (r *Repository) List(ctx context.Context, world contentworld.Key) ([]Category, error) {
	rows, err := r.q.ListCategories(ctx, string(world))
	if err != nil {
		return nil, fmt.Errorf("taxonomy: list categories: %w", err)
	}

	categories := make([]Category, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, categoryFromRow(rowFields{
			ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
			Description: row.Description, SortOrder: row.SortOrder,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}

	return categories, nil
}

// ListWithCounts returns every category with its publicly visible entry count.
//
// Categories holding nothing are included with a count of zero. Whether to show
// them is a display decision, and dropping them here would take it away.
func (r *Repository) ListWithCounts(ctx context.Context, world contentworld.Key) ([]CategoryWithCount, error) {
	rows, err := r.q.ListCategoriesWithCounts(ctx, string(world))
	if err != nil {
		return nil, fmt.Errorf("taxonomy: list categories with counts: %w", err)
	}

	categories := make([]CategoryWithCount, 0, len(rows))
	for _, row := range rows {
		categories = append(categories, CategoryWithCount{
			Category: categoryFromRow(rowFields{
				ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
				Description: row.Description, SortOrder: row.SortOrder,
				CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			}),
			EntryCount: row.EntryCount,
		})
	}

	return categories, nil
}

// Update applies a partial change to one category.
func (r *Repository) Update(ctx context.Context, params UpdateParams) (Category, error) {
	row, err := r.q.UpdateCategory(ctx, sqlcgen.UpdateCategoryParams{
		ID:             params.ID,
		SetName:        params.SetName,
		Name:           params.Name,
		SetSlug:        params.SetSlug,
		Slug:           params.Slug,
		SetDescription: params.SetDescription,
		Description:    optionalString(params.Description),
		SetSortOrder:   params.SetSortOrder,
		SortOrder:      int32(params.SortOrder),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// No row matched the id. Reported as not found rather than as a failed
			// write, since nothing was wrong with the request but the id.
			return Category{}, fmt.Errorf("%w: id %d", ErrCategoryNotFound, params.ID)
		}
		return Category{}, translateWriteError("update category", err, params.Slug)
	}

	return categoryFromRow(rowFields{
		ID: row.ID, World: contentworld.Key(row.World), Name: row.Name, Slug: row.Slug,
		Description: row.Description, SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// Delete removes one category and reports whether it found one.
//
// A physical delete. Entries pointing at it become uncategorised through the
// foreign key's ON DELETE SET NULL, without this statement mentioning them, so
// the delete succeeds quietly even when the category is in use.
func (r *Repository) Delete(ctx context.Context, id int64) (bool, error) {
	_, err := r.q.DeleteCategory(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("taxonomy: delete category: %w", err)
	}
	return true, nil
}

// SlugExists reports whether any category holds this slug.
func (r *Repository) SlugExists(ctx context.Context, world contentworld.Key, slug string) (bool, error) {
	exists, err := r.q.CategorySlugExists(ctx, sqlcgen.CategorySlugExistsParams{
		World: string(world),
		Slug:  slug,
	})
	if err != nil {
		return false, fmt.Errorf("taxonomy: category slug exists: %w", err)
	}
	return exists, nil
}

// SlugExistsExcluding reports whether a category other than excludedID holds the
// slug.
func (r *Repository) SlugExistsExcluding(ctx context.Context, world contentworld.Key, slug string, excludedID int64) (bool, error) {
	exists, err := r.q.CategorySlugExistsExcluding(ctx, sqlcgen.CategorySlugExistsExcludingParams{
		World:      string(world),
		Slug:       slug,
		ExcludedID: excludedID,
	})
	if err != nil {
		return false, fmt.Errorf("taxonomy: category slug exists excluding: %w", err)
	}
	return exists, nil
}

// rowFields is the shape every category-returning query produces.
//
// sqlc generates a distinct row type per query with identical fields, so without
// this the same conversion would be written once per query and a new column would
// have to be added to each copy.
type rowFields struct {
	ID          int64
	World       contentworld.Key
	Name        string
	Slug        string
	Description *string
	SortOrder   int32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// categoryFromRow converts a queried row into a domain Category.
func categoryFromRow(row rowFields) Category {
	return Category{
		ID:          row.ID,
		World:       row.World,
		Name:        row.Name,
		Slug:        row.Slug,
		Description: derefString(row.Description),
		SortOrder:   int(row.SortOrder),
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

// translateWriteError turns a driver error from a write into a domain error.
//
// A constraint violation is the database reporting an invalid request, not an
// internal failure. Passing these up as-is would make a duplicate slug look like
// a broken server rather than a client that needs to pick another slug.
//
// op names the operation, so a failed update does not report itself as a create.
func translateWriteError(op string, err error, slug string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("taxonomy: %s: %w", op, err)
	}

	if pgErr.Code == uniqueViolation && pgErr.ConstraintName == "categories_world_slug_key" {
		return fmt.Errorf("%w: %s", ErrSlugTaken, slug)
	}
	if pgErr.Code == checkViolation && pgErr.ConstraintName == "categories_world_check" {
		return ErrInvalidWorld
	}

	return fmt.Errorf("taxonomy: %s: %w", op, err)
}

// Conversions between the domain's zero value for an absent string and SQL NULL.
func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
