package entryhttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/entry"
)

// categorised is a published public entry filed under testCategoryID, with the
// joined name and slug filled as a read would fill them.
func categorised(id int64, slug string) entry.Entry {
	return entry.Entry{
		ID: id, World: contentworld.Journal, Slug: slug, Title: slug,
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
		CategoryID: testCategoryID, CategoryName: "旅行", CategorySlug: testCategorySlug,
	}
}

// uncategorised is the same entry with no category, which is a normal state.
func uncategorised(id int64, slug string) entry.Entry {
	return entry.Entry{
		ID: id, World: contentworld.Journal, Slug: slug, Title: slug,
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
	}
}

// categoryObject returns the nested category from an entry response, and reports
// whether it was an object rather than null.
func categoryObject(t *testing.T, item map[string]json.RawMessage) (map[string]json.RawMessage, bool) {
	t.Helper()

	raw, ok := item["category"]
	if !ok {
		t.Fatal("the entry response has no \"category\" key")
	}
	if string(raw) == "null" {
		return nil, false
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("\"category\" is neither an object nor null: %v", err)
	}
	return object, true
}

// TestListFiltersByCategory covers the filter's two halves: the page carries only
// the matching entries, and the total describes that same filtered set. A total
// counting the whole table would let a client page towards entries the filter will
// never return.
func TestListFiltersByCategory(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))
	store.Seed(categorised(2, "also-travel"))
	store.Seed(uncategorised(3, "no-category"))

	rec := do(t, handler, http.MethodGet, "/api/v1/journals?category="+testCategorySlug, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	items := dataArray(t, rec)
	if len(items) != 2 {
		t.Fatalf("got %d entries, want 2", len(items))
	}
	for _, item := range items {
		if slug := stringField(t, item, "slug"); slug == "no-category" {
			t.Errorf("an entry outside the category appears in the filtered list")
		}
	}

	var envelope struct {
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response has no readable meta: %v", err)
	}
	if envelope.Meta.Total != 2 {
		t.Errorf("total = %d, want 2; the total must describe the filtered list", envelope.Meta.Total)
	}
}

// TestAnUnknownCategoryIs404NotAnEmptyList is the reason the filter resolves a
// slug through the taxonomy service instead of being passed straight to SQL.
// /journals?category=nope and a category with nothing in it are different
// situations, and a client that cannot tell them apart shows "no posts here" for a
// typo.
func TestAnUnknownCategoryIs404NotAnEmptyList(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	rec := do(t, handler, http.MethodGet, "/api/v1/journals?category=nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

// TestAnEmptyCategoryParamIsTreatedAsAbsent covers the case a frontend produces by
// accident. A form whose category select is on "all" serialises to category=, and
// answering 400 there would break the one case the filter exists to serve.
func TestAnEmptyCategoryParamIsTreatedAsAbsent(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))
	store.Seed(uncategorised(2, "no-category"))

	rec := do(t, handler, http.MethodGet, "/api/v1/journals?category=", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if items := dataArray(t, rec); len(items) != 2 {
		t.Errorf("got %d entries, want both: an empty filter must not filter", len(items))
	}
}

// TestTheFilterStillHidesUnpublishedEntries covers the interaction between the two
// conditions. A category filter narrows what a reader can see; it must never widen
// it, and an OR where an AND belongs would publish every draft in a category.
func TestTheFilterStillHidesUnpublishedEntries(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	published := categorised(1, "published-in-travel")
	store.Seed(published)

	draft := categorised(2, "draft-in-travel")
	draft.Status = entry.StatusDraft
	store.Seed(draft)

	unlisted := categorised(3, "unlisted-in-travel")
	unlisted.Visibility = entry.VisibilityUnlisted
	store.Seed(unlisted)

	private := categorised(4, "private-in-travel")
	private.Visibility = entry.VisibilityPrivate
	store.Seed(private)

	items := dataArray(t, do(t, handler, http.MethodGet,
		"/api/v1/journals?category="+testCategorySlug, ""))

	if len(items) != 1 {
		t.Fatalf("got %d entries, want only the published public one\nbody: %v", len(items), items)
	}
	if got := stringField(t, items[0], "slug"); got != "published-in-travel" {
		t.Errorf("slug = %q, want published-in-travel", got)
	}
}

// TestPublicReadsCarryTheNestedCategory covers what a reader needs to render "in
// 旅行" and link to it. Both public shapes, because the list and the detail build
// the object through the same converter and a change to one would otherwise be
// caught in only one place.
func TestPublicReadsCarryTheNestedCategory(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	for _, tc := range []struct {
		name   string
		target string
		object func(*testing.T, *httptest.ResponseRecorder) map[string]json.RawMessage
	}{
		{
			"list", "/api/v1/journals",
			func(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
				items := dataArray(t, rec)
				if len(items) != 1 {
					t.Fatalf("got %d entries, want 1", len(items))
				}
				return items[0]
			},
		},
		{"detail", "/api/v1/journals/in-travel", dataObject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, tc.target, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
			}

			category, present := categoryObject(t, tc.object(t, rec))
			if !present {
				t.Fatal("category is null for an entry that has one")
			}
			if got := stringField(t, category, "name"); got != "旅行" {
				t.Errorf("category name = %q, want 旅行", got)
			}
			if got := stringField(t, category, "slug"); got != testCategorySlug {
				t.Errorf("category slug = %q, want %q", got, testCategorySlug)
			}
			// The id is what an admin form and a filter link both address, so it
			// ships even though the public entry shapes withhold the entry's own id.
			var id int64
			if err := json.Unmarshal(category["id"], &id); err != nil || id != testCategoryID {
				t.Errorf("category id = %s, want %d", category["id"], testCategoryID)
			}

			// No description and no entry_count: those belong to a category page, and
			// a count here would be a second query per entry in a list.
			for _, absent := range []string{"description", "entry_count", "sort_order"} {
				if _, present := category[absent]; present {
					t.Errorf("the nested category carries %s", absent)
				}
			}
		})
	}
}

// TestAnUncategorisedEntryReportsNullNotAnEmptyObject covers the normal case. An
// object with an empty name would describe a category that cannot exist, since
// name is NOT NULL with a length CHECK.
func TestAnUncategorisedEntryReportsNullNotAnEmptyObject(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(uncategorised(1, "no-category"))

	items := dataArray(t, do(t, handler, http.MethodGet, "/api/v1/journals", ""))
	if len(items) != 1 {
		t.Fatalf("got %d entries, want 1", len(items))
	}
	if _, present := categoryObject(t, items[0]); present {
		t.Errorf("category is an object for an uncategorised entry: %s", items[0]["category"])
	}
}

// TestAWriteReportsTheCategoryIDAndANullCategory covers the read/write asymmetry.
// RETURNING sees only the entry row, so a write knows the id and not the label. The
// editor confirming its change reads category_id, which is filled either way.
func TestAWriteReportsTheCategoryIDAndANullCategory(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", `{
		"world":"journal","title":"a trip","slug":"a-trip","content_md":"went somewhere","category_id":7
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	var id int64
	if err := json.Unmarshal(data["category_id"], &id); err != nil {
		t.Fatalf("the write response has no readable category_id: %s", rec.Body.String())
	}
	if id != testCategoryID {
		t.Errorf("category_id = %d, want %d", id, testCategoryID)
	}
	// Null rather than {"id":7,"name":"","slug":""}: the write cannot join, and an
	// empty name is not a category.
	if _, present := categoryObject(t, data); present {
		t.Errorf("a write response carries a joined category it could not have read: %s", data["category"])
	}
}

// TestUpdateMovesAnEntryBetweenCategories covers the ordinary case, and that an
// absent category_id leaves the category alone rather than clearing it.
func TestUpdateMovesAnEntryBetweenCategories(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	t.Run("a new id is written", func(t *testing.T) {
		rec := do(t, handler, http.MethodPatch, "/api/v1/entries/1", `{"revision":1,"category_id":9}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		var id int64
		if err := json.Unmarshal(dataObject(t, rec)["category_id"], &id); err != nil || id != 9 {
			t.Errorf("category_id = %s, want 9", dataObject(t, rec)["category_id"])
		}
	})

	t.Run("an absent id leaves it alone", func(t *testing.T) {
		rec := do(t, handler, http.MethodPatch, "/api/v1/entries/1", `{"revision":2,"title":"a new title"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		var id int64
		if err := json.Unmarshal(dataObject(t, rec)["category_id"], &id); err != nil || id != 9 {
			t.Errorf("category_id = %s, want it unchanged at 9", dataObject(t, rec)["category_id"])
		}
		if store.LastUpdate.SetCategoryID {
			t.Error("an absent category_id was flagged for writing")
		}
	})
}

// TestUpdateWithZeroUncategorisesTheEntry covers the convention the DTO documents.
// `null` is indistinguishable from an omitted key, so 0 is what removes a category,
// and this is the assertion that keeps it reachable over HTTP.
func TestUpdateWithZeroUncategorisesTheEntry(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/1", `{"revision":1,"category_id":0}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	var id int64
	if err := json.Unmarshal(data["category_id"], &id); err != nil || id != 0 {
		t.Errorf("category_id = %s, want 0", data["category_id"])
	}
	if !store.LastUpdate.SetCategoryID {
		t.Error("SetCategoryID is false, so the clear would not reach the column")
	}
	// And the stale joined name went with it, or the next read would report a
	// category the entry no longer has.
	if _, present := categoryObject(t, data); present {
		t.Errorf("the response still carries a category: %s", data["category"])
	}
}

// TestUpdateWithNullLeavesTheCategoryAlone is the other half of that convention,
// and the one a client is most likely to get wrong.
func TestUpdateWithNullLeavesTheCategoryAlone(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/1",
		`{"revision":1,"title":"a new title","category_id":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var id int64
	if err := json.Unmarshal(dataObject(t, rec)["category_id"], &id); err != nil || id != testCategoryID {
		t.Errorf("category_id = %s, want it unchanged at %d",
			dataObject(t, rec)["category_id"], testCategoryID)
	}
	if store.LastUpdate.SetCategoryID {
		t.Error("null was read as a submitted value")
	}
}

// TestAnUnknownCategoryOnAWriteIs400NamingTheField covers the foreign key's error
// path. The FK is the guarantee rather than a read-first check, because a read
// would still be racing a delete; what matters is that the violation arrives as a
// client error naming the field, not a 500.
func TestAnUnknownCategoryOnAWriteIs400NamingTheField(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.FailCreate = entry.ErrUnknownCategory

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", `{
		"world":"journal","title":"a trip","slug":"a-trip","content_md":"went somewhere","category_id":999
	}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if fields := errorFields(t, rec); fields["category_id"] == "" {
		t.Errorf("the error names no category_id field: %s", rec.Body.String())
	}
}

// TestUnlistedIsReachableByLinkAndAbsentFromTheList is the whole of what the
// visibility decision means. Asserted together in one test because either half
// alone is satisfied by the wrong implementation: hiding it everywhere, or showing
// it everywhere.
//
// Not access control. A slug is human readable and therefore guessable, so this
// asserts only that the URL opens and the list stays clean.
func TestUnlistedIsReachableByLinkAndAbsentFromTheList(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	unlisted := uncategorised(1, "quiet-one")
	unlisted.Visibility = entry.VisibilityUnlisted
	unlisted.ContentMD = "the body of quiet-one"
	store.Seed(unlisted)

	t.Run("the link opens", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/journals/quiet-one", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		// With the body, or the endpoint returned a row it will not render.
		if got := stringField(t, dataObject(t, rec), "content_md"); got != "the body of quiet-one" {
			t.Errorf("content_md = %q, want the body", got)
		}
	})

	t.Run("the list omits it", func(t *testing.T) {
		items := dataArray(t, do(t, handler, http.MethodGet, "/api/v1/journals", ""))
		if len(items) != 0 {
			t.Errorf("the public list carries %d entries, want none", len(items))
		}
	})

	t.Run("the filtered list omits it too", func(t *testing.T) {
		// The category filter must not be a way around the visibility rule.
		filtered := uncategorised(2, "quiet-in-travel")
		filtered.Visibility = entry.VisibilityUnlisted
		filtered.CategoryID = testCategoryID
		store.Seed(filtered)

		items := dataArray(t, do(t, handler, http.MethodGet,
			"/api/v1/journals?category="+testCategorySlug, ""))
		if len(items) != 0 {
			t.Errorf("the filtered list carries %d entries, want none", len(items))
		}
	})
}

// TestAPrivateEntryStaysHiddenFromItsLink is the boundary beside unlisted. The two
// differ in exactly one respect, and an implementation that widened the read too far
// would pass every unlisted assertion above while publishing private entries.
func TestAPrivateEntryStaysHiddenFromItsLink(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	private := uncategorised(1, "private-one")
	private.Visibility = entry.VisibilityPrivate
	private.ContentMD = "the body of private-one"
	store.Seed(private)

	rec := do(t, handler, http.MethodGet, "/api/v1/journals/private-one", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "the body of") {
		t.Errorf("the response carries entry content: %s", rec.Body.String())
	}
}

// TestAdminReadsCarryTheCategory covers the two authenticated shapes. The admin
// detail read is the one place a joined category appears on the owner shape, which
// is what makes the field worth having there at all.
func TestAdminReadsCarryTheCategory(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))

	t.Run("detail", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries/1", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}

		data := dataObject(t, rec)
		category, present := categoryObject(t, data)
		if !present {
			t.Fatal("the admin detail read reports a null category for an entry that has one")
		}
		if got := stringField(t, category, "name"); got != "旅行" {
			t.Errorf("category name = %q, want 旅行", got)
		}
	})

	t.Run("list", func(t *testing.T) {
		items := dataArray(t, do(t, handler, http.MethodGet, "/api/v1/admin/entries", ""))
		if len(items) != 1 {
			t.Fatalf("got %d entries, want 1", len(items))
		}
		if _, present := categoryObject(t, items[0]); !present {
			t.Error("the admin list reports a null category for an entry that has one")
		}
	})
}

// TestTheAdminListRejectsUnknownCategoryFilter keeps the admin directory's
// category semantics aligned with the public list: a typo is a 404, not an
// apparently empty or unfiltered result.
func TestTheAdminListRejectsUnknownCategoryFilter(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(categorised(1, "in-travel"))
	store.Seed(uncategorised(2, "no-category"))

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?world=journal&category=nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}
