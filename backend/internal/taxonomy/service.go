package taxonomy

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// Store is the storage the service needs.
//
// Declared next to the code that calls it, not next to the implementation. A
// method appears here only because something above it uses that method, and the
// service can be tested against a fake without a database.
type Store interface {
	Create(ctx context.Context, params CreateParams) (Category, error)
	GetByID(ctx context.Context, id int64) (Category, error)
	GetBySlug(ctx context.Context, slug string) (Category, error)

	// List returns every category in display order, without counts.
	List(ctx context.Context) ([]Category, error)

	// ListWithCounts returns every category with the number of entries a public
	// reader can see in it.
	ListWithCounts(ctx context.Context) ([]CategoryWithCount, error)

	Update(ctx context.Context, params UpdateParams) (Category, error)

	// Delete removes a category and reports whether it found one.
	Delete(ctx context.Context, id int64) (bool, error)

	SlugExists(ctx context.Context, slug string) (bool, error)

	// SlugExistsExcluding reports whether a category other than excludedID holds
	// the slug. Separate from SlugExists because a category keeping its own slug
	// through an update is not a conflict with itself.
	SlugExistsExcluding(ctx context.Context, slug string, excludedID int64) (bool, error)
}

// Checked at compile time so the repository cannot drift from the interface.
var _ Store = (*Repository)(nil)

// Service holds the rules about categories.
//
// No HTTP here: no status code, no request, no query string. The same methods
// back a future CLI command for setting up categories.
//
// No clock, unlike entry.Service. Nothing here stamps a time: created_at and
// updated_at are written by the column default and the trigger, because a
// category has no equivalent of "first published" that the domain decides.
type Service struct {
	store Store
	log   *slog.Logger
}

// ServiceOption adjusts a Service at construction.
type ServiceOption func(*Service)

// WithLogger sets where non-fatal problems are reported.
func WithLogger(l *slog.Logger) ServiceOption {
	return func(s *Service) {
		if l != nil {
			s.log = l
		}
	}
}

// NewService builds a Service over store.
func NewService(store Store, opts ...ServiceOption) *Service {
	s := &Service{store: store}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateInput carries a create request as the caller states it.
type CreateInput struct {
	Name        string
	Slug        string
	Description string

	// SortOrder defaults to 0, which puts a new category first among those that
	// have not been ordered. Zero is a real position, not "unspecified".
	SortOrder int
}

// Create validates a category and stores it.
//
// Validation first so a malformed request costs no query, then the slug check,
// because a conflict is the likely failure and saying so plainly beats surfacing
// a constraint violation.
func (s *Service) Create(ctx context.Context, in CreateInput) (Category, error) {
	if err := ValidateName(in.Name); err != nil {
		return Category{}, fmt.Errorf("%w: %s", err, in.Name)
	}
	// Supplied by the client, never derived from the name, for the same reason as
	// an entry slug: deriving one from Chinese text needs either percent-encoding
	// or a pinyin dependency in the backend.
	if err := ValidateSlug(in.Slug); err != nil {
		return Category{}, fmt.Errorf("%w: %s", err, in.Slug)
	}

	// Checked before the insert so the common conflict produces a clear error.
	// This is not the guarantee: two concurrent creates can both read false, and
	// categories_slug_key settles it. The repository translates that violation to
	// the same ErrSlugTaken, so both paths agree.
	taken, err := s.store.SlugExists(ctx, in.Slug)
	if err != nil {
		return Category{}, err
	}
	if taken {
		return Category{}, fmt.Errorf("%w: %s", ErrSlugTaken, in.Slug)
	}

	created, err := s.store.Create(ctx, CreateParams{
		Name:        in.Name,
		Slug:        in.Slug,
		Description: in.Description,
		SortOrder:   in.SortOrder,
	})
	if err != nil {
		if errors.Is(err, ErrSlugTaken) {
			// The pre-check said the slug was free and the constraint disagreed, so
			// two creates raced. Worth a line: it is the only evidence the race is
			// real rather than theoretical, and the client sees the same conflict.
			s.logger().WarnContext(ctx, "category slug conflict detected by the constraint, not the pre-check",
				slog.String("slug", in.Slug),
			)
		}
		return Category{}, err
	}

	return created, nil
}

// UpdateInput carries a partial change: which fields to touch, and their new
// values.
//
// Every field is a pointer, and nil means "not submitted, leave it alone", as in
// entry.UpdateInput. A pointer to a zero value is a submitted zero: *Description
// pointing at "" clears the description, and *SortOrder pointing at 0 moves the
// category to the front rather than meaning "unspecified".
type UpdateInput struct {
	Name        *string
	Slug        *string
	Description *string
	SortOrder   *int
}

// IsEmpty reports whether the input asks for no change at all.
func (in UpdateInput) IsEmpty() bool {
	return in.Name == nil &&
		in.Slug == nil &&
		in.Description == nil &&
		in.SortOrder == nil
}

// Update applies a partial change to one category.
//
// Only submitted fields are validated, for the same reason as in entry: checking
// the whole category would report errors about fields the client did not send.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Category, error) {
	if id <= 0 {
		return Category{}, fmt.Errorf("%w: id %d", ErrCategoryNotFound, id)
	}
	if in.IsEmpty() {
		// Refused rather than treated as a successful no-op: an empty body is a
		// client that built its request wrongly, and 200 would hide that.
		return Category{}, ErrNoUpdateFields
	}

	if in.Name != nil {
		if err := ValidateName(*in.Name); err != nil {
			return Category{}, fmt.Errorf("%w: %s", err, *in.Name)
		}
	}
	if in.Slug != nil {
		if err := ValidateSlug(*in.Slug); err != nil {
			return Category{}, fmt.Errorf("%w: %s", err, *in.Slug)
		}
	}

	params := UpdateParams{ID: id}

	if in.Name != nil {
		params.SetName, params.Name = true, *in.Name
	}
	if in.Slug != nil {
		// Excluding this category: submitting its own slug back is a no-op, not a
		// conflict with itself.
		taken, err := s.store.SlugExistsExcluding(ctx, *in.Slug, id)
		if err != nil {
			return Category{}, err
		}
		if taken {
			return Category{}, fmt.Errorf("%w: %s", ErrSlugTaken, *in.Slug)
		}
		params.SetSlug, params.Slug = true, *in.Slug
	}
	if in.Description != nil {
		params.SetDescription, params.Description = true, *in.Description
	}
	if in.SortOrder != nil {
		params.SetSortOrder, params.SortOrder = true, *in.SortOrder
	}

	updated, err := s.store.Update(ctx, params)
	if err != nil {
		if errors.Is(err, ErrSlugTaken) && in.Slug != nil {
			s.logger().WarnContext(ctx, "category slug conflict detected by the constraint, not the pre-check",
				slog.String("slug", *in.Slug),
				slog.Int64("category_id", id),
			)
		}
		return Category{}, err
	}

	return updated, nil
}

// Delete removes one category.
//
// The entries in it are not deleted and not refused: the foreign key sets their
// category_id to NULL, so they become uncategorised. That is why this logs. The
// delete leaves nothing behind recording which category those entries pointed at,
// so the log line is the only trace that the change happened at all.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id %d", ErrCategoryNotFound, id)
	}

	deleted, err := s.store.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("%w: id %d", ErrCategoryNotFound, id)
	}

	s.logger().InfoContext(ctx, "category deleted, its entries are now uncategorised",
		slog.Int64("category_id", id),
	)

	return nil
}

// GetByID returns one category.
func (s *Service) GetByID(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, fmt.Errorf("%w: id %d", ErrCategoryNotFound, id)
	}
	return s.store.GetByID(ctx, id)
}

// GetBySlug returns the category holding this slug.
//
// A malformed slug is answered without a query: it cannot match a row. The error
// is the same as for a well-formed slug that is absent, since a reader has no use
// for the difference.
func (s *Service) GetBySlug(ctx context.Context, slug string) (Category, error) {
	if err := ValidateSlug(slug); err != nil {
		return Category{}, fmt.Errorf("%w: %s", ErrCategoryNotFound, slug)
	}
	return s.store.GetBySlug(ctx, slug)
}

// List returns every category in display order.
//
// The admin read. No counts, because the admin list is a place to edit categories
// rather than to see how full they are.
func (s *Service) List(ctx context.Context) ([]Category, error) {
	return s.store.List(ctx)
}

// ListWithCounts returns every category with its publicly visible entry count.
//
// The public read. The count matches what opening the category will show, so a
// category holding only drafts reports zero.
func (s *Service) ListWithCounts(ctx context.Context) ([]CategoryWithCount, error) {
	return s.store.ListWithCounts(ctx)
}

// ResolveSlug turns a category slug into its id.
//
// This is what an entry list filtered by ?category=travel calls. It exists so
// that an unknown slug is ErrCategoryNotFound, which the caller answers as 404,
// rather than an empty list: "this URL is wrong" and "this category is empty" are
// different answers and a reader needs to tell them apart.
func (s *Service) ResolveSlug(ctx context.Context, slug string) (int64, error) {
	category, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return 0, err
	}
	return category.ID, nil
}

func (s *Service) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}
