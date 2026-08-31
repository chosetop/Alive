package aliyunoss_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/storage/aliyunoss"
)

func TestPresignPutTranslatesObjectAndContentType(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	client := aliyunoss.NewWithHTTPClient(config.OSSConfig{Region: "cn-hangzhou", Endpoint: server.URL, Bucket: "alive-media", AccessKeyID: "test-id", AccessKeySecret: "test-secret"}, server.Client())
	result, err := client.PresignPut(context.Background(), "media/7/42/2026/08/a.png", "image/png", 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if result.Method != "PUT" || !strings.Contains(result.URL, "a.png") {
		t.Fatalf("result = %+v", result)
	}
	if result.Headers["Content-Type"] != "image/png" {
		t.Fatalf("headers = %#v", result.Headers)
	}
}
