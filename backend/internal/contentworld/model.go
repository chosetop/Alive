package contentworld

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

type Key string

const (
	Journal Key = "journal"
	Saying  Key = "saying"
	Video   Key = "video"
)

type Status string

const (
	Unopened Status = "unopened"
	Open     Status = "open"
	Hidden   Status = "hidden"
)

type ViewMode string

const (
	ViewNone   ViewMode = ""
	ViewStream ViewMode = "stream"
	ViewWall   ViewMode = "wall"
	ViewFocus  ViewMode = "focus"
)

type MediaCapability string

const (
	MediaNone           MediaCapability = "none"
	MediaMarkdownImages MediaCapability = "markdown_images"
	MediaPrimaryVideo   MediaCapability = "primary_video"
)

type Definition struct {
	Key             Key
	DefaultLabel    string
	SortOrder       int
	AllowedViews    []ViewMode
	CategoryEnabled bool
	MediaCapability MediaCapability
}

var (
	ErrWorldNotFound   = errors.New("contentworld: world not found")
	ErrVersionConflict = errors.New("contentworld: version conflict")
	ErrInvalidStatus   = errors.New("contentworld: invalid status")
	ErrInvalidNavLabel = errors.New("contentworld: invalid nav label")
	ErrInvalidViewMode = errors.New("contentworld: invalid view mode")
)

type Setting struct {
	World       Key
	Status      Status
	NavLabel    string
	SortOrder   int
	DefaultView ViewMode
	Revision    int64
	UpdatedAt   time.Time
}

type UpdateInput struct {
	ExpectedRevision int64
	Status           *Status
	NavLabel         *string
	DefaultView      *ViewMode
}

var definitions = []Definition{
	{
		Key:             Journal,
		DefaultLabel:    "日志",
		SortOrder:       10,
		AllowedViews:    []ViewMode{ViewNone},
		CategoryEnabled: true,
		MediaCapability: MediaMarkdownImages,
	},
	{
		Key:             Saying,
		DefaultLabel:    "片语",
		SortOrder:       20,
		AllowedViews:    []ViewMode{ViewStream, ViewWall, ViewFocus},
		CategoryEnabled: true,
		MediaCapability: MediaNone,
	},
	{
		Key:             Video,
		DefaultLabel:    "影像",
		SortOrder:       30,
		AllowedViews:    []ViewMode{ViewNone},
		CategoryEnabled: true,
		MediaCapability: MediaPrimaryVideo,
	},
}

var definitionsByKey = func() map[Key]Definition {
	index := make(map[Key]Definition, len(definitions))
	for _, def := range definitions {
		index[def.Key] = cloneDefinition(def)
	}
	return index
}()

func Lookup(key Key) (Definition, bool) {
	def, ok := definitionsByKey[key]
	if !ok {
		return Definition{}, false
	}
	return cloneDefinition(def), true
}

func Keys() []Key {
	keys := make([]Key, 0, len(definitions))
	for _, def := range definitions {
		keys = append(keys, def.Key)
	}
	return keys
}

func cloneDefinition(def Definition) Definition {
	def.AllowedViews = append([]ViewMode(nil), def.AllowedViews...)
	return def
}

func sortSettings(settings []Setting) {
	sort.Slice(settings, func(i, j int) bool {
		if settings[i].SortOrder == settings[j].SortOrder {
			return settings[i].World < settings[j].World
		}
		return settings[i].SortOrder < settings[j].SortOrder
	})
}

func validStatus(status Status) bool {
	switch status {
	case Unopened, Open, Hidden:
		return true
	default:
		return false
	}
}

func validNavLabel(label string) bool {
	trimmed := strings.TrimSpace(label)
	runes := utf8.RuneCountInString(trimmed)
	return runes >= 1 && runes <= 64
}

func validViewForWorld(key Key, view ViewMode) bool {
	def, ok := Lookup(key)
	if !ok {
		return false
	}
	for _, allowed := range def.AllowedViews {
		if allowed == view {
			return true
		}
	}
	return false
}
