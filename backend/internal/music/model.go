package music

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

var (
	ErrInvalid     = errors.New("invalid music input")
	ErrNotFound    = errors.New("music item not found")
	ErrConflict    = errors.New("music changed; reload before saving")
	ErrUnavailable = errors.New("music storage unavailable")
)

type Asset struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	AuthorID  int64  `json:"-"`
	ObjectKey string `json:"-"`
	MIMEType  string `json:"-"`
	SizeBytes int64  `json:"-"`
}
type Track struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Artist   string  `json:"artist"`
	AudioURL string  `json:"audio_url"`
	CoverURL string  `json:"cover_url"`
	Duration float64 `json:"duration"`
	Revision int64   `json:"revision"`
}
type Playlist struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	CoverURL  string  `json:"cover_url"`
	IsPublic  bool    `json:"is_public"`
	IsDefault bool    `json:"is_default"`
	TrackIDs  []int64 `json:"track_ids"`
	Revision  int64   `json:"revision"`
}
type Catalog struct {
	Tracks    []Track    `json:"tracks"`
	Playlists []Playlist `json:"playlists"`
}

// OptionalID distinguishes omitting a cover (preserve) from null (clear).
type OptionalID struct {
	Set   bool
	Value *int64
}
type TrackInput struct {
	Title        string
	Artist       string
	AudioAssetID *int64
	CoverAssetID OptionalID
	Duration     float64
	Revision     int64
}
type PlaylistInput struct {
	Name         string
	CoverAssetID OptionalID
	IsPublic     bool
	IsDefault    bool
	TrackIDs     []int64
	Revision     int64
}

func validText(s string, required bool) bool {
	n := utf8.RuneCountInString(s)
	return n <= 200 && (!required || n > 0)
}
func (in *TrackInput) Validate(create bool) error {
	in.Title = strings.TrimSpace(in.Title)
	in.Artist = strings.TrimSpace(in.Artist)
	if !validText(in.Title, true) || !validText(in.Artist, false) || math.IsNaN(in.Duration) || math.IsInf(in.Duration, 0) || in.Duration < 0 || in.Duration > 86400 {
		return ErrInvalid
	}
	if create && in.AudioAssetID == nil || in.AudioAssetID != nil && *in.AudioAssetID <= 0 || in.CoverAssetID.Value != nil && *in.CoverAssetID.Value <= 0 || !create && in.Revision < 1 {
		return ErrInvalid
	}
	return nil
}
func (in *PlaylistInput) Validate(create bool) error {
	in.Name = strings.TrimSpace(in.Name)
	if !validText(in.Name, true) || in.IsDefault && !in.IsPublic || len(in.TrackIDs) > 1000 || !create && in.Revision < 1 || in.CoverAssetID.Value != nil && *in.CoverAssetID.Value <= 0 {
		return ErrInvalid
	}
	seen := map[int64]bool{}
	for _, id := range in.TrackIDs {
		if id <= 0 || seen[id] {
			return ErrInvalid
		}
		seen[id] = true
	}
	return nil
}
func UploadType(mime string, size int64) (kind, ext string, err error) {
	switch mime {
	case "audio/mpeg":
		kind, ext = "audio", "mp3"
	case "audio/mp4":
		kind, ext = "audio", "m4a"
	case "audio/ogg":
		kind, ext = "audio", "ogg"
	case "audio/wav":
		kind, ext = "audio", "wav"
	case "image/jpeg":
		kind, ext = "image", "jpg"
	case "image/png":
		kind, ext = "image", "png"
	case "image/webp":
		kind, ext = "image", "webp"
	case "image/gif":
		kind, ext = "image", "gif"
	default:
		return "", "", ErrInvalid
	}
	limit := int64(100 * 1024 * 1024)
	if kind == "image" {
		limit = 20 * 1024 * 1024
	}
	if size <= 0 || size > limit {
		return "", "", ErrInvalid
	}
	return
}
