package storage

import (
	"context"
	"time"
)

type PresignedPut struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
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
