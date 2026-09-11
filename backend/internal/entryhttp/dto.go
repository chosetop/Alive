package entryhttp

import (
	"encoding/json"
	"time"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/entry"
)

type dashboardMetrics struct {
	TotalEntries     int64 `json:"total_entries"`
	PublishedEntries int64 `json:"published_entries"`
	TotalWords       int64 `json:"total_words"`
}

type reorderEntriesRequest struct {
	World      string  `json:"world"`
	OrderedIDs []int64 `json:"ordered_ids"`
}

// createEntryRequest is the create body.
//
// All fields are optional because creation starts an incomplete draft. world and
// visibility are absent-able because the service owns their defaults: repeating
// the defaults here would mean two places to change when one of them moves.
//
// No `binding:"oneof=..."` on world or visibility. Those sets live in the
// domain and are checked there, and a binding tag would answer with gin's own
// message instead of this API's error envelope.
type createEntryRequest struct {
	World      string `json:"world"`
	Title      string `json:"title"`
	Slug       string `json:"slug"`
	Summary    string `json:"summary"`
	ContentMD  string `json:"content_md"`
	CoverURL   string `json:"cover_url"`
	Visibility string `json:"visibility"`

	// CategoryID is absent or 0 for an uncategorised entry, which is a normal state.
	// An id rather than a slug: the slug is what a URL uses, but an editor picks from
	// a list that already knows the ids, and accepting a slug here would mean a
	// lookup that could disagree with the foreign key.
	//
	// Not validated against the categories table before the insert. A read first
	// would still be racing a delete, and the foreign key is the actual guarantee;
	// an unknown id comes back as 400 naming the field.
	CategoryID int64 `json:"category_id"`

	// Meta is passed through as raw JSON. Decoding it into a map here and
	// re-encoding it would reorder keys and drop the distinction between an absent
	// meta and an empty one.
	Meta json.RawMessage `json:"meta"`

	// HappenedAt is when the thing happened, which is usually not now. A pointer
	// so that absent stays absent: a zero time.Time would be written as year 1
	// rather than left NULL.
	HappenedAt *time.Time `json:"happened_at"`
}

// toInput converts the body into what the service takes.
//
// A conversion rather than binding tags on entry.CreateInput, so the domain type
// carries no JSON names and a field added to it is not accepted from a request by
// accident.
func (r createEntryRequest) toInput(authorID int64) entry.CreateInput {
	in := entry.CreateInput{
		AuthorID:   authorID,
		CategoryID: r.CategoryID,
		World:      contentworld.Key(r.World),
		Title:      r.Title,
		Slug:       r.Slug,
		Summary:    r.Summary,
		ContentMD:  r.ContentMD,
		CoverURL:   r.CoverURL,
		Visibility: entry.Visibility(r.Visibility),
		Meta:       entry.Meta(r.Meta),
	}
	if r.HappenedAt != nil {
		in.HappenedAt = *r.HappenedAt
	}
	return in
}

// updateEntryRequest is the PATCH body.
//
// Every field is a pointer so that an absent one stays absent. With value fields
// a request changing only the title would carry empty strings for everything
// else, and the update would blank the summary, the body and the cover.
//
// Clearing a nullable field is done with an empty value, not with null:
// `{"summary": ""}` clears the summary, and the repository maps "" onto a SQL
// NULL as it already does for create. `{"summary": null}` is read as absent,
// because encoding/json leaves the pointer nil for an explicit null exactly as it
// does for a missing key, and telling those apart would mean decoding into a
// map[string]json.RawMessage and losing every type check the struct provides.
//
// The same applies to happened_at: send a zero time to clear it. That is the one
// place where this convention is visible, since "" is a natural empty string but a
// zero timestamp is not a natural empty date.
//
// Revision is the one required field. The editable fields stay pointers because
// a PATCH naming one field is the point of a PATCH. The service refuses a body
// that names no editable field.
type updateEntryRequest struct {
	Revision   int64   `json:"revision" binding:"required,min=1"`
	World      *string `json:"world"`
	Title      *string `json:"title"`
	Slug       *string `json:"slug"`
	Summary    *string `json:"summary"`
	ContentMD  *string `json:"content_md"`
	CoverURL   *string `json:"cover_url"`
	Visibility *string `json:"visibility"`

	// CategoryID follows the same convention as the rest: absent leaves the category
	// alone, and 0 removes it. `{"category_id": 0}` is how an entry becomes
	// uncategorised, since `null` is indistinguishable from an omitted key.
	CategoryID *int64 `json:"category_id"`

	// Meta is raw JSON, as in create. A pointer as well, so that omitting meta
	// leaves it alone rather than replacing it with {}.
	Meta *json.RawMessage `json:"meta"`

	HappenedAt *time.Time `json:"happened_at"`

	// Status is accepted only to be refused.
	//
	// Ignoring an unknown field would let a client send {"status":"published"},
	// receive 200, and find the entry still a draft. Publishing carries the
	// published_at rule and lives at its own endpoints, so this reports that
	// instead of failing quietly.
	Status *string `json:"status"`
}

// toInput converts the body into what the service takes.
func (r updateEntryRequest) toInput() entry.UpdateInput {
	in := entry.UpdateInput{
		ExpectedRevision: r.Revision,
		Title:            r.Title,
		Summary:          r.Summary,
		ContentMD:        r.ContentMD,
		CoverURL:         r.CoverURL,
		Slug:             r.Slug,
		HappenedAt:       r.HappenedAt,
		CategoryID:       r.CategoryID,
	}
	if r.Visibility != nil {
		v := entry.Visibility(*r.Visibility)
		in.Visibility = &v
	}
	if r.Meta != nil {
		m := entry.Meta(*r.Meta)
		in.Meta = &m
	}

	return in
}

// transitionEntryRequest is shared by publish, unpublish and archive. Each
// transition is a write and therefore carries the revision it was based on.
type transitionEntryRequest struct {
	Revision int64 `json:"revision" binding:"required,min=1"`
}

// entrySummary is one entry as the public list reports it.
//
// No content_md. The list is the first screen of the site, and shipping every
// body in it would inflate that response many times over for text nothing
// renders. The detail endpoint is where the body lives.
//
// No id either. A reader identifies an entry by slug, and ids are sequential
// across drafts as well as published rows, so publishing them would report how
// much unpublished work exists.
//
// tags and media are absent until the tables behind them exist.
type entrySummary struct {
	World     string          `json:"world"`
	Kind      string          `json:"kind"`
	Title     string          `json:"title"`
	Slug      string          `json:"slug"`
	Summary   string          `json:"summary"`
	CoverURL  string          `json:"cover_url"`
	Meta      json.RawMessage `json:"meta"`
	WordCount int             `json:"word_count"`

	// Category is null when the entry is uncategorised, which is a normal state
	// rather than missing data. A nested object rather than a bare id because
	// everything a reader does with a category needs the label and the link:
	// rendering "in 旅行" and pointing at /journals?category=travel. It is filled from
	// the LEFT JOIN, so it costs no extra query.
	Category *entryCategory `json:"category"`

	// Both nullable, and both meaningful when null: an entry may record no date
	// for when it happened, and one that has never been published has no
	// publication time.
	HappenedAt  *time.Time `json:"happened_at"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// entryCategory is the category an entry belongs to, as an entry response carries
// it.
//
// Not taxonomyhttp's publicCategory. That one carries an entry count, which would
// be a second query per entry in a list, and a description, which belongs on a
// category page rather than beside every entry. This is what rendering a link
// needs and nothing else.
//
// A separate type in this package also keeps the two adapters independent: neither
// imports the other, so a change to the category page's shape cannot alter what an
// entry response looks like.
type entryCategory struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// newEntryCategory builds the nested category from a read.
//
// nil when the entry is uncategorised, which serialises to null.
//
// Also nil when the id is set but the name is not. That combination is what a write
// returns: RETURNING sees only the entry row, so a create or update knows the id
// and not the label. Reporting {"id":3,"name":"","slug":""} there would publish a
// category whose name is the empty string, which is a thing that cannot exist —
// name is NOT NULL with a length CHECK. Null is the honest answer for "not loaded",
// and newOwnerEntryDetail's comment says where to read it from.
func newEntryCategory(e entry.Entry) *entryCategory {
	if e.CategoryID == 0 || e.CategoryName == "" {
		return nil
	}
	return &entryCategory{
		ID:   e.CategoryID,
		Name: e.CategoryName,
		Slug: e.CategorySlug,
	}
}

// publicEntryDetail is one entry as the public detail endpoint reports it.
//
// Adds the body and the last edit to the summary's fields. status and visibility
// are omitted: this endpoint only ever returns published public entries, so they
// would be two constants in every response.
type publicEntryDetail struct {
	entrySummary
	ContentMD    string        `json:"content_md"`
	UpdatedAt    time.Time     `json:"updated_at"`
	PrimaryMedia *primaryMedia `json:"primary_media,omitempty"`
}
type primaryMedia struct {
	URL      string `json:"url"`
	MIMEType string `json:"mime_type"`
}

// sayingListItem is one Saying as the public list reports it.
//
// No title, no timestamps, no cover: a Saying is surfaced as a short permanent
// piece of content rather than as a journal entry with metadata chrome.
type sayingListItem struct {
	ShortID  string         `json:"short_id"`
	Content  string         `json:"content_md"`
	Source   string         `json:"source,omitempty"`
	Author   string         `json:"author,omitempty"`
	Category *entryCategory `json:"category,omitempty"`
}

// sayingLink is the minimal permanent-link shape used for previous/next.
type sayingLink struct {
	ShortID string `json:"short_id"`
}

// sayingDetail is one Saying as the permanent-link endpoint reports it.
//
// The same trimmed body shape as the list, plus prev/next links for browse
// navigation. No journal chrome.
type sayingDetail struct {
	sayingListItem
	Previous *sayingLink `json:"previous,omitempty"`
	Next     *sayingLink `json:"next,omitempty"`
}

// ownerEntryDetail is an entry as its author sees it, in the create response.
//
// Carries id, revision, status and visibility, which the public shapes leave
// out. The author needs all four: id addresses later admin calls, revision guards
// later writes, and status and visibility describe whether readers can see it.
type ownerEntryDetail struct {
	ID       int64 `json:"id"`
	Revision int64 `json:"revision"`

	World      string          `json:"world"`
	Kind       string          `json:"kind"`
	Title      string          `json:"title"`
	Slug       string          `json:"slug"`
	Summary    string          `json:"summary"`
	ContentMD  string          `json:"content_md"`
	CoverURL   string          `json:"cover_url"`
	Status     string          `json:"status"`
	Visibility string          `json:"visibility"`
	Meta       json.RawMessage `json:"meta"`
	WordCount  int             `json:"word_count"`

	// CategoryID is what was stored, 0 for uncategorised. Present in addition to
	// Category because this shape answers writes as well as the admin detail read,
	// and a write cannot join: after a PATCH the id is known and the name is not.
	// An editor confirming that its change took effect reads this field, which is
	// filled either way, rather than Category, which is null after a write.
	CategoryID int64 `json:"category_id"`

	// Category is null after a write and filled on the admin detail read. See
	// newEntryCategory for why an unnamed category is reported as null rather than
	// as an object with an empty name.
	Category *entryCategory `json:"category"`

	HappenedAt  *time.Time `json:"happened_at"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// adminEntrySummary is one entry as the admin list reports it.
//
// Adds the three fields the public summary withholds. id because every admin
// action addresses an entry by it, and status and visibility because a list that
// shows drafts alongside published entries has to say which is which.
//
// Saying rows include content_md because the writing directory identifies them
// by their body. Other worlds keep the field omitted.
type adminEntrySummary struct {
	entrySummary
	ID         int64     `json:"id"`
	Status     string    `json:"status"`
	Visibility string    `json:"visibility"`
	ContentMD  string    `json:"content_md,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// newAdminEntrySummary converts a domain entry into the admin list shape.
func newAdminEntrySummary(e entry.Entry) adminEntrySummary {
	contentMD := ""
	if e.World == contentworld.Saying {
		contentMD = e.ContentMD
	}
	return adminEntrySummary{
		entrySummary: newEntrySummary(e),
		ID:           e.ID,
		Status:       string(e.Status),
		Visibility:   string(e.Visibility),
		ContentMD:    contentMD,
		CreatedAt:    e.CreatedAt,
		UpdatedAt:    e.UpdatedAt,
	}
}

// newAdminEntrySummaries converts a page of entries, empty slice rather than nil.
func newAdminEntrySummaries(entries []entry.Entry) []adminEntrySummary {
	out := make([]adminEntrySummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, newAdminEntrySummary(e))
	}
	return out
}

// paginationMeta is what a client needs to walk the rest of the collection.
//
// Total counts what the filter admits, not what the table holds, so a client
// cannot page towards entries it will never be shown.
type paginationMeta struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// newEntrySummary converts a domain entry into the list shape.
func newEntrySummary(e entry.Entry) entrySummary {
	return entrySummary{
		World:       string(e.World),
		Kind:        e.Kind,
		Title:       e.Title,
		Slug:        e.Slug,
		Summary:     e.Summary,
		CoverURL:    e.CoverURL,
		Meta:        metaJSON(e.Meta),
		WordCount:   e.WordCount,
		Category:    newEntryCategory(e),
		HappenedAt:  optionalTime(e.HappenedAt),
		PublishedAt: optionalTime(e.PublishedAt),
		CreatedAt:   e.CreatedAt,
	}
}

func newPublicEntryDetail(e entry.Entry) publicEntryDetail {
	return publicEntryDetail{
		entrySummary: newEntrySummary(e),
		ContentMD:    e.ContentMD,
		UpdatedAt:    e.UpdatedAt,
	}
}

// newSayingListItem converts a domain entry into the Saying list shape.
func newSayingListItem(e entry.Entry) sayingListItem {
	meta, _ := entry.DecodeSayingMeta(e.Meta)
	return sayingListItem{
		ShortID:  e.Slug,
		Content:  e.ContentMD,
		Source:   meta.Source,
		Author:   meta.Author,
		Category: newEntryCategory(e),
	}
}

// newSayingDetail converts a domain entry and its neighbours into the Saying
// detail shape.
func newSayingDetail(e entry.Entry, previous, next *entry.Entry) sayingDetail {
	return sayingDetail{
		sayingListItem: newSayingListItem(e),
		Previous:       newSayingLink(previous),
		Next:           newSayingLink(next),
	}
}

func newSayingLink(e *entry.Entry) *sayingLink {
	if e == nil {
		return nil
	}
	return &sayingLink{ShortID: e.Slug}
}

func newOwnerEntryDetail(e entry.Entry) ownerEntryDetail {
	return ownerEntryDetail{
		ID:          e.ID,
		Revision:    e.Revision,
		World:       string(e.World),
		Kind:        e.Kind,
		Title:       e.Title,
		Slug:        e.Slug,
		Summary:     e.Summary,
		ContentMD:   e.ContentMD,
		CoverURL:    e.CoverURL,
		Status:      string(e.Status),
		Visibility:  string(e.Visibility),
		Meta:        metaJSON(e.Meta),
		WordCount:   e.WordCount,
		CategoryID:  e.CategoryID,
		Category:    newEntryCategory(e),
		HappenedAt:  optionalTime(e.HappenedAt),
		PublishedAt: optionalTime(e.PublishedAt),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

// newEntrySummaries converts a page of entries.
//
// An empty slice rather than nil, because nil marshals to `null` and a client
// iterating "data" would have to handle two shapes for "no entries".
func newEntrySummaries(entries []entry.Entry) []entrySummary {
	out := make([]entrySummary, 0, len(entries))
	for _, e := range entries {
		out = append(out, newEntrySummary(e))
	}
	return out
}

// newSayingListItems converts a page of Saying entries.
func newSayingListItems(entries []entry.Entry) []sayingListItem {
	out := make([]sayingListItem, 0, len(entries))
	for _, e := range entries {
		out = append(out, newSayingListItem(e))
	}
	return out
}

// metaJSON presents a domain Meta as JSON.
//
// The conversion is required, not cosmetic. entry.Meta is a []byte, and
// encoding/json writes a []byte as a base64 string, so returning it directly
// would publish `"e2FiYyI6MX0="` where an object belongs. json.RawMessage is the
// same bytes with a MarshalJSON that emits them literally.
//
// An empty Meta becomes {} rather than null: the column is NOT NULL and every
// consumer expects to look up keys in it.
func metaJSON(m entry.Meta) json.RawMessage {
	if len(m) == 0 {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(m)
}

// optionalTime maps the domain's zero time onto a JSON null.
//
// The domain uses a zero time.Time for "not recorded" so that reads need no nil
// check. That zero is year 1, and publishing it would put "0001-01-01T00:00:00Z"
// in the response where the honest answer is null.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
