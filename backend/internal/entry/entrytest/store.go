// Package entrytest provides an in-memory entry.Store for tests.
//
// Its own package because two test packages need it: internal/entry tests the
// rules against it, and internal/entryhttp tests the HTTP surface over a real
// service. One copy per package would drift, and a field added to the domain
// would then be handled by one fake and not the other.
//
// Under internal/, so nothing outside this module can depend on it.
package entrytest

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/p30huiwei/alive/backend/internal/entry"
)

// Store is an in-memory entry.Store.
//
// The service's rules are decisions rather than queries: which defaults apply,
// whether published_at gets stamped, how a page number is clamped. Testing those
// against PostgreSQL would mean waiting on I/O to observe logic that never
// touches it. The repository's own translation of driver errors is tested against
// a real database, which is where that belongs.
type Store struct {
	entries map[int64]entry.Entry
	nextID  int64

	// Injected failures, one per method with a distinct failure path.
	FailCreate              error
	FailGetBySlug           error
	FailGetLinkBySlug       error
	FailListPublic          error
	FailSlugExists          error
	FailUpdate              error
	FailSoftDelete          error
	FailPublish             error
	FailUnpublish           error
	FailArchive             error
	FailGetByID             error
	FailListAdmin           error
	FailSlugExistsExcluding error

	// SlugExistsAlwaysFree makes the pre-check report every slug as available
	// while Create still refuses a duplicate.
	//
	// That is what happens when two creates race: both read a free slug, and the
	// unique index refuses the second write. Without this switch the pre-check
	// would catch every conflict here and the write's own path would never run in
	// a test, so the two would be free to report the situation differently.
	SlugExistsAlwaysFree bool

	// Call counts, for asserting that something did or did not happen.
	CreateCalls              int
	SlugExistsCalls          int
	UpdateCalls              int
	SlugExistsExcludingCalls int

	// LastCreate records what the service asked to be written. Assertions about
	// computed values (word count, published_at) read it rather than inferring
	// them from the returned entry.
	LastCreate entry.CreateParams

	// LastUpdate records the same for an update. The Set flags are the interesting
	// part: they are how "leave this alone" is told from "clear this", and a test
	// asserting on the returned entry alone could not tell the two apart.
	LastUpdate entry.UpdateParams

	// LastSoftDeleteAt records the timestamp the service supplied, so a test can
	// assert the delete used the injected clock rather than time.Now.
	LastSoftDeleteAt time.Time
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{entries: make(map[int64]entry.Entry), nextID: 1}
}

// Create records an entry and returns it with an id and timestamps.
func (s *Store) Create(ctx context.Context, params entry.CreateParams) (entry.Entry, error) {
	s.CreateCalls++
	s.LastCreate = params

	if s.FailCreate != nil {
		return entry.Entry{}, s.FailCreate
	}

	// The unique index is part of the contract this fake stands in for: the
	// service treats a conflict from the write as equivalent to one from the
	// pre-check, and that path needs to be reachable here.
	for _, existing := range s.entries {
		if existing.Slug == params.Slug {
			return entry.Entry{}, fmt.Errorf("%w: %s", entry.ErrSlugTaken, params.Slug)
		}
	}

	id := s.nextID
	s.nextID++

	stored := entry.Entry{
		ID:       id,
		AuthorID: params.AuthorID,
		// No CategoryName or CategorySlug: a write cannot join, and the real store
		// leaves them empty here too.
		CategoryID:  params.CategoryID,
		Type:        params.Type,
		Title:       params.Title,
		Slug:        params.Slug,
		Summary:     params.Summary,
		ContentMD:   params.ContentMD,
		CoverURL:    params.CoverURL,
		Status:      params.Status,
		Visibility:  params.Visibility,
		Meta:        params.Meta,
		WordCount:   params.WordCount,
		HappenedAt:  params.HappenedAt,
		PublishedAt: params.PublishedAt,
	}
	s.entries[id] = stored

	return stored, nil
}

// GetPublicBySlug returns the entry with this slug if it is publicly readable.
//
// The visibility rule is applied here as well as in SQL. Without it this fake
// would answer questions the real store refuses, and a handler test could pass
// while the endpoint leaks drafts.
func (s *Store) GetPublicBySlug(ctx context.Context, slug string) (entry.Entry, error) {
	if s.FailGetBySlug != nil {
		return entry.Entry{}, s.FailGetBySlug
	}

	for _, candidate := range s.entries {
		if candidate.Slug != slug {
			continue
		}
		if !candidate.IsPubliclyReadable() {
			break
		}
		return candidate, nil
	}

	return entry.Entry{}, fmt.Errorf("%w: %s", entry.ErrEntryNotFound, slug)
}

// GetLinkBySlug returns the entry with this slug if it is published and either
// public or unlisted.
//
// Uses IsLinkReadable, not IsPubliclyReadable. A fake that shared one predicate
// with GetPublicBySlug could not show the difference between the two reads, which
// is the only reason both exist.
func (s *Store) GetLinkBySlug(ctx context.Context, slug string) (entry.Entry, error) {
	if s.FailGetLinkBySlug != nil {
		return entry.Entry{}, s.FailGetLinkBySlug
	}

	for _, candidate := range s.entries {
		if candidate.Slug != slug {
			continue
		}
		if !candidate.IsLinkReadable() {
			break
		}
		return candidate, nil
	}

	return entry.Entry{}, fmt.Errorf("%w: %s", entry.ErrEntryNotFound, slug)
}

// ListPublic returns publicly readable entries ordered as the SQL orders them:
// happened_at descending with absent dates last, id descending to break ties.
//
// categoryID 0 means every category, as in the real store. The filter is applied
// to the total as well as the page: they describe the same set, and a fake that
// counted everything would let a wrong total pass here.
func (s *Store) ListPublic(ctx context.Context, categoryID int64, limit, offset int) ([]entry.Entry, int64, error) {
	if s.FailListPublic != nil {
		return nil, 0, s.FailListPublic
	}

	visible := make([]entry.Entry, 0, len(s.entries))
	for _, candidate := range s.entries {
		// Unlisted is excluded here because IsPubliclyReadable excludes it. That is
		// the difference this fake exists to preserve: GetLinkBySlug accepts it and
		// the list does not.
		if !candidate.IsPubliclyReadable() {
			continue
		}
		if categoryID != 0 && candidate.CategoryID != categoryID {
			continue
		}
		visible = append(visible, candidate)
	}

	sort.Slice(visible, func(i, j int) bool {
		a, b := visible[i], visible[j]
		switch {
		case a.HappenedAt.IsZero() != b.HappenedAt.IsZero():
			// Absent dates sort last, matching NULLS LAST.
			return b.HappenedAt.IsZero()
		case !a.HappenedAt.Equal(b.HappenedAt):
			return a.HappenedAt.After(b.HappenedAt)
		default:
			return a.ID > b.ID
		}
	})

	total := int64(len(visible))

	if offset >= len(visible) {
		return []entry.Entry{}, total, nil
	}
	end := offset + limit
	if end > len(visible) {
		end = len(visible)
	}

	// A copy of the slice range, so a caller cannot alter the stored order.
	page := make([]entry.Entry, end-offset)
	copy(page, visible[offset:end])

	return page, total, nil
}

// SlugExists reports whether any stored entry holds this slug.
//
// Every stored entry counts, whatever its status: a draft holds its slug so that
// publishing it later cannot collide. A soft deleted entry does not, because
// SoftDelete removes it from the map, which matches the partial unique index
// releasing the slug.
func (s *Store) SlugExists(ctx context.Context, slug string) (bool, error) {
	s.SlugExistsCalls++

	if s.FailSlugExists != nil {
		return false, s.FailSlugExists
	}
	if s.SlugExistsAlwaysFree {
		return false, nil
	}

	for _, candidate := range s.entries {
		if candidate.Slug == slug {
			return true, nil
		}
	}
	return false, nil
}

// SlugExistsExcluding reports whether an entry other than excludedID holds the
// slug.
//
// The exclusion is the point: an update that resubmits an entry's own slug must
// not be refused as a conflict with itself.
func (s *Store) SlugExistsExcluding(ctx context.Context, slug string, excludedID int64) (bool, error) {
	s.SlugExistsExcludingCalls++

	if s.FailSlugExistsExcluding != nil {
		return false, s.FailSlugExistsExcluding
	}
	if s.SlugExistsAlwaysFree {
		return false, nil
	}

	for _, candidate := range s.entries {
		if candidate.Slug == slug && candidate.ID != excludedID {
			return true, nil
		}
	}
	return false, nil
}

// Update applies the flagged fields and returns the entry as stored.
//
// Only fields whose Set flag is true are written, matching the CASE expressions in
// the SQL. A fake that wrote every field would let an update pass here while
// clearing columns the real statement leaves alone.
func (s *Store) Update(ctx context.Context, params entry.UpdateParams) (entry.Entry, error) {
	s.UpdateCalls++
	s.LastUpdate = params

	if s.FailUpdate != nil {
		return entry.Entry{}, s.FailUpdate
	}

	stored, ok := s.entries[params.ID]
	if !ok {
		return entry.Entry{}, fmt.Errorf("%w: id %d", entry.ErrEntryNotFound, params.ID)
	}

	// The unique index applies to an update too, and this fake stands in for it.
	if params.SetSlug {
		for _, candidate := range s.entries {
			if candidate.Slug == params.Slug && candidate.ID != params.ID {
				return entry.Entry{}, fmt.Errorf("%w: %s", entry.ErrSlugTaken, params.Slug)
			}
		}
	}

	if params.SetType {
		stored.Type = params.Type
	}
	if params.SetTitle {
		stored.Title = params.Title
	}
	if params.SetSlug {
		stored.Slug = params.Slug
	}
	if params.SetSummary {
		stored.Summary = params.Summary
	}
	if params.SetContentMD {
		stored.ContentMD = params.ContentMD
		stored.WordCount = params.WordCount
	}
	if params.SetCoverURL {
		stored.CoverURL = params.CoverURL
	}
	if params.SetVisibility {
		stored.Visibility = params.Visibility
	}
	if params.SetMeta {
		stored.Meta = params.Meta
	}
	if params.SetHappenedAt {
		stored.HappenedAt = params.HappenedAt
	}
	if params.SetCategoryID {
		// 0 clears it. The stored name and slug go with it, or a test would see an
		// entry that is uncategorised and still carries a category name.
		stored.CategoryID = params.CategoryID
		if params.CategoryID == 0 {
			stored.CategoryName, stored.CategorySlug = "", ""
		}
	}

	s.entries[params.ID] = stored
	return stored, nil
}

// SoftDelete removes an entry from view, reporting whether it found a live one.
//
// Deleted entries are dropped from the map rather than flagged. This fake has no
// column to filter on, and every method it stands in for excludes deleted rows, so
// removing them reproduces what those methods can see. The consequence a test may
// rely on is the same as in SQL: the slug is released.
func (s *Store) SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error) {
	s.LastSoftDeleteAt = at

	if s.FailSoftDelete != nil {
		return false, s.FailSoftDelete
	}

	if _, ok := s.entries[id]; !ok {
		// False, not an error. A second delete finds nothing live, which is what the
		// statement's own deleted_at filter produces.
		return false, nil
	}

	delete(s.entries, id)
	return true, nil
}

// Publish sets an entry published, stamping publishedAt only if it has none.
//
// The COALESCE from the SQL, in Go. Without the zero check here a test could not
// catch a publish that moved a first publication date.
func (s *Store) Publish(ctx context.Context, id int64, publishedAt time.Time) (entry.Entry, error) {
	if s.FailPublish != nil {
		return entry.Entry{}, s.FailPublish
	}

	stored, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, fmt.Errorf("%w: id %d", entry.ErrEntryNotFound, id)
	}

	stored.Status = entry.StatusPublished
	if stored.PublishedAt.IsZero() {
		stored.PublishedAt = publishedAt
	}

	s.entries[id] = stored
	return stored, nil
}

// Unpublish returns an entry to draft, leaving PublishedAt untouched.
func (s *Store) Unpublish(ctx context.Context, id int64) (entry.Entry, error) {
	if s.FailUnpublish != nil {
		return entry.Entry{}, s.FailUnpublish
	}

	stored, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, fmt.Errorf("%w: id %d", entry.ErrEntryNotFound, id)
	}

	stored.Status = entry.StatusDraft

	s.entries[id] = stored
	return stored, nil
}

// Archive sets an entry to archived, leaving PublishedAt untouched.
func (s *Store) Archive(ctx context.Context, id int64) (entry.Entry, error) {
	if s.FailArchive != nil {
		return entry.Entry{}, s.FailArchive
	}

	stored, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, fmt.Errorf("%w: id %d", entry.ErrEntryNotFound, id)
	}

	stored.Status = entry.StatusArchived

	s.entries[id] = stored
	return stored, nil
}

// GetByID returns one entry whatever its status.
//
// No visibility check, unlike GetPublicBySlug. That difference is the admin read's
// whole purpose, and a fake that filtered here would make the endpoint untestable.
func (s *Store) GetByID(ctx context.Context, id int64) (entry.Entry, error) {
	if s.FailGetByID != nil {
		return entry.Entry{}, s.FailGetByID
	}

	stored, ok := s.entries[id]
	if !ok {
		return entry.Entry{}, fmt.Errorf("%w: id %d", entry.ErrEntryNotFound, id)
	}
	return stored, nil
}

// ListAdmin returns entries of any status ordered as the SQL orders them:
// updated_at descending, id descending to break ties.
//
// A nil status means every status.
func (s *Store) ListAdmin(ctx context.Context, status *entry.Status, limit, offset int) ([]entry.Entry, int64, error) {
	if s.FailListAdmin != nil {
		return nil, 0, s.FailListAdmin
	}

	matching := make([]entry.Entry, 0, len(s.entries))
	for _, candidate := range s.entries {
		if status != nil && candidate.Status != *status {
			continue
		}
		matching = append(matching, candidate)
	}

	sort.Slice(matching, func(i, j int) bool {
		a, b := matching[i], matching[j]
		if !a.UpdatedAt.Equal(b.UpdatedAt) {
			return a.UpdatedAt.After(b.UpdatedAt)
		}
		return a.ID > b.ID
	})

	total := int64(len(matching))

	if offset >= len(matching) {
		return []entry.Entry{}, total, nil
	}
	end := offset + limit
	if end > len(matching) {
		end = len(matching)
	}

	page := make([]entry.Entry, end-offset)
	copy(page, matching[offset:end])

	return page, total, nil
}

// Seed inserts an entry directly, bypassing the service, so a test can arrange
// rows it did not create through Create.
func (s *Store) Seed(e entry.Entry) entry.Entry {
	if e.ID == 0 {
		e.ID = s.nextID
		s.nextID++
	} else if e.ID >= s.nextID {
		s.nextID = e.ID + 1
	}
	s.entries[e.ID] = e
	return e
}

// Checked at compile time: the fake must satisfy the same interface as the
// repository, or a test could pass against a shape the real store does not have.
var _ entry.Store = (*Store)(nil)
