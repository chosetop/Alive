package storage

import (
	"context"
	"time"
)

type PresignedPut struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

type ObjectInfo struct {
	SizeBytes int64
	MIMEType  string
	ETag      string
}

type Client interface {
	PresignPut(context.Context, string, string, time.Duration) (PresignedPut, error)
	Head(context.Context, string) (ObjectInfo, error)
}
