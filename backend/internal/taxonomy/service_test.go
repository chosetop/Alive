package taxonomy_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/taxonomy"
	"github.com/p30huiwei/alive/backend/internal/taxonomy/taxonomytest"
)

// newService builds a service over a fresh fake, with logging discarded so a
// WarnContext in a passing test does not litter the output.
func newService(t *testing.T) (*taxonomy.Service, *taxonomytest.Store) {
	t.Helper()

	store := taxonomytest.NewStore()
	service := taxonomy.NewService(store,
		taxonomy.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	return service, store
}

// validCreate is a category that passes every rule, so a test changing one field
// isolates the rule it is about.
func validCreate() taxonomy.CreateInput {
	return taxonomy.CreateInput{
		Name:        "旅行",
		Slug:        "travel",
		Description: "places and the getting there",
		SortOrder:   10,
	}
}

func TestCreateStoresEveryField(t *testing.T) {
	service, store := newService(t)

	created, err := service.Create(context.Background(), validCreate())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if created.ID == 0 {
		t.Error("the created category carries no id")
	}
	// Asserted against LastCreate rather than only the returned value, because the
	// return could be right while a field never reached the write.
	if store.LastCreate.Name != "旅行" {
		t.Errorf("stored name = %q, want 旅行", store.LastCreate.Name)
	}
	if store.LastCreate.Slug != "travel" {
		t.Errorf("stored slug = %q, want travel", store.LastCreate.Slug)
	}
	if store.LastCreate.Description != "places and the getting there" {
		t.Errorf("stored description = %q", store.LastCreate.Description)
	}
	if store.LastCreate.SortOrder != 10 {
		t.Errorf("stored sort_order = %d, want 10", store.LastCreate.SortOrder)
	}
}

// TestCreateRefusesInvalidInput covers each rule in one table, and asserts no
// write happened. Validation that reports an error and stores the row anyway is
// worse than no validation, because the response and the table disagree.
func TestCreateRefusesInvalidInput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mutate  func(*taxonomy.CreateInput)
		wantErr error
	}{
		{"empty name", func(in *taxonomy.CreateInput) { in.Name = "" }, taxonomy.ErrInvalidName},
		{
			"name over 64 characters",
			func(in *taxonomy.CreateInput) { in.Name = strings.Repeat("a", 65) },
			taxonomy.ErrInvalidName,
		},
		{"empty slug", func(in *taxonomy.CreateInput) { in.Slug = "" }, taxonomy.ErrInvalidSlug},
		{"uppercase slug", func(in *taxonomy.CreateInput) { in.Slug = "Travel" }, taxonomy.ErrInvalidSlug},
		{"slug with spaces", func(in *taxonomy.CreateInput) { in.Slug = "long trips" }, taxonomy.ErrInvalidSlug},
		{"slug with a double hyphen", func(in *taxonomy.CreateInput) { in.Slug = "a--b" }, taxonomy.ErrInvalidSlug},
		{"slug with a leading hyphen", func(in *taxonomy.CreateInput) { in.Slug = "-travel" }, taxonomy.ErrInvalidSlug},
		{"slug with a trailing hyphen", func(in *taxonomy.CreateInput) { in.Slug = "travel-" }, taxonomy.ErrInvalidSlug},
		{
			"slug over 64 characters",
			func(in *taxonomy.CreateInput) { in.Slug = strings.Repeat("a", 65) },
			taxonomy.ErrInvalidSlug,
		},
		{
			"non-ASCII slug",
			func(in *taxonomy.CreateInput) { in.Slug = "旅行" },
			taxonomy.ErrInvalidSlug,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newService(t)

			in := validCreate()
			tc.mutate(&in)

			_, err := service.Create(context.Background(), in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if store.CreateCalls != 0 {
				t.Errorf("a refused category was written, %d calls", store.CreateCalls)
			}
		})
	}
}

// TestCreateCountsNameInRunes covers the one length rule that is not bytes. A
// 64-character Chinese name is 192 bytes and the column accepts it, so measuring
// bytes here would refuse a name the database would take.
func TestCreateCountsNameInRunes(t *testing.T) {
	service, _ := newService(t)

	in := validCreate()
	in.Name = strings.Repeat("字", 64)

	if _, err := service.Create(context.Background(), in); err != nil {
		t.Fatalf("a 64-character name was refused: %v", err)
	}
}

func TestCreateRefusesADuplicateSlug(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{Name: "existing", Slug: "travel"})

	_, err := service.Create(context.Background(), validCreate())
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
	// The pre-check caught it, so nothing was written.
	if store.CreateCalls != 0 {
		t.Errorf("a duplicate slug reached the write, %d calls", store.CreateCalls)
	}
}

// TestCreateReportsAConflictFoundByTheConstraint covers the race the pre-check
// cannot close: two creates both read the slug as free, and categories_slug_key
// refuses the second write. The client must see the same conflict either way, or
// the same mistake would produce a 409 or a 500 depending on timing.
func TestCreateReportsAConflictFoundByTheConstraint(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{Name: "existing", Slug: "travel"})
	store.SlugExistsAlwaysFree = true

	_, err := service.Create(context.Background(), validCreate())
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
	if store.CreateCalls != 1 {
		t.Errorf("the write was not attempted, %d calls", store.CreateCalls)
	}
}

func TestCreateAcceptsAnAbsentDescriptionAndZeroOrder(t *testing.T) {
	service, store := newService(t)

	created, err := service.Create(context.Background(), taxonomy.CreateInput{
		Name: "杂记",
		Slug: "notes",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Description != "" {
		t.Errorf("description = %q, want empty", created.Description)
	}
	// Zero is a position, not a missing value: it puts the category first.
	if store.LastCreate.SortOrder != 0 {
		t.Errorf("sort_order = %d, want 0", store.LastCreate.SortOrder)
	}
}

// TestUpdateTouchesOnlySubmittedFields is what the Set flags exist for. A fake
// that wrote every field would let this pass while the real UPDATE blanked the
// columns the client never mentioned.
func TestUpdateTouchesOnlySubmittedFields(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{
		ID: 1, Name: "旅行", Slug: "travel",
		Description: "keep me", SortOrder: 10,
	})

	name := "远行"
	updated, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{Name: &name})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Name != "远行" {
		t.Errorf("name = %q, want 远行", updated.Name)
	}
	if updated.Description != "keep me" {
		t.Errorf("description = %q; an unmentioned field was overwritten", updated.Description)
	}
	if updated.Slug != "travel" {
		t.Errorf("slug = %q; an unmentioned field was overwritten", updated.Slug)
	}
	if updated.SortOrder != 10 {
		t.Errorf("sort_order = %d; an unmentioned field was overwritten", updated.SortOrder)
	}

	// The flags themselves, because they are the mechanism and the returned
	// category cannot show which columns the statement addressed.
	if !store.LastUpdate.SetName {
		t.Error("SetName is false for a submitted name")
	}
	if store.LastUpdate.SetDescription || store.LastUpdate.SetSlug || store.LastUpdate.SetSortOrder {
		t.Errorf("an unsubmitted field was flagged for writing: %+v", store.LastUpdate)
	}
}

// TestUpdateClearsADescriptionWithAnEmptyString covers the case COALESCE cannot
// express: a submitted "" must reach the column as a clear, and it is
// indistinguishable from an absent field unless the flag carries the difference.
func TestUpdateClearsADescriptionWithAnEmptyString(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel", Description: "remove me"})

	empty := ""
	updated, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{Description: &empty})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.Description != "" {
		t.Errorf("description = %q, want it cleared", updated.Description)
	}
	if !store.LastUpdate.SetDescription {
		t.Error("SetDescription is false, so the clear would not reach the column")
	}
}

// TestUpdateAcceptsAZeroSortOrder is the same distinction for a number. Zero is a
// real position, and reading it as "unspecified" would silently ignore a request
// to move a category to the front.
func TestUpdateAcceptsAZeroSortOrder(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel", SortOrder: 50})

	zero := 0
	updated, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{SortOrder: &zero})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	if updated.SortOrder != 0 {
		t.Errorf("sort_order = %d, want 0", updated.SortOrder)
	}
	if !store.LastUpdate.SetSortOrder {
		t.Error("SetSortOrder is false, so the move would not reach the column")
	}
}

func TestUpdateRefusesAnEmptyInput(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})

	_, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{})
	if !errors.Is(err, taxonomy.ErrNoUpdateFields) {
		t.Fatalf("error = %v, want ErrNoUpdateFields", err)
	}
	if store.UpdateCalls != 0 {
		t.Errorf("an empty update reached storage, %d calls", store.UpdateCalls)
	}
}

// TestUpdateValidatesOnlySubmittedFields covers why validation is per field
// rather than over the whole category: a PATCH that names one bad field must not
// report errors about fields the client never sent.
func TestUpdateValidatesOnlySubmittedFields(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      taxonomy.UpdateInput
		wantErr error
	}{
		{"empty name", taxonomy.UpdateInput{Name: pointer("")}, taxonomy.ErrInvalidName},
		{"long name", taxonomy.UpdateInput{Name: pointer(strings.Repeat("a", 65))}, taxonomy.ErrInvalidName},
		{"bad slug", taxonomy.UpdateInput{Slug: pointer("Not A Slug")}, taxonomy.ErrInvalidSlug},
		{"empty slug", taxonomy.UpdateInput{Slug: pointer("")}, taxonomy.ErrInvalidSlug},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newService(t)
			store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})

			_, err := service.Update(context.Background(), 1, tc.in)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
			if store.UpdateCalls != 0 {
				t.Errorf("a refused update reached storage, %d calls", store.UpdateCalls)
			}
		})
	}
}

// TestUpdateAcceptsACategorysOwnSlug is what SlugExistsExcluding is for. Sending
// the whole form back with the slug unchanged is the common case, and refusing it
// as a conflict would make a category impossible to edit.
func TestUpdateAcceptsACategorysOwnSlug(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})

	updated, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{
		Slug: pointer("travel"),
		Name: pointer("远行"),
	})
	if err != nil {
		t.Fatalf("a category resubmitting its own slug was refused: %v", err)
	}
	if updated.Slug != "travel" {
		t.Errorf("slug = %q, want travel", updated.Slug)
	}
	if store.SlugExistsExcludingCalls != 1 {
		t.Errorf("the exclusion check ran %d times, want 1", store.SlugExistsExcludingCalls)
	}
}

func TestUpdateRefusesAnotherCategorysSlug(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})
	store.Seed(taxonomy.Category{ID: 2, Name: "杂记", Slug: "notes"})

	_, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{Slug: pointer("notes")})
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
	if store.UpdateCalls != 0 {
		t.Errorf("a conflicting slug reached the write, %d calls", store.UpdateCalls)
	}
}

// TestUpdateReportsAConflictFoundByTheConstraint is the create race again, on the
// update path: the pre-check reads the slug as free and the unique index refuses
// the write.
func TestUpdateReportsAConflictFoundByTheConstraint(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})
	store.Seed(taxonomy.Category{ID: 2, Name: "杂记", Slug: "notes"})
	store.SlugExistsAlwaysFree = true

	_, err := service.Update(context.Background(), 1, taxonomy.UpdateInput{Slug: pointer("notes")})
	if !errors.Is(err, taxonomy.ErrSlugTaken) {
		t.Fatalf("error = %v, want ErrSlugTaken", err)
	}
	if store.UpdateCalls != 1 {
		t.Errorf("the write was not attempted, %d calls", store.UpdateCalls)
	}
}

// TestDeleteRemovesTheCategory covers the decision that a delete is physical and
// the entries survive it. Nothing here can observe the entries becoming
// uncategorised: that is ON DELETE SET NULL, which is the schema's job and is
// verified against a real database.
func TestDeleteRemovesTheCategory(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})

	if err := service.Delete(context.Background(), 1); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if store.Len() != 0 {
		t.Errorf("the category is still stored, %d remain", store.Len())
	}
}

// TestDeleteTwiceReportsNotFound covers the second call. A category that is gone
// is gone, and reporting success again would tell a client its request had an
// effect it did not have.
func TestDeleteTwiceReportsNotFound(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})

	if err := service.Delete(context.Background(), 1); err != nil {
		t.Fatalf("first Delete: %v", err)
	}
	if err := service.Delete(context.Background(), 1); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
		t.Fatalf("second Delete error = %v, want ErrCategoryNotFound", err)
	}
}

// TestNonPositiveIDsAreNotFoundWithoutAQuery covers the three id-taking methods.
// An id of 0 or below cannot exist, so it is answered as missing rather than sent
// to storage; the assertion on call counts is what proves the query was skipped.
func TestNonPositiveIDsAreNotFoundWithoutAQuery(t *testing.T) {
	for _, id := range []int64{0, -1} {
		t.Run("delete", func(t *testing.T) {
			service, store := newService(t)
			if err := service.Delete(context.Background(), id); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
				t.Errorf("Delete(%d) error = %v, want ErrCategoryNotFound", id, err)
			}
			if store.DeleteCalls != 0 {
				t.Errorf("Delete(%d) reached storage", id)
			}
		})
		t.Run("update", func(t *testing.T) {
			service, store := newService(t)
			_, err := service.Update(context.Background(), id, taxonomy.UpdateInput{Name: pointer("x")})
			if !errors.Is(err, taxonomy.ErrCategoryNotFound) {
				t.Errorf("Update(%d) error = %v, want ErrCategoryNotFound", id, err)
			}
			if store.UpdateCalls != 0 {
				t.Errorf("Update(%d) reached storage", id)
			}
		})
		t.Run("get by id", func(t *testing.T) {
			service, _ := newService(t)
			if _, err := service.GetByID(context.Background(), id); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
				t.Errorf("GetByID(%d) error = %v, want ErrCategoryNotFound", id, err)
			}
		})
	}
}

// TestGetBySlugRefusesAMalformedSlugAsNotFound covers the collapse of two cases
// into one answer. "Travel!" cannot match a row, and a reader following a stale
// link has no use for a message about the hyphen rule.
func TestGetBySlugRefusesAMalformedSlugAsNotFound(t *testing.T) {
	service, store := newService(t)

	for _, slug := range []string{"", "Travel", "a b", "旅行", strings.Repeat("a", 65)} {
		_, err := service.GetBySlug(context.Background(), slug)
		if !errors.Is(err, taxonomy.ErrCategoryNotFound) {
			t.Errorf("GetBySlug(%q) error = %v, want ErrCategoryNotFound", slug, err)
		}
	}

	// And it cost no query: the store was never asked.
	if store.Len() != 0 {
		t.Fatal("the fake was seeded; this test needs an empty one")
	}
}

// TestResolveSlugDistinguishesUnknownFromEmpty is the reason ResolveSlug exists.
// /entries?category=nope and /entries?category=an-empty-category are different
// situations, and flattening the first to an id of 0 would answer it with every
// entry on the site.
func TestResolveSlugDistinguishesUnknownFromEmpty(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 7, Name: "旅行", Slug: "travel"})

	id, err := service.ResolveSlug(context.Background(), "travel")
	if err != nil {
		t.Fatalf("ResolveSlug: %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}

	if _, err := service.ResolveSlug(context.Background(), "nope"); !errors.Is(err, taxonomy.ErrCategoryNotFound) {
		t.Errorf("unknown slug error = %v, want ErrCategoryNotFound", err)
	}
}

// TestListsReturnDisplayOrder covers the ordering both reads share: sort_order
// ascending, id ascending to break ties. Seeded out of order so passing means the
// order was produced rather than preserved.
func TestListsReturnDisplayOrder(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 3, Name: "third", Slug: "third", SortOrder: 20})
	store.Seed(taxonomy.Category{ID: 1, Name: "first", Slug: "first", SortOrder: 10})
	store.Seed(taxonomy.Category{ID: 2, Name: "second", Slug: "second", SortOrder: 10})

	want := []string{"first", "second", "third"}

	t.Run("admin list", func(t *testing.T) {
		categories, err := service.List(context.Background())
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		for i, slug := range want {
			if categories[i].Slug != slug {
				t.Errorf("position %d = %q, want %q", i, categories[i].Slug, slug)
			}
		}
	})

	t.Run("public list", func(t *testing.T) {
		categories, err := service.ListWithCounts(context.Background())
		if err != nil {
			t.Fatalf("ListWithCounts: %v", err)
		}
		for i, slug := range want {
			if categories[i].Slug != slug {
				t.Errorf("position %d = %q, want %q", i, categories[i].Slug, slug)
			}
		}
	})
}

// TestListWithCountsListsEmptyCategories covers the LEFT JOIN's contract as the
// service passes it on: a category holding nothing is listed with a count of zero,
// not omitted. An INNER JOIN would drop it, and a new category would be invisible
// until someone filed something under it.
func TestListWithCountsListsEmptyCategories(t *testing.T) {
	service, store := newService(t)
	store.Seed(taxonomy.Category{ID: 1, Name: "full", Slug: "full"})
	store.Seed(taxonomy.Category{ID: 2, Name: "empty", Slug: "empty"})
	store.EntryCounts[1] = 3

	categories, err := service.ListWithCounts(context.Background())
	if err != nil {
		t.Fatalf("ListWithCounts: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("got %d categories, want 2", len(categories))
	}

	counts := map[string]int64{}
	for _, category := range categories {
		counts[category.Slug] = category.EntryCount
	}
	if counts["full"] != 3 {
		t.Errorf("full count = %d, want 3", counts["full"])
	}
	if counts["empty"] != 0 {
		t.Errorf("empty count = %d, want 0", counts["empty"])
	}
}

// TestStoreFailuresReachTheCaller covers the difference between "no such
// category" and "the database is unreachable". A storage failure reported as
// not-found would have an editor believe their category was deleted.
func TestStoreFailuresReachTheCaller(t *testing.T) {
	boom := errors.New("connection refused")

	for _, tc := range []struct {
		name string
		fail func(*taxonomytest.Store)
		call func(*taxonomy.Service) error
	}{
		{
			// SlugExistsAlwaysFree as well, because the seeded category below holds the
			// slug validCreate asks for. Without it the pre-check refuses this as a
			// conflict and the write is never reached, which would make the test pass
			// for the wrong reason.
			"create write",
			func(s *taxonomytest.Store) { s.FailCreate, s.SlugExistsAlwaysFree = boom, true },
			func(s *taxonomy.Service) error {
				_, err := s.Create(context.Background(), validCreate())
				return err
			},
		},
		{
			"create pre-check",
			func(s *taxonomytest.Store) { s.FailSlugExists = boom },
			func(s *taxonomy.Service) error {
				_, err := s.Create(context.Background(), validCreate())
				return err
			},
		},
		{
			"update write",
			func(s *taxonomytest.Store) { s.FailUpdate = boom },
			func(s *taxonomy.Service) error {
				_, err := s.Update(context.Background(), 1, taxonomy.UpdateInput{Name: pointer("x")})
				return err
			},
		},
		{
			"update pre-check",
			func(s *taxonomytest.Store) { s.FailSlugExistsExcluding = boom },
			func(s *taxonomy.Service) error {
				_, err := s.Update(context.Background(), 1, taxonomy.UpdateInput{Slug: pointer("other")})
				return err
			},
		},
		{
			"delete",
			func(s *taxonomytest.Store) { s.FailDelete = boom },
			func(s *taxonomy.Service) error { return s.Delete(context.Background(), 1) },
		},
		{
			"get by id",
			func(s *taxonomytest.Store) { s.FailGetByID = boom },
			func(s *taxonomy.Service) error {
				_, err := s.GetByID(context.Background(), 1)
				return err
			},
		},
		{
			"resolve slug",
			func(s *taxonomytest.Store) { s.FailGetBySlug = boom },
			func(s *taxonomy.Service) error {
				_, err := s.ResolveSlug(context.Background(), "travel")
				return err
			},
		},
		{
			"admin list",
			func(s *taxonomytest.Store) { s.FailList = boom },
			func(s *taxonomy.Service) error {
				_, err := s.List(context.Background())
				return err
			},
		},
		{
			"public list",
			func(s *taxonomytest.Store) { s.FailListWithCounts = boom },
			func(s *taxonomy.Service) error {
				_, err := s.ListWithCounts(context.Background())
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, store := newService(t)
			store.Seed(taxonomy.Category{ID: 1, Name: "旅行", Slug: "travel"})
			tc.fail(store)

			err := tc.call(service)
			if !errors.Is(err, boom) {
				t.Fatalf("error = %v, want the storage failure", err)
			}
			// The failure must not be dressed up as a domain outcome, or the adapter
			// above would answer 404 or 409 for an unreachable database.
			if errors.Is(err, taxonomy.ErrCategoryNotFound) || errors.Is(err, taxonomy.ErrSlugTaken) {
				t.Errorf("a storage failure was reported as a domain error: %v", err)
			}
		})
	}
}

// pointer is the address of a value, for building the all-pointer UpdateInput
// inline. A literal cannot be addressed, and a named variable per field would
// double the length of every table entry.
func pointer[T any](v T) *T {
	return &v
}
