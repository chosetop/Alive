package mediahttp

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/media"
)

type AuthorResolver func(*gin.Context) (int64, bool)
type Service interface {
	Presign(context.Context, media.PresignInput) (media.SignedUpload, error)
	Register(context.Context, media.RegisterInput) (media.Media, error)
}

type Handler struct {
	service Service
	author  AuthorResolver
}

func NewHandler(service Service, author AuthorResolver) *Handler {
	return &Handler{service: service, author: author}
}
func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/admin/media", auth)
	g.POST("/presign", h.Presign)
	g.POST("", h.RegisterMedia)
}

type presignRequest struct {
	EntryID   int64  `json:"entry_id" binding:"required,min=1"`
	Filename  string `json:"filename" binding:"required"`
	MIMEType  string `json:"mime_type" binding:"required"`
	SizeBytes int64  `json:"size_bytes" binding:"required,min=1"`
}
type registerRequest struct {
	EntryID   int64  `json:"entry_id" binding:"required,min=1"`
	ObjectKey string `json:"object_key" binding:"required"`
	MIMEType  string `json:"mime_type" binding:"required"`
	SizeBytes int64  `json:"size_bytes" binding:"required,min=1"`
}

func (h *Handler) Presign(c *gin.Context) {
	author, ok := h.author(c)
	if !ok {
		httpx.Error(c, apperr.Unauthorized("authentication required"))
		return
	}
	var req presignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("invalid media presign request"))
		return
	}
	result, err := h.service.Presign(c.Request.Context(), media.PresignInput{AuthorID: author, EntryID: req.EntryID, Filename: req.Filename, MIMEType: req.MIMEType, SizeBytes: req.SizeBytes})
	if err != nil {
		h.writeErr(c, err)
		return
	}
	httpx.OK(c, gin.H{"object_key": result.ObjectKey, "upload": result.Upload})
}
func (h *Handler) RegisterMedia(c *gin.Context) {
	author, ok := h.author(c)
	if !ok {
		httpx.Error(c, apperr.Unauthorized("authentication required"))
		return
	}
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("invalid media registration request"))
		return
	}
	result, err := h.service.Register(c.Request.Context(), media.RegisterInput{AuthorID: author, EntryID: req.EntryID, ObjectKey: req.ObjectKey, MIMEType: req.MIMEType, SizeBytes: req.SizeBytes})
	if err != nil {
		h.writeErr(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, media.ErrInvalidMime), errors.Is(err, media.ErrInvalidSize):
		httpx.Error(c, apperr.InvalidInput(err.Error()))
	case err.Error() == "media: storage unavailable":
		httpx.Error(c, apperr.Unavailable("media storage unavailable"))
	default:
		httpx.Error(c, apperr.From(err))
	}
}
