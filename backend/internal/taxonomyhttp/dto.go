package taxonomyhttp

import (
	"time"

	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// createCategoryRequest is the create body.
//
// name and slug are required; a category with neither is not a category. slug is
// supplied rather than derived from the name, matching entries: deriving one from
// Chinese text needs either percent-encoding or a pinyin dependency here.
//
// No `binding:"max=64"` on either. The lengths live in the domain, next to the
// column widths they match, and a binding tag would answer with gin's own message
// instead of this API's error envelope.
type createCategoryRequest struct {
	Name        string `json:"name" binding:"required"`
	Slug        string `json:"slug" binding:"required"`
	Description string `json:"description"`

	// SortOrder is a pointer only so that an absent one is distinguishable in
	// principle; both absent and 0 mean the same position. It stays a pointer
	// because a client that sends 0 explicitly is saying "put this first", and
	// reading that as "unspecified" would be wrong the day a default other than 0
	// is introduced.
	SortOrder *int `json:"sort_order"`
}

// toInput converts the body into what the service takes.
//
// A conversion rather than binding tags on taxonomy.CreateInput, so the domain
// type carries no JSON names and a field added to it is not accepted from a
// request by accident.
func (r createCategoryRequest) toInput() taxonomy.CreateInput {
	in := taxonomy.CreateInput{
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
	}
	if r.SortOrder != nil {
		in.SortOrder = *r.SortOrder
	}
	return in
}

// updateCategoryRequest is the PATCH body.
//
// Every field is a pointer so an absent one stays absent. With value fields a
// request renaming a category would carry an empty description and blank it.
//
// Clearing the description is done with "", not null: `{"description": ""}` clears
// it, and the repository maps "" onto SQL NULL. `{"description": null}` reads as
// absent, because encoding/json leaves the pointer nil for an explicit null
// exactly as it does for a missing key. Same convention as entryhttp.
//
// No binding:"required" anywhere: naming one field is the point of a PATCH. The
// service refuses a body that names none.
type updateCategoryRequest struct {
	Name        *string `json:"name"`
	Slug        *string `json:"slug"`
	Description *string `json:"description"`
	SortOrder   *int    `json:"sort_order"`
}

// toInput converts the body into what the service takes.
//
// A straight copy: unlike entryhttp, no field here has a domain type distinct from
// its JSON one, so no pointer needs re-taking.
func (r updateCategoryRequest) toInput() taxonomy.UpdateInput {
	return taxonomy.UpdateInput{
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
		SortOrder:   r.SortOrder,
	}
}

// publicCategory is one category as the public list reports it.
//
// Carries the id, unlike entrySummary. An entry id would report how much
// unpublished work exists, because ids run across drafts too; a category id
// reveals only how many categories were ever created, and the entry list filter
// needs something to address a category by.
//
// No created_at or updated_at: a reader has no use for when a menu label was
// edited.
type publicCategory struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`

	// EntryCount counts entries a public reader can reach: not deleted, published,
	// public. Unlisted entries are excluded, so this number matches the list that
	// opening the category will show rather than exceeding it.
	EntryCount int64 `json:"entry_count"`
}

// adminCategory is one category as the authenticated endpoints report it.
//
// Adds the timestamps and drops the count. An editor needs to know when a category
// was last changed; the count belongs to the public list, and producing it here
// would mean the extra join on every write response.
type adminCategory struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// newPublicCategory converts a counted domain category into the public shape.
func newPublicCategory(c taxonomy.CategoryWithCount) publicCategory {
	return publicCategory{
		ID:          c.ID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		SortOrder:   c.SortOrder,
		EntryCount:  c.EntryCount,
	}
}

// newPublicCategories converts the public list.
//
// An empty slice rather than nil, because nil marshals to `null` and a client
// iterating "data" would have to handle two shapes for "no categories".
func newPublicCategories(categories []taxonomy.CategoryWithCount) []publicCategory {
	out := make([]publicCategory, 0, len(categories))
	for _, c := range categories {
		out = append(out, newPublicCategory(c))
	}
	return out
}

// newAdminCategory converts a domain category into the admin shape.
func newAdminCategory(c taxonomy.Category) adminCategory {
	return adminCategory{
		ID:          c.ID,
		Name:        c.Name,
		Slug:        c.Slug,
		Description: c.Description,
		SortOrder:   c.SortOrder,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// newAdminCategories converts the admin list, empty slice rather than nil.
func newAdminCategories(categories []taxonomy.Category) []adminCategory {
	out := make([]adminCategory, 0, len(categories))
	for _, c := range categories {
		out = append(out, newAdminCategory(c))
	}
	return out
}
