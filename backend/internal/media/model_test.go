package media

import (
	"strings"
	"testing"
	"time"
)

func TestValidateUpload(t *testing.T) {
	if err := ValidateUpload("video/mp4", MaxVideoBytes); err != nil {
		t.Fatal(err)
	}
	if err := ValidateUpload("video/mp4", MaxVideoBytes+1); err == nil {
		t.Fatal("oversized video accepted")
	}
	if err := ValidateUpload("video/webm", 1); err != ErrInvalidMime {
		t.Fatalf("mime error = %v", err)
	}
	if err := ValidateUpload("image/png", MaxImageBytes+1); err != ErrInvalidSize {
		t.Fatalf("image size error = %v", err)
	}
}

func TestObjectKeyUsesValidatedMIMEAndEntryPrefix(t *testing.T) {
	key, err := NewObjectKey(7, 42, "image/png", time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(key, "media/7/42/2026/08/") || !strings.HasSuffix(key, ".png") {
		t.Fatalf("key = %q", key)
	}
}
