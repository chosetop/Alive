package mediahttp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/media"
	"github.com/p30huiwei/alive/backend/internal/mediahttp"
)

type fakeService struct{}

func (fakeService) Presign(context.Context, media.PresignInput) (media.SignedUpload, error) {
	return media.SignedUpload{}, nil
}
func (fakeService) Register(context.Context, media.RegisterInput) (media.Media, error) {
	return media.Media{}, nil
}
func (fakeService) ListForEntry(context.Context, int64, int64) ([]media.Media, error) {
	return nil, nil
}
func (fakeService) SetPrimaryVideo(context.Context, int64, int64, int64, int64) (int64, error) {
	return 0, nil
}
func (fakeService) GetPrimaryVideoByEntry(context.Context, int64) (media.Media, error) {
	return media.Media{}, nil
}

func TestMediaRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	e := gin.New()
	api := e.Group("/api/v1")
	h := mediahttp.NewHandler(fakeService{}, func(*gin.Context) (int64, bool) { return 0, false })
	h.Register(api, func(c *gin.Context) { c.Next() })
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/media/presign", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
