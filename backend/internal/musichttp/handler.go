package musichttp

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/music"
)

type Handler struct {
	service *music.Service
	author  func(*gin.Context) (int64, bool)
}

func NewHandler(service *music.Service, author func(*gin.Context) (int64, bool)) *Handler {
	return &Handler{service, author}
}
func (h *Handler) Register(api *gin.RouterGroup, auth gin.HandlerFunc) {
	api.GET("/music", h.Public)
	g := api.Group("/admin/music", auth)
	g.GET("", h.Admin)
	g.POST("/uploads/presign", h.Presign)
	g.POST("/uploads", h.Upload)
	g.POST("/tracks", h.Track)
	g.PUT("/tracks/:id", h.Track)
	g.DELETE("/tracks/:id", h.DeleteTrack)
	g.POST("/playlists", h.Playlist)
	g.PUT("/playlists/:id", h.Playlist)
	g.DELETE("/playlists/:id", h.DeletePlaylist)
}
func (h *Handler) owner(c *gin.Context) (int64, bool) {
	id, ok := h.author(c)
	if !ok || id < 1 {
		httpx.Error(c, apperr.Unauthorized("authentication required"))
		return 0, false
	}
	return id, true
}
func writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, music.ErrInvalid):
		httpx.Error(c, apperr.InvalidInput(err.Error()))
	case errors.Is(err, music.ErrNotFound):
		httpx.Error(c, apperr.NotFound(err.Error()))
	case errors.Is(err, music.ErrConflict):
		httpx.Error(c, apperr.Conflict(err.Error()))
	case errors.Is(err, music.ErrUnavailable):
		httpx.Error(c, apperr.Unavailable(err.Error()))
	default:
		httpx.Error(c, apperr.From(err))
	}
}
func (h *Handler) Public(c *gin.Context) {
	result, err := h.service.Catalog(c.Request.Context(), 0, true)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) Admin(c *gin.Context) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	result, err := h.service.Catalog(c.Request.Context(), author, false)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func bind(c *gin.Context, v any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	if err := c.ShouldBindJSON(v); err != nil {
		writeError(c, music.ErrInvalid)
		return false
	}
	return true
}
func (h *Handler) Presign(c *gin.Context) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	var in music.UploadInput
	if !bind(c, &in) {
		return
	}
	result, err := h.service.Presign(c.Request.Context(), author, in)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) Upload(c *gin.Context) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	var in music.UploadInput
	if !bind(c, &in) {
		return
	}
	result, err := h.service.RegisterUpload(c.Request.Context(), author, in)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func parseID(c *gin.Context) (int64, bool) {
	if c.Request.Method == "POST" {
		return 0, true
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(c, music.ErrInvalid)
		return 0, false
	}
	return id, true
}
func cover(raw json.RawMessage) (music.OptionalID, error) {
	if len(raw) == 0 {
		return music.OptionalID{}, nil
	}
	var id *int64
	if err := json.Unmarshal(raw, &id); err != nil {
		return music.OptionalID{}, music.ErrInvalid
	}
	return music.OptionalID{Set: true, Value: id}, nil
}
func (h *Handler) Track(c *gin.Context) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		Title        string          `json:"title" binding:"required"`
		Artist       string          `json:"artist"`
		AudioAssetID *int64          `json:"audio_asset_id"`
		CoverAssetID json.RawMessage `json:"cover_asset_id"`
		Duration     float64         `json:"duration"`
		Revision     int64           `json:"revision"`
	}
	if !bind(c, &req) {
		return
	}
	cv, err := cover(req.CoverAssetID)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.service.SaveTrack(c.Request.Context(), author, id, music.TrackInput{Title: req.Title, Artist: req.Artist, AudioAssetID: req.AudioAssetID, CoverAssetID: cv, Duration: req.Duration, Revision: req.Revision})
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) Playlist(c *gin.Context) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req struct {
		Name         string          `json:"name" binding:"required"`
		CoverAssetID json.RawMessage `json:"cover_asset_id"`
		IsPublic     bool            `json:"is_public"`
		IsDefault    bool            `json:"is_default"`
		TrackIDs     []int64         `json:"track_ids"`
		Revision     int64           `json:"revision"`
	}
	if !bind(c, &req) {
		return
	}
	cv, err := cover(req.CoverAssetID)
	if err != nil {
		writeError(c, err)
		return
	}
	result, err := h.service.SavePlaylist(c.Request.Context(), author, id, music.PlaylistInput{Name: req.Name, CoverAssetID: cv, IsPublic: req.IsPublic, IsDefault: req.IsDefault, TrackIDs: req.TrackIDs, Revision: req.Revision})
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, result)
}
func (h *Handler) DeleteTrack(c *gin.Context)    { h.delete(c, false) }
func (h *Handler) DeletePlaylist(c *gin.Context) { h.delete(c, true) }
func (h *Handler) delete(c *gin.Context, playlist bool) {
	author, ok := h.owner(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	revision, err := strconv.ParseInt(c.Query("revision"), 10, 64)
	if err != nil {
		writeError(c, music.ErrInvalid)
		return
	}
	if err = h.service.Delete(c.Request.Context(), author, id, revision, playlist); err != nil {
		writeError(c, err)
		return
	}
	httpx.NoContent(c)
}
