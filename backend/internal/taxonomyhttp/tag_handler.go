package taxonomyhttp

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
	"strconv"
)

type TagHandler struct{ service *taxonomy.TagService }

func NewTagHandler(service *taxonomy.TagService) *TagHandler { return &TagHandler{service: service} }

type tagRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}
type tagPatch struct {
	Name *string `json:"name"`
	Slug *string `json:"slug"`
}
type tagResponse struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	UsageCount int64  `json:"usage_count"`
}
type publicTagEntry struct {
	World    string `json:"world"`
	Kind     string `json:"kind"`
	Slug     string `json:"slug"`
	Title    string `json:"title,omitempty"`
	Excerpt  string `json:"excerpt,omitempty"`
	CoverURL string `json:"cover_url,omitempty"`
}

func tagOut(t taxonomy.Tag) tagResponse {
	return tagResponse{ID: t.ID, Name: t.Name, Slug: t.Slug, UsageCount: t.UsageCount}
}
func (h *TagHandler) RegisterAdmin(api *gin.RouterGroup, auth gin.HandlerFunc) {
	g := api.Group("/admin/tags", auth)
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PATCH("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
}
func (h *TagHandler) RegisterPublic(api *gin.RouterGroup) {
	api.GET("/tags/:slug/entries", h.PublicEntries)
}
func (h *TagHandler) PublicEntries(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	size, _ := strconv.Atoi(c.Query("page_size"))
	tag, items, total, err := h.service.PublicEntries(c, c.Param("slug"), page, size)
	if err != nil {
		if errors.Is(err, taxonomy.ErrTagNotFound) {
			httpx.Error(c, apperr.NotFound("tag not found"))
		} else {
			httpx.Error(c, apperr.From(err))
		}
		return
	}
	out := make([]publicTagEntry, 0, len(items))
	for _, item := range items {
		out = append(out, publicTagEntry{World: item.World, Kind: item.Kind, Slug: item.Slug, Title: item.Title, Excerpt: item.Summary, CoverURL: item.CoverURL})
	}
	httpx.OK(c, gin.H{"tag": tagOut(tag), "items": out, "meta": gin.H{"page": maxInt(page, 1), "page_size": normalizedSize(size), "total": total}})
}
func maxInt(v, d int) int {
	if v < d {
		return d
	}
	return v
}
func normalizedSize(v int) int {
	if v < 1 {
		return 20
	}
	if v > 50 {
		return 50
	}
	return v
}
func (h *TagHandler) List(c *gin.Context) {
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if n, e := strconv.Atoi(raw); e == nil {
			limit = n
		}
	}
	tags, err := h.service.List(c, c.Query("q"), limit)
	if err != nil {
		httpx.Error(c, apperr.From(err))
		return
	}
	out := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		out = append(out, tagOut(t))
	}
	httpx.OK(c, out)
}
func (h *TagHandler) Create(c *gin.Context) {
	var req tagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("invalid tag request"))
		return
	}
	t, err := h.service.Create(c, taxonomy.CreateTagInput{Name: req.Name, Slug: req.Slug})
	if err != nil {
		h.writeErr(c, err)
		return
	}
	httpx.Created(c, tagOut(t))
}
func (h *TagHandler) Update(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		httpx.Error(c, apperr.InvalidInput("invalid tag id"))
		return
	}
	var req tagPatch
	if e := c.ShouldBindJSON(&req); e != nil {
		httpx.Error(c, apperr.InvalidInput("invalid tag request"))
		return
	}
	t, e := h.service.Update(c, id, taxonomy.UpdateTagInput{Name: req.Name, Slug: req.Slug})
	if e != nil {
		h.writeErr(c, e)
		return
	}
	httpx.OK(c, tagOut(t))
}
func (h *TagHandler) Delete(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil {
		httpx.Error(c, apperr.InvalidInput("invalid tag id"))
		return
	}
	if e = h.service.Delete(c, id); e != nil {
		h.writeErr(c, e)
		return
	}
	httpx.NoContent(c)
}
func (h *TagHandler) writeErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, taxonomy.ErrTagNotFound):
		httpx.Error(c, apperr.NotFound("tag not found"))
	case errors.Is(err, taxonomy.ErrTagNameTaken), errors.Is(err, taxonomy.ErrTagSlugTaken):
		httpx.Error(c, apperr.Conflict("tag already exists"))
	case errors.Is(err, taxonomy.ErrTagInUse):
		httpx.Error(c, apperr.Conflict("tag is in use"))
	default:
		httpx.Error(c, apperr.From(err))
	}
}
