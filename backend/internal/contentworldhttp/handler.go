package contentworldhttp

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/contentworld"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

type Handler struct {
	service *contentworld.Service
	logger  *slog.Logger
}

func NewHandler(service *contentworld.Service, logger *slog.Logger) *Handler {
	if service == nil {
		panic("contentworldhttp: service is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

func (h *Handler) Register(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	if api == nil {
		panic("contentworldhttp: api group is required")
	}
	if requireAuth == nil {
		panic("contentworldhttp: requireAuth is required")
	}

	api.GET("/worlds", h.ListOpen)

	admin := api.Group("/admin/worlds", requireAuth)
	admin.GET("", h.ListAdmin)
	admin.PATCH("/:key", h.Update)
}

func (h *Handler) ListOpen(c *gin.Context) {
	settings, err := h.service.ListOpen(c.Request.Context())
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "list open worlds failed", slog.String("error", err.Error()))
		httpx.Error(c, apperr.From(err))
		return
	}
	httpx.OK(c, newPublicWorldSettings(settings))
}

func (h *Handler) ListAdmin(c *gin.Context) {
	settings, err := h.service.ListAdmin(c.Request.Context())
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "list worlds failed", slog.String("error", err.Error()))
		httpx.Error(c, apperr.From(err))
		return
	}
	httpx.OK(c, newAdminWorldSettings(settings))
}

func (h *Handler) Update(c *gin.Context) {
	request, err := bindUpdateWorld(c)
	if err != nil {
		httpx.Error(c, apperr.InvalidInput("this request body is not valid").WithCause(err))
		return
	}

	updated, err := h.service.Update(c.Request.Context(), contentworld.Key(c.Param("key")), request.toInput())
	if err != nil {
		h.writeUpdateError(c, err)
		return
	}
	httpx.OK(c, newAdminWorldSetting(updated))
}

func (h *Handler) writeUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, contentworld.ErrWorldNotFound):
		httpx.Error(c, apperr.NotFound("no world matches this key").WithCause(err))
	case errors.Is(err, contentworld.ErrInvalidStatus):
		httpx.Error(c, apperr.InvalidInput("the requested status is not supported").
			WithField("status", "must be unopened, open, or hidden").
			WithCause(err))
	case errors.Is(err, contentworld.ErrInvalidNavLabel):
		httpx.Error(c, apperr.InvalidInput("the requested label is not valid").
			WithField("nav_label", "must be 1 to 64 characters").
			WithCause(err))
	case errors.Is(err, contentworld.ErrInvalidViewMode):
		httpx.Error(c, apperr.InvalidInput("the requested view is not supported for this world").
			WithField("default_view", "choose one of this world's allowed views").
			WithCause(err))
	case errors.Is(err, contentworld.ErrVersionConflict):
		httpx.Error(c, apperr.Conflict("world settings changed since they were loaded").
			WithField("revision", "reload the current world settings and try again").
			WithCause(err))
	default:
		h.logger.ErrorContext(c.Request.Context(), "update world failed", slog.String("error", err.Error()))
		httpx.Error(c, apperr.From(err))
	}
}
