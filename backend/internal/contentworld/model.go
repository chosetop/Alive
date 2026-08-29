package contentworld

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
