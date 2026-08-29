package taxonomy_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// The repository translates storage facts into domain facts, and that translation
// only exists against a real database: a fake returns whatever errors it was
// written to return. Three things here exist nowhere else — the unique index on
// slug, the entry count's LEFT JOIN, and ON DELETE SET NULL — and none of them can
// be checked without PostgreSQL.
//
// Skipped unless TEST_DATABASE_URL is set, and refused unless it names a database
// ending in _test. See dbtest.requireTestDatabase.
func newTestRepository(t *testing.T) (*taxonomy.Repository, *postgres.Pool) {
	t.Helper()

	pool := dbtest.Pool(t)
	dbtest.CleanupCategories(t, pool)

	return taxonomy.NewRepository(pool), pool
}

// storedCategory writes one category and fails the test if it cannot.
func storedCategory(t *testing.T, repo *taxonomy.Repository, label string) taxonomy.Category {
	t.Helper()

	created, err := repo.Create(context.Background(), taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  label,
		Slug:  dbtest.Slug(t, label),
	})
	if err != nil {
		t.Fatalf("create category %q: %v", label, err)
	}
	return created
}

func TestRepositoryCreateStoresEveryColumn(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	slug := dbtest.Slug(t, "travel")
	created, err := repo.Create(ctx, taxonomy.CreateParams{
		World:       contentworld.Journal,
		Name:        "旅行",
		Slug:        slug,
		Description: "places and the getting there",
		SortOrder:   10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == 0 {
		t.Error("the created category carries no id")
	}
	if created.Name != "旅行" {
		t.Errorf("name = %q, want 旅行", created.Name)
	}
	if created.Slug != slug {
		t.Errorf("slug = %q, want %q", created.Slug, slug)
	}
	if created.Description != "places and the getting there" {
		t.Errorf("description = %q", created.Description)
	}
	if created.SortOrder != 10 {
		t.Errorf("sort_order = %d, want 10", created.SortOrder)
	}
	// Both from the database: created_at is a column default and updated_at is a
	// trigger. Nothing in this package stamps either, which is why there is no
	// clock in the service.
	if created.CreatedAt.IsZero() {
		t.Error("created_at is zero; the column default did not fire")
	}
	if created.UpdatedAt.IsZero() {
		t.Error("updated_at is zero; the trigger did not fire")
	}
}

// TestRepositoryCreateMapsAnEmptyDescriptionToNULL covers the round trip through
// a nullable column. "" goes in, NULL is stored, "" comes back, so nothing above
// the repository needs a nil check.
func TestRepositoryCreateMapsAnEmptyDescriptionToNULL(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "杂记",
		Slug:  dbtest.Slug(t, "notes"),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Description != "" {
		t.Errorf("description = %q, want empty", created.Description)
	}

	// Asserted in the column, not only in the returned struct: an empty string
	// stored as '' rather than NULL would read back the same way here and differ in
	// every query that tests for NULL.
	var isNull bool
	if err := pool.QueryRow(ctx,
		"SELECT description IS NULL FROM categories WHERE id = $1", created.ID,
	).Scan(&isNull); err != nil {
		t.Fatalf("read description: %v", err)
	}
	if !isNull {
		t.Error("an empty description was stored as '' rather than NULL")
	}
}

// TestRepositoryCreateTranslatesTheUniqueIndex is the translation that cannot be
// faked. categories_slug_key is the actual guarantee behind the service's
// pre-check, and the client must see the same ErrSlugTaken from either path.
func TestRepositoryCreateTranslatesTheUniqueIndex(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	slug := dbtest.Slug(t, "travel")
	first := taxonomy.CreateParams{World: contentworld.Journal, Name: "旅行", Slug: slug}
	if _, err := repo.Create(ctx, first); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	_, err := repo.Create(ctx, taxonomy.CreateParams{World: contentworld.Journal, Name: "另一个", Slug: slug})
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
	// The driver's message must not survive: it carries the index name, and a
	// handler that matched on text rather than on the sentinel would break the day
	// the index is renamed.
	if errors.Is(err, taxonomy.ErrCategoryNotFound) {
		t.Errorf("a conflict was reported as not-found: %v", err)
	}
}

func TestRepositoryReadsFindTheSameRow(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	created := storedCategory(t, repo, "travel")

	byID, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	bySlug, err := repo.GetBySlug(ctx, contentworld.Journal, created.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}

	if byID.ID != created.ID || bySlug.ID != created.ID {
		t.Errorf("the two reads disagree: id %d and %d, want %d", byID.ID, bySlug.ID, created.ID)
	}
}

// TestRepositoryMissingRowsBecomeADomainError covers where pgx.ErrNoRows stops. A
// driver error reaching the handler would be answered as a 500 for a row that is
// simply absent.
func TestRepositoryMissingRowsBecomeADomainError(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	t.Run("by id", func(t *testing.T) {
		// An id no sequence will have reached.
		if _, err := repo.GetByID(ctx, 2147483000); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})
	t.Run("by slug", func(t *testing.T) {
		if _, err := repo.GetBySlug(ctx, contentworld.Journal, dbtest.Slug(t, "absent")); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})
	t.Run("update", func(t *testing.T) {
		_, err := repo.Update(ctx, taxonomy.UpdateParams{
			ID: 2147483000, SetName: true, Name: "x",
		})
		if !errors.Is(err, taxonomy.ErrCategoryNotFound) {
			t.Errorf("error = %v, want ErrCategoryNotFound", err)
		}
	})
	t.Run("delete reports a miss rather than erroring", func(t *testing.T) {
		deleted, err := repo.Delete(ctx, 2147483000)
		if err != nil {
			t.Fatalf("Delete: %v", err)
		}
		if deleted {
			t.Error("deleting an absent category reported success")
		}
	})
}

// TestRepositoryUpdateWritesOnlyFlaggedColumns is what the CASE expressions in the
// SQL are for. A statement that wrote every parameter would blank the columns the
// client never mentioned, and the unit tests cannot see that: they assert against a
// fake that reproduces the flags rather than the SQL.
func TestRepositoryUpdateWritesOnlyFlaggedColumns(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	slug := dbtest.Slug(t, "travel")
	created, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "旅行", Slug: slug,
		Description: "keep me", SortOrder: 10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := repo.Update(ctx, taxonomy.UpdateParams{
		ID: created.ID, SetName: true, Name: "远行",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Name != "远行" {
		t.Errorf("name = %q, want 远行", updated.Name)
	}
	if updated.Description != "keep me" {
		t.Errorf("description = %q; an unflagged column was written", updated.Description)
	}
	if updated.Slug != slug {
		t.Errorf("slug = %q; an unflagged column was written", updated.Slug)
	}
	if updated.SortOrder != 10 {
		t.Errorf("sort_order = %d; an unflagged column was written", updated.SortOrder)
	}
	// The trigger moved updated_at, which is how anything above here knows a write
	// happened without the domain stamping a time.
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("updated_at did not move: %v then %v", created.UpdatedAt, updated.UpdatedAt)
	}
}

// TestRepositoryUpdateClearsADescription is the case COALESCE cannot express. With
// COALESCE in the statement a submitted "" would be read as "leave it alone", and
// the description could never be removed.
func TestRepositoryUpdateClearsADescription(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "旅行", Slug: dbtest.Slug(t, "travel"), Description: "remove me",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := repo.Update(ctx, taxonomy.UpdateParams{
		ID: created.ID, SetDescription: true, Description: "",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Description != "" {
		t.Errorf("description = %q, want it cleared", updated.Description)
	}

	var isNull bool
	if err := pool.QueryRow(ctx,
		"SELECT description IS NULL FROM categories WHERE id = $1", created.ID,
	).Scan(&isNull); err != nil {
		t.Fatalf("read description: %v", err)
	}
	if !isNull {
		t.Error("a cleared description was stored as '' rather than NULL")
	}
}

// TestRepositoryUpdateTranslatesTheUniqueIndex is the conflict on the update path.
func TestRepositoryUpdateTranslatesTheUniqueIndex(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	first := storedCategory(t, repo, "travel")
	second := storedCategory(t, repo, "notes")

	_, err := repo.Update(ctx, taxonomy.UpdateParams{
		ID: second.ID, SetSlug: true, Slug: first.Slug,
	})
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
}

// TestRepositorySlugChecksAreScopedCorrectly covers the pair of existence queries,
// including the exclusion an update depends on: a category resubmitting its own
// slug must not be refused as a conflict with itself.
func TestRepositorySlugChecksAreScopedCorrectly(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	created := storedCategory(t, repo, "travel")

	t.Run("SlugExists finds a taken slug", func(t *testing.T) {
		taken, err := repo.SlugExists(ctx, contentworld.Journal, created.Slug)
		if err != nil {
			t.Fatalf("SlugExists: %v", err)
		}
		if !taken {
			t.Error("a stored slug reported free")
		}
	})

	t.Run("SlugExists reports a free slug", func(t *testing.T) {
		taken, err := repo.SlugExists(ctx, contentworld.Journal, dbtest.Slug(t, "unused"))
		if err != nil {
			t.Fatalf("SlugExists: %v", err)
		}
		if taken {
			t.Error("an unused slug reported taken")
		}
	})

	t.Run("SlugExistsExcluding ignores the category itself", func(t *testing.T) {
		taken, err := repo.SlugExistsExcluding(ctx, contentworld.Journal, created.Slug, created.ID)
		if err != nil {
			t.Fatalf("SlugExistsExcluding: %v", err)
		}
		if taken {
			t.Error("a category conflicts with itself, so it can never be edited")
		}
	})

	t.Run("SlugExistsExcluding still finds another category", func(t *testing.T) {
		other := storedCategory(t, repo, "notes")
		taken, err := repo.SlugExistsExcluding(ctx, contentworld.Journal, created.Slug, other.ID)
		if err != nil {
			t.Fatalf("SlugExistsExcluding: %v", err)
		}
		if !taken {
			t.Error("another category's slug reported free")
		}
	})
}

// TestRepositoryListsOrderBySortOrderThenID covers the ORDER BY both list
// statements share. Written out of order, so passing means the SQL produced the
// order rather than the insert sequence happening to match.
func TestRepositoryListsOrderBySortOrderThenID(t *testing.T) {
	repo, _ := newTestRepository(t)
	ctx := context.Background()

	// sort_order 20 first, then two at 10 whose tie the id has to break.
	last, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "last", Slug: dbtest.Slug(t, "last"), SortOrder: 20,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	firstAt10, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "first at ten", Slug: dbtest.Slug(t, "firstten"), SortOrder: 10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	secondAt10, err := repo.Create(ctx, taxonomy.CreateParams{
		World: contentworld.Journal,
		Name:  "second at ten", Slug: dbtest.Slug(t, "secondten"), SortOrder: 10,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	want := []int64{firstAt10.ID, secondAt10.ID, last.ID}

	t.Run("List", func(t *testing.T) {
		categories, err := repo.List(ctx, contentworld.Journal)
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		assertOrder(t, want, ids(categories))
	})

	t.Run("ListWithCounts", func(t *testing.T) {
		counted, err := repo.ListWithCounts(ctx, contentworld.Journal)
		if err != nil {
			t.Fatalf("ListWithCounts: %v", err)
		}
		plain := make([]taxonomy.Category, 0, len(counted))
		for _, c := range counted {
			plain = append(plain, c.Category)
		}
		assertOrder(t, want, ids(plain))
	})
}

// ids returns the ids of the categories this process created, in the order the
// query returned them.
//
// Filtered by prefix rather than compared to the whole result, because another
// package may be running against the same database and its categories would
// otherwise appear between these.
func ids(categories []taxonomy.Category) []int64 {
	out := make([]int64, 0, len(categories))
	for _, c := range categories {
		if len(c.Slug) >= len(dbtest.Prefix()) && c.Slug[:len(dbtest.Prefix())] == dbtest.Prefix() {
			out = append(out, c.ID)
		}
	}
	return out
}

func assertOrder(t *testing.T, want, got []int64) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("got %d categories, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %d, want %d (full order %v, want %v)", i, got[i], want[i], got, want)
		}
	}
}

// TestRepositoryEntryCountsMatchWhatAReaderCanSee is the test the public category
// list turns on, and it cannot be written against a fake: the number comes from a
// LEFT JOIN with the visibility filter in its ON clause.
//
// A count that included drafts would advertise "旅行 (5)" and open onto two
// entries. Each hidden case differs from the visible one in exactly one respect, so
// a filter missing one condition fails here rather than in production.
func TestRepositoryEntryCountsMatchWhatAReaderCanSee(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	// Entries need an author, and author_id is ON DELETE RESTRICT.
	dbtest.CleanupUsers(t, pool)
	owner, err := auth.NewRepository(pool).CreateUser(ctx, auth.CreateUserParams{
		Username:     dbtest.Username(t, "catowner"),
		PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	category := storedCategory(t, repo, "travel")
	empty := storedCategory(t, repo, "empty")

	entries := entry.NewRepository(pool)
	seed := func(label string, status entry.Status, visibility entry.Visibility) {
		t.Helper()

		params := entry.CreateParams{
			AuthorID:   owner.ID,
			CategoryID: category.ID,
			World:      contentworld.Journal,
			Kind:       "",
			Title:      label,
			Slug:       dbtest.Slug(t, label),
			ContentMD:  "x",
			Status:     status,
			Visibility: visibility,
			Meta:       entry.DefaultMeta(),
		}
		// entries_published_at_check: a published row must carry a publication time.
		// The service stamps this; here the params go straight to the repository.
		if status == entry.StatusPublished {
			params.PublishedAt = time.Now()
		}

		if _, err := entries.Create(ctx, params); err != nil {
			t.Fatalf("create %s entry: %v", label, err)
		}
	}

	seed("visible", entry.StatusPublished, entry.VisibilityPublic)
	seed("draft", entry.StatusDraft, entry.VisibilityPublic)
	seed("archived", entry.StatusArchived, entry.VisibilityPublic)
	seed("private", entry.StatusPublished, entry.VisibilityPrivate)
	// Unlisted is excluded too, so the count matches the list that opening the
	// category shows rather than exceeding it by the number of unlisted entries.
	seed("unlisted", entry.StatusPublished, entry.VisibilityUnlisted)

	counted, err := repo.ListWithCounts(ctx, contentworld.Journal)
	if err != nil {
		t.Fatalf("ListWithCounts: %v", err)
	}

	counts := map[int64]int64{}
	for _, c := range counted {
		counts[c.ID] = c.EntryCount
	}

	if counts[category.ID] != 1 {
		t.Errorf("entry_count = %d, want 1: only the published public entry counts",
			counts[category.ID])
	}
	// A LEFT JOIN, not an INNER one. An empty category has to appear with a count of
	// zero, or a new category would be invisible until something was filed under it.
	if got, present := counts[empty.ID]; !present || got != 0 {
		t.Errorf("the empty category is missing from the list or miscounted: present=%v count=%d",
			present, got)
	}
}

// TestRepositoryDeleteUncategorisesItsEntries is the decision "直接删，文章变未分类"
// as the schema enforces it. Three outcomes were possible here and only one is
// correct: refuse the delete (RESTRICT), delete the entries with it (CASCADE), or
// leave them uncategorised (SET NULL).
//
// This is the test that would catch a migration written with the wrong one, and
// nothing above the database can substitute for it: the service never sees the
// entries change.
func TestRepositoryDeleteUncategorisesItsEntries(t *testing.T) {
	repo, pool := newTestRepository(t)
	ctx := context.Background()

	dbtest.CleanupUsers(t, pool)
	owner, err := auth.NewRepository(pool).CreateUser(ctx, auth.CreateUserParams{
		Username:     dbtest.Username(t, "delowner"),
		PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	category := storedCategory(t, repo, "doomed")

	entries := entry.NewRepository(pool)
	filed, err := entries.Create(ctx, entry.CreateParams{
		AuthorID:   owner.ID,
		CategoryID: category.ID,
		World:      contentworld.Journal,
		Kind:       "",
		Title:      "survives its category",
		Slug:       dbtest.Slug(t, "survivor"),
		ContentMD:  "x",
		Status:     entry.StatusPublished,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
		// Required by entries_published_at_check for a published row.
		PublishedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}

	deleted, err := repo.Delete(ctx, category.ID)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if !deleted {
		t.Fatal("Delete reported no category removed")
	}

	// The entry is still there, which rules out CASCADE.
	after, err := entries.GetByID(ctx, filed.ID)
	if err != nil {
		t.Fatalf("the entry did not survive its category: %v", err)
	}
	// And it is uncategorised, which rules out a dangling id.
	if after.CategoryID != 0 {
		t.Errorf("category_id = %d, want 0: the entry still points at a deleted category",
			after.CategoryID)
	}
	if after.CategoryName != "" {
		t.Errorf("category_name = %q, want empty", after.CategoryName)
	}
}

// TestRepositoryRejectsAnUnknownCategoryOnAnEntry is the other side of the same
// foreign key, checked from the entry repository because that is where the
// violation surfaces. The FK is the guarantee rather than a read-first check, so
// what matters is that it arrives as a client error and not a 500.
func TestRepositoryRejectsAnUnknownCategoryOnAnEntry(t *testing.T) {
	_, pool := newTestRepository(t)
	ctx := context.Background()

	dbtest.CleanupUsers(t, pool)
	owner, err := auth.NewRepository(pool).CreateUser(ctx, auth.CreateUserParams{
		Username:     dbtest.Username(t, "fkowner"),
		PasswordHash: "x",
	})
	if err != nil {
		t.Fatalf("create author: %v", err)
	}

	_, err = entry.NewRepository(pool).Create(ctx, entry.CreateParams{
		AuthorID:   owner.ID,
		CategoryID: 2147483000,
		World:      contentworld.Journal,
		Kind:       "",
		Title:      "filed under nothing",
		Slug:       dbtest.Slug(t, "nocategory"),
		ContentMD:  "x",
		Status:     entry.StatusDraft,
		Visibility: entry.VisibilityPublic,
		Meta:       entry.DefaultMeta(),
	})

	// entry.ErrUnknownCategory, not taxonomy.ErrCategoryNotFound: entry does not
	// import taxonomy, and the two errors mean different things to their handlers.
	// This one is a 400 naming category_id; the other is a 404.
	if !errors.Is(err, entry.ErrUnknownCategory) {
		t.Fatalf("error = %v, want entry.ErrUnknownCategory", err)
	}
}
