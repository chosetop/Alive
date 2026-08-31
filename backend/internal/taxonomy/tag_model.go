package taxonomy

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrTagNotFound    = errors.New("taxonomy: tag not found")
	ErrTagNameTaken   = errors.New("taxonomy: tag name already taken")
	ErrTagSlugTaken   = errors.New("taxonomy: tag slug already taken")
	ErrTagInUse       = errors.New("taxonomy: tag in use")
	ErrInvalidTagName = errors.New("taxonomy: invalid tag name")
	ErrInvalidTagSlug = errors.New("taxonomy: invalid tag slug")
)

const (
	MaxTagNameLength = 64
	MaxTagSlugLength = 160
)

var tagSlugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

type Tag struct {
	ID         int64
	Name       string
	Slug       string
	UsageCount int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CreateTagInput struct {
	Name string
	Slug string
}

type UpdateTagInput struct {
	Name *string
	Slug *string
}

type CreateTagParams struct {
	Name string
	Slug string
}

type UpdateTagParams struct {
	ID      int64
	SetName bool
	Name    string
	SetSlug bool
	Slug    string
}

type TagStore interface {
	Create(ctx context.Context, params CreateTagParams) (Tag, error)
	GetByID(ctx context.Context, id int64) (Tag, error)
	GetBySlug(ctx context.Context, slug string) (Tag, error)
	List(ctx context.Context, query string, limit int) ([]Tag, error)
	Update(ctx context.Context, params UpdateTagParams) (Tag, error)
	Delete(ctx context.Context, id int64) (bool, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	NameExists(ctx context.Context, name string) (bool, error)
}

func NormalizeTagName(name string) string {
	return strings.TrimSpace(name)
}

func ValidateTagName(name string) error {
	name = NormalizeTagName(name)
	if name == "" {
		return ErrInvalidTagName
	}
	if utf8.RuneCountInString(name) > MaxTagNameLength {
		return ErrInvalidTagName
	}
	return nil
}

func NormalizeTagSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	b.Grow(len(value))
	lastHyphen := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if r > unicode.MaxASCII {
				lastHyphen = false
				continue
			}
			b.WriteRune(r)
			lastHyphen = false
		default:
			if b.Len() == 0 || lastHyphen {
				continue
			}
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if !tagSlugPattern.MatchString(out) {
		return ""
	}
	return out
}

func ValidateTagSlug(value string) error {
	if NormalizeTagSlug(value) == "" {
		return ErrInvalidTagSlug
	}
	if len(NormalizeTagSlug(value)) > MaxTagSlugLength {
		return ErrInvalidTagSlug
	}
	return nil
}
