package entry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

type VideoMeta struct {
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	CapturedAt      *string `json:"captured_at,omitempty"`
	Place           string  `json:"place,omitempty"`
}

var ErrInvalidVideoMeta = errors.New("entry: invalid video meta")

func DecodeVideoMeta(raw Meta) (VideoMeta, error) {
	var m VideoMeta
	b, e := json.Marshal(raw)
	if e != nil {
		return m, ErrInvalidVideoMeta
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&m); e != nil {
		return m, ErrInvalidVideoMeta
	}
	if m.DurationSeconds != nil && (*m.DurationSeconds < 1 || *m.DurationSeconds > 86400) {
		return m, ErrInvalidVideoMeta
	}
	if m.CapturedAt != nil {
		if _, e := time.Parse(time.RFC3339, *m.CapturedAt); e != nil {
			return m, ErrInvalidVideoMeta
		}
	}
	if utf8.RuneCountInString(m.Place) > 200 {
		return m, ErrInvalidVideoMeta
	}
	return m, nil
}
func ValidateVideoForPublish(e Entry, hasCover, hasPrimaryVideo bool) error {
	if e.World != "video" {
		return fmt.Errorf("%w: wrong world", ErrInvalidVideoMeta)
	}
	if e.Title == "" || e.Slug == "" || !hasCover || !hasPrimaryVideo {
		return ErrInvalidVideoMeta
	}
	return nil
}
