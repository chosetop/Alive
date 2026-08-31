package media

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/p30huiwei/alive/backend/internal/storage"
	"net/url"
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
	URL       string
	MimeType  string
	ByteSize  int64
	CreatedAt time.Time
}
type Store interface {
	Create(context.Context, Media) (Media, error)
	Get(context.Context, int64) (Media, error)
}
type Service struct{ store Store }

type PresignInput struct {
	AuthorID, EntryID  int64
	Filename, MIMEType string
	SizeBytes          int64
}
type SignedUpload struct {
	ObjectKey string
	Upload    storage.PresignedPut
}
type RegisterInput struct {
	AuthorID, EntryID   int64
	ObjectKey, MIMEType string
	SizeBytes           int64
	Width, Height       *int
}

type UploadSigner interface {
	PresignPut(context.Context, string, string, time.Duration) (storage.PresignedPut, error)
	Head(context.Context, string) (storage.ObjectInfo, error)
}

type ConfiguredService struct {
	store         Store
	signer        UploadSigner
	publicBaseURL string
	ttl           time.Duration
	now           func() time.Time
}

func NewConfiguredService(store Store, signer UploadSigner, publicBaseURL string, ttl time.Duration) *ConfiguredService {
	return &ConfiguredService{store: store, signer: signer, publicBaseURL: strings.TrimRight(publicBaseURL, "/"), ttl: ttl, now: time.Now}
}
func (s *ConfiguredService) Presign(ctx context.Context, in PresignInput) (SignedUpload, error) {
	if s.signer == nil {
		return SignedUpload{}, errors.New("media: storage unavailable")
	}
	if err := ValidateUpload(in.MIMEType, in.SizeBytes); err != nil {
		return SignedUpload{}, err
	}
	key, err := NewObjectKey(in.AuthorID, in.EntryID, in.MIMEType, s.now())
	if err != nil {
		return SignedUpload{}, err
	}
	signed, err := s.signer.PresignPut(ctx, key, in.MIMEType, s.ttl)
	if err != nil {
		return SignedUpload{}, err
	}
	return SignedUpload{ObjectKey: key, Upload: signed}, nil
}
func (s *ConfiguredService) Register(ctx context.Context, in RegisterInput) (Media, error) {
	if s.signer == nil {
		return Media{}, errors.New("media: storage unavailable")
	}
	prefix := fmt.Sprintf("media/%d/%d/", in.AuthorID, in.EntryID)
	if !strings.HasPrefix(in.ObjectKey, prefix) {
		return Media{}, errors.New("media: object key outside entry prefix")
	}
	if err := ValidateUpload(in.MIMEType, in.SizeBytes); err != nil {
		return Media{}, err
	}
	info, err := s.signer.Head(ctx, in.ObjectKey)
	if err != nil {
		return Media{}, err
	}
	if info.SizeBytes != in.SizeBytes || strings.ToLower(info.MIMEType) != strings.ToLower(in.MIMEType) {
		return Media{}, errors.New("media: uploaded object metadata mismatch")
	}
	m := Media{AuthorID: in.AuthorID, ObjectKey: in.ObjectKey, MimeType: in.MIMEType, ByteSize: in.SizeBytes}
	if s.publicBaseURL != "" {
		m.URL = s.publicBaseURL + "/" + url.PathEscape(in.ObjectKey)
	}
	return s.store.Create(ctx, m)
}

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
