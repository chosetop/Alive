package sitehttp

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/site"
)

type siteSettingsResponse struct {
	DefaultTheme string    `json:"default_theme"`
	Revision     int64     `json:"revision"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func newSiteSettingsResponse(settings site.SiteSettings) siteSettingsResponse {
	return siteSettingsResponse{
		DefaultTheme: settings.DefaultTheme,
		Revision:     settings.Revision,
		UpdatedAt:    settings.UpdatedAt,
	}
}

type updateSiteSettingsRequest struct {
	DefaultTheme string `json:"default_theme" binding:"required"`
	Revision     int64  `json:"revision" binding:"required,min=1"`
}

func (r updateSiteSettingsRequest) toInput() (string, int64) {
	return r.DefaultTheme, r.Revision
}

func bindUpdateSiteSettings(c *gin.Context) (updateSiteSettingsRequest, error) {
	var request updateSiteSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return updateSiteSettingsRequest{}, err
	}
	return request, nil
}
