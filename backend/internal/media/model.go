package media

import (
	"context"
	"errors"
	"time"
)

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
