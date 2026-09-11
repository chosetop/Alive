package entry_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// The repository translates storage facts into domain facts, and that
// translation only exists against a real database: a fake would return whatever
// errors it was written to return. The visibility filter is the same story, since
// it lives in the SQL.
//
// Skipped unless TEST_DATABASE_URL is set, and refused unless it names a database
// ending in _test. See requireTestDatabase.
func newTestRepository(t *testing.T) (*entry.Repository, *postgres.Pool, int64) {
	t.Helper()

	pool := dbtest.Pool(t)

	// Removes only the rows this process created. A TRUNCATE here would delete
	// what a package running in parallel is using: see the note in dbtest.
	dbtest.CleanupUsers(t, pool)

	// Entries need an author, and author_id is RESTRICT, so a real account has to
	// exist before any insert.
	owner, err := auth.NewRepository(pool).CreateUser(context.Background(), auth.CreateUserParams{
		Username:     dbtest.Username(t, "entryowner"),
		PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	return entry.NewRepository(pool), pool, owner.ID
}

// validCreate is a stored-ready entry with every required field filled.
func validCreate(authorID int64, slug string) entry.CreateParams {
	return entry.CreateParams{
		AuthorID:   authorID,
		World:      contentworld.Journal,
		Title:      "京都的春天",
		Slug:       slug,
		ContentMD:  "在鸭川边坐了一整个下午。",
		Status:     entry.StatusDraft,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
	}
}

func TestRepositoryCreate(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	happened := time.Date(2023, 4, 3, 12, 0, 0, 0, time.UTC)

	slug := dbtest.Slug(t, "kyoto-spring")
	params := validCreate(authorID, slug)
	params.Summary = "sat by the river"
	params.CoverURL = "https://cdn.example.com/a.webp"
	params.HappenedAt = happened
	params.WordCount = 12

	created, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == 0 {
		t.Error("Create returned id 0")
	}
	if created.Slug != slug {
		t.Errorf("slug = %q, want %q", created.Slug, slug)
	}
	if !created.HappenedAt.Equal(happened) {
		t.Errorf("happened_at = %v, want %v", created.HappenedAt, happened)
	}
	// A draft carries no publication time, and the domain reads that as a zero
	// value rather than a nil pointer.
	if !created.PublishedAt.IsZero() {
		t.Errorf("published_at = %v, want zero", created.PublishedAt)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created_at is zero; the database default should have filled it")
	}

	t.Run("absent text round-trips as an empty string, not a null pointer", func(t *testing.T) {
		row, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "no-optional")))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if row.Summary != "" {
			t.Errorf("summary = %q, want empty", row.Summary)
		}
		if row.CoverURL != "" {
			t.Errorf("cover_url = %q, want empty", row.CoverURL)
		}
		if !row.HappenedAt.IsZero() {
			t.Errorf("happened_at = %v, want zero", row.HappenedAt)
		}
	})
}

func TestRepositoryCreateTranslatesConstraintErrors(t *testing.T) {
	repo, pool, authorID := newTestRepository(t)
	ctx := context.Background()

	takenSlug := dbtest.Slug(t, "taken")
	if _, err := repo.Create(ctx, validCreate(authorID, takenSlug)); err != nil {
		t.Fatalf("seed: %v", err)
	}

	t.Run("a duplicate slug becomes ErrSlugTaken", func(t *testing.T) {
		// The unique index is what makes this true under concurrency, and this is
		// the path that runs when the service's pre-check has already passed.
		_, err := repo.Create(ctx, validCreate(authorID, takenSlug))
		if !errors.Is(err, entry.ErrSlugTaken) {
			t.Fatalf("Create = %v, want ErrSlugTaken", err)
		}
		// The driver's error must not escape: nothing above the repository should
		// have to know that 23505 means a unique violation.
		if strings.Contains(err.Error(), "SQLSTATE") {
			t.Errorf("error carries driver detail: %v", err)
		}
	})

	t.Run("a soft deleted row releases its slug", func(t *testing.T) {
		if _, err := pool.Exec(ctx,
			"UPDATE entries SET deleted_at = now() WHERE slug = $1", takenSlug); err != nil {
			t.Fatalf("soft delete: %v", err)
		}
		if _, err := repo.Create(ctx, validCreate(authorID, takenSlug)); err != nil {
			t.Errorf("Create after soft delete = %v, want success", err)
		}
	})

	t.Run("an unknown author is not reported as a client error", func(t *testing.T) {
		// author_id comes from the session, so this means the account went away
		// between authentication and the insert. Answering it as a bad request would
		// invite a client to fix something it does not control.
		_, err := repo.Create(ctx, validCreate(999999, dbtest.Slug(t, "orphan")))
		if err == nil {
			t.Fatal("Create succeeded with an unknown author")
		}
		for _, sentinel := range []error{
			entry.ErrSlugTaken, entry.ErrInvalidSlug, entry.ErrInvalidWorld,
			entry.ErrInvalidStatus, entry.ErrInvalidVisibility,
		} {
			if errors.Is(err, sentinel) {
				t.Errorf("error is %v, which a client would be told to fix", sentinel)
			}
		}
	})
}

// visibilityCase is one row that either must or must not be publicly readable.
//
// Each hidden case differs from the visible one in a single respect, so a filter
// missing one of its three conditions fails a named subtest rather than producing
// one vague count.
type visibilityCase struct {
	name    string
	slug    string
	status  entry.Status
	vis     entry.Visibility
	deleted bool
	visible bool
}

var repositoryVisibilityCases = []visibilityCase{
	{"published public", "visible", entry.StatusPublished, entry.VisibilityPublic, false, true},
	{"draft", "draft", entry.StatusDraft, entry.VisibilityPublic, false, false},
	{"archived", "archived", entry.StatusArchived, entry.VisibilityPublic, false, false},
	{"private", "private", entry.StatusPublished, entry.VisibilityPrivate, false, false},
	{"unlisted", "unlisted", entry.StatusPublished, entry.VisibilityUnlisted, false, false},
	{"soft deleted", "deleted", entry.StatusPublished, entry.VisibilityPublic, true, false},
}

func TestRepositoryPublicReadsHideEverythingElse(t *testing.T) {
	repo, pool, authorID := newTestRepository(t)
	ctx := context.Background()

	published := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Slugs are unique across the whole table and the list query reads the whole
	// table, so this test cannot use fixed slugs or count all the rows it sees:
	// another package's entries may be present at the same time. The slugs are
	// generated, and the assertions below look only at these.
	slugs := make(map[string]string, len(repositoryVisibilityCases))
	for _, tc := range repositoryVisibilityCases {
		slug := dbtest.Slug(t, tc.slug)
		slugs[tc.name] = slug

		params := validCreate(authorID, slug)
		params.Status = tc.status
		params.Visibility = tc.vis
		// The CHECK constraint requires a published row to carry a timestamp.
		if tc.status == entry.StatusPublished {
			params.PublishedAt = published
		}

		if _, err := repo.Create(ctx, params); err != nil {
			t.Fatalf("seed %s: %v", tc.name, err)
		}
		if tc.deleted {
			if _, err := pool.Exec(ctx,
				"UPDATE entries SET deleted_at = now() WHERE slug = $1", slug); err != nil {
				t.Fatalf("soft delete %s: %v", tc.name, err)
			}
		}
	}

	visibleSlug := slugs["published public"]

	t.Run("GetPublicByWorldSlug", func(t *testing.T) {
		for _, tc := range repositoryVisibilityCases {
			t.Run(tc.name, func(t *testing.T) {
				slug := slugs[tc.name]

				got, err := repo.GetPublicByWorldSlug(ctx, contentworld.Journal, slug)
				switch {
				case tc.visible && err != nil:
					t.Fatalf("want the entry, got %v", err)
				case tc.visible && got.Slug != slug:
					t.Errorf("slug = %q, want %q", got.Slug, slug)
				case !tc.visible && !errors.Is(err, entry.ErrEntryNotFound):
					t.Errorf("err = %v, want ErrEntryNotFound for a %s entry", err, tc.name)
				}
			})
		}
	})

	t.Run("ListPublic shows the published public entry and none of the others", func(t *testing.T) {
		entries, total, err := repo.ListPublic(ctx, contentworld.Journal, 0, 50, 0)
		if err != nil {
			t.Fatalf("ListPublic: %v", err)
		}
		if total < 1 {
			t.Fatalf("total = %d, want at least the one visible entry", total)
		}

		// Restricted to the rows this test created. A count of the whole result
		// would fail whenever another package has public entries in flight.
		seen := make(map[string]bool, len(entries))
		for _, e := range entries {
			seen[e.Slug] = true
		}

		if !seen[visibleSlug] {
			t.Errorf("the published public entry %q is absent from the list", visibleSlug)
		}
		for _, tc := range repositoryVisibilityCases {
			if tc.visible {
				continue
			}
			if seen[slugs[tc.name]] {
				t.Errorf("a %s entry appears in the public list", tc.name)
			}
		}
	})

	t.Run("the list carries no content_md", func(t *testing.T) {
		// The query does not select it, so a list response cannot accidentally ship
		// the bodies. The detail read is where the body belongs.
		entries, _, err := repo.ListPublic(ctx, contentworld.Journal, 0, 50, 0)
		if err != nil {
			t.Fatalf("ListPublic: %v", err)
		}
		for _, e := range entries {
			if e.ContentMD != "" {
				t.Errorf("content_md = %q in the list row %q, want empty", e.ContentMD, e.Slug)
				break
			}
		}

		detail, err := repo.GetPublicByWorldSlug(ctx, contentworld.Journal, visibleSlug)
		if err != nil {
			t.Fatalf("GetPublicByWorldSlug: %v", err)
		}
		if detail.ContentMD == "" {
			t.Error("content_md is empty in the detail read, want the body")
		}
	})
}

func TestRepositorySlugExists(t *testing.T) {
	repo, pool, authorID := newTestRepository(t)
	ctx := context.Background()

	heldSlug := dbtest.Slug(t, "held")
	releasedSlug := dbtest.Slug(t, "released")

	if _, err := repo.Create(ctx, validCreate(authorID, heldSlug)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := repo.Create(ctx, validCreate(authorID, releasedSlug)); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"UPDATE entries SET deleted_at = now() WHERE slug = $1", releasedSlug); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	cases := []struct {
		name string
		slug string
		want bool
	}{
		{"a draft holds its slug", heldSlug, true},
		{"a soft deleted row releases it", releasedSlug, false},
		{"an unused slug is free", dbtest.Slug(t, "never-used"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.SlugExists(ctx, contentworld.Journal, tc.slug)
			if err != nil {
				t.Fatalf("SlugExists: %v", err)
			}
			if got != tc.want {
				t.Errorf("SlugExists(%q) = %v, want %v", tc.slug, got, tc.want)
			}
		})
	}
}

func TestRepositoryReorderPublishedControlsPublicOrderWithoutTouchingEditTime(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	createPublished := func(label string) entry.Entry {
		params := validCreate(authorID, dbtest.Slug(t, label))
		params.Status = entry.StatusPublished
		params.PublishedAt = time.Now().UTC()
		created, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("create %s: %v", label, err)
		}
		return created
	}
	first := createPublished("order-first")
	second := createPublished("order-second")

	if err := repo.ReorderPublished(ctx, contentworld.Journal, []int64{first.ID, second.ID}); err != nil {
		t.Fatalf("ReorderPublished: %v", err)
	}
	ordered, err := repo.ListPublishedForOrdering(ctx, contentworld.Journal)
	if err != nil {
		t.Fatalf("ListPublishedForOrdering: %v", err)
	}
	positions := make(map[int64]int, len(ordered))
	for index, item := range ordered {
		positions[item.ID] = index
	}
	if positions[first.ID] >= positions[second.ID] {
		t.Fatalf("positions = %v, want first before second", positions)
	}

	after, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if !after.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("updated_at changed from %v to %v during reorder", first.UpdatedAt, after.UpdatedAt)
	}
}

func TestRepositoryMetaRoundTrip(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	params := validCreate(authorID, dbtest.Slug(t, "meta"))
	params.Meta = entry.Meta(`{"place":"京都","rating":5}`)

	created, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Decoded, not compared as bytes. jsonb stores a parsed structure and
	// re-serialises it on read, so spacing and key order are not preserved. Code
	// that compares raw jsonb bytes relies on formatting Postgres does not promise.
	var meta struct {
		Place  string `json:"place"`
		Rating int    `json:"rating"`
	}
	if err := json.Unmarshal(created.Meta, &meta); err != nil {
		t.Fatalf("meta is not valid JSON: %v", err)
	}
	if meta.Place != "京都" {
		t.Errorf("place = %q, want %q", meta.Place, "京都")
	}
	if meta.Rating != 5 {
		t.Errorf("rating = %d, want 5", meta.Rating)
	}

	t.Run("an absent meta is stored as an empty object", func(t *testing.T) {
		row, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "empty-meta")))
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		var probe map[string]any
		if err := json.Unmarshal(row.Meta, &probe); err != nil {
			t.Fatalf("meta is not valid JSON: %v", err)
		}
		if len(probe) != 0 {
			t.Errorf("meta = %s, want an empty object", row.Meta)
		}
	})
}

// TestRepositoryUpdateWritesOnlyFlaggedFields is what the CASE expressions in
// UpdateEntry exist for. This cannot be checked against a fake: the question is
// whether the statement leaves a column alone, and only the database can answer.
func TestRepositoryUpdateWritesOnlyFlaggedFields(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	params := validCreate(authorID, dbtest.Slug(t, "partial-update"))
	params.Summary = "the original summary"
	params.CoverURL = "https://example.test/cover.jpg"
	params.HappenedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	params.WordCount = 7

	created, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// One field flagged, everything else left off.
	updated, err := repo.Update(ctx, entry.UpdateParams{
		ID:               created.ID,
		ExpectedRevision: created.Revision,
		SetTitle:         true,
		Title:            "只改标题",
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "只改标题" {
		t.Errorf("title = %q, want the new one", updated.Title)
	}

	// The unflagged columns. A CASE branch written the wrong way round would clear
	// each of these, and the nullable three would clear silently.
	if updated.Summary != params.Summary {
		t.Errorf("summary = %q, want it untouched", updated.Summary)
	}
	if updated.CoverURL != params.CoverURL {
		t.Errorf("cover_url = %q, want it untouched", updated.CoverURL)
	}
	if !updated.HappenedAt.Equal(params.HappenedAt) {
		t.Errorf("happened_at = %v, want it untouched", updated.HappenedAt)
	}
	if updated.ContentMD != params.ContentMD {
		t.Errorf("content_md = %q, want it untouched", updated.ContentMD)
	}
	if updated.WordCount != params.WordCount {
		t.Errorf("word_count = %d, want it untouched", updated.WordCount)
	}
	if updated.Status != params.Status {
		t.Errorf("status = %q, want it untouched: this statement has no status parameter", updated.Status)
	}

	// The trigger from 000001, not something this layer writes.
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("updated_at = %v, want it moved past %v by the trigger",
			updated.UpdatedAt, created.UpdatedAt)
	}
}

// TestRepositoryUpdateClearsNullableFields is the case COALESCE could not express.
// Clearing a summary and leaving one alone are both an absent value, and the Set
// flag is the only thing that tells them apart.
func TestRepositoryUpdateClearsNullableFields(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	params := validCreate(authorID, dbtest.Slug(t, "clear-nullable"))
	params.Summary = "to be cleared"
	params.CoverURL = "https://example.test/gone.jpg"
	params.HappenedAt = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

	created, err := repo.Create(ctx, params)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Flagged, with the domain's absent value: the repository turns "" and a zero
	// time into SQL NULL.
	updated, err := repo.Update(ctx, entry.UpdateParams{
		ID:               created.ID,
		ExpectedRevision: created.Revision,
		SetSummary:       true,
		Summary:          "",
		SetCoverURL:      true,
		CoverURL:         "",
		SetHappenedAt:    true,
		HappenedAt:       time.Time{},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Summary != "" {
		t.Errorf("summary = %q, want cleared", updated.Summary)
	}
	if updated.CoverURL != "" {
		t.Errorf("cover_url = %q, want cleared", updated.CoverURL)
	}
	if !updated.HappenedAt.IsZero() {
		t.Errorf("happened_at = %v, want cleared", updated.HappenedAt)
	}
	// Not collateral damage from the three clears.
	if updated.Title != params.Title {
		t.Errorf("title = %q, want it untouched", updated.Title)
	}
}

// TestRepositoryUpdateReportsAnAbsentEntry covers the deleted_at filter in the
// WHERE. A soft deleted entry is not editable, and reports the same as an id that
// never existed.
func TestRepositoryUpdateReportsAnAbsentEntry(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "will-vanish")))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("an id that never existed", func(t *testing.T) {
		_, err := repo.Update(ctx, entry.UpdateParams{ID: 999999999, ExpectedRevision: 1, SetTitle: true, Title: "x"})
		if !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("update = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("a soft deleted entry", func(t *testing.T) {
		if _, err := repo.SoftDelete(ctx, created.ID, time.Now()); err != nil {
			t.Fatalf("soft delete: %v", err)
		}

		_, err := repo.Update(ctx, entry.UpdateParams{ID: created.ID, ExpectedRevision: created.Revision, SetTitle: true, Title: "x"})
		if !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("update = %v, want ErrEntryNotFound", err)
		}
	})
}

func TestRepositoryUpdateReportsVersionConflict(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "stale-update")))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.Update(ctx, entry.UpdateParams{
		ID:               created.ID,
		ExpectedRevision: created.Revision,
		SetTitle:         true,
		Title:            "first save",
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	_, err = repo.Update(ctx, entry.UpdateParams{
		ID:               created.ID,
		ExpectedRevision: created.Revision,
		SetTitle:         true,
		Title:            "stale save",
	})
	if !errors.Is(err, entry.ErrVersionConflict) {
		t.Fatalf("stale update = %v, want ErrVersionConflict", err)
	}
}

// TestRepositoryUpdateTranslatesASlugConflict covers the same translation the
// insert gets. The unique index is what settles a race, so both statements have to
// report it as one domain error.
func TestRepositoryUpdateTranslatesASlugConflict(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	takenSlug := dbtest.Slug(t, "already-taken")
	if _, err := repo.Create(ctx, validCreate(authorID, takenSlug)); err != nil {
		t.Fatalf("create the holder: %v", err)
	}

	mine, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "wants-the-slug")))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = repo.Update(ctx, entry.UpdateParams{ID: mine.ID, ExpectedRevision: mine.Revision, SetSlug: true, Slug: takenSlug})
	if !errors.Is(err, entry.ErrSlugTaken) {
		t.Fatalf("update = %v, want ErrSlugTaken", err)
	}
	// A driver message would tell the caller an index name; the domain error is what
	// becomes a 409.
	if strings.Contains(err.Error(), "create entry") {
		t.Errorf("the error names the wrong operation: %v", err)
	}
}

// TestRepositorySlugExistsExcluding covers the one difference from SlugExists: an
// entry does not conflict with itself.
func TestRepositorySlugExistsExcluding(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	slug := dbtest.Slug(t, "self-check")
	created, err := repo.Create(ctx, validCreate(authorID, slug))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("excluded from itself, so free", func(t *testing.T) {
		exists, err := repo.SlugExistsExcluding(ctx, contentworld.Journal, slug, created.ID)
		if err != nil {
			t.Fatalf("SlugExistsExcluding: %v", err)
		}
		if exists {
			t.Error("an entry conflicts with its own slug")
		}
	})

	t.Run("taken as far as anyone else is concerned", func(t *testing.T) {
		exists, err := repo.SlugExistsExcluding(ctx, contentworld.Journal, slug, created.ID+100000)
		if err != nil {
			t.Fatalf("SlugExistsExcluding: %v", err)
		}
		if !exists {
			t.Error("the slug reads as free to another entry")
		}
	})
}

// TestRepositorySoftDeleteReleasesTheSlug covers the consequence of uk_entries_slug
// being partial on deleted_at IS NULL. It is the reason the index is written that
// way, and nothing but the database can confirm it.
func TestRepositorySoftDeleteReleasesTheSlug(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	slug := dbtest.Slug(t, "released-on-delete")
	created, err := repo.Create(ctx, validCreate(authorID, slug))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	deleted, err := repo.SoftDelete(ctx, created.ID, time.Now())
	if err != nil {
		t.Fatalf("soft delete: %v", err)
	}
	if !deleted {
		t.Fatal("soft delete reported no live entry")
	}

	t.Run("a second delete finds nothing", func(t *testing.T) {
		// False rather than an error, and the deleted_at in the WHERE is what makes it
		// so: a repeat must not move the timestamp.
		again, err := repo.SoftDelete(ctx, created.ID, time.Now())
		if err != nil {
			t.Fatalf("soft delete: %v", err)
		}
		if again {
			t.Error("a second delete reported a live entry")
		}
	})

	t.Run("the slug is free again", func(t *testing.T) {
		exists, err := repo.SlugExists(ctx, contentworld.Journal, slug)
		if err != nil {
			t.Fatalf("SlugExists: %v", err)
		}
		if exists {
			t.Error("the slug still reads as taken")
		}

		if _, err := repo.Create(ctx, validCreate(authorID, slug)); err != nil {
			t.Errorf("create with the released slug = %v, want it accepted", err)
		}
	})

	t.Run("out of reach from every read", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("GetByID = %v, want ErrEntryNotFound", err)
		}
	})
}

// TestRepositoryPublishStampsOnce covers the COALESCE, and with it the constraint
// that makes a two-statement version impossible: entries_published_at_check refuses
// a published row with a NULL published_at.
func TestRepositoryPublishStampsOnce(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "publish-once")))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created.PublishedAt.IsZero() {
		t.Fatalf("a draft was created with published_at = %v", created.PublishedAt)
	}

	first := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	published, err := repo.Publish(ctx, created.ID, created.Revision, first)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if published.Status != entry.StatusPublished {
		t.Errorf("status = %q, want published", published.Status)
	}
	if !published.PublishedAt.Equal(first) {
		t.Errorf("published_at = %v, want %v", published.PublishedAt, first)
	}
	current := published

	t.Run("publishing again does not move the date", func(t *testing.T) {
		again, err := repo.Publish(ctx, created.ID, current.Revision, first.Add(72*time.Hour))
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if !again.PublishedAt.Equal(first) {
			t.Errorf("published_at = %v, want the original %v", again.PublishedAt, first)
		}
		current = again
	})

	t.Run("unpublish and archive keep it", func(t *testing.T) {
		drafted, err := repo.Unpublish(ctx, created.ID, current.Revision)
		if err != nil {
			t.Fatalf("unpublish: %v", err)
		}
		if drafted.Status != entry.StatusDraft {
			t.Errorf("status = %q, want draft", drafted.Status)
		}
		if !drafted.PublishedAt.Equal(first) {
			t.Errorf("published_at = %v, want the original %v", drafted.PublishedAt, first)
		}

		archived, err := repo.Archive(ctx, created.ID, drafted.Revision)
		if err != nil {
			t.Fatalf("archive: %v", err)
		}
		if archived.Status != entry.StatusArchived {
			t.Errorf("status = %q, want archived", archived.Status)
		}
		if !archived.PublishedAt.Equal(first) {
			t.Errorf("published_at = %v, want the original %v", archived.PublishedAt, first)
		}
		current = archived
	})

	t.Run("republishing after a withdrawal keeps the first date", func(t *testing.T) {
		again, err := repo.Publish(ctx, created.ID, current.Revision, first.Add(240*time.Hour))
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if !again.PublishedAt.Equal(first) {
			t.Errorf("published_at = %v, want the original %v", again.PublishedAt, first)
		}
	})
}

// TestRepositoryStateTransitionsReportAnAbsentEntry covers the deleted_at filter on
// all three transition statements.
func TestRepositoryStateTransitionsReportAnAbsentEntry(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "transitions-gone")))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := repo.SoftDelete(ctx, created.ID, time.Now()); err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	for _, tc := range []struct {
		name string
		call func(int64) error
	}{
		{"publish", func(id int64) error { _, err := repo.Publish(ctx, id, 1, time.Now()); return err }},
		{"unpublish", func(id int64) error { _, err := repo.Unpublish(ctx, id, 1); return err }},
		{"archive", func(id int64) error { _, err := repo.Archive(ctx, id, 1); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(created.ID); !errors.Is(err, entry.ErrEntryNotFound) {
				t.Errorf("%s on a deleted entry = %v, want ErrEntryNotFound", tc.name, err)
			}
			if err := tc.call(999999999); !errors.Is(err, entry.ErrEntryNotFound) {
				t.Errorf("%s on an unknown id = %v, want ErrEntryNotFound", tc.name, err)
			}
		})
	}
}

// TestRepositoryAdminReadsSeeEveryStatus is the counterpart to
// TestRepositoryPublicReadsHideEverythingElse. The same rows, the other audience:
// these statements carry no visibility filter, which is why they are separate
// statements rather than the public ones with a parameter.
func TestRepositoryAdminReadsSeeEveryStatus(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	type seeded struct {
		id     int64
		slug   string
		status entry.Status
	}

	// One row per status and one non-public visibility, so a stray filter on either
	// column loses a named case.
	wanted := []struct {
		label  string
		status entry.Status
		vis    entry.Visibility
	}{
		{"published-public", entry.StatusPublished, entry.VisibilityPublic},
		{"draft", entry.StatusDraft, entry.VisibilityPublic},
		{"archived", entry.StatusArchived, entry.VisibilityPublic},
		{"private", entry.StatusPublished, entry.VisibilityPrivate},
		{"unlisted", entry.StatusPublished, entry.VisibilityUnlisted},
	}

	var rows []seeded
	for _, tc := range wanted {
		params := validCreate(authorID, dbtest.Slug(t, tc.label))
		params.Status = tc.status
		params.Visibility = tc.vis
		if tc.status == entry.StatusPublished {
			// entries_published_at_check refuses a published row without one.
			params.PublishedAt = time.Now()
		}

		created, err := repo.Create(ctx, params)
		if err != nil {
			t.Fatalf("create %s: %v", tc.label, err)
		}
		rows = append(rows, seeded{created.ID, created.Slug, created.Status})
	}

	t.Run("detail by id reaches all of them", func(t *testing.T) {
		for _, row := range rows {
			got, err := repo.GetByID(ctx, row.id)
			if err != nil {
				t.Errorf("GetByID(%s) = %v, want the entry", row.slug, err)
				continue
			}
			// The admin detail read selects content_md, unlike either list.
			if got.ContentMD == "" {
				t.Errorf("GetByID(%s) returned no body", row.slug)
			}
		}
	})

	t.Run("the list reaches all of them", func(t *testing.T) {
		// Filtered to this process's rows by prefix, since another package may be
		// running against the same database.
		listed, total, err := repo.ListAdmin(ctx, nil, 0, nil, nil, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if total < int64(len(rows)) {
			t.Errorf("total = %d, want at least %d", total, len(rows))
		}

		found := make(map[string]bool, len(listed))
		for _, e := range listed {
			found[e.Slug] = true
			// No content_md in this query's column list, for the same reason the public
			// list omits it.
			if e.ContentMD != "" {
				t.Errorf("the admin list carries a body for %s", e.Slug)
			}
		}
		for _, row := range rows {
			if !found[row.slug] {
				t.Errorf("the admin list is missing %s (%s)", row.slug, row.status)
			}
		}
	})

	t.Run("filtered by status", func(t *testing.T) {
		draft := entry.StatusDraft
		listed, _, err := repo.ListAdmin(ctx, nil, 0, &draft, nil, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		for _, e := range listed {
			if e.Status != entry.StatusDraft {
				t.Errorf("status=draft returned a %s entry", e.Status)
			}
		}
	})

	t.Run("ordered by the last edit", func(t *testing.T) {
		// Touching the oldest row moves it to the front, which happened_at ordering
		// would not do.
		oldest := rows[0]
		current, err := repo.GetByID(ctx, oldest.id)
		if err != nil {
			t.Fatalf("read oldest entry: %v", err)
		}
		if _, err := repo.Update(ctx, entry.UpdateParams{
			ID: oldest.id, ExpectedRevision: current.Revision,
			SetTitle: true, Title: "touched last",
		}); err != nil {
			t.Fatalf("update: %v", err)
		}

		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, nil, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) == 0 {
			t.Fatal("the admin list is empty")
		}
		if listed[0].ID != oldest.id {
			t.Errorf("first id = %d, want the just-touched %d: the list is not sorted by updated_at",
				listed[0].ID, oldest.id)
		}
	})

	t.Run("deleted rows are out of reach here too", func(t *testing.T) {
		gone := rows[len(rows)-1]
		if _, err := repo.SoftDelete(ctx, gone.id, time.Now()); err != nil {
			t.Fatalf("soft delete: %v", err)
		}

		if _, err := repo.GetByID(ctx, gone.id); !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("GetByID = %v, want ErrEntryNotFound", err)
		}

		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, nil, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		for _, e := range listed {
			if e.ID == gone.id {
				t.Error("the admin list still reports a deleted entry")
			}
		}
	})
}

func TestRepositoryGetByIDIncludesTags(t *testing.T) {
	repo, pool, authorID := newTestRepository(t)
	ctx := context.Background()
	dbtest.CleanupTags(t, pool)

	created, err := repo.Create(ctx, validCreate(authorID, dbtest.Slug(t, "tagged-entry")))
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}

	tagRepo := taxonomy.NewTagRepository(pool)
	travel, err := tagRepo.Create(ctx, taxonomy.CreateTagParams{
		Name: "Travel",
		Slug: dbtest.Slug(t, "travel"),
	})
	if err != nil {
		t.Fatalf("create travel tag: %v", err)
	}
	food, err := tagRepo.Create(ctx, taxonomy.CreateTagParams{
		Name: "Food",
		Slug: dbtest.Slug(t, "food"),
	})
	if err != nil {
		t.Fatalf("create food tag: %v", err)
	}

	if _, err := pool.Exec(ctx,
		`INSERT INTO entry_tags (entry_id, tag_id) VALUES ($1, $2), ($1, $3)`,
		created.ID, travel.ID, food.ID); err != nil {
		t.Fatalf("seed entry_tags: %v", err)
	}

	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("len(tags) = %d, want 2", len(got.Tags))
	}
	if got.Tags[0].Name != "Food" || got.Tags[1].Name != "Travel" {
		t.Fatalf("tags = %#v, want Food then Travel", got.Tags)
	}
	if got.Tags[0].Slug != food.Slug || got.Tags[1].Slug != travel.Slug {
		t.Fatalf("tags slugs = %#v, want sorted by name", got.Tags)
	}
}

// TestRepositoryAdminListSearches covers the search predicate in real SQL rather
// than against the in-memory fake: ILIKE's case handling, the three searched
// columns, the NULL-means-unfiltered branch, and that the total is computed under
// the same predicate as the page.
//
// Every assertion is scoped to a token unique to this test, because another
// package may be writing to the same database. Counting "every row matching
// 'mountain'" would be counting other processes' rows too.
func TestRepositoryAdminListSearches(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	// A token no other row can contain, so a count is exact rather than "at least".
	token := dbtest.Slug(t, "needle")

	for _, tc := range []struct {
		label   string
		title   string
		summary string
		status  entry.Status
	}{
		// The token reaches each searched column in turn, so dropping any one of the
		// three ORed conditions fails a named case.
		{"in-title", "山中 " + token, "walked up", entry.StatusPublished},
		{"in-summary", "海边", "a trip past " + token, entry.StatusDraft},
		{"in-slug-only", "京都的春天", "by the river", entry.StatusDraft},
		{"no-match", "无关", "nothing relevant", entry.StatusDraft},
	} {
		params := validCreate(authorID, dbtest.Slug(t, tc.label))
		params.Title = tc.title
		params.Summary = tc.summary
		params.Status = tc.status
		if tc.label == "in-slug-only" {
			params.Slug = token + "-in-slug"
		}
		if tc.status == entry.StatusPublished {
			// entries_published_at_check refuses a published row without one.
			params.PublishedAt = time.Now()
		}
		if _, err := repo.Create(ctx, params); err != nil {
			t.Fatalf("create %s: %v", tc.label, err)
		}
	}

	t.Run("matches title, summary, and slug", func(t *testing.T) {
		listed, total, err := repo.ListAdmin(ctx, nil, 0, nil, &token, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) != 3 {
			t.Fatalf("got %d entries, want the three rows carrying the token", len(listed))
		}
		// The total is what a pagination control reads, so it must be computed under
		// the search predicate rather than over the collection.
		if total != 3 {
			t.Errorf("total = %d, want 3: the count query filters differently than the list", total)
		}
	})

	t.Run("ILIKE is case-insensitive", func(t *testing.T) {
		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, ptr(strings.ToUpper(token)), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) != 3 {
			t.Errorf("got %d entries for the uppercased query, want the same 3", len(listed))
		}
	})

	t.Run("intersects with the status filter", func(t *testing.T) {
		draft := entry.StatusDraft
		listed, total, err := repo.ListAdmin(ctx, nil, 0, &draft, &token, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		// Two of the three token rows are drafts; the third is published.
		if len(listed) != 2 || total != 2 {
			t.Fatalf("entries = %d, total = %d, want 2 and 2", len(listed), total)
		}
		for _, e := range listed {
			if e.Status != entry.StatusDraft {
				t.Errorf("%s is a %s, want the filters ANDed", e.Slug, e.Status)
			}
		}
	})

	t.Run("a nil search is not a filter", func(t *testing.T) {
		_, total, err := repo.ListAdmin(ctx, nil, 0, nil, nil, entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		// At least this test's four rows; other packages may hold more.
		if total < 4 {
			t.Errorf("total = %d, want every row when no query is given", total)
		}
	})

	t.Run("no match is an empty page, not an error", func(t *testing.T) {
		listed, total, err := repo.ListAdmin(ctx, nil, 0, nil, ptr(token+"-absent"), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) != 0 || total != 0 {
			t.Errorf("entries = %d, total = %d, want both zero", len(listed), total)
		}
	})
}

// TestRepositoryAdminListSearchTreatsQueryAsLiteralText covers the LIKE
// metacharacters, which are the one way a search field can silently answer a
// different question than the one typed.
//
// The query is interpolated into an ILIKE pattern, so `%`, `_`, and `\` arrive as
// pattern syntax rather than as text unless something escapes them. Untreated,
// `_` matches any single character and turns a one-character query into "every
// article", and a title like "读完了 80% 的书" cannot be searched for at all. None
// of this is an injection risk — the value is a bound parameter — but a directory
// that over-matches is a directory that lies.
func TestRepositoryAdminListSearchTreatsQueryAsLiteralText(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	token := dbtest.Slug(t, "literal")

	// Titles chosen so a metacharacter interpreted as a wildcard would match the
	// wrong row: "80%" and "read_me" each contain a metacharacter, and "80 percent"
	// and "readXme" are what a wildcard would wrongly reach.
	for _, tc := range []struct {
		label string
		title string
	}{
		{"percent-literal", token + " 读完了 80% 的书"},
		{"percent-decoy", token + " 读完了 80 的书"},
		{"underscore-literal", token + " read_me"},
		{"underscore-decoy", token + " readXme"},
	} {
		params := validCreate(authorID, dbtest.Slug(t, tc.label))
		params.Title = tc.title
		if _, err := repo.Create(ctx, params); err != nil {
			t.Fatalf("create %s: %v", tc.label, err)
		}
	}

	t.Run("underscore is a literal underscore, not any character", func(t *testing.T) {
		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, ptr("read_me"), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("got %d entries, want only the row containing a real underscore", len(listed))
		}
		if !strings.Contains(listed[0].Title, "read_me") {
			t.Errorf("matched %q, want the underscore row", listed[0].Title)
		}
	})

	t.Run("a bare underscore does not match everything", func(t *testing.T) {
		// The worst case: one keystroke returning the whole directory unfiltered.
		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, ptr("_"), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		for _, e := range listed {
			if !strings.Contains(e.Title, "_") && !strings.Contains(e.Slug, "_") &&
				!strings.Contains(e.Summary, "_") {
				t.Errorf("%q matched a bare underscore without containing one", e.Title)
			}
		}
	})

	t.Run("percent is a literal percent sign", func(t *testing.T) {
		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, ptr("80%"), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(listed) != 1 {
			t.Fatalf("got %d entries, want only the row containing a real percent sign", len(listed))
		}
		if !strings.Contains(listed[0].Title, "80%") {
			t.Errorf("matched %q, want the percent row", listed[0].Title)
		}
	})

	t.Run("a lone backslash matches nothing rather than erroring", func(t *testing.T) {
		// An escape character with nothing to escape is a malformed pattern in some
		// engines. It must be ordinary text here.
		listed, _, err := repo.ListAdmin(ctx, nil, 0, nil, ptr(`\`), entry.MaxPageSize, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		for _, e := range listed {
			if !strings.Contains(e.Title+e.Slug+e.Summary, `\`) {
				t.Errorf("%q matched a backslash without containing one", e.Title)
			}
		}
	})
}

// TestRepositoryAdminListSearchIgnoresTheBody pins the decision that the search
// reads three columns and not the Markdown body.
//
// Without this, adding `content_md ILIKE ...` to the predicate would break
// nothing and no test would object. The reason it is excluded: a common word
// inside a long article would rank alongside the article actually named that, and
// the directory's job is to find a known article rather than to search prose.
func TestRepositoryAdminListSearchIgnoresTheBody(t *testing.T) {
	repo, _, authorID := newTestRepository(t)
	ctx := context.Background()

	// A token that exists only in the body, in none of the searched columns.
	bodyOnly := dbtest.Slug(t, "bodyonly")

	params := validCreate(authorID, dbtest.Slug(t, "body-search"))
	params.ContentMD = "这段正文里出现了 " + bodyOnly + " 这个词。"
	if _, err := repo.Create(ctx, params); err != nil {
		t.Fatalf("create: %v", err)
	}

	listed, total, err := repo.ListAdmin(ctx, nil, 0, nil, &bodyOnly, entry.MaxPageSize, 0)
	if err != nil {
		t.Fatalf("ListAdmin: %v", err)
	}
	if len(listed) != 0 || total != 0 {
		t.Errorf("entries = %d, total = %d, want none: the search reached into content_md",
			len(listed), total)
	}
}
