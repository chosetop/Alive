package aliyunoss

import (
	"context"
	"fmt"
	"time"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/storage"
)

type Client struct {
	client *oss.Client
	bucket string
}

func New(cfg config.OSSConfig) *Client {
	c := oss.LoadDefaultConfig().WithRegion(cfg.Region).WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.AccessKeySecret))
	if cfg.Endpoint != "" {
		c = c.WithEndpoint(cfg.Endpoint)
	}
	return &Client{client: oss.NewClient(c), bucket: cfg.Bucket}
}

func (c *Client) PresignPut(ctx context.Context, key, mime string, expires time.Duration) (storage.PresignedPut, error) {
	if expires <= 0 {
		return storage.PresignedPut{}, fmt.Errorf("aliyunoss: invalid expiration")
	}
	result, err := c.client.Presign(ctx, &oss.PutObjectRequest{Bucket: oss.Ptr(c.bucket), Key: oss.Ptr(key), ContentType: oss.Ptr(mime)}, oss.PresignExpires(expires))
	if err != nil {
		return storage.PresignedPut{}, fmt.Errorf("aliyunoss: presign put: %w", err)
	}
	return storage.PresignedPut{URL: result.URL, Method: result.Method, Headers: result.SignedHeaders, ExpiresAt: result.Expiration}, nil
}

func (c *Client) Head(ctx context.Context, key string) (storage.ObjectInfo, error) {
	result, err := c.client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(c.bucket), Key: oss.Ptr(key)})
	if err != nil {
		return storage.ObjectInfo{}, fmt.Errorf("aliyunoss: head object: %w", err)
	}
	info := storage.ObjectInfo{SizeBytes: result.ContentLength}
	if result.ContentType != nil {
		info.MIMEType = *result.ContentType
	}
	if result.ETag != nil {
		info.ETag = *result.ETag
	}
	return info, nil
}
