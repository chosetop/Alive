package media

import "errors"

var ErrInvalidMime = errors.New("media: invalid mime type")
var ErrInvalidSize = errors.New("media: invalid size")

const (
	MaxImageBytes = 20 * 1024 * 1024
	MaxVideoBytes = 500 * 1024 * 1024
)

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
