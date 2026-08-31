package media

import "testing"

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
