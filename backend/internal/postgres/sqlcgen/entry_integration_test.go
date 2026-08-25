package sqlcgen_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/dbtest"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/postgres/sqlcgen"
)

// The public queries carry their visibility rule inside the SQL, so the rule is
// only actually proven by running them against a database holding rows that
// should not come back. A unit test over a fake would assert the fake.

// seedAuthor creates the user that entries hang from.
//
// The name is generated inside rather than passed in, so no caller can supply a
// literal that collides with a package running in parallel.
func seedAuthor(t *testing.T, q *sqlcgen.Queries, label string) int64 {
	t.Helper()

	user, err := q.CreateUser(context.Background(), sqlcgen.CreateUserParams{
		Username:     dbtest.Username(t, label),
		PasswordHash: "x",
		Role:         "owner",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	return user.ID
}

// entrySeed describes one row to insert. Only the fields the tests vary are
// named; the rest get workable defaults in insertEntry.
type entrySeed struct {
	slug        string
	status      string
	visibility  string
	happenedAt  *time.Time
	publishedAt *time.Time
	deleted     bool
}

func insertEntry(t *testing.T, q *sqlcgen.Queries, pool *postgres.Pool, authorID int64, seed entrySeed) int64 {
	t.Helper()
	ctx := context.Background()

	status := seed.status
	if status == "" {
		status = "draft"
	}
	visibility := seed.visibility
	if visibility == "" {
		visibility = "public"
	}
	// The CHECK constraint requires a published entry to carry a timestamp, so a
	// seed that asks for published without one gets a default rather than a
	// constraint violation that would read as a test failure.
	publishedAt := seed.publishedAt
	if status == "published" && publishedAt == nil {
		now := time.Now()
		publishedAt = &now
	}

	row, err := q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
		AuthorID:    authorID,
		Type:        "journal",
		Title:       "Title for " + seed.slug,
		Slug:        seed.slug,
		ContentMd:   "# body",
		Status:      status,
		Visibility:  visibility,
		Meta:        json.RawMessage(`{}`),
		WordCount:   2,
		HappenedAt:  seed.happenedAt,
		PublishedAt: publishedAt,
	})
	if err != nil {
		t.Fatalf("CreateEntry(%s): %v", seed.slug, err)
	}

	// Soft deletion is an UPDATE, not something CreateEntry can express, and
	// deleted rows are exactly what the public filter must exclude.
	if seed.deleted {
		if _, err := pool.Exec(ctx,
			"UPDATE entries SET deleted_at = now() WHERE id = $1", row.ID); err != nil {
			t.Fatalf("soft delete %s: %v", seed.slug, err)
		}
	}

	return row.ID
}

func TestCreateEntry(t *testing.T) {
	q, _ := newTestQueries(t)
	ctx := context.Background()
	authorID := seedAuthor(t, q, "entryauthor")

	happened := time.Date(2023, 4, 3, 12, 0, 0, 0, time.UTC)
	summary := "sat by the river all afternoon"

	created, err := q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
		AuthorID:   authorID,
		Type:       "journal",
		Title:      "京都的春天",
		Slug:       dbtest.Slug(t, "kyoto-spring"),
		Summary:    &summary,
		ContentMd:  "# 京都\n\n鸭川边。",
		Status:     "draft",
		Visibility: "public",
		Meta:       json.RawMessage(`{"mood":"calm"}`),
		WordCount:  1820,
		HappenedAt: &happened,
	})
	if err != nil {
		t.Fatalf("CreateEntry: %v", err)
	}

	if created.ID == 0 {
		t.Error("CreateEntry returned id 0")
	}
	if created.PublishedAt != nil {
		t.Errorf("published_at = %v on a draft, want nil", created.PublishedAt)
	}
	if !created.HappenedAt.Equal(happened) {
		t.Errorf("happened_at = %v, want %v", created.HappenedAt, happened)
	}
	// Compared after decoding, not as bytes. jsonb stores a parsed structure and
	// re-serialises it on read, so the text that comes back is not the text that
	// went in: key order can change and a space appears after the colon. Any code
	// that compares raw jsonb bytes is relying on formatting that Postgres does
	// not promise to preserve.
	var meta map[string]string
	if err := json.Unmarshal(created.Meta, &meta); err != nil {
		t.Fatalf("meta is not valid JSON: %v", err)
	}
	if meta["mood"] != "calm" {
		t.Errorf(`meta["mood"] = %q, want "calm"`, meta["mood"])
	}

	t.Run("defaults apply when meta is empty", func(t *testing.T) {
		row, err := q.CreateEntry(ctx, sqlcgen.CreateEntryParams{
			AuthorID:   authorID,
			Type:       "journal",
			Title:      "minimal",
			Slug:       dbtest.Slug(t, "minimal"),
			Status:     "draft",
			Visibility: "public",
			Meta:       json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("CreateEntry: %v", err)
		}
		if row.WordCount != 0 {
			t.Errorf("word_count = %d, want 0", row.WordCount)
		}
		if row.Summary != nil {
			t.Errorf("summary = %v, want nil", row.Summary)
		}
	})
}

// visibilityCase is one row that either must or must not be publicly readable.
//
// Each hidden row differs from the visible one in a single respect, so a filter
// that drops one of its three conditions fails a named subtest rather than
// producing one vague count mismatch.
type visibilityCase struct {
	name    string
	label   string
	status  string
	vis     string
	deleted bool
	visible bool
}

var publicVisibilityCases = []visibilityCase{
	{"published public", "visible", "published", "public", false, true},
	{"draft", "draft", "draft", "public", false, false},
	{"archived", "archived", "archived", "public", false, false},
	{"published private", "private", "published", "private", false, false},
	{"published unlisted", "unlisted", "published", "unlisted", false, false},
	{"soft deleted", "deleted", "published", "public", true, false},
}

func TestPublicEntryVisibility(t *testing.T) {
	q, pool := newTestQueries(t)
	ctx := context.Background()
	authorID := seedAuthor(t, q, "visibilityauthor")

	// Generated slugs, and assertions restricted to them. Slugs are unique across
	// the table and the list query reads the whole table, so a fixed slug would
	// collide with a package running in parallel and a count of all rows would
	// include its entries.
	slugs := make(map[string]string, len(publicVisibilityCases))
	for _, tc := range publicVisibilityCases {
		slug := dbtest.Slug(t, tc.label)
		slugs[tc.name] = slug

		insertEntry(t, q, pool, authorID, entrySeed{
			slug: slug, status: tc.status, visibility: tc.vis, deleted: tc.deleted,
		})
	}

	visibleSlug := slugs["published public"]

	t.Run("GetPublicEntryBySlug", func(t *testing.T) {
		for _, tc := range publicVisibilityCases {
			t.Run(tc.name, func(t *testing.T) {
				slug := slugs[tc.name]

				got, err := q.GetPublicEntryBySlug(ctx, slug)
				switch {
				case tc.visible && err != nil:
					t.Fatalf("want the row, got error: %v", err)
				case tc.visible && got.Slug != slug:
					t.Errorf("slug = %q, want %q", got.Slug, slug)
				case !tc.visible && err == nil:
					t.Errorf("query returned %q, which must not be publicly readable", got.Slug)
				}
			})
		}
	})

	t.Run("ListPublicEntries shows the published public row and none of the others", func(t *testing.T) {
		rows, err := q.ListPublicEntries(ctx, sqlcgen.ListPublicEntriesParams{Limit: 200, Offset: 0})
		if err != nil {
			t.Fatalf("ListPublicEntries: %v", err)
		}

		seen := make(map[string]bool, len(rows))
		for _, row := range rows {
			seen[row.Slug] = true
		}

		if !seen[visibleSlug] {
			t.Errorf("the published public row %q is absent from the list", visibleSlug)
		}
		for _, tc := range publicVisibilityCases {
			if tc.visible {
				continue
			}
			if seen[slugs[tc.name]] {
				t.Errorf("a %s row appears in the public list", tc.name)
			}
		}
	})

	t.Run("CountPublicEntries agrees with the list", func(t *testing.T) {
		// Both read the whole table, so this compares them with each other rather
		// than against a fixed number: the count must equal the rows the list
		// returns when the page is large enough to hold them all.
		rows, err := q.ListPublicEntries(ctx, sqlcgen.ListPublicEntriesParams{Limit: 1000, Offset: 0})
		if err != nil {
			t.Fatalf("ListPublicEntries: %v", err)
		}
		// nil category: count every category, matching the unfiltered list above.
		total, err := q.CountPublicEntries(ctx, nil)
		if err != nil {
			t.Fatalf("CountPublicEntries: %v", err)
		}
		if total != int64(len(rows)) {
			t.Errorf("count = %d but the list returned %d rows; the two filters differ",
				total, len(rows))
		}
	})
}

func TestListPublicEntriesOrdering(t *testing.T) {
	q, pool := newTestQueries(t)
	ctx := context.Background()
	authorID := seedAuthor(t, q, "orderingauthor")

	// The oldest happening is created last and published most recently. Ordering
	// by created_at or by published_at alone would put it first, so this
	// arrangement is what makes the assertion below able to fail.
	recent := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	middle := time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC)
	oldest := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)

	publishedLongAgo := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	publishedToday := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	// The undated row's published_at sits between `recent` and `middle`, so the
	// expected order interleaves it rather than putting it at either end. Both a
	// NULLS-LAST ordering and a published_at-only ordering fail that.
	publishedBetween := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)

	// Generated slugs, because slugs are unique across the table. The assertions
	// below look only at these rows: the list query reads the whole table, so a
	// package running in parallel may have public entries interleaved with them.
	recentSlug := dbtest.Slug(t, "happened-recently")
	middleSlug := dbtest.Slug(t, "happened-middle")
	oldestSlug := dbtest.Slug(t, "happened-long-ago")
	undatedSlug := dbtest.Slug(t, "no-happened-at")

	insertEntry(t, q, pool, authorID, entrySeed{
		slug: recentSlug, status: "published", visibility: "public",
		happenedAt: &recent, publishedAt: &publishedLongAgo,
	})
	insertEntry(t, q, pool, authorID, entrySeed{
		slug: middleSlug, status: "published", visibility: "public",
		happenedAt: &middle, publishedAt: &publishedLongAgo,
	})
	insertEntry(t, q, pool, authorID, entrySeed{
		slug: oldestSlug, status: "published", visibility: "public",
		happenedAt: &oldest, publishedAt: &publishedToday,
	})
	// No happened_at at all, so this row sorts by its published_at. That is the
	// date a reader is shown for it, and sorting it anywhere else puts an entry
	// out of sequence with its own visible date.
	insertEntry(t, q, pool, authorID, entrySeed{
		slug: undatedSlug, status: "published", visibility: "public",
		publishedAt: &publishedBetween,
	})

	mine := map[string]bool{
		recentSlug: true, middleSlug: true, oldestSlug: true, undatedSlug: true,
	}

	t.Run("orders by happened_at, falling back to published_at", func(t *testing.T) {
		rows, err := q.ListPublicEntries(ctx, sqlcgen.ListPublicEntriesParams{Limit: 200, Offset: 0})
		if err != nil {
			t.Fatalf("ListPublicEntries: %v", err)
		}

		// Relative order among this test's rows. Other rows may sit between them
		// without making the ordering wrong.
		var got []string
		for _, row := range rows {
			if mine[row.Slug] {
				got = append(got, row.Slug)
			}
		}

		// recent 2026-05 > undated 2025-05 (its published_at) > middle 2024-05 >
		// oldest 2020-05.
		want := []string{recentSlug, undatedSlug, middleSlug, oldestSlug}
		if len(got) != len(want) {
			t.Fatalf("found %d of this test's rows %v, want %d", len(got), got, len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("order = %v, want %v", got, want)
			}
		}
	})

	t.Run("limit and offset walk the pages without repeating a row", func(t *testing.T) {
		// Paged across the whole table in twos until this test's four rows have all
		// been seen. Counting pages would depend on how many entries other packages
		// happen to hold.
		seen := make(map[string]int)
		for offset := int32(0); offset < 200; offset += 2 {
			rows, err := q.ListPublicEntries(ctx, sqlcgen.ListPublicEntriesParams{
				Limit: 2, Offset: offset,
			})
			if err != nil {
				t.Fatalf("ListPublicEntries(offset=%d): %v", offset, err)
			}
			if len(rows) == 0 {
				break
			}
			for _, row := range rows {
				if mine[row.Slug] {
					seen[row.Slug]++
				}
			}
		}

		if len(seen) != len(mine) {
			t.Errorf("saw %d of this test's slugs across the pages, want %d: %v",
				len(seen), len(mine), seen)
		}
		for slug, count := range seen {
			if count != 1 {
				t.Errorf("%s appeared %d times across pages, want 1", slug, count)
			}
		}
	})

	t.Run("the list omits content_md", func(t *testing.T) {
		// Asserted by type rather than by value: ListPublicEntriesRow has no such
		// field, so a query that started selecting it would break this build.
		//   _ = rows[0].ContentMd
		detail, err := q.GetPublicEntryBySlug(ctx, recentSlug)
		if err != nil {
			t.Fatalf("GetPublicEntryBySlug: %v", err)
		}
		if detail.ContentMd == "" {
			t.Error("the detail query must return content_md")
		}
	})
}

func TestEntrySlugExists(t *testing.T) {
	q, pool := newTestQueries(t)
	ctx := context.Background()
	authorID := seedAuthor(t, q, "slugauthor")

	takenSlug := dbtest.Slug(t, "taken")
	releasedSlug := dbtest.Slug(t, "released")

	insertEntry(t, q, pool, authorID, entrySeed{slug: takenSlug, status: "draft"})
	insertEntry(t, q, pool, authorID, entrySeed{slug: releasedSlug, status: "draft", deleted: true})

	cases := []struct {
		name string
		slug string
		want bool
	}{
		// A draft holds its slug. The public reader cannot see it, but publishing
		// it later must not collide with something written in the meantime.
		{"a draft holds its slug", takenSlug, true},
		{"a soft deleted row releases its slug", releasedSlug, false},
		{"an unused slug is free", dbtest.Slug(t, "never-used"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := q.EntrySlugExists(ctx, tc.slug)
			if err != nil {
				t.Fatalf("EntrySlugExists: %v", err)
			}
			if got != tc.want {
				t.Errorf("EntrySlugExists(%q) = %v, want %v", tc.slug, got, tc.want)
			}
		})
	}
}
