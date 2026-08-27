package site

import (
	"errors"
	"time"
)

var (
	ErrInvalidTheme    = errors.New("site: invalid theme")
	ErrVersionConflict = errors.New("site: version conflict")
)

type SiteSettings struct {
	DefaultTheme string
	Revision     int64
	UpdatedAt    time.Time
}

func validTheme(theme string) bool {
	switch theme {
	case "ink", "lamp", "codex-lavender", "night-ink":
		return true
	default:
		return false
	}
}
