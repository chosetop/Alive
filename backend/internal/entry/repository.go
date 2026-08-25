package entry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

// Repository reads and writes entries.
//
// This file is the only place in the package that imports pgx. Above it,
// everything sees domain types and the sentinel errors from model.go, so the
// driver could be replaced without touching the service.
type Repository struct {
	q *sqlcgen.Queries
}

// NewRepository wires a repository to a connection pool.
func NewRepository(pool *postgres.Pool) *Repository {
	return &Repository{q: sqlcgen.New(pool)}
}

// PostgreSQL error codes translated below.
const (
	uniqueViolation     = "23505"
	checkViolation      = "23514"
	foreignKeyViolation = "23503"
)

// CreateParams is a validated entry ready to be written.
//
// The service builds this. By the time it arrives the slug format is checked, the
// word count is computed and PublishedAt is set or deliberately absent: this
// layer makes no decisions.
type CreateParams struct {
	AuthorID int64

	// CategoryID is 0 for an uncategorised entry, which becomes NULL.
	CategoryID int64

	Type        Type
	Title       string
	Slug        string
	Summary     string
	ContentMD   string
	CoverURL    string
	Status      Status
	Visibility  Visibility
	Meta        Meta
	WordCount   int
	HappenedAt  time.Time
	PublishedAt time.Time
}

// UpdateParams is a validated partial change ready to be written.
//
// Each field is paired with a Set flag. The flag is not redundant with a zero
// value: for the three nullable columns, "clear this" and "leave this alone" are
// both an absent value, and only the flag tells them apart. The service sets
// these; this layer writes exactly what it is given.
//
// No Status. Publishing writes published_at under a rule that Publish owns, and a
// status field here would be a second way to reach it.
type UpdateParams struct {
	ID int64

	SetType bool
	Type    Type

	SetTitle bool
	Title    string

	SetSlug bool
	Slug    string

	SetSummary bool
	Summary    string

	// SetContentMD covers WordCount too: the count is derived from the body, so the
	// two are written together or not at all.
	SetContentMD bool
	ContentMD    string
	WordCount    int

	SetCoverURL bool
	CoverURL    string

	SetVisibility bool
	Visibility    Visibility

	SetMeta bool
	Meta    Meta

	SetHappenedAt bool
	HappenedAt    time.Time

	// SetCategoryID with CategoryID 0 is what makes an entry uncategorised again.
	// Without the flag that would be indistinguishable from leaving the category
	// alone, which is the reason every field here is a pair.
	SetCategoryID bool
	CategoryID    int64
}

// Create inserts an entry and returns it as stored.
//
// The returned value comes from the database rather than from the input, so the
// caller sees the real created_at and the id.
func (r *Repository) Create(ctx context.Context, params CreateParams) (Entry, error) {
	row, err := r.q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
		AuthorID:    params.AuthorID,
		CategoryID:  optionalInt64(params.CategoryID),
		Type:        string(params.Type),
		Title:       params.Title,
		Slug:        params.Slug,
		Summary:     optionalString(params.Summary),
		ContentMd:   params.ContentMD,
		CoverUrl:    optionalString(params.CoverURL),
		Status:      string(params.Status),
		Visibility:  string(params.Visibility),
		Meta:        params.Meta.ForStorage().raw(),
		WordCount:   int32(params.WordCount),
		HappenedAt:  optionalTime(params.HappenedAt),
		PublishedAt: optionalTime(params.PublishedAt),
	})
	if err != nil {
		return Entry{}, translateWriteError("create entry", err, params.Slug)
	}

	// No category name or slug: RETURNING sees only the inserted row. A caller that
	// needs them reads the entry back.
	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		Type: row.Type, Title: row.Title, Slug: row.Slug, Summary: row.Summary,
		ContentMD: row.ContentMd, CoverURL: row.CoverUrl, Status: row.Status,
		Visibility: row.Visibility, Meta: row.Meta, WordCount: row.WordCount,
		HappenedAt: row.HappenedAt, PublishedAt: row.PublishedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// GetPublicBySlug returns the published, public entry with this slug.
//
// A draft, a private entry and a soft deleted one are all ErrEntryNotFound, and
// so is a slug that was never used. The filter is in the SQL, so this cannot be
// bypassed by a caller forgetting a condition.
func (r *Repository) GetPublicBySlug(ctx context.Context, slug string) (Entry, error) {
	row, err := r.q.GetPublicEntryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: %s", ErrEntryNotFound, slug)
		}
		return Entry{}, fmt.Errorf("entry: get public entry by slug: %w", err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		CategoryName: row.CategoryName, CategorySlug: row.CategorySlug,
		Type: row.Type, Title: row.Title, Slug: row.Slug, Summary: row.Summary,
		ContentMD: row.ContentMd, CoverURL: row.CoverUrl, Status: row.Status,
		Visibility: row.Visibility, Meta: row.Meta, WordCount: row.WordCount,
		HappenedAt: row.HappenedAt, PublishedAt: row.PublishedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// GetLinkBySlug returns the entry with this slug if it is published and either
// public or unlisted.
//
// The read behind a shared link, and the only one that accepts unlisted. Separate
// from GetPublicBySlug rather than a flag on it, matching the two SQL statements:
// the lists and counts must stay public-only, and a boolean would make that
// depend on an argument.
//
// Unlisted is not access control. A slug is guessable, so this keeps an entry out
// of lists and search engines and nothing more.
func (r *Repository) GetLinkBySlug(ctx context.Context, slug string) (Entry, error) {
	row, err := r.q.GetLinkEntryBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: %s", ErrEntryNotFound, slug)
		}
		return Entry{}, fmt.Errorf("entry: get link entry by slug: %w", err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		CategoryName: row.CategoryName, CategorySlug: row.CategorySlug,
		Type: row.Type, Title: row.Title, Slug: row.Slug, Summary: row.Summary,
		ContentMD: row.ContentMd, CoverURL: row.CoverUrl, Status: row.Status,
		Visibility: row.Visibility, Meta: row.Meta, WordCount: row.WordCount,
		HappenedAt: row.HappenedAt, PublishedAt: row.PublishedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// ListPublic returns one page of published, public entries, newest happening
// first, along with the total under the same filter.
//
// Two queries, and deliberately not one with a window function: count(*) OVER ()
// yields nothing when no row does, so an offset past the end would report a total
// of zero and the client could not tell that from an empty site.
//
// The entries have no ContentMD. The query does not select it, so a list response
// cannot accidentally carry six bodies.
//
// categoryID of 0 means every category. It is an id rather than a slug because the
// service resolves the slug first: that way an unknown category is a 404 saying the
// URL is wrong, not an empty list saying the category is empty. Both queries take
// the same filter, so the total always describes the list beside it.
func (r *Repository) ListPublic(ctx context.Context, categoryID int64, limit, offset int) ([]Entry, int64, error) {
	filter := optionalInt64(categoryID)

	rows, err := r.q.ListPublicEntries(ctx, sqlcgen.ListPublicEntriesParams{
		CategoryID: filter,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("entry: list public entries: %w", err)
	}

	total, err := r.q.CountPublicEntries(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("entry: count public entries: %w", err)
	}

	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		// No ContentMD: this query does not select it, so it stays "" here.
		entries = append(entries, entryFromRow(rowFields{
			ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
			CategoryName: row.CategoryName, CategorySlug: row.CategorySlug,
			Type: row.Type, Title: row.Title, Slug: row.Slug, Summary: row.Summary,
			CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
			Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
			PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}

	return entries, total, nil
}

// SlugExists reports whether a live entry holds this slug.
//
// Drafts count: one holds its slug so that publishing it later cannot collide
// with something written in the meantime. Soft deleted rows do not, matching the
// partial unique index.
func (r *Repository) SlugExists(ctx context.Context, slug string) (bool, error) {
	exists, err := r.q.EntrySlugExists(ctx, slug)
	if err != nil {
		return false, fmt.Errorf("entry: check slug: %w", err)
	}
	return exists, nil
}

// SlugExistsExcluding reports whether a live entry other than excludedID holds
// this slug.
//
// The exclusion is what lets an update resubmit an entry's own slug: without it,
// every save that included the unchanged slug would be refused as a conflict with
// itself.
func (r *Repository) SlugExistsExcluding(ctx context.Context, slug string, excludedID int64) (bool, error) {
	exists, err := r.q.EntrySlugExistsExcluding(ctx, sqlcgen.EntrySlugExistsExcludingParams{
		Slug:       slug,
		ExcludedID: excludedID,
	})
	if err != nil {
		return false, fmt.Errorf("entry: check slug excluding %d: %w", excludedID, err)
	}
	return exists, nil
}

// Update writes a partial change and returns the entry as stored.
//
// ErrEntryNotFound when no live entry has this id: the statement filters on
// deleted_at, so a soft deleted entry is not updatable and reports the same as one
// that never existed.
func (r *Repository) Update(ctx context.Context, params UpdateParams) (Entry, error) {
	row, err := r.q.UpdateEntry(ctx, sqlcgen.UpdateEntryParams{
		ID:            params.ID,
		SetType:       params.SetType,
		Type:          string(params.Type),
		SetTitle:      params.SetTitle,
		Title:         params.Title,
		SetSlug:       params.SetSlug,
		Slug:          params.Slug,
		SetSummary:    params.SetSummary,
		Summary:       optionalString(params.Summary),
		SetContentMd:  params.SetContentMD,
		ContentMd:     params.ContentMD,
		WordCount:     int32(params.WordCount),
		SetCoverUrl:   params.SetCoverURL,
		CoverUrl:      optionalString(params.CoverURL),
		SetVisibility: params.SetVisibility,
		Visibility:    string(params.Visibility),
		SetMeta:       params.SetMeta,
		Meta:          params.Meta.ForStorage().raw(),
		SetHappenedAt: params.SetHappenedAt,
		HappenedAt:    optionalTime(params.HappenedAt),
		SetCategoryID: params.SetCategoryID,
		CategoryID:    optionalInt64(params.CategoryID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, params.ID)
		}
		return Entry{}, translateWriteError("update entry", err, params.Slug)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		Type: row.Type, Title: row.Title,
		Slug: row.Slug, Summary: row.Summary, ContentMD: row.ContentMd,
		CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
		Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// SoftDelete marks an entry deleted, reporting whether it found a live one.
//
// False rather than an error for an id that is absent or already deleted: which of
// those two it was is not knowable from here, and the caller answers 404 either
// way. The statement's own deleted_at filter is what stops a second call from
// moving the timestamp.
func (r *Repository) SoftDelete(ctx context.Context, id int64, at time.Time) (bool, error) {
	_, err := r.q.SoftDeleteEntry(ctx, sqlcgen.SoftDeleteEntryParams{
		ID:        id,
		DeletedAt: &at,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("entry: soft delete entry %d: %w", id, err)
	}
	return true, nil
}

// Publish sets an entry published, stamping publishedAt only if it has none.
//
// One statement, not a read followed by a write. entries_published_at_check
// refuses a published row with a NULL publication time, so the two changes cannot
// be separated, and COALESCE in the SQL is what keeps a first publication date
// from moving on a second publish.
func (r *Repository) Publish(ctx context.Context, id int64, publishedAt time.Time) (Entry, error) {
	row, err := r.q.PublishEntry(ctx, sqlcgen.PublishEntryParams{
		ID:          id,
		PublishedAt: &publishedAt,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
		}
		return Entry{}, fmt.Errorf("entry: publish entry %d: %w", id, err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		Type: row.Type, Title: row.Title,
		Slug: row.Slug, Summary: row.Summary, ContentMD: row.ContentMd,
		CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
		Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// Unpublish returns an entry to draft, leaving published_at where it is.
func (r *Repository) Unpublish(ctx context.Context, id int64) (Entry, error) {
	row, err := r.q.UnpublishEntry(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
		}
		return Entry{}, fmt.Errorf("entry: unpublish entry %d: %w", id, err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		Type: row.Type, Title: row.Title,
		Slug: row.Slug, Summary: row.Summary, ContentMD: row.ContentMd,
		CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
		Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// Archive sets an entry to archived, leaving published_at where it is.
func (r *Repository) Archive(ctx context.Context, id int64) (Entry, error) {
	row, err := r.q.ArchiveEntry(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
		}
		return Entry{}, fmt.Errorf("entry: archive entry %d: %w", id, err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		Type: row.Type, Title: row.Title,
		Slug: row.Slug, Summary: row.Summary, ContentMD: row.ContentMd,
		CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
		Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// GetByID returns one live entry whatever its status, body included.
//
// The admin detail read. Addressed by id rather than slug because a draft is
// edited before its slug is settled, and because the slug may change while the
// thing being edited does not.
func (r *Repository) GetByID(ctx context.Context, id int64) (Entry, error) {
	row, err := r.q.GetAdminEntryByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Entry{}, fmt.Errorf("%w: id %d", ErrEntryNotFound, id)
		}
		return Entry{}, fmt.Errorf("entry: get entry %d: %w", id, err)
	}

	return entryFromRow(rowFields{
		ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
		CategoryName: row.CategoryName, CategorySlug: row.CategorySlug,
		Type: row.Type, Title: row.Title,
		Slug: row.Slug, Summary: row.Summary, ContentMD: row.ContentMd,
		CoverURL: row.CoverUrl, Status: row.Status, Visibility: row.Visibility,
		Meta: row.Meta, WordCount: row.WordCount, HappenedAt: row.HappenedAt,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}), nil
}

// ListAdmin returns one page of live entries of any status, newest edit first,
// with the total under the same filter.
//
// status nil means every status. Two queries rather than a window function, for
// the reason given on ListPublic.
//
// No ContentMD: the query does not select it.
func (r *Repository) ListAdmin(ctx context.Context, status *Status, limit, offset int) ([]Entry, int64, error) {
	var filter *string
	if status != nil {
		s := string(*status)
		filter = &s
	}

	rows, err := r.q.ListAdminEntries(ctx, sqlcgen.ListAdminEntriesParams{
		Status: filter,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("entry: list admin entries: %w", err)
	}

	total, err := r.q.CountAdminEntries(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("entry: count admin entries: %w", err)
	}

	entries := make([]Entry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, entryFromRow(rowFields{
			ID: row.ID, AuthorID: row.AuthorID, CategoryID: row.CategoryID,
			CategoryName: row.CategoryName, CategorySlug: row.CategorySlug,
			Type: row.Type, Title: row.Title,
			Slug: row.Slug, Summary: row.Summary, CoverURL: row.CoverUrl,
			Status: row.Status, Visibility: row.Visibility, Meta: row.Meta,
			WordCount: row.WordCount, HappenedAt: row.HappenedAt,
			PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}))
	}

	return entries, total, nil
}

// rowFields is the shape every entry-returning query produces.
//
// sqlc generates a distinct row type per query with identical fields, so without
// this the same nineteen-field conversion would be written once per query and a
// new column would have to be added to each copy. Values arrive in the generated
// types' own form; entryFromRow does the domain conversion in one place.
//
// CategoryName and CategorySlug are absent from the write queries' row types,
// because RETURNING cannot join. Those callers leave them nil, which converts to
// "" — the same as an uncategorised entry. The distinction is documented on
// Entry; it is not one this struct can carry.
type rowFields struct {
	ID           int64
	AuthorID     int64
	CategoryID   *int64
	CategoryName *string
	CategorySlug *string
	Type         string
	Title        string
	Slug         string
	Summary      *string
	ContentMD    string
	CoverURL     *string
	Status       string
	Visibility   string
	Meta         json.RawMessage
	WordCount    int32
	HappenedAt   *time.Time
	PublishedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// entryFromRow converts a queried row into a domain Entry.
func entryFromRow(row rowFields) Entry {
	return Entry{
		ID:           row.ID,
		AuthorID:     row.AuthorID,
		CategoryID:   derefInt64(row.CategoryID),
		CategoryName: derefString(row.CategoryName),
		CategorySlug: derefString(row.CategorySlug),
		Type:         Type(row.Type),
		Title:        row.Title,
		Slug:         row.Slug,
		Summary:      derefString(row.Summary),
		ContentMD:    row.ContentMD,
		CoverURL:     derefString(row.CoverURL),
		Status:       Status(row.Status),
		Visibility:   Visibility(row.Visibility),
		Meta:         Meta(row.Meta),
		WordCount:    int(row.WordCount),
		HappenedAt:   derefTime(row.HappenedAt),
		PublishedAt:  derefTime(row.PublishedAt),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

// translateWriteError turns a driver error from a write into a domain error.
//
// A constraint violation is not an internal failure: the database is reporting
// that the request was invalid. Passing these up as-is would make every one of
// them a 500, and a duplicate slug would look like a broken server rather than a
// client that needs to pick another slug.
//
// op names the operation for the messages that are not domain errors. Both the
// insert and the update reach here, and "create entry: ..." on a failed update
// would send a reader looking at the wrong statement.
func translateWriteError(op string, err error, slug string) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return fmt.Errorf("entry: %s: %w", op, err)
	}

	switch pgErr.Code {
	case uniqueViolation:
		// uk_entries_slug is the only unique index on the table.
		return fmt.Errorf("%w: %s", ErrSlugTaken, slug)

	case checkViolation:
		// The service validates each of these before reaching here, so a violation
		// means the two definitions have drifted apart. Named individually because
		// the constraint name is the fastest route to which one.
		switch pgErr.ConstraintName {
		case "entries_slug_format_check":
			return fmt.Errorf("%w: %s", ErrInvalidSlug, slug)
		case "entries_type_check":
			return ErrInvalidType
		case "entries_status_check", "entries_published_at_check":
			return ErrInvalidStatus
		case "entries_visibility_check":
			return ErrInvalidVisibility
		}
		return fmt.Errorf("entry: %s: check %q failed: %w", op, pgErr.ConstraintName, err)

	case foreignKeyViolation:
		// Two foreign keys reach here, and they mean opposite things.
		if pgErr.ConstraintName == "entries_category_id_fkey" {
			// The client chose a category that does not exist, or one deleted between
			// the form loading and the save. A client error: the id came from the
			// request, and the answer is to pick another category.
			return ErrUnknownCategory
		}
		// author_id points at no user. Not a client error: the author comes from
		// the authenticated session, so this means the account was deleted between
		// authentication and this insert.
		return fmt.Errorf("entry: %s: unknown author: %w", op, err)
	}

	return fmt.Errorf("entry: %s: %w", op, err)
}

// raw exposes Meta's bytes to the generated code, which wants json.RawMessage.
func (m Meta) raw() []byte { return []byte(m) }

// Conversions between domain zero values and SQL NULL. The domain uses "" and a
// zero time.Time for absent values; the database uses NULL.

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

// optionalInt64 and derefInt64 do the same for category_id, where the domain's
// absent value is 0. No category has id 0, so the mapping is unambiguous.
func optionalInt64(n int64) *int64 {
	if n == 0 {
		return nil
	}
	return &n
}

func derefInt64(n *int64) int64 {
	if n == nil {
		return 0
	}
	return *n
}

func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}
