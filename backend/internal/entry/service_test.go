package entry_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/entry/entrytest"
)

// fixedTime is the clock every test uses, so an assertion about published_at
// compares against a known value rather than a window around now.
var fixedTime = time.Date(2026, 8, 24, 15, 4, 5, 0, time.UTC)

func newTestService(t *testing.T) (*entry.Service, *entrytest.Store) {
	t.Helper()

	store := entrytest.NewStore()
	service := entry.NewService(store, entry.WithClock(func() time.Time { return fixedTime }))
	return service, store
}

// validInput is a create request with every required field filled, for tests
// that vary one thing at a time.
func validInput() entry.CreateInput {
	return entry.CreateInput{
		AuthorID:  1,
		Title:     "京都的春天",
		Slug:      "kyoto-spring",
		ContentMD: "在鸭川边坐了一整个下午。",
	}
}

func TestCreateDefaults(t *testing.T) {
	service, store := newTestService(t)

	created, err := service.Create(context.Background(), validInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Draft by default, and this is the important one. An unwanted draft is
	// invisible; an unwanted publication is already in the feed and the RSS reader.
	if created.Status != entry.StatusDraft {
		t.Errorf("status = %q, want %q", created.Status, entry.StatusDraft)
	}
	if created.Type != entry.TypeJournal {
		t.Errorf("type = %q, want %q", created.Type, entry.TypeJournal)
	}
	if created.Visibility != entry.VisibilityPublic {
		t.Errorf("visibility = %q, want %q", created.Visibility, entry.VisibilityPublic)
	}
	if string(store.LastCreate.Meta) != "{}" {
		t.Errorf("meta = %q, want %q", store.LastCreate.Meta, "{}")
	}

	// A draft has never been published, so it carries no publication time. If this
	// were stamped now, the first edit after publishing would already have the
	// wrong date.
	if !created.PublishedAt.IsZero() {
		t.Errorf("published_at = %v on a draft, want zero", created.PublishedAt)
	}

	if store.LastCreate.WordCount != entry.CountWords(validInput().ContentMD) {
		t.Errorf("word_count = %d, want %d",
			store.LastCreate.WordCount, entry.CountWords(validInput().ContentMD))
	}
}

func TestCreateIncompleteDraft(t *testing.T) {
	service, store := newTestService(t)

	created, err := service.Create(context.Background(), entry.CreateInput{AuthorID: 1})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != entry.StatusDraft {
		t.Errorf("status = %q, want draft", created.Status)
	}
	if created.Title != "" || created.Slug != "" {
		t.Errorf("created = %+v, want an incomplete draft", created)
	}
	if store.SlugExistsCalls != 0 {
		t.Errorf("empty draft checked its slug %d times", store.SlugExistsCalls)
	}
}

func TestCreateAllowsMultipleEmptySlugDrafts(t *testing.T) {
	service, _ := newTestService(t)

	for i := 0; i < 2; i++ {
		created, err := service.Create(context.Background(), entry.CreateInput{AuthorID: 1})
		if err != nil {
			t.Fatalf("Create empty-slug draft %d: %v", i+1, err)
		}
		if created.Slug != "" {
			t.Errorf("slug = %q, want empty", created.Slug)
		}
	}
}

func TestUpdateAcceptsIncompleteDraftFields(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Revision: 1, Status: entry.StatusDraft})

	updated, err := service.Update(context.Background(), 7, entry.UpdateInput{
		ExpectedRevision: 1,
		Title:            ptr(""),
		Slug:             ptr(""),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "" || updated.Slug != "" {
		t.Errorf("updated = %+v, want empty draft fields", updated)
	}
	if store.SlugExistsExcludingCalls != 0 {
		t.Errorf("empty draft slug checked %d times", store.SlugExistsExcludingCalls)
	}
}

func TestUpdateClearsSlugBesideAnEmptySlugDraft(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Revision: 1, Status: entry.StatusDraft})
	store.Seed(entry.Entry{ID: 8, Revision: 1, Slug: "second", Status: entry.StatusDraft})

	updated, err := service.Update(context.Background(), 8, entry.UpdateInput{
		ExpectedRevision: 1,
		Slug:             ptr(""),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Slug != "" {
		t.Errorf("slug = %q, want cleared", updated.Slug)
	}
}

func TestPublishIncomplete(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Revision: 1, Status: entry.StatusDraft, Visibility: entry.VisibilityPublic})

	_, err := service.Publish(context.Background(), 7, 1)
	if !errors.Is(err, entry.ErrInvalidTitle) {
		t.Fatalf("Publish = %v, want ErrInvalidTitle", err)
	}
	got, getErr := store.GetByID(context.Background(), 7)
	if getErr != nil {
		t.Fatalf("GetByID: %v", getErr)
	}
	if got.Status != entry.StatusDraft {
		t.Errorf("status = %q, want publish validation to leave it draft", got.Status)
	}
}

func TestUpdateVersionConflict(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Revision: 1, Status: entry.StatusDraft})
	store.FailUpdate = entry.ErrVersionConflict

	_, err := service.Update(context.Background(), 7, entry.UpdateInput{
		ExpectedRevision: 1,
		Summary:          ptr("save this"),
	})
	if !errors.Is(err, entry.ErrVersionConflict) {
		t.Fatalf("Update = %v, want ErrVersionConflict", err)
	}
}

func TestCreateAlwaysStoresADraft(t *testing.T) {
	service, store := newTestService(t)

	in := validInput()
	in.Status = entry.StatusPublished

	created, err := service.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.Status != entry.StatusDraft {
		t.Errorf("status = %q, want draft", created.Status)
	}
	if !created.PublishedAt.IsZero() || !store.LastCreate.PublishedAt.IsZero() {
		t.Errorf("published_at = %v, want zero on a newly created draft", created.PublishedAt)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*entry.CreateInput)
		wantErr error
	}{
		{"over-long title", func(in *entry.CreateInput) {
			in.Title = strings.Repeat("a", entry.MaxTitleLength+1)
		}, entry.ErrInvalidTitle},

		{"uppercase slug", func(in *entry.CreateInput) { in.Slug = "Kyoto" }, entry.ErrInvalidSlug},
		{"chinese slug", func(in *entry.CreateInput) { in.Slug = "京都" }, entry.ErrInvalidSlug},
		{"slug with space", func(in *entry.CreateInput) { in.Slug = "kyoto spring" }, entry.ErrInvalidSlug},

		{"unknown type", func(in *entry.CreateInput) { in.Type = "joural" }, entry.ErrInvalidType},
		{"unknown visibility", func(in *entry.CreateInput) { in.Visibility = "hidden" }, entry.ErrInvalidVisibility},

		{"meta is an array", func(in *entry.CreateInput) { in.Meta = entry.Meta(`[1]`) }, entry.ErrInvalidMeta},
		{"meta is malformed", func(in *entry.CreateInput) { in.Meta = entry.Meta(`{`) }, entry.ErrInvalidMeta},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newTestService(t)

			in := validInput()
			tc.mutate(&in)

			_, err := service.Create(context.Background(), in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Create = %v, want %v", err, tc.wantErr)
			}

			// Validation runs before any storage work, so a malformed request costs
			// no round trip.
			if store.CreateCalls != 0 {
				t.Errorf("Create reached the store %d times, want 0", store.CreateCalls)
			}
			if store.SlugExistsCalls != 0 {
				t.Errorf("SlugExists ran %d times, want 0", store.SlugExistsCalls)
			}
		})
	}
}

func TestCreateSlugConflict(t *testing.T) {
	service, store := newTestService(t)
	ctx := context.Background()

	if _, err := service.Create(ctx, validInput()); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	t.Run("the pre-check reports the conflict", func(t *testing.T) {
		_, err := service.Create(ctx, validInput())
		if !errors.Is(err, entry.ErrSlugTaken) {
			t.Fatalf("Create = %v, want ErrSlugTaken", err)
		}
		if store.CreateCalls != 1 {
			t.Errorf("Create reached the store %d times, want 1 (the pre-check should stop the second)",
				store.CreateCalls)
		}
	})

	t.Run("a draft holds its slug", func(t *testing.T) {
		// The draft is invisible to a reader but still owns the slug, so publishing
		// it later cannot collide with something written in the meantime.
		in := validInput()
		in.Slug = "held-by-a-draft"
		in.Status = entry.StatusDraft
		if _, err := service.Create(ctx, in); err != nil {
			t.Fatalf("Create: %v", err)
		}

		second := validInput()
		second.Slug = "held-by-a-draft"
		if _, err := service.Create(ctx, second); !errors.Is(err, entry.ErrSlugTaken) {
			t.Errorf("Create = %v, want ErrSlugTaken", err)
		}
	})

	t.Run("a conflict raised by the write is reported the same way", func(t *testing.T) {
		// Two concurrent creates can both read a free slug, and the unique index
		// refuses the second write. The loser must get the same error as one caught
		// by the pre-check, or one situation would produce two different responses
		// depending on timing.
		fresh := entrytest.NewStore()
		service := entry.NewService(fresh, entry.WithClock(func() time.Time { return fixedTime }))

		fresh.Seed(entry.Entry{Slug: "raced-slug", Status: entry.StatusDraft})

		// The pre-check is made to miss, so the refusal comes from the write. This
		// is the path that a seeded row alone would never reach.
		fresh.SlugExistsAlwaysFree = true

		in := validInput()
		in.Slug = "raced-slug"

		_, err := service.Create(ctx, in)
		if !errors.Is(err, entry.ErrSlugTaken) {
			t.Fatalf("Create = %v, want ErrSlugTaken", err)
		}
		if fresh.CreateCalls != 1 {
			t.Errorf("Create reached the store %d times, want 1: the write must be what refused it",
				fresh.CreateCalls)
		}
	})
}

func TestCreateRequiresAuthor(t *testing.T) {
	service, store := newTestService(t)

	in := validInput()
	in.AuthorID = 0

	// Not one of the input errors: the author comes from the session, so a zero
	// here is a wiring mistake rather than something a client sent.
	_, err := service.Create(context.Background(), in)
	if err == nil {
		t.Fatal("Create succeeded with no author, want an error")
	}
	for _, sentinel := range []error{
		entry.ErrInvalidTitle, entry.ErrInvalidSlug, entry.ErrInvalidType,
	} {
		if errors.Is(err, sentinel) {
			t.Errorf("error is %v, which would be answered as a bad request", sentinel)
		}
	}
	if store.CreateCalls != 0 {
		t.Errorf("Create reached the store %d times, want 0", store.CreateCalls)
	}
}

func TestGetPublicBySlug(t *testing.T) {
	service, store := newTestService(t)
	ctx := context.Background()

	store.Seed(entry.Entry{
		Slug: "published-public", Title: "visible",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
	})
	store.Seed(entry.Entry{
		Slug: "a-draft", Title: "hidden",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})
	store.Seed(entry.Entry{
		Slug: "private-one", Title: "hidden",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPrivate,
	})
	store.Seed(entry.Entry{
		Slug: "unlisted-one", Title: "hidden",
		Status: entry.StatusPublished, Visibility: entry.VisibilityUnlisted,
	})

	t.Run("returns a published public entry", func(t *testing.T) {
		got, err := service.GetPublicBySlug(ctx, "published-public")
		if err != nil {
			t.Fatalf("GetPublicBySlug: %v", err)
		}
		if got.Slug != "published-public" {
			t.Errorf("slug = %q, want %q", got.Slug, "published-public")
		}
	})

	// Every hidden case answers exactly as an unused slug does. A reader who can
	// tell "this draft exists" from "nothing here" can enumerate unpublished work.
	hidden := []string{"a-draft", "private-one", "unlisted-one", "never-used"}
	for _, slug := range hidden {
		t.Run("not readable: "+slug, func(t *testing.T) {
			_, err := service.GetPublicBySlug(ctx, slug)
			if !errors.Is(err, entry.ErrEntryNotFound) {
				t.Errorf("GetPublicBySlug(%q) = %v, want ErrEntryNotFound", slug, err)
			}
		})
	}

	t.Run("a malformed slug is not found, not invalid", func(t *testing.T) {
		// Answered without a query, and with the same error as a well-formed slug
		// that is absent. A distinct error would tell a client which of its guesses
		// were even shaped like real slugs.
		before := store.SlugExistsCalls
		_, err := service.GetPublicBySlug(ctx, "Not A Slug")
		if !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("GetPublicBySlug = %v, want ErrEntryNotFound", err)
		}
		if errors.Is(err, entry.ErrInvalidSlug) {
			t.Error("error is ErrInvalidSlug; a reader must not be able to tell the two apart")
		}
		if store.SlugExistsCalls != before {
			t.Error("a malformed slug reached the store; it cannot match a row")
		}
	})
}

func TestListPublicPagination(t *testing.T) {
	cases := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{"defaults", 0, 0, 1, entry.DefaultPageSize},
		{"explicit values pass through", 2, 10, 2, 10},
		{"negative page becomes the first", -3, 10, 1, 10},
		{"zero size becomes the default", 1, 0, 1, entry.DefaultPageSize},
		{"negative size becomes the default", 1, -5, 1, entry.DefaultPageSize},
		// A cap on the work one request can ask for. Without it page_size=100000 is
		// a request to serialise the whole table.
		{"over-large size is clamped", 1, 100000, 1, entry.MaxPageSize},
		{"at the maximum", 1, entry.MaxPageSize, 1, entry.MaxPageSize},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			service, _ := newTestService(t)

			page, err := service.ListPublic(context.Background(), 0, tc.page, tc.pageSize)
			if err != nil {
				t.Fatalf("ListPublic: %v", err)
			}
			if page.Page != tc.wantPage {
				t.Errorf("page = %d, want %d", page.Page, tc.wantPage)
			}
			if page.PageSize != tc.wantPageSize {
				t.Errorf("page_size = %d, want %d", page.PageSize, tc.wantPageSize)
			}
		})
	}
}

func TestListPublicFiltersAndOrders(t *testing.T) {
	service, store := newTestService(t)
	ctx := context.Background()

	recent := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	older := time.Date(2020, 5, 1, 0, 0, 0, 0, time.UTC)

	store.Seed(entry.Entry{
		ID: 1, Slug: "older-public", Status: entry.StatusPublished,
		Visibility: entry.VisibilityPublic, HappenedAt: older,
	})
	store.Seed(entry.Entry{
		ID: 2, Slug: "recent-public", Status: entry.StatusPublished,
		Visibility: entry.VisibilityPublic, HappenedAt: recent,
	})
	store.Seed(entry.Entry{
		ID: 3, Slug: "a-draft", Status: entry.StatusDraft,
		Visibility: entry.VisibilityPublic, HappenedAt: recent,
	})
	store.Seed(entry.Entry{
		ID: 4, Slug: "private-one", Status: entry.StatusPublished,
		Visibility: entry.VisibilityPrivate, HappenedAt: recent,
	})

	got, err := service.ListPublic(ctx, 0, 1, entry.DefaultPageSize)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}

	want := []string{"recent-public", "older-public"}
	if len(got.Entries) != len(want) {
		slugs := make([]string, len(got.Entries))
		for i, e := range got.Entries {
			slugs[i] = e.Slug
		}
		t.Fatalf("entries = %v, want %v", slugs, want)
	}
	for i, slug := range want {
		if got.Entries[i].Slug != slug {
			t.Errorf("entry %d = %q, want %q", i, got.Entries[i].Slug, slug)
		}
	}

	// The total counts what the filter admits, not what the table holds. Reporting
	// 4 here would have a client paging towards entries it can never see.
	if got.Total != 2 {
		t.Errorf("total = %d, want 2", got.Total)
	}
}

func TestListPublicPastTheEndIsEmptyNotAnError(t *testing.T) {
	service, store := newTestService(t)

	store.Seed(entry.Entry{
		ID: 1, Slug: "only-one", Status: entry.StatusPublished,
		Visibility: entry.VisibilityPublic,
	})

	// The collection shrinks when an entry is unpublished, so a page that existed a
	// moment ago legitimately may not now. A 400 would blame a client that did
	// nothing wrong.
	page, err := service.ListPublic(context.Background(), 0, 99, 20)
	if err != nil {
		t.Fatalf("ListPublic: %v", err)
	}
	if len(page.Entries) != 0 {
		t.Errorf("entries = %d, want 0", len(page.Entries))
	}
	if page.Total != 1 {
		t.Errorf("total = %d, want 1; the total must still describe the collection", page.Total)
	}
}

func TestStoreFailuresReachTheCaller(t *testing.T) {
	failure := errors.New("connection refused")

	t.Run("Create", func(t *testing.T) {
		store := entrytest.NewStore()
		store.FailCreate = failure
		service := entry.NewService(store)

		if _, err := service.Create(context.Background(), validInput()); !errors.Is(err, failure) {
			t.Errorf("Create = %v, want the store's error", err)
		}
	})

	t.Run("Create when the pre-check fails", func(t *testing.T) {
		store := entrytest.NewStore()
		store.FailSlugExists = failure
		service := entry.NewService(store)

		// The slug cannot be confirmed free, so the insert is not attempted. Writing
		// anyway would leave the unique index as the only check, which is a worse
		// error for the same outcome.
		if _, err := service.Create(context.Background(), validInput()); !errors.Is(err, failure) {
			t.Errorf("Create = %v, want the store's error", err)
		}
		if store.CreateCalls != 0 {
			t.Errorf("Create reached the store %d times, want 0", store.CreateCalls)
		}
	})

	t.Run("ListPublic", func(t *testing.T) {
		store := entrytest.NewStore()
		store.FailListPublic = failure
		service := entry.NewService(store)

		if _, err := service.ListPublic(context.Background(), 0, 1, 20); !errors.Is(err, failure) {
			t.Errorf("ListPublic = %v, want the store's error", err)
		}
	})

	t.Run("GetPublicBySlug", func(t *testing.T) {
		store := entrytest.NewStore()
		store.FailGetBySlug = failure
		service := entry.NewService(store)

		// An infrastructure failure must not be reported as ErrEntryNotFound: a 404
		// would tell the owner their entry is gone when the database is unreachable.
		_, err := service.GetPublicBySlug(context.Background(), "any-slug")
		if !errors.Is(err, failure) {
			t.Errorf("GetPublicBySlug = %v, want the store's error", err)
		}
		if errors.Is(err, entry.ErrEntryNotFound) {
			t.Error("a store failure was reported as ErrEntryNotFound")
		}
	})
}

// ptr is a one-liner because UpdateInput is all pointers: nil is "not submitted",
// and a literal cannot be addressed inline.
func ptr[T any](v T) *T { return &v }

// TestUpdateSubmitsOnlyNamedFields is the reason UpdateInput uses pointers. The
// Set flags are what the SQL branches on, so they are what the assertion reads:
// the returned entry looks the same whether a field was left alone or rewritten
// with the value it already had.
func TestUpdateSubmitsOnlyNamedFields(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{
		ID: 7, Slug: "before", Title: "Before", ContentMD: "the old body",
		Summary: "the old summary", Status: entry.StatusPublished,
	})

	if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
		ExpectedRevision: 1,
		Title:            ptr("After"),
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got := store.LastUpdate
	if !got.SetTitle || got.Title != "After" {
		t.Errorf("the title was not submitted: %+v", got)
	}

	for _, flag := range []struct {
		name string
		set  bool
	}{
		{"type", got.SetType},
		{"slug", got.SetSlug},
		{"summary", got.SetSummary},
		{"content_md", got.SetContentMD},
		{"cover_url", got.SetCoverURL},
		{"visibility", got.SetVisibility},
		{"meta", got.SetMeta},
		{"happened_at", got.SetHappenedAt},
	} {
		if flag.set {
			t.Errorf("%s was submitted by an update that did not name it", flag.name)
		}
	}
}

// TestUpdateDistinguishesClearingFromLeavingAlone covers the pair of cases that
// COALESCE could not express, which is why the SQL uses a flag per field.
func TestUpdateDistinguishesClearingFromLeavingAlone(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Slug: "before", Title: "Before", Summary: "present"})

	t.Run("submitting an empty value clears", func(t *testing.T) {
		updated, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 1,
			Summary:          ptr(""),
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if !store.LastUpdate.SetSummary {
			t.Error("set_summary is false, so the clear would not be written")
		}
		if updated.Summary != "" {
			t.Errorf("summary = %q, want cleared", updated.Summary)
		}
	})

	t.Run("omitting leaves it alone", func(t *testing.T) {
		store.Seed(entry.Entry{ID: 8, Slug: "other", Title: "Other", Summary: "keep me"})

		updated, err := service.Update(context.Background(), 8, entry.UpdateInput{
			ExpectedRevision: 1,
			Title:            ptr("Renamed"),
		})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if updated.Summary != "keep me" {
			t.Errorf("summary = %q, want it untouched", updated.Summary)
		}
	})
}

// TestUpdateRecomputesWordCountWithTheBody covers the one derived field. It is
// never accepted from a caller, so the two cannot disagree.
func TestUpdateRecomputesWordCountWithTheBody(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Slug: "before", Title: "Before", ContentMD: "one", WordCount: 1})

	updated, err := service.Update(context.Background(), 7, entry.UpdateInput{
		ExpectedRevision: 1,
		ContentMD:        ptr("one two three"),
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if !store.LastUpdate.SetContentMD {
		t.Fatal("set_content_md is false")
	}
	if updated.WordCount != 3 {
		t.Errorf("word count = %d, want 3", updated.WordCount)
	}

	t.Run("and not without it", func(t *testing.T) {
		if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 2,
			Title:            ptr("Renamed"),
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		// The flag covers the count too, so an update that does not touch the body
		// cannot touch the count either.
		if store.LastUpdate.SetContentMD {
			t.Error("set_content_md is true for an update that named no body")
		}
	})
}

// TestUpdateRefusesAnEmptyInput covers the case where a caller submitted nothing.
// Success here would report a save that did not happen.
func TestUpdateRefusesAnEmptyInput(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Slug: "before", Title: "Before"})

	_, err := service.Update(context.Background(), 7, entry.UpdateInput{})
	if !errors.Is(err, entry.ErrNoUpdateFields) {
		t.Fatalf("Update = %v, want ErrNoUpdateFields", err)
	}
	if store.UpdateCalls != 0 {
		t.Errorf("an empty update reached the store, %d calls", store.UpdateCalls)
	}
}

// TestUpdateValidatesOnlyWhatWasSubmitted covers the difference from create. A
// full validation would report errors about fields the caller never mentioned.
func TestUpdateValidatesOnlyWhatWasSubmitted(t *testing.T) {
	service, store := newTestService(t)
	// Seeded with a title that create would refuse, to prove no re-validation of
	// stored values happens on an unrelated update.
	store.Seed(entry.Entry{ID: 7, Slug: "before", Title: strings.Repeat("x", 300)})

	t.Run("an unrelated update succeeds", func(t *testing.T) {
		if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 1,
			Summary:          ptr("fine"),
		}); err != nil {
			t.Fatalf("Update = %v, want the stored title left unchecked", err)
		}
	})

	for _, tc := range []struct {
		name string
		in   entry.UpdateInput
		want error
	}{
		{"bad type", entry.UpdateInput{ExpectedRevision: 1, Type: ptr(entry.Type("recipe"))}, entry.ErrInvalidType},
		{"bad visibility", entry.UpdateInput{ExpectedRevision: 1, Visibility: ptr(entry.Visibility("secret"))}, entry.ErrInvalidVisibility},
		{"long title", entry.UpdateInput{ExpectedRevision: 1, Title: ptr(strings.Repeat("x", 256))}, entry.ErrInvalidTitle},
		{"bad slug", entry.UpdateInput{ExpectedRevision: 1, Slug: ptr("Not A Slug")}, entry.ErrInvalidSlug},
		{"bad meta", entry.UpdateInput{ExpectedRevision: 1, Meta: ptr(entry.Meta(`["a"]`))}, entry.ErrInvalidMeta},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := store.UpdateCalls
			if _, err := service.Update(context.Background(), 7, tc.in); !errors.Is(err, tc.want) {
				t.Fatalf("Update = %v, want %v", err, tc.want)
			}
			if store.UpdateCalls != before {
				t.Error("an invalid update reached the store")
			}
		})
	}
}

// TestUpdateSlugConflictExcludesItself covers the check that make a resubmitted
// unchanged slug a conflict with itself. Every save from an editor that sends the
// whole form would fail.
func TestUpdateSlugConflictExcludesItself(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{ID: 7, Slug: "mine", Title: "Mine"})
	store.Seed(entry.Entry{ID: 8, Slug: "theirs", Title: "Theirs"})

	t.Run("its own slug is free", func(t *testing.T) {
		if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 1,
			Slug:             ptr("mine"), Title: ptr("Renamed"),
		}); err != nil {
			t.Fatalf("Update = %v, want its own slug accepted", err)
		}
	})

	t.Run("another entry's slug is taken", func(t *testing.T) {
		if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 2,
			Slug:             ptr("theirs"),
		}); !errors.Is(err, entry.ErrSlugTaken) {
			t.Fatalf("Update = %v, want ErrSlugTaken", err)
		}
	})

	t.Run("no check when the slug is not submitted", func(t *testing.T) {
		before := store.SlugExistsExcludingCalls
		if _, err := service.Update(context.Background(), 7, entry.UpdateInput{
			ExpectedRevision: 2,
			Title:            ptr("Again"),
		}); err != nil {
			t.Fatalf("Update: %v", err)
		}
		if store.SlugExistsExcludingCalls != before {
			t.Error("the slug was checked by an update that did not name one")
		}
	})
}

// TestSoftDeleteHidesTheEntryAndFreesTheSlug covers both consequences. The second
// one is why the unique index is partial: a slug typed once must be reusable after
// the entry holding it is deleted.
func TestSoftDeleteHidesTheEntryAndFreesTheSlug(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{
		ID: 7, Slug: "freed", Title: "Freed",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
	})

	if err := service.SoftDelete(context.Background(), 7); err != nil {
		t.Fatalf("SoftDelete: %v", err)
	}
	if !store.LastSoftDeleteAt.Equal(fixedTime) {
		t.Errorf("deleted_at = %v, want the injected clock %v", store.LastSoftDeleteAt, fixedTime)
	}

	t.Run("gone from the public read", func(t *testing.T) {
		if _, err := service.GetPublicBySlug(context.Background(), "freed"); !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("GetPublicBySlug = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("gone from the admin read too", func(t *testing.T) {
		if _, err := service.GetByID(context.Background(), 7); !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("GetByID = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("deleting again is not found", func(t *testing.T) {
		if err := service.SoftDelete(context.Background(), 7); !errors.Is(err, entry.ErrEntryNotFound) {
			t.Errorf("SoftDelete = %v, want ErrEntryNotFound", err)
		}
	})

	t.Run("the slug can be reused", func(t *testing.T) {
		in := validInput()
		in.Slug = "freed"
		if _, err := service.Create(context.Background(), in); err != nil {
			t.Errorf("Create with the freed slug = %v, want it accepted", err)
		}
	})
}

// TestPublishStampsOnce covers the write-once rule. The date is when the entry
// first went out, so a withdrawal and a second publication must not move it.
func TestPublishStampsOnce(t *testing.T) {
	service, store := newTestService(t)
	store.Seed(entry.Entry{
		ID: 7, Slug: "p", Title: "P", ContentMD: "body",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	published, err := service.Publish(context.Background(), 7, 1)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if published.Status != entry.StatusPublished {
		t.Errorf("status = %q, want published", published.Status)
	}
	if !published.PublishedAt.Equal(fixedTime) {
		t.Errorf("published_at = %v, want %v", published.PublishedAt, fixedTime)
	}

	t.Run("republishing keeps the first date", func(t *testing.T) {
		if _, err := service.Unpublish(context.Background(), 7, 2); err != nil {
			t.Fatalf("Unpublish: %v", err)
		}

		// A clock that moved, so a rewrite would be visible rather than coincidentally
		// equal.
		later := entry.NewService(store, entry.WithClock(func() time.Time {
			return fixedTime.Add(48 * time.Hour)
		}))

		again, err := later.Publish(context.Background(), 7, 3)
		if err != nil {
			t.Fatalf("Publish: %v", err)
		}
		if !again.PublishedAt.Equal(fixedTime) {
			t.Errorf("published_at = %v, want the original %v", again.PublishedAt, fixedTime)
		}
	})
}

// TestUnpublishAndArchiveKeepPublishedAt covers what withdrawing leaves behind.
// The two differ in the status they write and in nothing else.
func TestUnpublishAndArchiveKeepPublishedAt(t *testing.T) {
	original := fixedTime.Add(-72 * time.Hour)

	for _, tc := range []struct {
		name string
		call func(*entry.Service, context.Context) (entry.Entry, error)
		want entry.Status
	}{
		{"unpublish", func(s *entry.Service, ctx context.Context) (entry.Entry, error) {
			return s.Unpublish(ctx, 7, 1)
		}, entry.StatusDraft},
		{"archive", func(s *entry.Service, ctx context.Context) (entry.Entry, error) {
			return s.Archive(ctx, 7, 1)
		}, entry.StatusArchived},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newTestService(t)
			store.Seed(entry.Entry{
				ID: 7, Slug: "w", Title: "W", Status: entry.StatusPublished,
				Visibility: entry.VisibilityPublic, PublishedAt: original,
			})

			got, err := tc.call(service, context.Background())
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got.Status != tc.want {
				t.Errorf("status = %q, want %q", got.Status, tc.want)
			}
			if !got.PublishedAt.Equal(original) {
				t.Errorf("published_at = %v, want the original %v", got.PublishedAt, original)
			}

			// Both take the entry off the site. Only the status differs, and only one
			// of them can be undone by publishing again without losing the distinction
			// between a draft and a retired entry.
			if _, err := service.GetPublicBySlug(context.Background(), "w"); !errors.Is(err, entry.ErrEntryNotFound) {
				t.Errorf("GetPublicBySlug = %v, want ErrEntryNotFound", err)
			}
			// Still there for an editor.
			if _, err := service.GetByID(context.Background(), 7); err != nil {
				t.Errorf("GetByID = %v, want the entry still readable", err)
			}
		})
	}
}

// TestListAdminIsAWorkQueue covers the two ways it differs from the public list:
// it shows every status, and it sorts by last edit rather than by when the thing
// happened.
func TestListAdminIsAWorkQueue(t *testing.T) {
	service, store := newTestService(t)

	// happened_at descending and updated_at descending disagree here on purpose, so
	// a list sorted by the wrong column fails rather than coincidentally passing.
	for i, tc := range []struct {
		slug     string
		status   entry.Status
		happened time.Time
		updated  time.Time
	}{
		{"oldest-edit", entry.StatusPublished, fixedTime, fixedTime.Add(-3 * time.Hour)},
		{"middle-edit", entry.StatusDraft, fixedTime.Add(-24 * time.Hour), fixedTime.Add(-2 * time.Hour)},
		{"newest-edit", entry.StatusArchived, fixedTime.Add(-48 * time.Hour), fixedTime.Add(-1 * time.Hour)},
	} {
		store.Seed(entry.Entry{
			ID: int64(i + 1), Slug: tc.slug, Title: tc.slug, Status: tc.status,
			Visibility: entry.VisibilityPublic, HappenedAt: tc.happened, UpdatedAt: tc.updated,
		})
	}

	page, err := service.ListAdmin(context.Background(), nil, "", 0, 0)
	if err != nil {
		t.Fatalf("ListAdmin: %v", err)
	}

	if page.Total != 3 {
		t.Errorf("total = %d, want every status counted", page.Total)
	}
	if len(page.Entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(page.Entries))
	}
	if got := page.Entries[0].Slug; got != "newest-edit" {
		t.Errorf("first slug = %q, want newest-edit: the list is not sorted by updated_at", got)
	}

	t.Run("the public list disagrees, as it should", func(t *testing.T) {
		public, err := service.ListPublic(context.Background(), 0, 0, 0)
		if err != nil {
			t.Fatalf("ListPublic: %v", err)
		}
		// One published public entry out of three.
		if public.Total != 1 {
			t.Errorf("public total = %d, want 1", public.Total)
		}
	})

	t.Run("filtered by status", func(t *testing.T) {
		draft := entry.StatusDraft
		page, err := service.ListAdmin(context.Background(), &draft, "", 0, 0)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if page.Total != 1 || len(page.Entries) != 1 {
			t.Fatalf("total = %d, entries = %d, want 1 and 1", page.Total, len(page.Entries))
		}
		if page.Entries[0].Status != entry.StatusDraft {
			t.Errorf("status = %q, want draft", page.Entries[0].Status)
		}
	})

	t.Run("an unknown status is refused", func(t *testing.T) {
		unknown := entry.Status("stauts")
		if _, err := service.ListAdmin(context.Background(), &unknown, "", 0, 0); !errors.Is(err, entry.ErrInvalidStatus) {
			t.Errorf("ListAdmin = %v, want ErrInvalidStatus", err)
		}
	})

	t.Run("pagination is clamped like the public list", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "", -5, 10_000)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if page.Page != 1 {
			t.Errorf("page = %d, want 1", page.Page)
		}
		if page.PageSize != entry.MaxPageSize {
			t.Errorf("page size = %d, want the maximum %d", page.PageSize, entry.MaxPageSize)
		}
	})

	t.Run("past the end is an empty page, not an error", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "", 99, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(page.Entries) != 0 {
			t.Errorf("got %d entries, want none", len(page.Entries))
		}
		// The total still reports the collection size, so a client can tell an
		// overshot page from an empty site.
		if page.Total != 3 {
			t.Errorf("total = %d, want 3", page.Total)
		}
	})
}

// TestIDBoundsAreRefusedWithoutAQuery covers the ids that cannot name a row. Every
// method that takes one refuses it before reaching the store, so a zero from a
// caller's uninitialised variable does not become a query.
func TestIDBoundsAreRefusedWithoutAQuery(t *testing.T) {
	for _, bound := range []struct {
		name string
		id   int64
	}{
		{"zero", 0},
		{"negative", -1},
	} {
		id := bound.id
		t.Run(bound.name, func(t *testing.T) {
			service, store := newTestService(t)

			for _, tc := range []struct {
				name string
				call func() error
			}{
				{"Update", func() error {
					_, err := service.Update(context.Background(), id, entry.UpdateInput{ExpectedRevision: 1, Title: ptr("x")})
					return err
				}},
				{"SoftDelete", func() error { return service.SoftDelete(context.Background(), id) }},
				{"Publish", func() error {
					_, err := service.Publish(context.Background(), id, 1)
					return err
				}},
				{"Unpublish", func() error {
					_, err := service.Unpublish(context.Background(), id, 1)
					return err
				}},
				{"Archive", func() error {
					_, err := service.Archive(context.Background(), id, 1)
					return err
				}},
				{"GetByID", func() error {
					_, err := service.GetByID(context.Background(), id)
					return err
				}},
			} {
				t.Run(tc.name, func(t *testing.T) {
					if err := tc.call(); !errors.Is(err, entry.ErrEntryNotFound) {
						t.Errorf("%s = %v, want ErrEntryNotFound", tc.name, err)
					}
				})
			}

			if store.UpdateCalls != 0 {
				t.Errorf("an impossible id reached the store, %d update calls", store.UpdateCalls)
			}
		})
	}
}

// TestListAdminSearchesTheDirectory covers the article-directory search: the
// admin list accepts a free-text query matched against title, slug, and summary.
//
// Case-insensitive because the directory is a find-as-you-type field, and an
// editor does not shift-key their way to their own draft. Matched against three
// columns rather than the body, so a common word buried in a long article does
// not drown the article actually named that.
func TestListAdminSearchesTheDirectory(t *testing.T) {
	service, store := newTestService(t)

	for i, tc := range []struct {
		slug    string
		title   string
		summary string
		status  entry.Status
	}{
		{"mountain-trip", "山中 Mountain", "walked up", entry.StatusPublished},
		{"kyoto-spring", "京都的春天", "by the river", entry.StatusDraft},
		{"sea-notes", "海边", "a mountain seen from the sea", entry.StatusDraft},
	} {
		store.Seed(entry.Entry{
			ID: int64(i + 1), Slug: tc.slug, Title: tc.title, Summary: tc.summary,
			Status: tc.status, Visibility: entry.VisibilityPublic,
			UpdatedAt: fixedTime.Add(-time.Duration(i) * time.Hour),
		})
	}

	t.Run("matches the title", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "Mountain", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		// Two rows carry "mountain": one in a title, one in a summary.
		if got := slugsOf(page.Entries); !slices.Equal(got, []string{"mountain-trip", "sea-notes"}) {
			t.Errorf("slugs = %v, want the title and summary matches", got)
		}
	})

	t.Run("is case-insensitive", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "MOUNTAIN", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(page.Entries) != 2 {
			t.Errorf("got %d entries, want the same 2 as the lowercase query", len(page.Entries))
		}
	})

	t.Run("matches the slug", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "kyoto", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if got := slugsOf(page.Entries); !slices.Equal(got, []string{"kyoto-spring"}) {
			t.Errorf("slugs = %v, want the slug match", got)
		}
	})

	t.Run("intersects with the status filter", func(t *testing.T) {
		draft := entry.StatusDraft
		page, err := service.ListAdmin(context.Background(), &draft, "mountain", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		// mountain-trip matches the text but is published, so the two filters are
		// ANDed rather than unioned.
		if got := slugsOf(page.Entries); !slices.Equal(got, []string{"sea-notes"}) {
			t.Errorf("slugs = %v, want only the draft match", got)
		}
	})

	t.Run("the total counts the filtered set", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "mountain", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if page.Total != 2 {
			t.Errorf("total = %d, want the matched count rather than the collection size", page.Total)
		}
	})

	t.Run("a whitespace-only query behaves as absent", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "   ", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if page.Total != 3 {
			t.Errorf("total = %d, want every entry: a blank query is not a filter", page.Total)
		}
	})

	t.Run("surrounding whitespace is trimmed", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "  kyoto  ", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if got := slugsOf(page.Entries); !slices.Equal(got, []string{"kyoto-spring"}) {
			t.Errorf("slugs = %v, want the trimmed query to match", got)
		}
	})

	t.Run("no match is an empty page, not an error", func(t *testing.T) {
		page, err := service.ListAdmin(context.Background(), nil, "nothing-here", 1, 20)
		if err != nil {
			t.Fatalf("ListAdmin: %v", err)
		}
		if len(page.Entries) != 0 || page.Total != 0 {
			t.Errorf("entries = %d, total = %d, want both zero", len(page.Entries), page.Total)
		}
	})
}

// slugsOf reads the slugs out of a page in order, so a test can assert on the
// whole result rather than indexing into it field by field.
func slugsOf(entries []entry.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Slug)
	}
	return out
}
