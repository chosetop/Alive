package entryhttp_test

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
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/entry/entrytest"
	"github.com/p30huiwei/alive/backend/internal/entryhttp"
)

// testAuthorID is the account every authenticated request in this file acts as.
const testAuthorID int64 = 42

// fixedTime is the clock the service uses here, so an assertion about
// published_at compares against a known value rather than a window around now.
var fixedTime = time.Date(2026, 8, 24, 15, 4, 5, 0, time.UTC)

func init() {
	// Off, or every test writes gin's request log to the test output.
	gin.SetMode(gin.TestMode)
}

// testCategorySlug resolves to testCategoryID here. Any other slug is unknown, so
// one server covers both the filter working and the filter naming nothing.
const (
	testCategorySlug       = "travel"
	testCategoryID   int64 = 7
)

// newTestServer builds the real routes over a fake store.
//
// authorID is who the request acts as; zero means the request carries no
// identity, which is what the create route must refuse.
func newTestServer(t *testing.T, authorID int64) (http.Handler, *entrytest.Store) {
	t.Helper()

	store := entrytest.NewStore()
	service := entry.NewService(store, entry.WithClock(func() time.Time { return fixedTime }))

	handler := entryhttp.NewHandler(
		service,
		func(*gin.Context) (int64, bool) { return authorID, authorID != 0 },
		testCategoryResolver,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	engine := gin.New()
	api := engine.Group("/api/v1")

	// A guard that passes everything. What RequireAuth decides is authhttp's
	// business and is tested there; this file is about what the entry routes do
	// once a request is through it.
	pass := func(c *gin.Context) { c.Next() }

	handler.Register(api, pass)
	handler.RegisterAdmin(api, pass)

	return engine, store
}

// testCategoryResolver stands in for the one the router builds over taxonomy.
//
// It returns an *apperr.Error for an unknown slug, exactly as the real one does.
// That is what keeps this a test of the handler and not of taxonomy: no categories
// table, no service, and no import of the package the adapter is not allowed to
// depend on.
func testCategoryResolver(_ *gin.Context, slug string) (int64, error) {
	if slug != testCategorySlug {
		return 0, apperr.NotFound("no category matches this slug")
	}
	return testCategoryID, nil
}

// do performs a request against the handler and returns the recorder.
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
// Decoded into a map rather than a typed struct on purpose: these tests assert
// which keys ship, and a struct would silently accept a body missing half of
// them.
func decodeEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("response is not a JSON object: %v\nbody: %s", err, rec.Body.String())
	}
	return envelope
}

// dataObject returns the "data" value as an object, failing if the envelope
// carries anything else at the top level.
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

// dataArray returns the "data" value as an array of objects.
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

// errorCode returns the code from a failure envelope.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()

	envelope := decodeEnvelope(t, rec)
	raw, ok := envelope["error"]
	if !ok {
		t.Fatalf("response has no \"error\" key: %s", rec.Body.String())
	}

	var detail struct {
		Code   string            `json:"code"`
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal(raw, &detail); err != nil {
		t.Fatalf("\"error\" is not the expected shape: %v", err)
	}
	return detail.Code
}

// errorFields returns the per-field detail from a failure envelope.
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

// stringField reads a string value out of a decoded JSON object.
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

func intField(t *testing.T, object map[string]json.RawMessage, name string) int {
	t.Helper()
	raw, ok := object[name]
	if !ok {
		t.Fatalf("object has no %q key", name)
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("%q is not an integer: %v", name, err)
	}
	return value
}

// visibilityCase is one seeded entry and what the two public endpoints owe it.
//
// Two flags rather than one "visible", because the two reads no longer agree. An
// unlisted entry is absent from the list and reachable by its URL, which is the
// whole of what unlisted means, and a single flag could not express it.
//
// Each hidden case differs from the visible one in exactly one respect, so a
// filter that drops one of its conditions fails a named subtest rather than
// producing one vague count mismatch.
type visibilityCase struct {
	name   string
	slug   string
	status entry.Status
	vis    entry.Visibility

	// inList is whether GET /entries carries it.
	inList bool

	// byLink is whether GET /entries/:slug returns it. Never true when inList is
	// false except for unlisted: anything else that is missing from the list is
	// missing because it is not published, and a URL cannot change that.
	byLink bool
}

var visibilityCases = []visibilityCase{
	{"published public", "published-public", entry.StatusPublished, entry.VisibilityPublic, true, true},
	{"draft", "a-draft", entry.StatusDraft, entry.VisibilityPublic, false, false},
	{"archived", "archived-one", entry.StatusArchived, entry.VisibilityPublic, false, false},
	{"private", "private-one", entry.StatusPublished, entry.VisibilityPrivate, false, false},

	// The one asymmetric row. Not access control: the slug is guessable, and this
	// asserts only that a link opens while the list stays clean.
	{"unlisted", "unlisted-one", entry.StatusPublished, entry.VisibilityUnlisted, false, true},
}

// TestPublicEndpointsHideEverythingUnpublished is the test the whole stage turns
// on. The filter is enforced in SQL and again by the fake, and this asserts it
// survives the last translation into a response body: the reader sees what comes
// out of the endpoint, not what the store returned.
func TestPublicEndpointsHideEverythingUnpublished(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	for i, tc := range visibilityCases {
		store.Seed(entry.Entry{
			ID: int64(i + 1), Slug: tc.slug, Title: tc.name,
			Status: tc.status, Visibility: tc.vis,
			ContentMD: "the body of " + tc.slug,
		})
	}

	t.Run("detail", func(t *testing.T) {
		for _, tc := range visibilityCases {
			t.Run(tc.name, func(t *testing.T) {
				rec := do(t, handler, http.MethodGet, "/api/v1/entries/"+tc.slug, "")

				if tc.byLink {
					if rec.Code != http.StatusOK {
						t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
					}
					if got := stringField(t, dataObject(t, rec), "slug"); got != tc.slug {
						t.Errorf("slug = %q, want %q", got, tc.slug)
					}
					return
				}

				// 404, and the same 404 an unused slug gets. 403 would confirm that
				// the slug holds something, which is how a draft title space gets
				// enumerated.
				if rec.Code != http.StatusNotFound {
					t.Fatalf("status = %d, want 404 for a %s entry\nbody: %s",
						rec.Code, tc.name, rec.Body.String())
				}
				if code := errorCode(t, rec); code != "NOT_FOUND" {
					t.Errorf("code = %q, want NOT_FOUND", code)
				}
				// The body must not appear anywhere in the response, including in an
				// error message.
				if strings.Contains(rec.Body.String(), "the body of") {
					t.Errorf("the response carries entry content: %s", rec.Body.String())
				}
			})
		}
	})

	t.Run("an unused slug answers exactly as a hidden entry does", func(t *testing.T) {
		hidden := do(t, handler, http.MethodGet, "/api/v1/entries/a-draft", "")
		unused := do(t, handler, http.MethodGet, "/api/v1/entries/never-used", "")

		if hidden.Code != unused.Code {
			t.Errorf("status %d for a draft and %d for an unused slug; the two must match",
				hidden.Code, unused.Code)
		}
		if hidden.Body.String() != unused.Body.String() {
			t.Errorf("bodies differ:\n draft:  %s\n unused: %s", hidden.Body, unused.Body)
		}
	})

	t.Run("list carries only the published public entry", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/entries", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}

		items := dataArray(t, rec)
		seen := make(map[string]bool, len(items))
		for _, item := range items {
			seen[stringField(t, item, "slug")] = true
		}

		for _, tc := range visibilityCases {
			switch {
			case tc.inList && !seen[tc.slug]:
				t.Errorf("the published public entry %q is absent from the list", tc.slug)
			case !tc.inList && seen[tc.slug]:
				t.Errorf("a %s entry appears in the public list", tc.name)
			}
		}

		if len(items) != 1 {
			t.Errorf("list returned %d entries, want 1", len(items))
		}
	})
}

// TestListOmitsContentMD covers the one field that must never reach a list
// response. The body is many times the size of everything else in the row, and
// the front page renders none of it.
func TestListOmitsContentMD(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	store.Seed(entry.Entry{
		ID: 1, Slug: "published-public", Title: "visible",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
		ContentMD: "SHOULD-NOT-APPEAR-IN-A-LIST",
	})

	rec := do(t, handler, http.MethodGet, "/api/v1/entries", "")

	items := dataArray(t, rec)
	if len(items) != 1 {
		t.Fatalf("list returned %d entries, want 1", len(items))
	}
	if _, present := items[0]["content_md"]; present {
		t.Error("the list response carries content_md")
	}
	if strings.Contains(rec.Body.String(), "SHOULD-NOT-APPEAR-IN-A-LIST") {
		t.Errorf("the body text appears in the list response: %s", rec.Body.String())
	}

	// And the detail response does carry it, or the field would be unreachable.
	detail := do(t, handler, http.MethodGet, "/api/v1/entries/published-public", "")
	if got := stringField(t, dataObject(t, detail), "content_md"); got != "SHOULD-NOT-APPEAR-IN-A-LIST" {
		t.Errorf("detail content_md = %q, want the body", got)
	}
}

// TestListPagination covers what the query string does to the page, including the
// values a client should not be able to turn into a full-table scan.
func TestListPagination(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	// Three visible entries, dated so their order is defined.
	for i := 1; i <= 3; i++ {
		store.Seed(entry.Entry{
			ID: int64(i), Slug: "entry-" + strconv.Itoa(i), Title: "visible",
			Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
			HappenedAt: time.Date(2020+i, 1, 1, 0, 0, 0, 0, time.UTC),
		})
	}

	cases := []struct {
		name         string
		query        string
		wantStatus   int
		wantCount    int
		wantPage     int
		wantPageSize int
	}{
		{"no query gets the defaults", "", http.StatusOK, 3, 1, entry.DefaultPageSize},
		{"explicit page and size", "?page=1&page_size=2", http.StatusOK, 2, 1, 2},
		{"the second page", "?page=2&page_size=2", http.StatusOK, 1, 2, 2},
		// Empty, not an error: the collection shrinks when an entry is unpublished,
		// so a page that existed a moment ago legitimately may not now.
		{"past the end is an empty page", "?page=99&page_size=2", http.StatusOK, 0, 99, 2},
		// Clamped rather than refused. page_size=100000 is a request to serialise
		// the whole table, and answering 400 would make a client handle a limit it
		// can simply be given.
		{"an over-large size is clamped", "?page_size=100000", http.StatusOK, 3, 1, entry.MaxPageSize},
		{"a negative page becomes the first", "?page=-3", http.StatusOK, 3, 1, entry.DefaultPageSize},
		// A mistake in the client rather than a request for the default. Serving
		// page 1 silently would hide it.
		{"a non-numeric page is refused", "?page=abc", http.StatusBadRequest, 0, 0, 0},
		{"a non-numeric size is refused", "?page_size=lots", http.StatusBadRequest, 0, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, handler, http.MethodGet, "/api/v1/entries"+tc.query, "")

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				if code := errorCode(t, rec); code != "INVALID_INPUT" {
					t.Errorf("code = %q, want INVALID_INPUT", code)
				}
				return
			}

			if got := len(dataArray(t, rec)); got != tc.wantCount {
				t.Errorf("returned %d entries, want %d", got, tc.wantCount)
			}

			var envelope struct {
				Meta struct {
					Page     int   `json:"page"`
					PageSize int   `json:"page_size"`
					Total    int64 `json:"total"`
				} `json:"meta"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode meta: %v", err)
			}
			if envelope.Meta.Page != tc.wantPage {
				t.Errorf("meta.page = %d, want %d", envelope.Meta.Page, tc.wantPage)
			}
			if envelope.Meta.PageSize != tc.wantPageSize {
				t.Errorf("meta.page_size = %d, want %d", envelope.Meta.PageSize, tc.wantPageSize)
			}
			// The total describes the collection, not the page, and it counts only
			// what the filter admits. Reporting anything else would have a client
			// paging towards entries it can never see.
			if envelope.Meta.Total != 3 {
				t.Errorf("meta.total = %d, want 3", envelope.Meta.Total)
			}
		})
	}
}

// TestEmptyListIsAnArray covers the difference between [] and null. Every client
// can iterate the first; the second needs a special case in each one.
func TestEmptyListIsAnArray(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodGet, "/api/v1/entries", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	envelope := decodeEnvelope(t, rec)
	if got := string(envelope["data"]); got != "[]" {
		t.Errorf("data = %s, want []", got)
	}
}

// validCreateBody is a create request with every required field, for tests that
// vary one thing at a time.
const validCreateBody = `{
	"title": "京都的春天",
	"slug": "kyoto-spring",
	"content_md": "在鸭川边坐了一整个下午。"
}`

// TestCreateEmptyDraft protects the draft-first contract: creation must not
// require publication-stage fields, and the returned owner shape must expose
// the revision needed by the first save.
func TestCreateEmptyDraft(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", `{}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	for _, field := range []string{"title", "slug", "content_md"} {
		if got := stringField(t, data, field); got != "" {
			t.Errorf("%s = %q, want empty", field, got)
		}
	}
	if got := string(data["revision"]); got != "1" {
		t.Errorf("revision = %s, want 1", got)
	}
}

func TestCreate(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", validCreateBody)

	// 201, not 200. The response reports a resource that did not exist before the
	// request, and a client can tell a create from an idempotent update by it.
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)

	// Draft by default. An unwanted draft is invisible; an unwanted publication is
	// already in the feed and the RSS reader.
	if got := stringField(t, data, "status"); got != "draft" {
		t.Errorf("status = %q, want draft", got)
	}
	if got := stringField(t, data, "type"); got != "journal" {
		t.Errorf("type = %q, want journal", got)
	}
	if got := stringField(t, data, "visibility"); got != "public" {
		t.Errorf("visibility = %q, want public", got)
	}
	if got := string(data["meta"]); got != "{}" {
		t.Errorf("meta = %s, want {}", got)
	}
	// A draft has never been published, so it carries no publication time. Null
	// rather than the zero time: publishing "0001-01-01T00:00:00Z" would be a date
	// a client might render.
	if got := string(data["published_at"]); got != "null" {
		t.Errorf("published_at = %s, want null", got)
	}
	if got := string(data["happened_at"]); got != "null" {
		t.Errorf("happened_at = %s, want null", got)
	}

	// The author comes from the session. Asserted on what reached the store,
	// because the response deliberately does not carry an author id.
	if store.LastCreate.AuthorID != testAuthorID {
		t.Errorf("stored author_id = %d, want %d", store.LastCreate.AuthorID, testAuthorID)
	}

	t.Run("the word count is computed, not taken from the request", func(t *testing.T) {
		want := entry.CountWords("在鸭川边坐了一整个下午。")
		if store.LastCreate.WordCount != want {
			t.Errorf("word_count = %d, want %d", store.LastCreate.WordCount, want)
		}
	})
}

// TestCreateIgnoresClientSuppliedOwnership covers the two fields a client must
// not be able to set. An author_id in the body would let one account write
// entries attributed to another, and a word_count would let a client report a
// number that does not describe its own text.
func TestCreateIgnoresClientSuppliedOwnership(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", `{
		"title": "attempt",
		"slug": "attempt",
		"content_md": "one two three",
		"author_id": 9999,
		"word_count": 99999,
		"id": 4242
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	if store.LastCreate.AuthorID != testAuthorID {
		t.Errorf("stored author_id = %d, want %d: the body must not choose the author",
			store.LastCreate.AuthorID, testAuthorID)
	}
	if want := entry.CountWords("one two three"); store.LastCreate.WordCount != want {
		t.Errorf("stored word_count = %d, want %d: the body must not choose the count",
			store.LastCreate.WordCount, want)
	}
}

// TestCreateAcceptsHappenedAtAndMeta covers the two fields that are neither
// required nor plain strings.
func TestCreateAcceptsHappenedAtAndMeta(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", `{
		"title": "京都",
		"slug": "kyoto",
		"content_md": "body",
		"type": "travel",
		"happened_at": "2023-04-03T12:00:00Z",
		"meta": {"place": "京都", "rating": 5}
	}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	want := time.Date(2023, 4, 3, 12, 0, 0, 0, time.UTC)
	if !store.LastCreate.HappenedAt.Equal(want) {
		t.Errorf("stored happened_at = %v, want %v", store.LastCreate.HappenedAt, want)
	}

	// Decoded, not compared as bytes. Meta travels as raw JSON and nothing
	// promises the spacing a client sent survives, so an assertion on the text
	// would be testing the encoder.
	var meta struct {
		Place  string `json:"place"`
		Rating int    `json:"rating"`
	}
	if err := json.Unmarshal(store.LastCreate.Meta, &meta); err != nil {
		t.Fatalf("stored meta is not valid JSON: %v", err)
	}
	if meta.Place != "京都" || meta.Rating != 5 {
		t.Errorf("stored meta = %+v, want place 京都 and rating 5", meta)
	}
}

// TestCreateSlugConflict covers the response a client is most likely to hit, and
// the one it can act on: ask for a different slug.
func TestCreateSlugConflict(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	if rec := do(t, handler, http.MethodPost, "/api/v1/entries", validCreateBody); rec.Code != http.StatusCreated {
		t.Fatalf("first create: status = %d, want 201", rec.Code)
	}

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", validCreateBody)

	// 409, not 400. The request is well formed and would succeed with another
	// slug, which is a different situation from a malformed body.
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "CONFLICT" {
		t.Errorf("code = %q, want CONFLICT", code)
	}
	if fields := errorFields(t, rec); fields["slug"] == "" {
		t.Errorf("fields = %v, want the slug named", fields)
	}
}

// TestCreateRejectsInvalidInput covers each domain error creation can still
// produce for a partially filled draft and asserts the field at fault.
func TestCreateRejectsInvalidInput(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"malformed JSON", `{"title":`, ""},

		// Caught by the domain, and each names its field.
		{"uppercase slug", `{"title":"t","slug":"Kyoto","content_md":"b"}`, "slug"},
		{"slug with a space", `{"title":"t","slug":"kyoto spring","content_md":"b"}`, "slug"},
		{"chinese slug", `{"title":"t","slug":"京都","content_md":"b"}`, "slug"},
		{"unknown type", `{"title":"t","slug":"s","content_md":"b","type":"joural"}`, "type"},
		{"unknown visibility", `{"title":"t","slug":"s","content_md":"b","visibility":"hidden"}`, "visibility"},
		{"meta is an array", `{"title":"t","slug":"s","content_md":"b","meta":[1]}`, "meta"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, store := newTestServer(t, testAuthorID)

			rec := do(t, handler, http.MethodPost, "/api/v1/entries", tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
			}
			if code := errorCode(t, rec); code != "INVALID_INPUT" {
				t.Errorf("code = %q, want INVALID_INPUT", code)
			}
			if tc.wantField != "" {
				if fields := errorFields(t, rec); fields[tc.wantField] == "" {
					t.Errorf("fields = %v, want %q named", fields, tc.wantField)
				}
			}

			// Nothing was written. A refused request must not leave a row behind.
			if store.CreateCalls != 0 {
				t.Errorf("the store was asked to write %d times, want 0", store.CreateCalls)
			}
		})
	}
}

// TestCreateWithoutAnIdentity covers the case where the route's guard is present
// but the identity is missing, which is a wiring mistake rather than a client
// error. 401 would send an already-logged-in client to log in again.
func TestCreateWithoutAnIdentity(t *testing.T) {
	handler, store := newTestServer(t, 0)

	rec := do(t, handler, http.MethodPost, "/api/v1/entries", validCreateBody)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.CreateCalls != 0 {
		t.Errorf("an entry was written with no author, %d calls", store.CreateCalls)
	}
}

// TestRegisterRefusesAMissingGuard covers the mistake that would make the create
// endpoint public. Registering the route with a nil guard has to fail at startup,
// because nothing about the running service would look wrong afterwards.
func TestRegisterRefusesAMissingGuard(t *testing.T) {
	newHandler := func() *entryhttp.Handler {
		return entryhttp.NewHandler(
			entry.NewService(entrytest.NewStore()),
			func(*gin.Context) (int64, bool) { return testAuthorID, true },
			testCategoryResolver,
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
	}

	// Both registrations, because the admin one is the worse omission: its routes
	// are the only ones that can return a draft.
	for _, tc := range []struct {
		name     string
		register func(*entryhttp.Handler, *gin.RouterGroup)
	}{
		{"write routes", func(h *entryhttp.Handler, g *gin.RouterGroup) { h.Register(g, nil) }},
		{"admin routes", func(h *entryhttp.Handler, g *gin.RouterGroup) { h.RegisterAdmin(g, nil) }},
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

// TestUpdateWritesOnlySubmittedFields is the point of PATCH. A save that mentions
// the title must not touch the body, or two tabs open on one entry silently
// overwrite each other.
func TestUpdateWritesOnlySubmittedFields(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "original", Title: "Original", ContentMD: "the original body",
		Summary: "the original summary", WordCount: 3,
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"revision":1,"title":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	if !store.LastUpdate.SetTitle || store.LastUpdate.Title != "Renamed" {
		t.Errorf("the title was not submitted: %+v", store.LastUpdate)
	}
	// The Set flags rather than the returned entry: a store that wrote an empty body
	// would return one, and so would a store that left the body alone.
	if store.LastUpdate.SetContentMD {
		t.Error("an update naming only the title also wrote content_md")
	}
	if store.LastUpdate.SetSummary {
		t.Error("an update naming only the title also wrote the summary")
	}

	if got := stringField(t, dataObject(t, rec), "content_md"); got != "the original body" {
		t.Errorf("content_md = %q, want the body left alone", got)
	}
}

// TestUpdateRequiresRevision protects the compare-and-swap precondition at the
// HTTP boundary. A missing revision must be rejected before it can overwrite a
// newer edit.
func TestUpdateRequiresRevision(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Revision: 1, Slug: "original", Title: "Original"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"title":"Renamed"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_INPUT" {
		t.Errorf("code = %q, want INVALID_INPUT", code)
	}
	if store.UpdateCalls != 0 {
		t.Errorf("an update without a revision reached the store, %d calls", store.UpdateCalls)
	}
}

// TestUpdateStaleRevision protects the exact recovery contract consumed by the
// editor when another save wins first.
func TestUpdateStaleRevision(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Revision: 2, Slug: "original", Title: "Newer"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7",
		`{"revision":1,"title":"Stale"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	want := `{"error":{"code":"CONFLICT","message":"entry changed since it was loaded","fields":{"revision":"请重新载入或保留当前内容为恢复草稿"}}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s\nwant = %s", got, want)
	}
}

// TestUpdateClearsAFieldWithAnEmptyValue covers the other half of the pointer
// scheme: an empty string is a submitted value, not an absent one.
func TestUpdateClearsAFieldWithAnEmptyValue(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "original", Title: "Original", Summary: "to be removed",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"revision":1,"summary":""}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if !store.LastUpdate.SetSummary || store.LastUpdate.Summary != "" {
		t.Errorf("the summary was not cleared: %+v", store.LastUpdate)
	}
	if got := stringField(t, dataObject(t, rec), "summary"); got != "" {
		t.Errorf("summary = %q, want it cleared", got)
	}
}

// TestUpdateRecomputesTheWordCount covers the one derived field. A new body beside
// the old count misreports reading time, and the count is never taken from the
// client.
func TestUpdateRecomputesTheWordCount(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "original", Title: "Original", ContentMD: "one", WordCount: 1,
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7",
		`{"revision":1,"content_md":"one two three four","word_count":999}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.LastUpdate.WordCount != 4 {
		t.Errorf("word_count = %d, want 4 counted from the body", store.LastUpdate.WordCount)
	}
}

// TestUpdateRefusesStatus covers the endpoint boundary. Ignoring the field would
// answer 200 and leave the entry unpublished, and the client would never find the
// route that does publish.
func TestUpdateRefusesStatus(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Slug: "original", Title: "Original", Status: entry.StatusDraft})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"revision":1,"status":"published"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if _, ok := errorFields(t, rec)["status"]; !ok {
		t.Errorf("the error does not name the status field: %s", rec.Body.String())
	}
	if store.UpdateCalls != 0 {
		t.Errorf("the update ran anyway, %d calls", store.UpdateCalls)
	}
}

// TestUpdateRefusesAnEmptyBody covers a client that built its request wrongly. A
// 200 here would look like a save that worked.
func TestUpdateRefusesAnEmptyBody(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Slug: "original", Title: "Original"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"revision":1}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.UpdateCalls != 0 {
		t.Errorf("an empty update reached the store, %d calls", store.UpdateCalls)
	}
}

// TestUpdateKeepingItsOwnSlugIsNotAConflict covers the exclusion in the slug
// check. Without it every save that resubmitted the unchanged slug would be a 409.
func TestUpdateKeepingItsOwnSlugIsNotAConflict(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Slug: "keeps-this", Title: "Original"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7",
		`{"revision":1,"slug":"keeps-this","title":"Renamed"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if store.SlugExistsExcludingCalls != 1 {
		t.Errorf("the excluding check ran %d times, want 1", store.SlugExistsExcludingCalls)
	}
}

// TestUpdateRefusesASlugAnotherEntryHolds is the same conflict as on create, at
// 409 rather than 400: the request is well formed and another slug would work.
func TestUpdateRefusesASlugAnotherEntryHolds(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 7, Slug: "mine", Title: "Mine"})
	store.Seed(entry.Entry{ID: 8, Slug: "taken", Title: "Theirs"})

	rec := do(t, handler, http.MethodPatch, "/api/v1/entries/7", `{"revision":1,"slug":"taken"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	if _, ok := errorFields(t, rec)["slug"]; !ok {
		t.Errorf("the error does not name the slug: %s", rec.Body.String())
	}
}

// TestWriteRoutesRefuseAMalformedID covers every id-addressed route at once. A
// path segment that is not a number is a 400: there is no resource to be missing.
func TestWriteRoutesRefuseAMalformedID(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	for _, tc := range []struct{ method, target, body string }{
		{http.MethodPatch, "/api/v1/entries/abc", `{"title":"X"}`},
		{http.MethodDelete, "/api/v1/entries/abc", ""},
		{http.MethodPost, "/api/v1/entries/abc/publish", ""},
		{http.MethodPost, "/api/v1/entries/abc/unpublish", ""},
		{http.MethodPost, "/api/v1/entries/abc/archive", ""},
		{http.MethodGet, "/api/v1/admin/entries/abc", ""},
	} {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			rec := do(t, handler, tc.method, tc.target, tc.body)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
			}
			if _, ok := errorFields(t, rec)["id"]; !ok {
				t.Errorf("the error does not name the id: %s", rec.Body.String())
			}
		})
	}
}

// TestIDRoutesAnswer404ForAnAbsentEntry covers the same set against a well formed
// id that matches nothing.
func TestIDRoutesAnswer404ForAnAbsentEntry(t *testing.T) {
	handler, _ := newTestServer(t, testAuthorID)

	for _, tc := range []struct{ method, target, body string }{
		{http.MethodPatch, "/api/v1/entries/999", `{"revision":1,"title":"X"}`},
		{http.MethodDelete, "/api/v1/entries/999", ""},
		{http.MethodPost, "/api/v1/entries/999/publish", `{"revision":1}`},
		{http.MethodPost, "/api/v1/entries/999/unpublish", `{"revision":1}`},
		{http.MethodPost, "/api/v1/entries/999/archive", `{"revision":1}`},
		{http.MethodGet, "/api/v1/admin/entries/999", ""},
	} {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			rec := do(t, handler, tc.method, tc.target, tc.body)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404\nbody: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestDeleteRemovesTheEntryFromEveryRead covers what a soft delete means from
// outside: the second call is a 404, and the entry is gone from the admin list too.
func TestDeleteRemovesTheEntryFromEveryRead(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "goes-away", Title: "Goes away",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodDelete, "/api/v1/entries/7", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204\nbody: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 carries a body: %s", rec.Body.String())
	}

	// The injected clock, not time.Now: the delete timestamp is the service's
	// decision and a test has to be able to see which one was written.
	if !store.LastSoftDeleteAt.Equal(fixedTime) {
		t.Errorf("deleted_at = %v, want the injected clock %v", store.LastSoftDeleteAt, fixedTime)
	}

	t.Run("a second delete is a 404", func(t *testing.T) {
		if rec := do(t, handler, http.MethodDelete, "/api/v1/entries/7", ""); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("gone from the public read", func(t *testing.T) {
		if rec := do(t, handler, http.MethodGet, "/api/v1/entries/goes-away", ""); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("gone from the admin list", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries", "")
		if items := dataArray(t, rec); len(items) != 0 {
			t.Errorf("the admin list still reports %d entries", len(items))
		}
	})
}

// TestPublishStampsPublishedAtOnceOnly is the write-once rule, seen through the
// endpoints. An entry published, withdrawn and published again keeps its original
// date, so a typo fixed years later does not reappear at the top of the feed.
func TestPublishStampsPublishedAtOnceOnly(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "to-publish", Title: "To publish", ContentMD: "body",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/publish", `{"revision":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	first := stringField(t, dataObject(t, rec), "published_at")
	if first == "" {
		t.Fatal("published_at is empty after publishing")
	}
	if got := stringField(t, dataObject(t, rec), "status"); got != "published" {
		t.Errorf("status = %q, want published", got)
	}

	// A draft is invisible; publishing is what puts it on the site.
	t.Run("now on the public read", func(t *testing.T) {
		if rec := do(t, handler, http.MethodGet, "/api/v1/entries/to-publish", ""); rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})

	t.Run("republishing keeps the first date", func(t *testing.T) {
		do(t, handler, http.MethodPost, "/api/v1/entries/7/unpublish", `{"revision":2}`)
		rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/publish", `{"revision":3}`)

		if got := stringField(t, dataObject(t, rec), "published_at"); got != first {
			t.Errorf("published_at = %q, want the original %q", got, first)
		}
	})
}

// TestPublishIncomplete protects the point at which draft leniency ends. A
// body-less draft may be saved, but it may not become visible.
func TestPublishIncomplete(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Revision: 1, Slug: "incomplete", Title: "Incomplete",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/publish", `{"revision":1}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
	if code := errorCode(t, rec); code != "INVALID_INPUT" {
		t.Errorf("code = %q, want INVALID_INPUT", code)
	}
	if fields := errorFields(t, rec); fields["content_md"] == "" {
		t.Errorf("fields = %v, want content_md named", fields)
	}
}

// TestPublishStaleRevision protects transitions from publishing a version the
// editor did not review.
func TestPublishStaleRevision(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Revision: 2, Slug: "complete", Title: "Complete", ContentMD: "body",
		Status: entry.StatusDraft, Visibility: entry.VisibilityPublic,
	})

	rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/publish", `{"revision":1}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	want := `{"error":{"code":"CONFLICT","message":"entry changed since it was loaded","fields":{"revision":"请重新载入或保留当前内容为恢复草稿"}}}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body = %s\nwant = %s", got, want)
	}
}

// TestUnpublishLeavesPublishedAt covers what withdrawing does and does not undo.
// Clearing the date would make a later republication look like a first one.
func TestUnpublishLeavesPublishedAt(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "to-withdraw", Title: "To withdraw",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
		PublishedAt: fixedTime.Add(-72 * time.Hour),
	})

	rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/unpublish", `{"revision":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	if got := stringField(t, data, "status"); got != "draft" {
		t.Errorf("status = %q, want draft", got)
	}
	if stringField(t, data, "published_at") == "" {
		t.Error("published_at was cleared by unpublishing")
	}

	t.Run("off the public read at once", func(t *testing.T) {
		if rec := do(t, handler, http.MethodGet, "/api/v1/entries/to-withdraw", ""); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})
}

// TestArchiveIsNeitherADraftNorADelete covers why the third state exists. An
// archived entry is off the site like a draft, but still in the admin list, and
// unlike a soft delete it is still reachable there.
func TestArchiveIsNeitherADraftNorADelete(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{
		ID: 7, Slug: "to-retire", Title: "To retire",
		Status: entry.StatusPublished, Visibility: entry.VisibilityPublic,
		PublishedAt: fixedTime.Add(-72 * time.Hour),
	})

	rec := do(t, handler, http.MethodPost, "/api/v1/entries/7/archive", `{"revision":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	data := dataObject(t, rec)
	if got := stringField(t, data, "status"); got != "archived" {
		t.Errorf("status = %q, want archived", got)
	}
	if stringField(t, data, "published_at") == "" {
		t.Error("published_at was cleared by archiving")
	}

	t.Run("off the public read", func(t *testing.T) {
		if rec := do(t, handler, http.MethodGet, "/api/v1/entries/to-retire", ""); rec.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 404", rec.Code)
		}
	})

	t.Run("still in the admin list", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries", "")
		if items := dataArray(t, rec); len(items) != 1 {
			t.Fatalf("the admin list reports %d entries, want 1", len(items))
		}
	})

	t.Run("still readable by id", func(t *testing.T) {
		if rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries/7", ""); rec.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", rec.Code)
		}
	})
}

// TestAdminReadsSeeEveryStatus is the counterpart to
// TestPublicEndpointsHideEverythingUnpublished: the same seeded rows, the other
// audience. Both must hold, or one of the two query sets is wrong.
func TestAdminReadsSeeEveryStatus(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

	for i, tc := range visibilityCases {
		store.Seed(entry.Entry{
			ID: int64(i + 1), Slug: tc.slug, Title: tc.name,
			Status: tc.status, Visibility: tc.vis,
			ContentMD: "the body of " + tc.slug,
			// Distinct edit times, so the ordering assertion below is not comparing
			// rows that tie.
			UpdatedAt: fixedTime.Add(time.Duration(i) * time.Minute),
		})
	}

	t.Run("list", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}

		items := dataArray(t, rec)
		if len(items) != len(visibilityCases) {
			t.Fatalf("the admin list reports %d entries, want all %d", len(items), len(visibilityCases))
		}

		// updated_at DESC: the admin list is a work queue, so the last thing touched
		// comes first. The public list sorts by happened_at and would order these
		// differently.
		if got := stringField(t, items[0], "slug"); got != visibilityCases[len(visibilityCases)-1].slug {
			t.Errorf("first slug = %q, want the most recently edited", got)
		}

		// The summary shape carries the fields an editor needs and the public one
		// omits.
		for _, name := range []string{"id", "status", "visibility", "created_at", "updated_at"} {
			if _, ok := items[0][name]; !ok {
				t.Errorf("the admin summary has no %q key", name)
			}
		}
	})

	t.Run("detail by id", func(t *testing.T) {
		for i, tc := range visibilityCases {
			t.Run(tc.name, func(t *testing.T) {
				target := "/api/v1/admin/entries/" + strconv.Itoa(i+1)

				rec := do(t, handler, http.MethodGet, target, "")
				if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
				}
				if got := stringField(t, dataObject(t, rec), "content_md"); got == "" {
					t.Error("the admin detail read carries no body")
				}
			})
		}
	})
}

// TestAdminListFiltersByStatus covers the one parameterised filter in these
// queries, and the refusal that keeps a typo from reading as an empty site.
func TestAdminListFiltersByStatus(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	for i, tc := range visibilityCases {
		store.Seed(entry.Entry{
			ID: int64(i + 1), Slug: tc.slug, Title: tc.name,
			Status: tc.status, Visibility: tc.vis,
		})
	}

	t.Run("draft", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?status=draft", "")
		items := dataArray(t, rec)
		if len(items) != 1 {
			t.Fatalf("status=draft reports %d entries, want 1", len(items))
		}
		if got := stringField(t, items[0], "status"); got != "draft" {
			t.Errorf("status = %q, want draft", got)
		}
	})

	t.Run("an unknown status is refused, not empty", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?status=stauts", "")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
		}
		if _, ok := errorFields(t, rec)["status"]; !ok {
			t.Errorf("the error does not name the status field: %s", rec.Body.String())
		}
	})
}

func TestAdminListFiltersByCategorySlug(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 1, Slug: "travel", Title: "Travel", CategoryID: testCategoryID, Status: entry.StatusDraft})
	store.Seed(entry.Entry{ID: 2, Slug: "work", Title: "Work", CategoryID: 8, Status: entry.StatusDraft})

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?category="+testCategorySlug, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	items := dataArray(t, rec)
	if len(items) != 1 || stringField(t, items[0], "slug") != "travel" {
		t.Errorf("items = %v, want only travel", items)
	}
}

func TestAdminDashboardReturnsMetrics(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)
	store.Seed(entry.Entry{ID: 1, Slug: "published", Status: entry.StatusPublished, WordCount: 120})
	store.Seed(entry.Entry{ID: 2, Slug: "draft", Status: entry.StatusDraft, WordCount: 35})

	rec := do(t, handler, http.MethodGet, "/api/v1/admin/dashboard", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	data := dataObject(t, rec)
	if got := intField(t, data, "total_entries"); got != 2 {
		t.Errorf("total_entries = %d, want 2", got)
	}
	if got := intField(t, data, "published_entries"); got != 1 {
		t.Errorf("published_entries = %d, want 1", got)
	}
	if got := intField(t, data, "total_words"); got != 155 {
		t.Errorf("total_words = %d, want 155", got)
	}
}

// TestStoreFailureIsNotReportedAsNotFound covers the difference between "your
// entry is gone" and "the database is unreachable". A 404 for the second would
// have the owner believe their content was lost.
func TestStoreFailureIsNotReportedAsNotFound(t *testing.T) {
	store := entrytest.NewStore()
	// FailGetLinkBySlug, not FailGetBySlug: the public detail read goes through the
	// link read now, which is what makes an unlisted entry reachable by URL.
	store.FailGetLinkBySlug = errors.New("connection refused")

	engine := gin.New()
	entryhttp.NewHandler(
		entry.NewService(store),
		func(*gin.Context) (int64, bool) { return testAuthorID, true },
		testCategoryResolver,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	).Register(engine.Group("/api/v1"), func(c *gin.Context) { c.Next() })

	rec := do(t, engine, http.MethodGet, "/api/v1/entries/any-slug", "")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500\nbody: %s", rec.Code, rec.Body.String())
	}
	// The cause reaches the log, never the response: a driver message hands table
	// and column names to whoever asked.
	if strings.Contains(rec.Body.String(), "connection refused") {
		t.Errorf("the response carries the driver error: %s", rec.Body.String())
	}
}

// TestAdminListSearchesByQuery covers the directory search parameter: `q` is
// free text matched against title, slug, and summary, and it intersects with the
// status filter rather than replacing it.
func TestAdminListSearchesByQuery(t *testing.T) {
	handler, store := newTestServer(t, testAuthorID)

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
			UpdatedAt: fixedTime.Add(-time.Duration(i) * time.Minute),
		})
	}

	t.Run("matches title, slug, and summary", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?q=mountain", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		if items := dataArray(t, rec); len(items) != 2 {
			t.Fatalf("q=mountain reports %d entries, want 2", len(items))
		}
	})

	t.Run("intersects with status", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?q=mountain&status=draft", "")
		items := dataArray(t, rec)
		if len(items) != 1 {
			t.Fatalf("reports %d entries, want the single draft match", len(items))
		}
		if got := stringField(t, items[0], "slug"); got != "sea-notes" {
			t.Errorf("slug = %q, want sea-notes", got)
		}
	})

	t.Run("a blank q is not a filter", func(t *testing.T) {
		// A find-as-you-type field sends q= on every keystroke, including after the
		// editor clears it. Treating that as a search for the empty string would
		// report an empty directory.
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?q=%20%20", "")
		if items := dataArray(t, rec); len(items) != 3 {
			t.Errorf("reports %d entries, want all 3", len(items))
		}
	})

	t.Run("no match is an empty list, not an error", func(t *testing.T) {
		rec := do(t, handler, http.MethodGet, "/api/v1/admin/entries?q=nothing-here", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
		}
		if items := dataArray(t, rec); len(items) != 0 {
			t.Errorf("reports %d entries, want none", len(items))
		}
	})
}
