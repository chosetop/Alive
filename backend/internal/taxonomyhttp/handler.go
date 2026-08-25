// Package taxonomyhttp is the HTTP adapter over internal/taxonomy.
//
// It parses requests, chooses status codes and shapes responses. No rule about
// what a category may contain lives here: those are in internal/taxonomy, which
// is why the same rules can back a CLI command that sets up categories without a
// request.
package taxonomyhttp

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
)

// Handler serves the category endpoints.
//
// No AuthorResolver, unlike entryhttp. A category has no owner: it is site
// structure rather than content, so the only thing authentication decides here is
// whether the request may write at all.
type Handler struct {
	service *taxonomy.Service
	logger  *slog.Logger
}

// NewHandler wires a handler to the service.
func NewHandler(service *taxonomy.Service, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, logger: logger}
}

// Register adds the category routes to the API group.
//
// One public read and four guarded writes. The read is public because the site's
// navigation is: a reader has no session, and every category is meant to be seen.
// That is the whole difference from entries, where the queries themselves have to
// withhold rows.
//
// The writes are addressed by id while the public read has no addressed form yet.
// A category page will be GET /categories/:slug when the frontend needs one; today
// the entry list's ?category= filter is what a slug is used for.
func (h *Handler) Register(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	group := api.Group("/categories")

	// The guard is a parameter rather than something this file builds, so a route
	// cannot be registered with authentication accidentally left off: a nil guard is
	// refused here rather than silently making the writes public.
	if requireAuth == nil {
		panic("taxonomyhttp: requireAuth is required for the write routes")
	}

	group.POST("", requireAuth, h.Create)
	group.PATCH("/:id", requireAuth, h.Update)
	group.DELETE("/:id", requireAuth, h.Delete)

	// The public read. Last purely for readability; gin's tree resolves static and
	// parameter segments regardless of order.
	group.GET("", h.List)
}

// RegisterAdmin adds the authenticated read routes under the admin group.
//
// A separate list from the public one, and a separate query behind it. The public
// list carries entry counts, which cost a join and describe what a reader can see;
// this one is the editing surface and carries timestamps instead.
//
// The guard is on the group rather than per route, matching entryhttp: a route added
// here later is authenticated by construction, where a per-route guard is one
// omission away from a public route.
func (h *Handler) RegisterAdmin(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	if requireAuth == nil {
		panic("taxonomyhttp: requireAuth is required for the admin routes")
	}

	group := api.Group("/admin/categories", requireAuth)

	group.GET("", h.ListAdmin)
	group.GET("/:id", h.GetByID)
}

// Create stores one category.
func (h *Handler) Create(c *gin.Context) {
	var req createCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("this request body is not valid JSON").WithCause(err))
		return
	}

	created, err := h.service.Create(c.Request.Context(), req.toInput())
	if err != nil {
		h.writeCategoryError(c, "create category", err)
		return
	}

	h.logger.InfoContext(c.Request.Context(), "category created",
		slog.Int64("category_id", created.ID),
		slog.String("slug", created.Slug),
	)

	httpx.Created(c, newAdminCategory(created))
}

// List returns every category with the number of entries a reader can see in each.
//
// Not paginated. Categories are navigation, and a navigation structure that needs
// paging is one nobody can navigate. That is a claim about the data, not an
// oversight: a site with 200 categories has a different problem than a missing
// page parameter.
func (h *Handler) List(c *gin.Context) {
	categories, err := h.service.ListWithCounts(c.Request.Context())
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "list categories failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	// No pagination meta, so this is a plain OK rather than httpx.List: a meta block
	// reporting page 1 of 1 for a collection that is never paged would be noise a
	// client has to read past.
	httpx.OK(c, newPublicCategories(categories))
}

// ListAdmin returns every category with its timestamps, without counts.
func (h *Handler) ListAdmin(c *gin.Context) {
	categories, err := h.service.List(c.Request.Context())
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "list categories failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	httpx.OK(c, newAdminCategories(categories))
}

// GetByID returns one category.
//
// By id rather than slug because this is what the edit form loads, and the slug is
// one of the things being edited.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := categoryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	found, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.writeCategoryError(c, "read category", err)
		return
	}

	httpx.OK(c, newAdminCategory(found))
}

// Update applies a partial change to one category.
//
// PATCH, not PUT. A PUT would require the client to send the whole category back,
// which turns two editors on two tabs into a silent overwrite.
func (h *Handler) Update(c *gin.Context) {
	id, err := categoryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	var req updateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("this request body is not valid JSON").WithCause(err))
		return
	}

	updated, err := h.service.Update(c.Request.Context(), id, req.toInput())
	if err != nil {
		h.writeCategoryError(c, "update category", err)
		return
	}

	h.logger.InfoContext(c.Request.Context(), "category updated",
		slog.Int64("category_id", updated.ID),
		slog.String("slug", updated.Slug),
	)

	httpx.OK(c, newAdminCategory(updated))
}

// Delete removes one category.
//
// 204 on the first call and 404 on the second, matching the entry delete: after
// the first call no category has this id.
//
// The entries in it are not deleted and the request is not refused. They become
// uncategorised, through the foreign key's ON DELETE SET NULL. Nothing afterwards
// records which category they pointed at, which is why the service logs the delete:
// a 204 here can quietly change many entries.
func (h *Handler) Delete(c *gin.Context) {
	id, err := categoryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		h.writeCategoryError(c, "delete category", err)
		return
	}

	h.logger.InfoContext(c.Request.Context(), "category deleted",
		slog.Int64("category_id", id),
	)

	httpx.NoContent(c)
}

// writeCategoryError maps a domain error onto a status code.
//
// One place for the mapping, so two endpoints cannot answer the same error
// differently.
func (h *Handler) writeCategoryError(c *gin.Context, op string, err error) {
	switch {
	case errors.Is(err, taxonomy.ErrCategoryNotFound):
		httpx.Error(c, apperr.NotFound("no category matches this id"))

	case errors.Is(err, taxonomy.ErrNoUpdateFields):
		httpx.Error(c, apperr.InvalidInput("this request changes nothing").
			WithField("body", "name at least one field to change").
			WithCause(err))

	case errors.Is(err, taxonomy.ErrSlugTaken):
		// 409, not 400. The request is well formed and would succeed with a different
		// slug, and a client can act on that.
		httpx.Error(c, apperr.Conflict("a category already uses this slug").
			WithField("slug", "already taken").
			WithCause(err))

	case errors.Is(err, taxonomy.ErrInvalidSlug):
		httpx.Error(c, invalidField("slug",
			"must be lowercase letters and digits joined by single hyphens, at most 64 characters", err))

	case errors.Is(err, taxonomy.ErrInvalidName):
		httpx.Error(c, invalidField("name",
			"must be present and at most 64 characters", err))

	default:
		h.logger.ErrorContext(c.Request.Context(), op+" failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
	}
}

// categoryID reads the :id path parameter.
//
// Refused rather than defaulted to zero. An unparseable id is a client mistake, and
// a zero would reach the service and come back as a 404, which reads as "that
// category is gone" for a request that never named one.
func categoryID(c *gin.Context) (int64, error) {
	raw := c.Param("id")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		// 400, not 404. The path segment is not an id at all, so there is no resource
		// to be missing.
		return 0, apperr.InvalidInput("this category id is not a positive whole number").
			WithField("id", "must be a positive whole number")
	}

	return id, nil
}

// invalidField builds a 400 naming the field that was refused.
func invalidField(name, reason string, cause error) *apperr.Error {
	return apperr.InvalidInput("this category cannot be saved as submitted").
		WithField(name, reason).
		WithCause(cause)
}
