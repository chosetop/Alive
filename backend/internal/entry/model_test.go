package entry_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/p30huiwei/alive/backend/internal/entry"
)

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		name string
		slug string
		ok   bool
	}{
		{"single word", "hello", true},
		{"hyphenated", "kyoto-spring", true},
		{"digits", "2026-review", true},
		{"digits only", "2026", true},
		{"many groups", "a-b-c-d", true},

		{"empty", "", false},
		{"uppercase", "Hello", false},
		{"underscore", "hello_world", false},
		{"leading hyphen", "-hello", false},
		{"trailing hyphen", "hello-", false},
		{"double hyphen", "a--b", false},
		{"space", "hello world", false},
		{"chinese", "京都的春天", false},
		{"percent encoded", "%E4%BA%AC", false},
		{"slash", "a/b", false},
		{"dot", "a.b", false},
		{"too long", strings.Repeat("a", entry.MaxSlugLength+1), false},
		{"at the limit", strings.Repeat("a", entry.MaxSlugLength), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := entry.ValidateSlug(tc.slug)
			if tc.ok && err != nil {
				t.Errorf("ValidateSlug(%q) = %v, want nil", tc.slug, err)
			}
			if !tc.ok {
				if err == nil {
					t.Errorf("ValidateSlug(%q) = nil, want an error", tc.slug)
				} else if !errors.Is(err, entry.ErrInvalidSlug) {
					t.Errorf("ValidateSlug(%q) = %v, want ErrInvalidSlug", tc.slug, err)
				}
			}
		})
	}
}

func TestValidateTitle(t *testing.T) {
	cases := []struct {
		name  string
		title string
		ok    bool
	}{
		{"plain", "Hello", true},
		{"chinese", "京都的春天", true},
		// Runes, not bytes: 255 Chinese characters are 765 bytes and the column
		// counts characters, so a byte limit would reject a title the database
		// accepts.
		{"255 chinese characters", strings.Repeat("春", entry.MaxTitleLength), true},
		{"256 characters", strings.Repeat("a", entry.MaxTitleLength+1), false},
		{"empty", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := entry.ValidateTitle(tc.title)
			if tc.ok && err != nil {
				t.Errorf("ValidateTitle(%d chars) = %v, want nil", len([]rune(tc.title)), err)
			}
			if !tc.ok && !errors.Is(err, entry.ErrInvalidTitle) {
				t.Errorf("ValidateTitle(%d chars) = %v, want ErrInvalidTitle", len([]rune(tc.title)), err)
			}
		})
	}
}

func TestValidateForPublish(t *testing.T) {
	valid := entry.Entry{
		Title:      "Title",
		Slug:       "title",
		ContentMD:  "body",
		Visibility: entry.VisibilityPublic,
	}
	if err := entry.ValidateForPublish(valid); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		mutate func(*entry.Entry)
		want   error
	}{
		{"title", func(e *entry.Entry) { e.Title = "" }, entry.ErrInvalidTitle},
		{"slug", func(e *entry.Entry) { e.Slug = "" }, entry.ErrInvalidSlug},
		{"body", func(e *entry.Entry) { e.ContentMD = "  " }, entry.ErrEmptyContent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := valid
			tc.mutate(&got)
			if err := entry.ValidateForPublish(got); !errors.Is(err, tc.want) {
				t.Errorf("ValidateForPublish() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestTypeValid(t *testing.T) {
	valid := []entry.Type{
		entry.TypeJournal, entry.TypeBook, entry.TypeMovie,
		entry.TypeMusic, entry.TypeTravel, entry.TypePhoto,
	}
	for _, kind := range valid {
		if !kind.Valid() {
			t.Errorf("%q.Valid() = false, want true", kind)
		}
	}

	// The typo case, and the reason the CHECK constraint exists. Without one, this
	// value inserts and the row then disappears from every query filtering by
	// type, reporting no error anywhere.
	invalid := []entry.Type{"", "joural", "Journal", "JOURNAL", "note", "journal "}
	for _, kind := range invalid {
		if kind.Valid() {
			t.Errorf("%q.Valid() = true, want false", kind)
		}
	}
}

func TestStatusAndVisibilityValid(t *testing.T) {
	for _, s := range []entry.Status{entry.StatusDraft, entry.StatusPublished, entry.StatusArchived} {
		if !s.Valid() {
			t.Errorf("status %q.Valid() = false, want true", s)
		}
	}
	for _, s := range []entry.Status{"", "live", "Draft", "deleted"} {
		if s.Valid() {
			t.Errorf("status %q.Valid() = true, want false", s)
		}
	}

	for _, v := range []entry.Visibility{entry.VisibilityPublic, entry.VisibilityPrivate, entry.VisibilityUnlisted} {
		if !v.Valid() {
			t.Errorf("visibility %q.Valid() = false, want true", v)
		}
	}
	for _, v := range []entry.Visibility{"", "hidden", "Public", "secret"} {
		if v.Valid() {
			t.Errorf("visibility %q.Valid() = true, want false", v)
		}
	}
}

func TestCountWords(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{"empty", "", 0},
		{"one word", "hello", 1},
		{"two words", "hello world", 2},
		{"punctuation does not split a word", "hello, world!", 2},
		{"newlines separate words", "hello\nworld", 2},
		{"repeated spaces count once", "hello    world", 2},
		{"digits are words", "in 2026 I read 12 books", 6},
		{"an apostrophe splits, which is accepted", "don't", 2},

		// Chinese is not space-delimited, so strings.Fields would report this
		// whole line as one word. Each character counts instead.
		{"chinese characters count individually", "京都的春天", 5},
		{"chinese punctuation does not count", "京都，春天。", 4},
		// 在 + Kyoto + 的 + 春 + 天: one Latin run among four CJK characters.
		{"mixed scripts", "在 Kyoto 的春天", 5},

		// Markdown syntax is counted as written. Teaching this to parse Markdown
		// would mean keeping a parser in step with whatever the frontend renders.
		{"markdown heading counts its words", "# Hello world", 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := entry.CountWords(tc.content); got != tc.want {
				t.Errorf("CountWords(%q) = %d, want %d", tc.content, got, tc.want)
			}
		})
	}
}

func TestMetaValidate(t *testing.T) {
	cases := []struct {
		name string
		meta entry.Meta
		ok   bool
	}{
		{"empty means no attributes", entry.Meta(nil), true},
		{"empty object", entry.Meta(`{}`), true},
		{"object with keys", entry.Meta(`{"place":"京都"}`), true},
		{"nested object", entry.Meta(`{"book":{"rating":5}}`), true},

		// Valid JSON but not an object. jsonb would store any of these, and every
		// reader of this column expects to look up keys in it.
		{"array", entry.Meta(`[1,2]`), false},
		{"number", entry.Meta(`7`), false},
		{"string", entry.Meta(`"place"`), false},
		{"null", entry.Meta(`null`), true},

		{"malformed", entry.Meta(`{`), false},
		{"trailing comma", entry.Meta(`{"a":1,}`), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.meta.Validate()
			if tc.ok && err != nil {
				t.Errorf("Validate() = %v, want nil", err)
			}
			if !tc.ok && !errors.Is(err, entry.ErrInvalidMeta) {
				t.Errorf("Validate() = %v, want ErrInvalidMeta", err)
			}
		})
	}

	t.Run("ForStorage substitutes an empty object", func(t *testing.T) {
		if got := string(entry.Meta(nil).ForStorage()); got != "{}" {
			t.Errorf("ForStorage() = %q, want %q", got, "{}")
		}
		if got := string(entry.Meta(`{"a":1}`).ForStorage()); got != `{"a":1}` {
			t.Errorf("ForStorage() = %q, want the value unchanged", got)
		}
	})

	t.Run("DefaultMeta returns a copy per call", func(t *testing.T) {
		// Appending to one result must not be visible in the next. A shared
		// backing array would let one entry's meta alter another's.
		first := entry.DefaultMeta()
		first = append(first, 'x')
		if got := string(entry.DefaultMeta()); got != "{}" {
			t.Errorf("DefaultMeta() = %q after a caller appended, want %q", got, "{}")
		}
		_ = first
	})
}

func TestIsPubliclyReadable(t *testing.T) {
	cases := []struct {
		name       string
		status     entry.Status
		visibility entry.Visibility
		want       bool
	}{
		{"published public", entry.StatusPublished, entry.VisibilityPublic, true},
		{"draft public", entry.StatusDraft, entry.VisibilityPublic, false},
		{"archived public", entry.StatusArchived, entry.VisibilityPublic, false},
		{"published private", entry.StatusPublished, entry.VisibilityPrivate, false},
		{"published unlisted", entry.StatusPublished, entry.VisibilityUnlisted, false},
		{"draft private", entry.StatusDraft, entry.VisibilityPrivate, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := entry.Entry{Status: tc.status, Visibility: tc.visibility}
			if got := e.IsPubliclyReadable(); got != tc.want {
				t.Errorf("IsPubliclyReadable() = %v, want %v", got, tc.want)
			}
		})
	}
}
