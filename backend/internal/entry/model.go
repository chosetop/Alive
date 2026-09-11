// Package entry owns recorded content across Alive's installed worlds.
//
// Like auth, this package imports no database driver and no HTTP framework. The
// rules here are the same whether they are reached from a request or from a CLI
// command, and the adapter that turns them into statuses lives beside it.
package entry

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// Sentinel errors. Callers compare with errors.Is.
//
// Storage errors stop at the repository: pgx.ErrNoRows is a fact about a driver,
// ErrEntryNotFound is a fact about this domain.
var (
	// ErrEntryNotFound reports that no entry matches, or that the one that does
	// is not readable by this caller.
	//
	// One error for both. A public reader asking for a draft gets the same answer
	// as one asking for a slug nobody ever used, so the response cannot be used
	// to find out which drafts exist.
	ErrEntryNotFound = errors.New("entry: not found")

	// ErrSlugTaken reports that a live entry already holds the slug.
	ErrSlugTaken = errors.New("entry: slug already taken")

	// ErrInvalidSlug reports a slug that does not match the required format.
	ErrInvalidSlug = errors.New("entry: invalid slug")

	// ErrInvalidWorld reports a world outside the installed set.
	ErrInvalidWorld = errors.New("entry: invalid world")

	// ErrInvalidStatus reports a status outside the accepted set.
	ErrInvalidStatus = errors.New("entry: invalid status")

	// ErrInvalidVisibility reports a visibility outside the accepted set.
	ErrInvalidVisibility = errors.New("entry: invalid visibility")

	// ErrInvalidTitle reports an empty or over-long title.
	ErrInvalidTitle = errors.New("entry: invalid title")

	// ErrEmptyContent reports a publication attempt with no body.
	ErrEmptyContent = errors.New("entry: empty content")

	// ErrVersionConflict reports a write based on an entry revision that is no
	// longer current.
	ErrVersionConflict = errors.New("entry: version conflict")

	// ErrInvalidMeta reports a meta payload that is not a JSON object.
	ErrInvalidMeta = errors.New("entry: invalid meta")

	// ErrNoUpdateFields reports an update that named no field to change.
	//
	// An error rather than a successful no-op. A request that changes nothing is a
	// client that built it wrongly, and answering 200 would report a save that did
	// not happen.
	ErrNoUpdateFields = errors.New("entry: update names no fields")

	// ErrUnknownCategory reports a category_id no category holds.
	//
	// Its own error rather than taxonomy.ErrCategoryNotFound, because this package
	// does not import taxonomy: an entry references a category by id and needs to
	// know nothing else about it. A client sent an id that is wrong or was deleted
	// between choosing it and saving, and either way the answer names the field.
	ErrUnknownCategory = errors.New("entry: unknown category")

	// ErrWorldNotOpen reports a public publish attempt into an unopened world.
	ErrWorldNotOpen = errors.New("entry: world not open")

	// ErrInvalidDisplayOrder reports a reorder payload that is incomplete,
	// duplicated, or contains an entry outside the requested world.
	ErrInvalidDisplayOrder = errors.New("entry: invalid display order")
)

// Status is how finished an entry is.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusPublished Status = "published"
	StatusArchived  Status = "archived"
)

// Visibility is who may see an entry.
//
// Separate from Status because they answer different questions. Merged, they
// would grow into draft / published / published_private / published_unlisted and
// keep going.
type Visibility string

const (
	// VisibilityPublic appears in lists and is readable by anyone.
	VisibilityPublic Visibility = "public"

	// VisibilityPrivate is readable by the owner only.
	VisibilityPrivate Visibility = "private"

	// VisibilityUnlisted is readable by anyone holding the link but absent from
	// lists, counts and the sitemap. GetLinkByWorldSlug is the one read that accepts it.
	//
	// This is "not advertised", not access control. A slug is human readable and
	// therefore guessable, so unlisted keeps an entry off the front page and out of
	// search engines and nothing more. An entry a stranger must not read is
	// VisibilityPrivate, which no public read accepts.
	VisibilityUnlisted Visibility = "unlisted"
)

// validStatuses and validVisibilities back the Valid methods.
//
// Maps rather than switch statements so that the sets can also be ranged over,
// which is what lets a test assert that these agree with the CHECK constraints
// rather than restating the same values a third time.
var (
	validStatuses = map[Status]struct{}{
		StatusDraft: {}, StatusPublished: {}, StatusArchived: {},
	}
	validVisibilities = map[Visibility]struct{}{
		VisibilityPublic: {}, VisibilityPrivate: {}, VisibilityUnlisted: {},
	}
)

// Valid reports whether s is one of the accepted statuses.
func (s Status) Valid() bool {
	_, ok := validStatuses[s]
	return ok
}

// Valid reports whether v is one of the accepted visibilities.
func (v Visibility) Valid() bool {
	_, ok := validVisibilities[v]
	return ok
}

// MaxTitleLength and MaxSlugLength match the column widths. Checking here turns
// what would be a constraint violation into a domain error carrying a message
// worth showing.
const (
	MaxTitleLength = 255
	MaxSlugLength  = 255
)

// slugPattern is the accepted slug format: lowercase letters and digits in
// groups joined by single hyphens.
//
// The same expression is a CHECK constraint on the column. Both exist on
// purpose: this one produces a message a client can act on, and the database one
// holds for rows written by psql or by a later CLI command.
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateSlug checks the format and length of a slug.
func ValidateSlug(slug string) error {
	if slug == "" {
		return ErrInvalidSlug
	}
	// Bytes, matching the varchar(255) limit. A slug is ASCII by the pattern
	// below, so bytes and characters agree here anyway.
	if len(slug) > MaxSlugLength {
		return ErrInvalidSlug
	}
	if !slugPattern.MatchString(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// ValidateTitle checks that a title is present and fits the column.
//
// Runes, not bytes: varchar(255) counts characters, so measuring bytes would
// reject a 100-character Chinese title that the database accepts.
func ValidateTitle(title string) error {
	if title == "" {
		return ErrInvalidTitle
	}
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrInvalidTitle
	}
	return nil
}

// ValidateForPublish checks the fields that must be complete before an entry is
// visible outside the editor. Drafts deliberately do not use this validator.
func ValidateForPublish(e Entry) error {
	if e.World == contentworld.Saying {
		if e.Title != "" {
			if err := validateDraftTitle(e.Title); err != nil {
				return err
			}
		}
	} else {
		if err := ValidateTitle(e.Title); err != nil {
			return err
		}
	}
	if err := ValidateSlug(e.Slug); err != nil {
		return err
	}
	if e.World != contentworld.Video && strings.TrimSpace(e.ContentMD) == "" {
		return ErrEmptyContent
	}
	if !e.Visibility.Valid() {
		return ErrInvalidVisibility
	}
	return nil
}

func validateDraftTitle(title string) error {
	if utf8.RuneCountInString(title) > MaxTitleLength {
		return ErrInvalidTitle
	}
	return nil
}

func validateDraftSlug(slug string) error {
	if slug == "" {
		return nil
	}
	return ValidateSlug(slug)
}

// Meta holds the attributes that belong to one world and not to the others.
//
// Raw JSON rather than a Go struct per type. A struct per type would need this
// package to know every world's shape, and adding a field to one of them would
// be a change here rather than in the code that owns that world. The first three
// worlds use the empty object today.
type Meta []byte

// emptyMetaObject is what an absent meta becomes: the column is NOT NULL with a
// default of '{}', and sending nil would be a null.
var emptyMetaObject = Meta(`{}`)

// DefaultMeta returns the empty JSON object.
func DefaultMeta() Meta {
	// A copy, so a caller that appends to the result cannot alter the package
	// level value that every other caller shares.
	out := make(Meta, len(emptyMetaObject))
	copy(out, emptyMetaObject)
	return out
}

// Validate reports whether m is a JSON object.
//
// An object specifically, not merely valid JSON. `[1,2]` and `7` both parse, and
// jsonb would store either happily, but every consumer of this column expects to
// look up keys in it. Rejecting a non-object here keeps that assumption true.
//
// An empty Meta is valid and means "no attributes"; ForStorage turns it into {}.
func (m Meta) Validate() error {
	if len(m) == 0 {
		return nil
	}
	var probe map[string]any
	if err := json.Unmarshal(m, &probe); err != nil {
		return fmt.Errorf("%w: not a JSON object", ErrInvalidMeta)
	}
	return nil
}

// ForStorage returns the value to write to the column, substituting {} for an
// absent one.
func (m Meta) ForStorage() Meta {
	if len(m) == 0 {
		return DefaultMeta()
	}
	return m
}

// Entry is one recorded thing.
//
// Absent values are zero values, not pointers: "" for text that was not given,
// and a zero time.Time for a date that was not. Pointers would put a nil check
// in front of every read of a field that is usually present.
type Entry struct {
	ID       int64
	Revision int64
	AuthorID int64

	// CategoryID is the category this entry belongs to, zero when uncategorised.
	//
	// Uncategorised is a normal state, not missing data: an entry does not need a
	// category to be complete. Zero rather than a pointer for the reason above, and
	// because no category has id 0.
	CategoryID int64

	// CategoryName and CategorySlug are filled by the reads, which LEFT JOIN
	// categories, and empty on the result of a write.
	//
	// The asymmetry is not an oversight. RETURNING sees only the row being written,
	// so a write cannot join; a caller that needs the name after a write reads the
	// entry back. They are here rather than a nested Category value because a
	// pointer to one would be nil in two different situations — uncategorised, and
	// written rather than read — and a caller cannot tell those apart.
	CategoryName string
	CategorySlug string

	// Tags are filled by the reads, which join through entry_tags and load the
	// global taxonomy separately. Writes leave this empty until the read path asks
	// for it.
	Tags []taxonomy.Tag

	World      contentworld.Key
	Kind       string
	Title      string
	Slug       string
	Summary    string
	ContentMD  string
	CoverURL   string
	Status     Status
	Visibility Visibility

	// Meta holds attributes specific to World. It is raw JSON because its shape
	// differs per world, and the code that owns a world decodes it.
	//
	// Never compare these bytes to a literal. jsonb stores a parsed structure and
	// re-serialises it on read, so key order and spacing are not preserved.
	Meta Meta

	// WordCount is computed on write so that a list response never has to read
	// ContentMD to report it.
	WordCount int

	// HappenedAt is when the thing happened, which is not when it was written.
	// Zero when unrecorded. The public timeline sorts on this.
	HappenedAt time.Time

	// PublishedAt is first publication and does not move afterwards. Zero while
	// an entry has never been published.
	PublishedAt time.Time

	CreatedAt time.Time
	UpdatedAt time.Time
}

// IsPubliclyReadable reports whether an anonymous reader may see this entry.
//
// The queries enforce this in SQL, so nothing depends on a caller remembering to
// ask. It exists to make the rule assertable in a test, and readable in one
// place, rather than only inferable from three WHERE clauses.
func (e Entry) IsPubliclyReadable() bool {
	return e.Status == StatusPublished && e.Visibility == VisibilityPublic
}

// IsLinkReadable reports whether someone holding this entry's URL may see it.
//
// Wider than IsPubliclyReadable by exactly one value: unlisted. This is what the
// detail endpoint allows, while the lists and counts stay on IsPubliclyReadable.
// Two methods rather than one taking a flag, matching the two SQL statements: a
// boolean deciding whether unlisted counts is one wrong argument away from putting
// unlisted entries on the front page.
func (e Entry) IsLinkReadable() bool {
	return e.Status == StatusPublished &&
		(e.Visibility == VisibilityPublic || e.Visibility == VisibilityUnlisted)
}

// CountWords counts words in Markdown source.
//
// Two rules, because one would be wrong for this project's content. A run of
// Latin letters or digits is one word. Each CJK character is one word, since
// Chinese is not space-delimited and strings.Fields would report a whole
// paragraph as a single word.
//
// The result is an approximation used for a reading-time estimate, not a
// figure anything depends on. Markdown syntax is counted as it appears; teaching
// this function to parse Markdown would mean keeping a parser in step with
// whatever the frontend renders with.
func CountWords(content string) int {
	count := 0
	inWord := false

	for _, r := range content {
		switch {
		case isCJK(r):
			// Each character counts, and it also ends any Latin run before it.
			count++
			inWord = false
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if !inWord {
				count++
				inWord = true
			}
		default:
			inWord = false
		}
	}

	return count
}

// isCJK reports whether r is a Chinese, Japanese or Korean character.
//
// The ranges cover unified ideographs, their first extension, compatibility
// ideographs, and the Japanese syllabaries. Punctuation is excluded: counting
// full-width commas as words would inflate the total.
func isCJK(r rune) bool {
	switch {
	case r >= 0x4E00 && r <= 0x9FFF, // CJK Unified Ideographs
		r >= 0x3400 && r <= 0x4DBF, // Extension A
		r >= 0xF900 && r <= 0xFAFF, // Compatibility Ideographs
		r >= 0x3040 && r <= 0x309F, // Hiragana
		r >= 0x30A0 && r <= 0x30FF, // Katakana
		r >= 0xAC00 && r <= 0xD7AF: // Hangul syllables
		return true
	}
	return false
}

// ValidateWorld checks that world names one installed content world.
func ValidateWorld(world contentworld.Key) error {
	if _, ok := contentworld.Lookup(world); !ok {
		return ErrInvalidWorld
	}
	return nil
}
