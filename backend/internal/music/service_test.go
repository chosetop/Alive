package music

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/storage"
)

type assetStore struct {
	Store
	saved []Asset
}

func (s *assetStore) SaveAsset(_ context.Context, a Asset) (Asset, error) {
	s.saved = append(s.saved, a)
	a.ID = 1
	return a, nil
}

type signerFake struct {
	info  storage.ObjectInfo
	heads int
}

func (s *signerFake) PresignPut(_ context.Context, key, mime string, _ time.Duration) (storage.PresignedPut, error) {
	return storage.PresignedPut{URL: "https://upload.test/" + key, Method: "PUT", Headers: map[string]string{"Content-Type": mime}}, nil
}
func (s *signerFake) Head(context.Context, string) (storage.ObjectInfo, error) {
	s.heads++
	return s.info, nil
}
func TestUploadOwnershipAndMetadata(t *testing.T) {
	ctx := context.Background()
	store := &assetStore{}
	signer := &signerFake{info: storage.ObjectInfo{SizeBytes: 12, MIMEType: "audio/mpeg"}}
	s := NewService(store, signer, "https://cdn.test/", time.Minute)
	in := UploadInput{Filename: "song.mp3", MIMEType: "audio/mpeg", SizeBytes: 12}
	signed, err := s.Presign(ctx, 7, in)
	if err != nil {
		t.Fatal(err)
	}
	in.ObjectKey = signed.ObjectKey
	if !strings.HasPrefix(in.ObjectKey, "music/7/") {
		t.Fatal(in.ObjectKey)
	}
	if _, err = s.RegisterUpload(ctx, 8, in); err != ErrInvalid || signer.heads != 0 {
		t.Fatalf("foreign prefix accepted: %v heads=%d", err, signer.heads)
	}
	original := in.ObjectKey
	in.ObjectKey = "music/7/../" + strings.TrimPrefix(original, "music/7/")
	if _, err = s.RegisterUpload(ctx, 7, in); err != ErrInvalid {
		t.Fatal("traversal accepted")
	}
	in.ObjectKey = original
	signer.info.SizeBytes = 13
	if _, err = s.RegisterUpload(ctx, 7, in); err != ErrInvalid {
		t.Fatal("wrong size accepted")
	}
	signer.info.SizeBytes = 12
	signer.info.MIMEType = "image/png"
	if _, err = s.RegisterUpload(ctx, 7, in); err != ErrInvalid {
		t.Fatal("wrong mime accepted")
	}
	signer.info.MIMEType = "audio/mpeg"
	a, err := s.RegisterUpload(ctx, 7, in)
	if err != nil || a.Kind != "audio" || a.URL != "https://cdn.test/"+in.ObjectKey || len(store.saved) != 1 {
		t.Fatalf("registration: %+v %v", a, err)
	}
}
func TestUnavailableStorage(t *testing.T) {
	s := NewService(nil, nil, "", time.Minute)
	if _, err := s.Presign(context.Background(), 1, UploadInput{}); err != ErrUnavailable {
		t.Fatal(err)
	}
	if _, err := s.RegisterUpload(context.Background(), 1, UploadInput{}); err != ErrUnavailable {
		t.Fatal(err)
	}
}
func TestMusicValidation(t *testing.T) {
	for _, mime := range []string{"audio/mpeg", "audio/mp4", "audio/ogg", "audio/wav"} {
		if kind, _, err := UploadType(mime, 100*1024*1024); err != nil || kind != "audio" {
			t.Fatal(mime, err)
		}
	}
	for _, in := range []struct {
		mime string
		size int64
	}{{"text/html", 1}, {"audio/mpeg", 0}, {"audio/mpeg", 100*1024*1024 + 1}, {"image/png", 20*1024*1024 + 1}} {
		if _, _, err := UploadType(in.mime, in.size); err != ErrInvalid {
			t.Fatal(in)
		}
	}
	id := int64(1)
	valid := TrackInput{Title: " song ", AudioAssetID: &id}
	if err := valid.Validate(true); err != nil || valid.Title != "song" {
		t.Fatal(err)
	}
	for _, duration := range []float64{-1, 86401, math.NaN(), math.Inf(1)} {
		in := TrackInput{Title: "song", AudioAssetID: &id, Duration: duration}
		if err := in.Validate(true); err != ErrInvalid {
			t.Fatal(duration)
		}
	}
	for _, in := range []PlaylistInput{{Name: ""}, {Name: "private", IsDefault: true}, {Name: "duplicate", TrackIDs: []int64{1, 1}}, {Name: "bad", TrackIDs: []int64{-1}}} {
		if err := in.Validate(true); err != ErrInvalid {
			t.Fatal(in)
		}
	}
}
