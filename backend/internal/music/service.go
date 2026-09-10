package music

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/p30huiwei/alive/backend/internal/storage"
)

type Store interface {
	SaveAsset(context.Context, Asset) (Asset, error)
	Catalog(context.Context, int64, bool) (Catalog, error)
	SaveTrack(context.Context, int64, int64, TrackInput) (Track, error)
	SavePlaylist(context.Context, int64, int64, PlaylistInput) (Playlist, error)
	Delete(context.Context, int64, int64, int64, bool) error
}
type Service struct {
	store   Store
	signer  storage.Client
	baseURL string
	ttl     time.Duration
}

func NewService(store Store, signer storage.Client, baseURL string, ttl time.Duration) *Service {
	return &Service{store, signer, strings.TrimRight(baseURL, "/"), ttl}
}

type UploadInput struct {
	Filename  string `json:"filename"`
	ObjectKey string `json:"object_key"`
	MIMEType  string `json:"mime_type"`
	SizeBytes int64  `json:"size_bytes"`
}
type SignedUpload struct {
	ObjectKey string               `json:"object_key"`
	Upload    storage.PresignedPut `json:"upload"`
}

func (s *Service) Presign(ctx context.Context, author int64, in UploadInput) (SignedUpload, error) {
	if s.signer == nil || s.baseURL == "" {
		return SignedUpload{}, ErrUnavailable
	}
	_, ext, err := UploadType(in.MIMEType, in.SizeBytes)
	if err != nil || author < 1 || strings.TrimSpace(in.Filename) == "" {
		return SignedUpload{}, ErrInvalid
	}
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return SignedUpload{}, err
	}
	key := fmt.Sprintf("music/%d/%x.%s", author, b, ext)
	upload, err := s.signer.PresignPut(ctx, key, in.MIMEType, s.ttl)
	return SignedUpload{key, upload}, err
}

var objectName = regexp.MustCompile(`^[a-f0-9]{32}\.[a-z0-9]+$`)

func (s *Service) RegisterUpload(ctx context.Context, author int64, in UploadInput) (Asset, error) {
	if s.signer == nil || s.baseURL == "" {
		return Asset{}, ErrUnavailable
	}
	kind, ext, err := UploadType(in.MIMEType, in.SizeBytes)
	if err != nil || author < 1 {
		return Asset{}, ErrInvalid
	}
	prefix := fmt.Sprintf("music/%d/", author)
	name := strings.TrimPrefix(in.ObjectKey, prefix)
	if !strings.HasPrefix(in.ObjectKey, prefix) || !objectName.MatchString(name) || !strings.HasSuffix(name, "."+ext) {
		return Asset{}, ErrInvalid
	}
	info, err := s.signer.Head(ctx, in.ObjectKey)
	if err != nil {
		return Asset{}, err
	}
	if info.SizeBytes != in.SizeBytes || !strings.EqualFold(info.MIMEType, in.MIMEType) {
		return Asset{}, ErrInvalid
	}
	return s.store.SaveAsset(ctx, Asset{AuthorID: author, ObjectKey: in.ObjectKey, URL: s.baseURL + "/" + in.ObjectKey, Kind: kind, MIMEType: in.MIMEType, SizeBytes: in.SizeBytes})
}
func (s *Service) Catalog(ctx context.Context, author int64, public bool) (Catalog, error) {
	return s.store.Catalog(ctx, author, public)
}
func (s *Service) SaveTrack(ctx context.Context, author, id int64, in TrackInput) (Track, error) {
	if author < 1 || id < 0 {
		return Track{}, ErrInvalid
	}
	if err := in.Validate(id == 0); err != nil {
		return Track{}, err
	}
	return s.store.SaveTrack(ctx, author, id, in)
}
func (s *Service) SavePlaylist(ctx context.Context, author, id int64, in PlaylistInput) (Playlist, error) {
	if author < 1 || id < 0 {
		return Playlist{}, ErrInvalid
	}
	if err := in.Validate(id == 0); err != nil {
		return Playlist{}, err
	}
	return s.store.SavePlaylist(ctx, author, id, in)
}
func (s *Service) Delete(ctx context.Context, author, id, revision int64, playlist bool) error {
	if author < 1 || id < 1 || revision < 1 {
		return ErrInvalid
	}
	return s.store.Delete(ctx, author, id, revision, playlist)
}
