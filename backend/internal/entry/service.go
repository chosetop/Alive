package entry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
)

// Store is the storage the service needs.
//
// Declared next to the code that calls it, not next to the implementation. A
// method appears here only because something above it uses that method, and the
// service can be tested against a fake without a database.
type Store interface {
	Create(ctx context.Context, params CreateParams) (Entry, error)
	GetPublicByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error)

	// GetLinkByWorldSlug reads a published entry that is public or unlisted.
	//
	// Separate from GetPublicByWorldSlug rather than a flag on it. The lists below stay
	// public-only, and a boolean deciding whether unlisted counts would put that
	// guarantee in the hands of whoever passes the argument.
	GetLinkByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error)

	// ListPublic returns one page of public entries. categoryID 0 means every
	// category; it is an id because the service resolves a slug before calling.
	ListPublic(ctx context.Context, world contentworld.Key, categoryID int64, limit, offset int) ([]Entry, int64, error)

	SlugExists(ctx context.Context, world contentworld.Key, slug string) (bool, error)

	// Update applies a partial change. Which fields to write is decided by the
	// service and carried in the params, not inferred here.
	Update(ctx context.Context, params UpdateParams) (Entry, error)

	// SoftDelete marks an entry deleted and reports whether it found a live one.
	SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error)

	// Publish sets status to published, stamping publishedAt only if the entry has
	// never been published.
	Publish(ctx context.Context, id, expectedRevision int64, publishedAt time.Time) (Entry, error)

	// Unpublish returns an entry to draft, leaving its first publication time.
	Unpublish(ctx context.Context, id, expectedRevision int64) (Entry, error)

	// Archive retires an entry: off the site, still listed in admin, not deleted.
	Archive(ctx context.Context, id, expectedRevision int64) (Entry, error)

	// GetByID reads one live entry whatever its status. The admin read.
	GetByID(ctx context.Context, id int64) (Entry, error)

	// ListAdmin returns one page of live entries, any status, newest edit first,
	// optionally narrowed to a text search. A nil search is no text filter.
	// A nil status means every status.
	ListAdmin(ctx context.Context, world *contentworld.Key, categoryID int64, status *Status, search *string, limit, offset int) ([]Entry, int64, error)
	DashboardMetrics(ctx context.Context) (DashboardMetrics, error)

	// SlugExistsExcluding reports whether a live entry other than excludedID holds
	// the slug. Separate from SlugExists because an entry keeping its own slug
	// through an update is not a conflict with itself.
	SlugExistsExcluding(ctx context.Context, world contentworld.Key, slug string, excludedID int64) (bool, error)
}

// DashboardMetrics is the complete set of counters shown on the writing dashboard.
type DashboardMetrics struct {
	TotalEntries     int64
	PublishedEntries int64
	TotalWords       int64
}

// Checked at compile time so the repository cannot drift from the interface.
var _ Store = (*Repository)(nil)

// Pagination bounds for the public list.
//
// The maximum is a limit on the work one request can ask for, not a preference.
// Without it, page_size=100000 is a request to serialise the whole table.
const (
	DefaultPageSize = 20
	MaxPageSize     = 50
)

// Clock returns the current time. A field rather than time.Now inline, so a test
// can assert exactly which timestamp was written.
type Clock func() time.Time

// WorldService is the publish lifecycle dependency this package needs.
type WorldService interface {
	AllowsPublicPublish(ctx context.Context, key contentworld.Key) (bool, error)
}

// Service holds the rules about entries.
//
// No HTTP here: no status code, no request, no query string. The same methods
// back a future CLI import command.
type Service struct {
	store  Store
	worlds WorldService
	now    Clock
	log    *slog.Logger
}

// ServiceOption adjusts a Service at construction.
type ServiceOption func(*Service)

// WithClock overrides the time source. For tests.
func WithClock(c Clock) ServiceOption {
	return func(s *Service) {
		if c != nil {
			s.now = c
		}
	}
}

// WithLogger sets where non-fatal problems are reported.
func WithLogger(l *slog.Logger) ServiceOption {
	return func(s *Service) {
		if l != nil {
			s.log = l
		}
	}
}

// WithWorldService sets the lifecycle policy used during public publication.
func WithWorldService(worlds WorldService) ServiceOption {
	return func(s *Service) {
		s.worlds = worlds
	}
}

// NewService builds a Service over store.
func NewService(store Store, opts ...ServiceOption) *Service {
	s := &Service{store: store, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateParams carries a create request as the caller states it.
//
// Separate from the repository's CreateParams of the same name in a different
// package: this one is what a client asked for, the other is what will be
// written. Between them sit validation, the word count and the publication
// timestamp.
type CreateInput struct {
	AuthorID int64

	// CategoryID is 0 for uncategorised, which is a normal state rather than a
	// missing field. Not validated against the categories table here: the foreign
	// key does that, and a read first would still be racing a delete. The
	// repository translates the violation to ErrUnknownCategory.
	CategoryID int64

	World      contentworld.Key
	Title      string
	Slug       string
	Summary    string
	ContentMD  string
	CoverURL   string
	Status     Status
	Visibility Visibility
	Meta       Meta
	HappenedAt time.Time
}

// Create validates an entry and stores it.
//
// Order matters. Validation comes first so a malformed request costs no query.
// The slug check comes next, because a conflict is the likely failure and saying
// so plainly beats surfacing a constraint violation.
func (s *Service) Create(ctx context.Context, in CreateInput) (Entry, error) {
	if err := s.validateCreate(&in); err != nil {
		return Entry{}, err
	}

	// Checked before the insert so that the common conflict produces a clear
	// error. This is not the guarantee: two concurrent creates can both read
	// false, and the partial unique index settles it. The repository translates
	// that violation to the same ErrSlugTaken, so both paths agree.
	if in.Slug != "" {
		taken, err := s.store.SlugExists(ctx, in.World, in.Slug)
		if err != nil {
			return Entry{}, err
		}
		if taken {
			return Entry{}, fmt.Errorf("%w: %s", ErrSlugTaken, in.Slug)
		}
	}

	created, err := s.store.Create(ctx, CreateParams{
		AuthorID:   in.AuthorID,
		CategoryID: in.CategoryID,
		World:      in.World,
		Kind:       "",
		Title:      in.Title,
		Slug:       in.Slug,
		Summary:    in.Summary,
		ContentMD:  in.ContentMD,
		CoverURL:   in.CoverURL,
		Status:     StatusDraft,
		Visibility: in.Visibility,
		Meta:       in.Meta.ForStorage(),
		// Computed here, not in SQL: what counts as a word is a domain rule, and
		// it is computed once on write so no list query has to read the body.
		WordCount:   CountWords(in.ContentMD),
		HappenedAt:  in.HappenedAt,
		PublishedAt: time.Time{},
	})
	if err != nil {
		// The pre-check said the slug was free and the index disagreed, so two
		// creates raced and this one lost. Worth a log line: it is the only
		// evidence that the race is real rather than theoretical, and the client
		// sees the same conflict either way.
		if errors.Is(err, ErrSlugTaken) {
			s.logger().WarnContext(ctx, "slug conflict detected by the unique index, not the pre-check",
				slog.String("slug", in.Slug),
			)
		}
		return Entry{}, err
	}

	return created, nil
}

// UpdateInput carries a partial change: which fields to touch, and their new
// values.
//
// Every field is a pointer, and nil means "not submitted, leave it alone". This
// is the whole reason the type differs from CreateInput: with value fields there
// is no way to tell "clear the summary" from "do not touch the summary", and one
// of those two would have to be impossible to express.
//
// A pointer to a zero value is a submitted zero. *Summary pointing at "" clears
// the summary; *HappenedAt pointing at a zero time.Time clears the date. Neither
// is treated as absent.
//
// Status is absent by design. Publishing carries the "published_at is written
// once" rule, and Publish owns it; accepting status here would put that rule in
// two places. The handler refuses a status field rather than ignoring it, so a
// client trying to publish this way is told instead of silently failing.
type UpdateInput struct {
	ExpectedRevision int64

	Title      *string
	Slug       *string
	Summary    *string
	ContentMD  *string
	CoverURL   *string
	Visibility *Visibility
	Meta       *Meta
	HappenedAt *time.Time

	// CategoryID pointing at 0 removes the category, making the entry uncategorised.
	// That is the submitted-zero rule again: 0 is a value here, and nil is the way
	// to leave the category as it is.
	CategoryID *int64
}

// IsEmpty reports whether the input asks for no change at all.
func (in UpdateInput) IsEmpty() bool {
	return in.Title == nil &&
		in.Slug == nil &&
		in.Summary == nil &&
		in.ContentMD == nil &&
		in.CoverURL == nil &&
		in.Visibility == nil &&
		in.Meta == nil &&
		in.HappenedAt == nil &&
		in.CategoryID == nil
}

// Update applies a partial change to one entry.
//
// Only submitted fields are validated. Validating the whole entry would mean
// reading it first and re-checking values that were already accepted, and it
// would report errors about fields the client did not send.
func (s *Service) Update(ctx context.Context, id int64, in UpdateInput) (Entry, error) {
	if id <= 0 {
		return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}
	if in.IsEmpty() {
		// Refused rather than treated as a successful no-op. An empty body is a
		// client that built its request wrongly, and answering 200 would hide that
		// while the edit silently failed to save.
		return Entry{}, ErrNoUpdateFields
	}
	if in.ExpectedRevision < 1 {
		return Entry{}, ErrVersionConflict
	}

	if err := s.validateUpdate(in); err != nil {
		return Entry{}, err
	}
	current, err := s.store.GetByID(ctx, id)
	if err != nil {
		return Entry{}, err
	}

	params := UpdateParams{ID: id, ExpectedRevision: in.ExpectedRevision}
	if in.Title != nil {
		params.SetTitle, params.Title = true, *in.Title
	}
	if in.Slug != nil {
		// Checked against every other live entry, excluding this one: submitting an
		// entry's own slug back is a no-op, not a conflict with itself. As in Create
		// this is not the guarantee, only the clear error; the partial unique index
		// settles a race and the repository reports it as the same ErrSlugTaken.
		if *in.Slug != "" {
			taken, err := s.store.SlugExistsExcluding(ctx, current.World, *in.Slug, id)
			if err != nil {
				return Entry{}, err
			}
			if taken {
				return Entry{}, fmt.Errorf("%w: %s", ErrSlugTaken, *in.Slug)
			}
		}
		params.SetSlug, params.Slug = true, *in.Slug
	}
	if in.Summary != nil {
		params.SetSummary, params.Summary = true, *in.Summary
	}
	if in.ContentMD != nil {
		// The word count travels with the body and is recomputed here. Leaving the
		// old count beside a new body would misreport reading time, and accepting a
		// count from the client would let the two disagree.
		params.SetContentMD, params.ContentMD = true, *in.ContentMD
		params.WordCount = CountWords(*in.ContentMD)
	}
	if in.CoverURL != nil {
		params.SetCoverURL, params.CoverURL = true, *in.CoverURL
	}
	if in.Visibility != nil {
		params.SetVisibility, params.Visibility = true, *in.Visibility
	}
	if in.Meta != nil {
		params.SetMeta, params.Meta = true, in.Meta.ForStorage()
	}
	if in.HappenedAt != nil {
		params.SetHappenedAt, params.HappenedAt = true, *in.HappenedAt
	}
	if in.CategoryID != nil {
		params.SetCategoryID, params.CategoryID = true, *in.CategoryID
	}

	updated, err := s.store.Update(ctx, params)
	if err != nil {
		if errors.Is(err, ErrSlugTaken) && in.Slug != nil {
			s.logger().WarnContext(ctx, "slug conflict detected by the unique index, not the pre-check",
				slog.String("slug", *in.Slug),
				slog.Int64("entry_id", id),
			)
		}
		return Entry{}, err
	}

	return updated, nil
}

// validateUpdate checks the fields that were submitted, and only those.
//
// No defaults are applied. A default is what an absent field means at creation;
// here an absent field means "unchanged", so filling one in would rewrite a value
// the client did not mention.
func (s *Service) validateUpdate(in UpdateInput) error {
	if in.Visibility != nil && !in.Visibility.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidVisibility, *in.Visibility)
	}
	if in.Title != nil {
		if err := validateDraftTitle(*in.Title); err != nil {
			return err
		}
	}
	if in.Slug != nil {
		if err := validateDraftSlug(*in.Slug); err != nil {
			return fmt.Errorf("%w: %s", err, *in.Slug)
		}
	}
	if in.Meta != nil {
		if err := in.Meta.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// SoftDelete marks one entry deleted.
//
// Not idempotent in its result, and deliberately so: the second call reports
// ErrEntryNotFound because there is no longer a live entry with that id. Reporting
// success would require either moving deleted_at, which loses when the delete
// happened, or looking the row up again to distinguish the cases. The handler
// answers 404, which is the truth about a resource that is no longer there.
func (s *Service) SoftDelete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}

	deleted, err := s.store.SoftDelete(ctx, id, s.now())
	if err != nil {
		return err
	}
	if !deleted {
		return fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}

	// Worth a line in the log. This is the only operation that removes something
	// from the site, and the row itself records only when, not that a request asked.
	s.logger().InfoContext(ctx, "entry soft deleted", slog.Int64("entry_id", id))

	return nil
}

// Publish makes one entry published.
//
// The first publication time is stamped only when there is none. An entry
// published, withdrawn and published again keeps its original date, so a fix to a
// three-year-old entry does not reappear at the top of the feed. The store does
// this in one statement, because entries_published_at_check refuses a published
// row with no publication time and a two-step write would fail it.
//
// Publishing an already published entry is not an error: the end state is the one
// asked for, and the timestamp does not move.
func (s *Service) Publish(ctx context.Context, id, expectedRevision int64) (Entry, error) {
	if id <= 0 {
		return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}
	if expectedRevision < 1 {
		return Entry{}, ErrVersionConflict
	}

	current, err := s.store.GetByID(ctx, id)
	if err != nil {
		return Entry{}, err
	}
	if current.Revision != expectedRevision {
		return Entry{}, ErrVersionConflict
	}
	if err := ValidateForPublish(current); err != nil {
		return Entry{}, err
	}
	if current.Visibility == VisibilityPublic && s.worlds != nil {
		allowed, err := s.worlds.AllowsPublicPublish(ctx, current.World)
		if err != nil {
			return Entry{}, err
		}
		if !allowed {
			return Entry{}, fmt.Errorf("%w: %s", ErrWorldNotOpen, current.World)
		}
	}

	published, err := s.store.Publish(ctx, id, expectedRevision, s.now())
	if err != nil {
		return Entry{}, err
	}

	s.logger().InfoContext(ctx, "entry published",
		slog.Int64("entry_id", published.ID),
		slog.String("slug", published.Slug),
	)

	return published, nil
}

// Unpublish returns one entry to draft.
//
// published_at is left alone. It records that the entry was once public, which
// withdrawing does not undo, and clearing it would make a later re-publication
// look like a first one.
func (s *Service) Unpublish(ctx context.Context, id, expectedRevision int64) (Entry, error) {
	if id <= 0 {
		return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}
	if expectedRevision < 1 {
		return Entry{}, ErrVersionConflict
	}

	drafted, err := s.store.Unpublish(ctx, id, expectedRevision)
	if err != nil {
		return Entry{}, err
	}

	s.logger().InfoContext(ctx, "entry unpublished",
		slog.Int64("entry_id", drafted.ID),
		slog.String("slug", drafted.Slug),
	)

	return drafted, nil
}

// Archive retires one entry.
//
// Three states rather than two, because "unfinished" and "finished but withdrawn"
// are different things and the admin list treats them differently: drafts are work
// still to do. Distinct from a soft delete, which takes the entry out of every
// list including the admin one.
//
// published_at survives, as it does for Unpublish, and for the same reason.
func (s *Service) Archive(ctx context.Context, id, expectedRevision int64) (Entry, error) {
	if id <= 0 {
		return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}
	if expectedRevision < 1 {
		return Entry{}, ErrVersionConflict
	}

	archived, err := s.store.Archive(ctx, id, expectedRevision)
	if err != nil {
		return Entry{}, err
	}

	s.logger().InfoContext(ctx, "entry archived",
		slog.Int64("entry_id", archived.ID),
		slog.String("slug", archived.Slug),
	)

	return archived, nil
}

// GetByID returns one live entry whatever its status.
//
// The admin read. Its caller is behind authentication, which is why this may
// answer with a draft where GetPublicByWorldSlug may not.
func (s *Service) GetByID(ctx context.Context, id int64) (Entry, error) {
	if id <= 0 {
		return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
	}
	return s.store.GetByID(ctx, id)
}

// ListAdmin returns one page of entries of any status, newest edit first,
// optionally narrowed by a free-text query.
//
// status filters when given and is validated: an unknown status would otherwise
// match nothing and look like an empty site rather than a typo.
//
// query is trimmed, and a query that is empty once trimmed is dropped rather
// than passed down. The directory search field sends its value on every
// keystroke, so it also sends whitespace and the empty string as the editor
// clears it; treating those as a search for "nothing" would blank the directory
// at exactly the moment the editor expects it back. Unlike status, an unmatched
// query is not an error: no match is a legitimate answer about the collection,
// where an unknown status is a malformed request.
func (s *Service) ListAdmin(ctx context.Context, world *contentworld.Key, categoryID int64, status *Status, query string, page, pageSize int) (Page, error) {
	if status != nil && !status.Valid() {
		return Page{}, fmt.Errorf("%w: %s", ErrInvalidStatus, *status)
	}
	if world != nil {
		if err := ValidateWorld(*world); err != nil {
			return Page{}, fmt.Errorf("%w: %s", err, *world)
		}
	}

	var search *string
	if trimmed := strings.TrimSpace(query); trimmed != "" {
		search = &trimmed
	}

	page, pageSize = normalisePagination(page, pageSize)

	entries, total, err := s.store.ListAdmin(ctx, world, categoryID, status, search, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}

	return Page{
		Entries:  entries,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// DashboardMetrics reads the complete active-library counters from storage.
func (s *Service) DashboardMetrics(ctx context.Context) (DashboardMetrics, error) {
	return s.store.DashboardMetrics(ctx)
}

// validateCreate checks the input and fills in defaults, mutating in place.
//
// Defaults are applied here rather than by the caller so that every entry point
// gets the same ones. The column defaults would not be enough: an empty string
// is a value, not an absent one, so it would fail the CHECK rather than fall back.
func (s *Service) validateCreate(in *CreateInput) error {
	if in.AuthorID == 0 {
		// A wiring mistake, not a client error: the author comes from the
		// authenticated session. Reported as internal so it is not answered as a
		// bad request that a client could try to fix.
		return errors.New("entry: create requires an author")
	}

	if err := ValidateWorld(in.World); err != nil {
		return fmt.Errorf("%w: %s", err, in.World)
	}

	if in.Visibility == "" {
		in.Visibility = VisibilityPublic
	}
	if !in.Visibility.Valid() {
		return fmt.Errorf("%w: %s", ErrInvalidVisibility, in.Visibility)
	}

	if err := validateDraftTitle(in.Title); err != nil {
		return err
	}
	// Supplied by the client, never derived from the title. Deriving a slug from
	// Chinese text yields either a percent-encoded URL or a pinyin dependency in
	// the backend, and both make a presentation problem into a storage one.
	if err := validateDraftSlug(in.Slug); err != nil {
		return fmt.Errorf("%w: %s", err, in.Slug)
	}
	if err := in.Meta.Validate(); err != nil {
		return err
	}

	return nil
}

// GetPublicByWorldSlug returns one published, public entry.
//
// ErrEntryNotFound covers both "no such slug" and "exists but not readable". The
// two must be indistinguishable, or the endpoint becomes a way to discover which
// drafts exist.
func (s *Service) GetPublicByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error) {
	if err := ValidateWorld(world); err != nil {
		return Entry{}, fmt.Errorf("%w: %s", err, world)
	}
	// A slug that cannot be valid cannot match a row, so this is answered without
	// a query. The error is the same as for a well-formed slug that is absent: a
	// different one would let a client tell "malformed" from "not here", which is
	// no use to a reader and is a probing aid.
	if err := ValidateSlug(slug); err != nil {
		return Entry{}, fmt.Errorf("%w: %s", ErrEntryNotFound, slug)
	}

	return s.store.GetPublicByWorldSlug(ctx, world, slug)
}

// GetLinkByWorldSlug returns one published entry that is public or unlisted.
//
// What the detail endpoint calls, so that a shared link to an unlisted entry
// opens. Private and draft entries are still ErrEntryNotFound, for the reason on
// GetPublicByWorldSlug: telling those apart from an unused slug reveals what exists.
//
// Unlisted is reachable by anyone who has the slug, including anyone who guesses
// it. See VisibilityUnlisted: this hides an entry from lists, not from readers.
func (s *Service) GetLinkByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error) {
	if err := ValidateWorld(world); err != nil {
		return Entry{}, fmt.Errorf("%w: %s", err, world)
	}
	if err := ValidateSlug(slug); err != nil {
		return Entry{}, fmt.Errorf("%w: %s", ErrEntryNotFound, slug)
	}

	return s.store.GetLinkByWorldSlug(ctx, world, slug)
}

// Page is one page of entries plus what a client needs to walk the rest.
type Page struct {
	Entries  []Entry
	Page     int
	PageSize int
	Total    int64
}

// ListPublic returns one page of published, public entries, optionally in one
// category.
//
// Unlisted entries are absent here, which is the whole point of the value: they
// are reachable by link and by nothing else.
//
// categoryID 0 means every category. An id rather than a slug because this package
// knows nothing about the categories table: the caller resolves the slug through
// taxonomy first, and an unknown slug becomes a 404 there rather than an empty list
// here. Those are different answers — "this URL is wrong" and "this category is
// empty" — and a reader needs to tell them apart.
//
// page and pageSize are clamped rather than rejected. A page number past the end
// yields an empty page, which is the honest answer: the collection shrinks as
// entries are unpublished, so a page that existed a moment ago legitimately may
// not now, and 400 would be wrong for a client that did nothing incorrect.
func (s *Service) ListPublic(ctx context.Context, world contentworld.Key, categoryID int64, page, pageSize int) (Page, error) {
	if err := ValidateWorld(world); err != nil {
		return Page{}, fmt.Errorf("%w: %s", err, world)
	}
	page, pageSize = normalisePagination(page, pageSize)

	// A negative id is treated as no filter rather than refused: no category has
	// one, so filtering on it could only ever return nothing.
	if categoryID < 0 {
		categoryID = 0
	}

	entries, total, err := s.store.ListPublic(ctx, world, categoryID, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}

	return Page{
		Entries:  entries,
		Page:     page,
		PageSize: pageSize,
		Total:    total,
	}, nil
}

// normalisePagination clamps a requested page and size into the allowed range.
func normalisePagination(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	switch {
	case pageSize < 1:
		pageSize = DefaultPageSize
	case pageSize > MaxPageSize:
		pageSize = MaxPageSize
	}
	return page, pageSize
}

func (s *Service) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}
