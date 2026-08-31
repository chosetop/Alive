package taxonomyhttp_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
	"github.com/p30huiwei/alive/backend/internal/taxonomy/taxonomytest"
	"github.com/p30huiwei/alive/backend/internal/taxonomyhttp"
)

func init() {
	// Off, or every test writes gin's request log to the test output.
	gin.SetMode(gin.TestMode)
}

// newTestServer builds the real routes over a fake store.
//
// No author parameter, unlike entryhttp: a category has no owner, so the only
// thing authentication decides here is whether a request may write at all, and
// that is the guard's business rather than this handler's.
func newTestServer(t *testing.T) (http.Handler, *taxonomytest.Store) {
	t.Helper()

	store := taxonomytest.NewStore()
	service := taxonomy.NewService(store,
		taxonomy.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))

	handler := taxonomyhttp.NewHandler(service, slog.New(slog.NewTextHandler(io.Discard, nil)))

	engine := gin.New()
	api := engine.Group("/api/v1")

	// A guard that passes everything. What RequireAuth decides is authhttp's
	// business and is tested there.
	pass := func(c *gin.Context) { c.Next() }

	handler.Register(api, pass)
	handler.RegisterAdmin(api, pass)

	return engine, store
}

func do(t *testing.T, handler http.Handler, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// decodeEnvelope decodes a response body as the raw envelope.
//
// A map rather than a typed struct, because these tests assert which keys ship and
// a struct would silently accept a body missing half of them.
func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not a JSON object: %v\nbody: %s", err, rec.Body.String())
	}
	return envelope
}

func dataObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()

	envelope := decodeEnvelope(t, rec)
	raw, ok := envelope["data"]
	if !ok {
		t.Fatalf("response has no \"data\" key: %s", rec.Body.String())
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("\"data\" is not an object: %v", err)
	}
	return object
}

func dataArray(t *testing.T, rec *httptest.ResponseRecorder) []map[string]json.RawMessage {
	t.Helper()

	envelope := decodeEnvelope(t, rec)
	raw, ok := envelope["data"]
	if !ok {
		t.Fatalf("response has no \"data\" key: %s", rec.Body.String())
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("\"data\" is not an array: %v\nbody: %s", err, rec.Body.String())
	}
	return items
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	envelope := decodeEnvelope(t, rec)
	raw, ok := envelope["error"]
	if !ok {
		t.Fatalf("response has no \"error\" key: %s", rec.Body.String())
	}

	var detail struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("\"error\" is not the expected shape: %v", err)
	}
	return detail.Code
}

func errorFields(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()

	envelope := decodeEnvelope(t, rec)
	var detail struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(envelope["error"], &detail); err != nil {
		t.Fatalf("\"error\" is not the expected shape: %v", err)
	}
	return detail.Fields
}

func stringField(t *testing.T, object map[string]json.RawMessage, name string) string {
	t.Helper()

	raw, ok := object[name]
	if !ok {
		t.Fatalf("object has no %q key", name)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("%q is not a string: %v", name, err)
	}
	return value
}

func numberField(t *testing.T, object map[string]json.RawMessage, name string) int64 {
	t.Helper()

	raw, ok := object[name]
	if !ok {
		t.Fatalf("object has no %q key", name)
	}
	var value int64
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("%q is not a number: %v", name, err)
	}
	return value
}

const validCreateBody = `{"world":"journal","name":"旅行","slug":"travel","description":"places","sort_order":10}`

func TestCreateReturns201WithEveryField(t *testing.T) {
	handler, store := newTestServer(t)

	rec := do(t, handler, http.MethodPost, "/api/v1/categories", validCreateBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	if got := stringField(t, data, "name"); got != "旅行" {
		t.Errorf("name = %q, want 旅行", got)
	}
	if got := stringField(t, data, "slug"); got != "travel" {
		t.Errorf("slug = %q, want travel", got)
	}
	if got := stringField(t, data, "world"); got != "journal" {
		t.Errorf("world = %q, want journal", got)
	}
	if got := numberField(t, data, "sort_order"); got != 10 {
		t.Errorf("sort_order = %d, want 10", got)
	}
	if numberField(t, data, "id") == 0 {
		t.Error("the response carries no id, so a client cannot address the category it just made")
	}
	if store.CreateCalls != 1 {
		t.Errorf("the category was written %d times, want 1", store.CreateCalls)
	}

	// The create response is the admin shape, so it carries timestamps and no
	// count. A count here would mean the extra join on every write.
	if _, present := data["entry_count"]; present {
		t.Error("the write response carries entry_count")
	}
	if _, present := data["created_at"]; !present {
		t.Error("the write response carries no created_at")
	}
}

func TestPublicAndAdminListsRequireAWorldFilter(t *testing.T) {
	handler, _ := newTestServer(t)

	for _, path := range []string{"/api/v1/categories", "/api/v1/admin/categories"} {
		rec := do(t, handler, http.MethodGet, path, "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400\nbody: %s", path, rec.Code, rec.Body.String())
		}
		if fields := errorFields(t, rec); fields["world"] == "" {
			t.Fatalf("%s fields = %v, want world error", path, fields)
		}
	}
}

// TestCreateRequiresNameAndSlug covers the two binding tags. Both are refused
// before the service is reached, which is why the call count is asserted.
func TestCreateRequiresNameAndSlug(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"no name", `{"slug":"travel"}`},
		{"no slug", `{"name":"旅行"}`},
		{"neither", `{}`},
		{"not JSON", `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newTestServer(t)

			rec := do(t, handler, http.MethodPost, "/api/v1/categories", tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
			}
			if store.CreateCalls != 0 {
				t.Errorf("a refused body reached the write, %d calls", store.CreateCalls)
			}
		})
	}
}

// TestCreateRefusesABadSlugNamingTheField covers the mapping from a domain error
// onto a 400 that says which field to fix. A bare 400 would leave a client
// guessing between the name and the slug.
func TestCreateRefusesABadSlugNamingTheField(t *testing.T) {
	handler, _ := newTestServer(t)

	rec := do(t, handler, http.MethodPost, "/api/v1/categories",
		`{"world":"journal","name":"旅行","slug":"Not A Slug"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if fields := errorFields(t, rec); fields["slug"] == "" {
		t.Errorf("the error names no slug field: %s", rec.Body.String())
	}
}

// TestCreateRefusesADuplicateSlugWith409 covers the one conflict that is not a
// 400. The request is well formed and would succeed with a different slug, and a
// client can act on that difference.
func TestCreateRefusesADuplicateSlugWith409(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{World: contentworld.Journal, Name: "existing", Slug: "travel"})

	rec := do(t, handler, http.MethodPost, "/api/v1/categories", validCreateBody)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "CONFLICT" {
		t.Errorf("code = %q, want CONFLICT", code)
	}
}

// TestPublicListCarriesCountsAndNoTimestamps covers the split between the two
// list shapes. A reader gets what renders a menu; when a menu label was last
// edited is not part of that.
func TestPublicListCarriesCountsAndNoTimestamps(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel", SortOrder: 10})
	store.EntryCounts[1] = 4

	rec := do(t, handler, http.MethodGet, "/api/v1/categories?world=journal", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	items := dataArray(t, rec)
	if len(items) != 1 {
		t.Fatalf("got %d categories, want 1", len(items))
	}
	if got := numberField(t, items[0], "entry_count"); got != 4 {
		t.Errorf("entry_count = %d, want 4", got)
	}
	for _, absent := range []string{"created_at", "updated_at"} {
		if _, present := items[0][absent]; present {
			t.Errorf("the public list carries %s", absent)
		}
	}
	// The id does ship, unlike an entry's: it is how the entry list's filter
	// addresses a category, and it reveals only how many categories exist.
	if numberField(t, items[0], "id") != 1 {
		t.Error("the public list carries no id, so nothing can address a category")
	}
}

// TestPublicListIsNotPaginated covers the decision to serve the whole set. A meta
// block reporting page 1 of 1 for a collection that is never paged is noise a
// client has to read past, so the envelope must not carry one.
func TestPublicListIsNotPaginated(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel"})

	rec := do(t, handler, http.MethodGet, "/api/v1/categories?world=journal", "")

	if _, present := decodeEnvelope(t, rec)["meta"]; present {
		t.Errorf("the category list carries pagination meta: %s", rec.Body.String())
	}
}

// TestEmptyListsSerialiseAsAnArray covers both lists. A nil slice marshals to
// null, and a client iterating "data" would need a special case for "no
// categories" that it needs nowhere else.
func TestEmptyListsSerialiseAsAnArray(t *testing.T) {
	handler, _ := newTestServer(t)

	for _, path := range []string{"/api/v1/categories?world=journal", "/api/v1/admin/categories?world=journal"} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, path, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
			}

			raw := decodeEnvelope(t, rec)["data"]
			if string(raw) != "[]" {
				t.Errorf("data = %s, want []", raw)
			}
		})
	}
}

// TestAdminListCarriesTimestampsAndNoCounts is the other half of the split.
func TestAdminListCarriesTimestampsAndNoCounts(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel"})
	store.EntryCounts[1] = 4

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/categories?world=journal", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	items := dataArray(t, rec)
	if len(items) != 1 {
		t.Fatalf("got %d categories, want 1", len(items))
	}
	if _, present := items[0]["entry_count"]; present {
		t.Error("the admin list carries entry_count, which costs a join it does not need")
	}
	for _, required := range []string{"created_at", "updated_at"} {
		if _, present := items[0][required]; !present {
			t.Errorf("the admin list carries no %s", required)
		}
	}
}

// TestListsReturnDisplayOrder covers the order both lists share. Seeded out of
// order, so passing means the order was produced rather than preserved.
func TestListsReturnDisplayOrder(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 3, World: contentworld.Journal, Name: "third", Slug: "third", SortOrder: 20})
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "first", Slug: "first", SortOrder: 10})
	store.Seed(taxonomy.Category{ID: 2, World: contentworld.Journal, Name: "second", Slug: "second", SortOrder: 10})

	for _, path := range []string{"/api/v1/categories?world=journal", "/api/v1/admin/categories?world=journal"} {
		t.Run(path, func(t *testing.T) {
			items := dataArray(t, do(t, handler, http.MethodGet, path, ""))
			for i, want := range []string{"first", "second", "third"} {
				if got := stringField(t, items[i], "slug"); got != want {
					t.Errorf("position %d = %q, want %q", i, got, want)
				}
			}
		})
	}
}

func TestUpdateAppliesOnlySubmittedFields(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{
		ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel",
		Description: "keep me", SortOrder: 10,
	})

	rec := do(t, handler, http.MethodPatch, "/api/v1/categories/1", `{"name":"远行"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	if got := stringField(t, data, "name"); got != "远行" {
		t.Errorf("name = %q, want 远行", got)
	}
	if got := stringField(t, data, "description"); got != "keep me" {
		t.Errorf("description = %q; an unmentioned field was overwritten", got)
	}
}

// TestUpdateClearsADescriptionWithAnEmptyString covers the convention the DTO
// documents. `null` is indistinguishable from an omitted key, so "" is what
// clears a field, and this is the assertion that keeps that reachable over HTTP.
func TestUpdateClearsADescriptionWithAnEmptyString(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel", Description: "remove me"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/categories/1", `{"description":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if got := stringField(t, dataObject(t, rec), "description"); got != "" {
		t.Errorf("description = %q, want it cleared", got)
	}
	if !store.LastUpdate.SetDescription {
		t.Error("SetDescription is false, so the clear would not reach the column")
	}
}

// TestUpdateWithNullLeavesTheFieldAlone is the other half of that convention, and
// the one a client is most likely to get wrong. An explicit null must not clear.
func TestUpdateWithNullLeavesTheFieldAlone(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel", Description: "keep me"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/categories/1",
		`{"name":"远行","description":null}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if got := stringField(t, dataObject(t, rec), "description"); got != "keep me" {
		t.Errorf("description = %q; null cleared a field instead of leaving it alone", got)
	}
	if store.LastUpdate.SetDescription {
		t.Error("null was read as a submitted value")
	}
}

func TestUpdateRefusesABodyThatChangesNothing(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/categories/1", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.UpdateCalls != 0 {
		t.Errorf("an empty update reached storage, %d calls", store.UpdateCalls)
	}
}

func TestUpdateOfAMissingCategoryIs404(t *testing.T) {
	handler, _ := newTestServer(t)

	rec := do(t, handler, http.MethodPatch, "/api/v1/categories/99", `{"name":"远行"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "NOT_FOUND" {
		t.Errorf("code = %q, want NOT_FOUND", code)
	}
}

func TestUpdateRefusesAnotherCategorysSlugWith409(t *testing.T) {
	handler, _ := newTestServer(t)
	rec := do(t, handler, http.MethodPost, "/api/v1/categories", validCreateBody)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup create failed: %s", rec.Body.String())
	}
	if rec = do(t, handler, http.MethodPost, "/api/v1/categories",
		`{"world":"journal","name":"杂记","slug":"notes"}`); rec.Code != http.StatusCreated {
		t.Fatalf("setup create failed: %s", rec.Body.String())
	}

	id := numberField(t, dataObject(t, rec), "id")
	rec = do(t, handler, http.MethodPatch, "/api/v1/categories/"+itoa(id), `{"slug":"travel"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
}

// TestDeleteReturns204ThenNotFound covers both calls. The first removes the
// category, and the second must say it is gone rather than report success again.
func TestDeleteReturns204ThenNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel"})

	rec := do(t, handler, http.MethodDelete, "/api/v1/categories/1", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204\nbody: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("a 204 carries a body: %s", rec.Body.String())
	}
	if store.Len() != 0 {
		t.Errorf("the category is still stored, %d remain", store.Len())
	}

	if rec = do(t, handler, http.MethodDelete, "/api/v1/categories/1", ""); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

// TestGetByIDReturnsTheAdminShape covers the read the edit form loads. By id
// rather than slug, because the slug is one of the things being edited.
func TestGetByIDReturnsTheAdminShape(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel", Description: "places"})

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/categories/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	if got := stringField(t, data, "slug"); got != "travel" {
		t.Errorf("slug = %q, want travel", got)
	}
	if _, present := data["created_at"]; !present {
		t.Error("the admin read carries no created_at")
	}
}

func TestGetByIDOfAMissingCategoryIs404(t *testing.T) {
	handler, _ := newTestServer(t)

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/categories/99", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
	}
}

// TestANonNumericIDIs400NotNotFound covers the difference between a path that
// names no category and one that names a missing category. A 404 for "abc" reads
// as "that category is gone" for a request that never identified one.
func TestANonNumericIDIs400NotNotFound(t *testing.T) {
	handler, store := newTestServer(t)
	store.Seed(taxonomy.Category{ID: 1, World: contentworld.Journal, Name: "旅行", Slug: "travel"})

	for _, tc := range []struct {
		name, method, path, body string
	}{
		{"read", http.MethodGet, "/api/v1/admin/categories/abc", ""},
		{"update", http.MethodPatch, "/api/v1/categories/abc", `{"name":"x"}`},
		{"delete", http.MethodDelete, "/api/v1/categories/abc", ""},
		{"zero", http.MethodDelete, "/api/v1/categories/0", ""},
		{"negative", http.MethodDelete, "/api/v1/categories/-1", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, handler, tc.method, tc.path, tc.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
			}
			if fields := errorFields(t, rec); fields["id"] == "" {
				t.Errorf("the error names no id field: %s", rec.Body.String())
			}
		})
	}

	if store.DeleteCalls != 0 || store.UpdateCalls != 0 {
		t.Errorf("an unparseable id reached storage: %d deletes, %d updates",
			store.DeleteCalls, store.UpdateCalls)
	}
}

// TestRegisterRefusesAMissingGuard covers the mistake that would make the write
// endpoints public. It has to fail at startup, because nothing about the running
// service would look wrong afterwards.
func TestRegisterRefusesAMissingGuard(t *testing.T) {
	newHandler := func() *taxonomyhttp.Handler {
		return taxonomyhttp.NewHandler(
			taxonomy.NewService(taxonomytest.NewStore()),
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}

	for _, tc := range []struct {
		name     string
		register func(*taxonomyhttp.Handler, *gin.RouterGroup)
	}{
		{"write routes", func(h *taxonomyhttp.Handler, g *gin.RouterGroup) { h.Register(g, nil) }},
		{"admin routes", func(h *taxonomyhttp.Handler, g *gin.RouterGroup) { h.RegisterAdmin(g, nil) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Error("a nil guard was accepted, leaving these routes public")
				}
			}()
			tc.register(newHandler(), gin.New().Group("/api/v1"))
		})
	}
}

// TestStoreFailureIsNotReportedAsNotFound covers the difference between "no such
// category" and "the database is unreachable", and that the driver's message stays
// out of the response: it hands table and column names to whoever asked.
func TestStoreFailureIsNotReportedAsNotFound(t *testing.T) {
	store := taxonomytest.NewStore()
	store.FailListWithCounts = errors.New("connection refused")

	engine := gin.New()
	taxonomyhttp.NewHandler(
		taxonomy.NewService(store),
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	).Register(engine.Group("/api/v1"), func(c *gin.Context) { c.Next() })

	rec := do(t, engine, http.MethodGet, "/api/v1/categories?world=journal", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500\nbody: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("the response carries the driver error: %s", rec.Body.String())
	}
}

// itoa keeps the id-in-a-path lines short.
func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
