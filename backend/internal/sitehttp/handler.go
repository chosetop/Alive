package sitehttp

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/site"
)

type Handler struct {
	service *site.Service
	logger  *slog.Logger
}

func NewHandler(service *site.Service, logger *slog.Logger) *Handler {
	if service == nil {
		panic("sitehttp: service is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Register(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	if api == nil {
		panic("sitehttp: api group is required")
	}
	if requireAuth == nil {
		panic("sitehttp: requireAuth is required")
	}

	api.GET("/site", h.Get)
	api.PATCH("/admin/site", requireAuth, h.Update)
}

func (h *Handler) Get(c *gin.Context) {
	settings, err := h.service.Get(c.Request.Context())
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "get site settings failed", slog.String("error", err.Error()))
		httpx.Error(c, apperr.From(err))
		return
	}
	httpx.OK(c, newSiteSettingsResponse(settings))
}

func (h *Handler) Update(c *gin.Context) {
	request, err := bindUpdateSiteSettings(c)
	if err != nil {
		httpx.Error(c, apperr.InvalidInput("this request body is not valid").WithCause(err))
		return
	}

	theme, revision := request.toInput()
	updated, err := h.service.UpdateTheme(c.Request.Context(), theme, revision)
	if err != nil {
		h.writeUpdateError(c, err)
		return
	}
	httpx.OK(c, newSiteSettingsResponse(updated))
}

func (h *Handler) writeUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, site.ErrInvalidTheme):
		httpx.Error(c, apperr.InvalidInput("the requested theme is not supported").WithField("default_theme", "must be ink, lamp, or codex-lavender").WithCause(err))
	case errors.Is(err, site.ErrVersionConflict):
		httpx.Error(c, apperr.Conflict("site settings changed since they were loaded").WithField("revision", "reload the current site settings and try again").WithCause(err))
	default:
		h.logger.ErrorContext(c.Request.Context(), "update site settings failed", slog.String("error", err.Error()))
		httpx.Error(c, apperr.From(err))
	}
}
