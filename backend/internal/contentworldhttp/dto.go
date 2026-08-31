package contentworldhttp

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/contentworld"
)

type publicWorldSetting struct {
	World       string `json:"world"`
	NavLabel    string `json:"nav_label"`
	SortOrder   int    `json:"sort_order"`
	DefaultView string `json:"default_view"`
}

type adminWorldSetting struct {
	World       string    `json:"world"`
	Status      string    `json:"status"`
	NavLabel    string    `json:"nav_label"`
	SortOrder   int       `json:"sort_order"`
	DefaultView string    `json:"default_view"`
	Revision    int64     `json:"revision"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type updateWorldRequest struct {
	Status      *string `json:"status"`
	NavLabel    *string `json:"nav_label"`
	DefaultView *string `json:"default_view"`
	Revision    int64   `json:"revision" binding:"required,min=1"`
}

func bindUpdateWorld(c *gin.Context) (updateWorldRequest, error) {
	var request updateWorldRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		return updateWorldRequest{}, err
	}
	return request, nil
}

func (r updateWorldRequest) toInput() contentworld.UpdateInput {
	input := contentworld.UpdateInput{ExpectedRevision: r.Revision}
	if r.Status != nil {
		status := contentworld.Status(*r.Status)
		input.Status = &status
	}
	if r.NavLabel != nil {
		input.NavLabel = r.NavLabel
	}
	if r.DefaultView != nil {
		view := contentworld.ViewMode(*r.DefaultView)
		input.DefaultView = &view
	}
	return input
}

func newPublicWorldSettings(settings []contentworld.Setting) []publicWorldSetting {
	out := make([]publicWorldSetting, 0, len(settings))
	for _, setting := range settings {
		out = append(out, publicWorldSetting{
			World:       string(setting.World),
			NavLabel:    setting.NavLabel,
			SortOrder:   setting.SortOrder,
			DefaultView: string(setting.DefaultView),
		})
	}
	return out
}

func newAdminWorldSettings(settings []contentworld.Setting) []adminWorldSetting {
	out := make([]adminWorldSetting, 0, len(settings))
	for _, setting := range settings {
		out = append(out, newAdminWorldSetting(setting))
	}
	return out
}

func newAdminWorldSetting(setting contentworld.Setting) adminWorldSetting {
	return adminWorldSetting{
		World:       string(setting.World),
		Status:      string(setting.Status),
		NavLabel:    setting.NavLabel,
		SortOrder:   setting.SortOrder,
		DefaultView: string(setting.DefaultView),
		Revision:    setting.Revision,
		UpdatedAt:   setting.UpdatedAt,
	}
}
