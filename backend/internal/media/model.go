package media

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func NewObjectKey(authorID, entryID int64, mime string, now time.Time) (string, error) {
	if authorID <= 0 || entryID <= 0 {
		return "", errors.New("media: invalid owner")
	}
	ext := map[string]string{"image/jpeg": "jpg", "image/png": "png", "image/webp": "webp", "image/gif": "gif", "video/mp4": "mp4"}[strings.ToLower(mime)]
	if ext == "" {
		return "", ErrInvalidMime
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("media: random key: %w", err)
	}
	return filepath.ToSlash(fmt.Sprintf("media/%d/%d/%04d/%02d/%x.%s", authorID, entryID, now.UTC().Year(), int(now.UTC().Month()), b, ext)), nil
}

var ErrInvalidMime = errors.New("media: invalid mime type")
var ErrInvalidSize = errors.New("media: invalid size")

const (
	MaxImageBytes = 20 * 1024 * 1024
	MaxVideoBytes = 500 * 1024 * 1024
)

type Media struct {
	ID        int64
	AuthorID  int64
	ObjectKey string
	MimeType  string
	ByteSize  int64
	CreatedAt time.Time
}
type Store interface {
	Create(context.Context, Media) (Media, error)
	Get(context.Context, int64) (Media, error)
}
type Service struct{ store Store }

func NewService(store Store) *Service { return &Service{store: store} }
func (s *Service) Register(ctx context.Context, m Media) (Media, error) {
	if m.AuthorID <= 0 || m.ObjectKey == "" {
		return Media{}, errors.New("media: invalid metadata")
	}
	if err := ValidateUpload(m.MimeType, m.ByteSize); err != nil {
		return Media{}, err
	}
	return s.store.Create(ctx, m)
}

func ValidateUpload(mime string, size int64) error {
	if size <= 0 {
		return ErrInvalidSize
	}
	if mime == "video/mp4" {
		if size > MaxVideoBytes {
			return ErrInvalidSize
		}
		return nil
	}
	switch mime {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
		if size > MaxImageBytes {
			return ErrInvalidSize
		}
		return nil
	}
	return ErrInvalidMime
}
