// Package taxonomy owns the structures that group content: categories today,
// tags later.
//
// One package for both because they answer the same kind of question about an
// entry and will share the shape of their reads. They are not the same thing: a
// category answers "what is this, essentially" and an entry has at most one, so
// the id lives on the entry; a tag answers "what is this about" and an entry has
// many, so tags need a join table. Merging them would let the navigation
// structure grow without bound.
//
// Like auth and entry, this package imports no database driver and no HTTP
// framework. The adapter that turns these errors into status codes lives beside
// it, in taxonomyhttp.
package taxonomy

import (
	"errors"
	"regexp"
	"time"
	"unicode/utf8"
)

// Sentinel errors. Callers compare with errors.Is.
//
// Storage errors stop at the repository: pgx.ErrNoRows is a fact about a driver,
// ErrCategoryNotFound is a fact about this domain.
var (
	// ErrCategoryNotFound reports that no category matches.
	//
	// Unlike an entry, there is nothing to conceal here. Every category is
	// readable by everyone, so this error means only what it says.
	ErrCategoryNotFound = errors.New("taxonomy: category not found")

	// ErrSlugTaken reports that a category already holds the slug.
	ErrSlugTaken = errors.New("taxonomy: slug already taken")

	// ErrInvalidSlug reports a slug that does not match the required format.
	ErrInvalidSlug = errors.New("taxonomy: invalid slug")

	// ErrInvalidName reports an empty or over-long name.
	ErrInvalidName = errors.New("taxonomy: invalid name")

	// ErrNoUpdateFields reports an update that named no field to change.
	//
	// An error rather than a successful no-op, matching entry.ErrNoUpdateFields: a
	// request that changes nothing was built wrongly, and 200 would report a save
	// that did not happen.
	ErrNoUpdateFields = errors.New("taxonomy: update names no fields")
)

// MaxNameLength and MaxSlugLength match the column widths. Checking here turns
// what would be a constraint violation into a domain error carrying a message
// worth showing.
//
// Both are 64, narrower than an entry's 255. A category name is a navigation
// label, and one that does not fit in a menu is not a category.
const (
	MaxNameLength = 64
	MaxSlugLength = 64
)

// slugPattern is the accepted slug format: lowercase letters and digits in
// groups joined by single hyphens.
//
// The same expression is a CHECK constraint on the column, and the same one
// entry uses. Both layers exist on purpose: this produces a message a client can
// act on, and the database one holds for rows written by psql.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug checks the format and length of a category slug.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrInvalidSlug
	}
	// Bytes, matching varchar(64). The pattern below admits ASCII only, so bytes
	// and characters agree here anyway.
	if len(slug) > MaxSlugLength {
		return ErrInvalidSlug
	}
	if !slugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// ValidateName checks that a name is present and fits the column.
//
// Runes, not bytes: varchar(64) counts characters, so measuring bytes would
// reject a 30-character Chinese name the database accepts.
func ValidateName(name string) error {
	if name == "" {
		return ErrInvalidName
	}
	if utf8.RuneCountInString(name) > MaxNameLength {
		return ErrInvalidName
	}
	return nil
}

// Category is one grouping of content.
//
// Absent values are zero values, not pointers: "" for a description that was not
// given. That matches entry.Entry, and keeps a nil check out of every read.
type Category struct {
	ID          int64
	Name        string
	Slug        string
	Description string

	// SortOrder is manual ordering for display. Categories are navigation, and
	// the order that suits a reader is neither alphabetical nor by creation date.
	// Ties are broken by id so the order is stable.
	SortOrder int

	CreatedAt time.Time
	UpdatedAt time.Time
}

// CategoryWithCount is a category and the number of entries a public reader can
// see in it.
//
// A separate type rather than a field on Category, because the count is not a
// property of the category: it depends on who is asking and on how many entries
// happen to be published right now. Putting it on Category would leave every
// other read handing back a zero that means "not counted" rather than "empty".
type CategoryWithCount struct {
	Category

	// EntryCount counts entries that are not deleted, are published, and are
	// public. Unlisted entries are excluded on purpose: the count describes the
	// list that opening this category will show, and an unlisted entry is not in
	// it.
	EntryCount int64
}
