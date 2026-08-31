// Package taxonomytest provides an in-memory taxonomy.Store for tests.
//
// Its own package for the same reason as entrytest: two test packages need it,
// internal/taxonomy for the rules and internal/taxonomyhttp for the HTTP surface
// over a real service. A copy in each would drift, and a field added to the domain
// would then be handled by one fake and not the other.
//
// Under internal/, so nothing outside this module can depend on it.
package taxonomytest

import (
	"context"
	"fmt"
	"sort"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// Store is an in-memory taxonomy.Store.
//
// What the service decides is which fields are valid, whether a slug conflicts,
// and what an empty update means. None of that needs a database. The repository's
// own translation of constraint violations is tested against a real one, which is
// where that belongs.
type Store struct {
	categories map[int64]taxonomy.Category
	nextID     int64

	// Injected failures, one per method with a distinct failure path.
	FailCreate              error
	FailGetByID             error
	FailGetBySlug           error
	FailList                error
	FailListWithCounts      error
	FailUpdate              error
	FailDelete              error
	FailSlugExists          error
	FailSlugExistsExcluding error

	// SlugExistsAlwaysFree makes the pre-check report every slug as available
	// while Create still refuses a duplicate.
	//
	// That is what two racing creates see: both read a free slug and the unique
	// constraint refuses the second write. Without this switch the pre-check would
	// catch every conflict and the write's own path would never run in a test.
	SlugExistsAlwaysFree bool

	// EntryCounts supplies what ListWithCounts reports, keyed by category id.
	//
	// This fake holds no entries, so it cannot derive a count. Setting it here is
	// not a weaker test: the count is produced by a LEFT JOIN whose correctness is
	// a property of the SQL, verified against a real database. What the service
	// does with the number is pass it along, and that is what this exercises. A
	// category absent from the map reports zero.
	EntryCounts map[int64]int64

	// Call counts, for asserting that something did or did not happen.
	CreateCalls              int
	UpdateCalls              int
	DeleteCalls              int
	SlugExistsCalls          int
	SlugExistsExcludingCalls int

	// LastCreate records what the service asked to be written.
	LastCreate taxonomy.CreateParams

	// LastUpdate records the same for an update. The Set flags are the interesting
	// part: they are how "leave this alone" is told from "clear this", and the
	// returned category cannot distinguish the two.
	LastUpdate taxonomy.UpdateParams
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		categories:  make(map[int64]taxonomy.Category),
		EntryCounts: make(map[int64]int64),
		nextID:      1,
	}
}

// Create records a category and returns it with an id.
func (s *Store) Create(ctx context.Context, params taxonomy.CreateParams) (taxonomy.Category, error) {
	s.CreateCalls++
	s.LastCreate = params

	if s.FailCreate != nil {
		return taxonomy.Category{}, s.FailCreate
	}

	// categories_slug_key is part of the contract this fake stands in for. The
	// service treats a conflict from the write as equivalent to one from the
	// pre-check, so that path has to be reachable here.
	for _, existing := range s.categories {
		if existing.World == params.World && existing.Slug == params.Slug {
			return taxonomy.Category{}, fmt.Errorf("%w: %s", taxonomy.ErrSlugTaken, params.Slug)
		}
	}

	id := s.nextID
	s.nextID++

	stored := taxonomy.Category{
		ID:          id,
		World:       params.World,
		Name:        params.Name,
		Slug:        params.Slug,
		Description: params.Description,
		SortOrder:   params.SortOrder,
	}
	s.categories[id] = stored

	return stored, nil
}

// GetByID returns one category.
func (s *Store) GetByID(ctx context.Context, id int64) (taxonomy.Category, error) {
	if s.FailGetByID != nil {
		return taxonomy.Category{}, s.FailGetByID
	}

	stored, ok := s.categories[id]
	if !ok {
		return taxonomy.Category{}, fmt.Errorf("%w: id %d", taxonomy.ErrCategoryNotFound, id)
	}
	return stored, nil
}

// GetBySlug returns the category holding this slug.
func (s *Store) GetBySlug(ctx context.Context, world contentworld.Key, slug string) (taxonomy.Category, error) {
	if s.FailGetBySlug != nil {
		return taxonomy.Category{}, s.FailGetBySlug
	}

	for _, candidate := range s.categories {
		if candidate.World == world && candidate.Slug == slug {
			return candidate, nil
		}
	}
	return taxonomy.Category{}, fmt.Errorf("%w: %s", taxonomy.ErrCategoryNotFound, slug)
}

// List returns every category ordered as the SQL orders them: sort_order
// ascending, id ascending to break ties.
//
// The order is reproduced rather than left to map iteration, which is randomised
// in Go. Without it a test asserting on order would pass or fail at random.
func (s *Store) List(ctx context.Context, world contentworld.Key) ([]taxonomy.Category, error) {
	if s.FailList != nil {
		return nil, s.FailList
	}
	return s.sortedByWorld(world), nil
}

// ListWithCounts returns every category with the count from EntryCounts.
func (s *Store) ListWithCounts(ctx context.Context, world contentworld.Key) ([]taxonomy.CategoryWithCount, error) {
	if s.FailListWithCounts != nil {
		return nil, s.FailListWithCounts
	}

	ordered := s.sortedByWorld(world)
	out := make([]taxonomy.CategoryWithCount, 0, len(ordered))
	for _, category := range ordered {
		out = append(out, taxonomy.CategoryWithCount{
			Category: category,
			// Absent means zero, matching the LEFT JOIN: an empty category is listed
			// with a count of zero rather than omitted.
			EntryCount: s.EntryCounts[category.ID],
		})
	}

	return out, nil
}

// Update applies the flagged fields and returns the category as stored.
//
// Only fields whose Set flag is true are written, matching the CASE expressions
// in the SQL. A fake that wrote every field would let an update pass here while
// the real statement left those columns alone.
func (s *Store) Update(ctx context.Context, params taxonomy.UpdateParams) (taxonomy.Category, error) {
	s.UpdateCalls++
	s.LastUpdate = params

	if s.FailUpdate != nil {
		return taxonomy.Category{}, s.FailUpdate
	}

	stored, ok := s.categories[params.ID]
	if !ok {
		return taxonomy.Category{}, fmt.Errorf("%w: id %d", taxonomy.ErrCategoryNotFound, params.ID)
	}

	if params.SetSlug {
		// The constraint again: an update onto a slug another category holds is
		// refused by the database, and the service handles that error.
		for id, existing := range s.categories {
			if id != params.ID && existing.Slug == params.Slug {
				return taxonomy.Category{}, fmt.Errorf("%w: %s", taxonomy.ErrSlugTaken, params.Slug)
			}
		}
		stored.Slug = params.Slug
	}
	if params.SetName {
		stored.Name = params.Name
	}
	if params.SetDescription {
		stored.Description = params.Description
	}
	if params.SetSortOrder {
		stored.SortOrder = params.SortOrder
	}

	s.categories[params.ID] = stored
	return stored, nil
}

// Delete removes a category, reporting whether it found one.
//
// Nothing here reproduces ON DELETE SET NULL: this fake holds no entries, and the
// foreign key's effect is a property of the schema, tested against a real
// database. What the service does with a delete is log it and report a miss as not
// found, which is what this exercises.
func (s *Store) Delete(ctx context.Context, id int64) (bool, error) {
	s.DeleteCalls++

	if s.FailDelete != nil {
		return false, s.FailDelete
	}

	if _, ok := s.categories[id]; !ok {
		return false, nil
	}
	delete(s.categories, id)
	return true, nil
}

// SlugExists reports whether any stored category holds this slug.
func (s *Store) SlugExists(ctx context.Context, world contentworld.Key, slug string) (bool, error) {
	s.SlugExistsCalls++

	if s.FailSlugExists != nil {
		return false, s.FailSlugExists
	}
	if s.SlugExistsAlwaysFree {
		return false, nil
	}

	for _, candidate := range s.categories {
		if candidate.World == world && candidate.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

// SlugExistsExcluding reports whether a category other than excludedID holds the
// slug.
//
// The exclusion is the point: an update resubmitting a category's own slug must
// not be refused as a conflict with itself.
func (s *Store) SlugExistsExcluding(ctx context.Context, world contentworld.Key, slug string, excludedID int64) (bool, error) {
	s.SlugExistsExcludingCalls++

	if s.FailSlugExistsExcluding != nil {
		return false, s.FailSlugExistsExcluding
	}
	if s.SlugExistsAlwaysFree {
		return false, nil
	}

	for id, candidate := range s.categories {
		if id != excludedID && candidate.World == world && candidate.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

// Seed places a category directly in the store, bypassing validation.
//
// Tests use it to arrange a starting state without going through Create. The id
// is assigned here, and nextID moves past it so a later Create cannot collide.
func (s *Store) Seed(category taxonomy.Category) taxonomy.Category {
	if category.ID == 0 {
		category.ID = s.nextID
	}
	if category.ID >= s.nextID {
		s.nextID = category.ID + 1
	}
	s.categories[category.ID] = category
	return category
}

// Len reports how many categories are stored, for asserting that a delete
// removed one.
func (s *Store) Len() int {
	return len(s.categories)
}

// sorted returns the stored categories in display order.
func (s *Store) sorted() []taxonomy.Category {
	out := make([]taxonomy.Category, 0, len(s.categories))
	for _, category := range s.categories {
		out = append(out, category)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})

	return out
}

func (s *Store) sortedByWorld(world contentworld.Key) []taxonomy.Category {
	ordered := s.sorted()
	out := make([]taxonomy.Category, 0, len(ordered))
	for _, category := range ordered {
		if category.World == world {
			out = append(out, category)
		}
	}
	return out
}

// Checked at compile time: the fake must satisfy the same interface as the
// repository, or a test could pass against a shape the real store does not have.
var _ taxonomy.Store = (*Store)(nil)
