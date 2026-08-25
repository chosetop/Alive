// Package entryhttp is the HTTP adapter over internal/entry.
//
// It parses requests, chooses status codes and shapes responses. No rule about
// what an entry may contain or who may read one lives here: those are in
// internal/entry, which is why the same rules can back a CLI import without a
// request.
package entryhttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/httpx"
)

// CategoryResolver turns a category slug into its id.
//
// Declared here, at the consumer, rather than importing the taxonomy adapter or
// the taxonomy service. The two adapters then have no dependency between them, and
// a handler test can resolve a slug without a categories table.
//
// The resolver maps its own errors onto statuses and returns an *apperr.Error,
// rather than returning a domain sentinel for this handler to interpret. It has to
// be that way round: recognising taxonomy.ErrCategoryNotFound here would mean
// importing taxonomy, which is the dependency this type exists to avoid. Anything
// else it returns becomes a 500, which is the right answer for a failed query.
//
// Nil is allowed: the ?category= filter is then refused rather than silently
// ignored, because ignoring it would answer a filtered request with the unfiltered
// list.
type CategoryResolver func(c *gin.Context, slug string) (int64, error)

// AuthorResolver reports which account a request is acting as.
//
// Declared here, at the consumer, rather than importing the auth adapter. The two
// adapters then have no dependency between them, and a handler test can supply an
// author without a session, a cookie or a database.
//
// The second result is false when the request carries no identity.
type AuthorResolver func(c *gin.Context) (int64, bool)

// Handler serves the entry endpoints.
type Handler struct {
	service  *entry.Service
	author   AuthorResolver
	category CategoryResolver
	logger   *slog.Logger
}

// NewHandler wires a handler to the service.
//
// author may be nil, in which case the create endpoint reports an internal error
// rather than writing an entry with no owner.
//
// category may be nil, in which case a request carrying ?category= is refused. The
// unfiltered list still works: the filter is the only thing that needs it.
func NewHandler(service *entry.Service, author AuthorResolver, category CategoryResolver, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{service: service, author: author, category: category, logger: logger}
}

// Register adds the entry routes to the API group.
//
// requireAuth guards the writes. The two reads are public because the site's
// front page is: a reader has no session, and the queries behind those routes
// return published public rows and nothing else.
//
// The writes are addressed by id while the public detail read is addressed by
// slug. That is not an inconsistency: a reader knows an entry by its URL, and an
// editor is changing an entry whose slug may be what is being changed.
func (h *Handler) Register(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	group := api.Group("/entries")

	// The guard is a parameter rather than something this file builds, so the
	// route cannot be registered with authentication accidentally left off: a nil
	// guard is refused here rather than silently making the endpoint public.
	if requireAuth == nil {
		panic("entryhttp: requireAuth is required for the write routes")
	}

	group.POST("", requireAuth, h.Create)
	group.PATCH("/:id", requireAuth, h.Update)
	group.DELETE("/:id", requireAuth, h.Delete)

	// Publishing is its own route rather than a status field on PATCH. The
	// published_at rule (written once, never moved) belongs to one place, and a
	// client asking to publish is asking for a state change rather than editing a
	// value.
	group.POST("/:id/publish", requireAuth, h.Publish)
	group.POST("/:id/unpublish", requireAuth, h.Unpublish)

	// The third state transition. Without it 'archived' would be a status the
	// database accepts and nothing can reach, since PATCH refuses status and the
	// two routes above write only 'published' and 'draft'.
	group.POST("/:id/archive", requireAuth, h.Archive)

	// Registered after the id routes purely for readability; gin's tree resolves
	// static and parameter segments regardless of order. These two are last because
	// they are the only public ones.
	group.GET("", h.List)
	group.GET("/:slug", h.GetBySlug)
}

// RegisterAdmin adds the authenticated read routes under the admin group.
//
// A separate group, and a separate pair of queries behind it. The public reads
// carry their visibility filter in SQL, so no argument to them can produce a
// draft; these are the statements that can, and they sit behind a path that says
// so and a guard on the whole group rather than per route.
//
// Registering the guard on the group is the point: a route added to this group
// later is authenticated by construction, where a per-route guard is one omission
// away from a public draft list.
func (h *Handler) RegisterAdmin(api *gin.RouterGroup, requireAuth gin.HandlerFunc) {
	if requireAuth == nil {
		panic("entryhttp: requireAuth is required for the admin routes")
	}

	group := api.Group("/admin/entries", requireAuth)

	group.GET("", h.ListAdmin)
	group.GET("/:id", h.GetByID)
}

// Create stores a new entry owned by the authenticated account.
//
// The author comes from the session, never from the body. A client-supplied
// author_id would let one account write entries attributed to another, and with
// a single-account site today it would be a field with nothing checking it.
func (h *Handler) Create(c *gin.Context) {
	authorID, ok := h.resolveAuthor(c)
	if !ok {
		// Unreachable while the route keeps its middleware. Reported as internal
		// rather than 401: a missing identity behind a guarded route is a wiring
		// mistake, and answering 401 would send a logged-in client to log in again.
		h.logger.ErrorContext(c.Request.Context(), "authenticated identity missing from context")
		httpx.Error(c, apperr.Internal("authentication context missing"))
		return
	}

	var req createEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("title, slug and content_md are required").WithCause(err))
		return
	}

	created, err := h.service.Create(c.Request.Context(), req.toInput(authorID))
	if err != nil {
		h.writeCreateError(c, err)
		return
	}

	h.logger.InfoContext(c.Request.Context(), "entry created",
		slog.Int64("entry_id", created.ID),
		slog.String("slug", created.Slug),
		slog.String("status", string(created.Status)),
	)

	httpx.Created(c, newOwnerEntryDetail(created))
}

// List returns one page of published public entries, newest happening first.
func (h *Handler) List(c *gin.Context) {
	page, err := intQuery(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := intQuery(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}

	// Resolved before the list runs, so an unknown category is a 404 rather than an
	// empty page. Those are different answers: one says the URL is wrong, the other
	// says the category is empty, and a reader cannot act on the first if it is
	// reported as the second.
	categoryID, err := h.resolveCategory(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	// Out-of-range values are clamped by the service rather than refused. A page
	// past the end is an empty page, because the collection shrinks when an entry
	// is unpublished and a client that asked for page 4 a moment ago did nothing
	// wrong.
	result, err := h.service.ListPublic(c.Request.Context(), categoryID, page, pageSize)
	if err != nil {
		h.logger.ErrorContext(c.Request.Context(), "list entries failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	// A non-nil empty slice, so an empty page serialises as [] rather than null.
	// Every client can iterate the first; the second needs a special case.
	summaries := make([]entrySummary, 0, len(result.Entries))
	for _, e := range result.Entries {
		summaries = append(summaries, newEntrySummary(e))
	}

	httpx.List(c, summaries, paginationMeta{
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	})
}

// GetBySlug returns one published entry that is public or unlisted, body included.
//
// GetLinkBySlug rather than GetPublicBySlug, which is what makes an unlisted entry
// reachable: a link to one opens, while the list this sits beside still excludes it.
//
// Unlisted is not access control. A slug is human readable and therefore guessable,
// so this endpoint hides an unlisted entry from lists and search engines and from
// nobody who has the URL. An entry a stranger must not read is private, and private
// is absent from both reads.
func (h *Handler) GetBySlug(c *gin.Context) {
	found, err := h.service.GetLinkBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		if errors.Is(err, entry.ErrEntryNotFound) {
			// 404 for a draft, a private entry, a deleted one and a slug that never
			// existed alike. 403 on the first three would confirm that a slug holds
			// something, which turns this endpoint into a way to enumerate
			// unpublished work.
			httpx.Error(c, apperr.NotFound("no entry matches this slug"))
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "read entry failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	httpx.OK(c, newPublicEntryDetail(found))
}

// Update applies a partial change to one entry.
//
// PATCH, not PUT. A PUT would require the client to send the whole entry back,
// which turns two editors on two tabs into a silent overwrite: whoever saves last
// replaces fields the other changed and never mentioned.
func (h *Handler) Update(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	var req updateEntryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperr.InvalidInput("this request body is not valid JSON").WithCause(err))
		return
	}

	// Refused rather than ignored. A client sending status here would otherwise get
	// 200 and find nothing published, and the endpoint that does publish would never
	// be reached.
	if req.Status != nil {
		httpx.Error(c, apperr.InvalidInput("status cannot be changed here").
			WithField("status", "use POST /entries/:id/publish or /unpublish"))
		return
	}

	updated, err := h.service.Update(c.Request.Context(), id, req.toInput())
	if err != nil {
		h.writeEntryError(c, "update entry", err)
		return
	}

	h.logger.InfoContext(c.Request.Context(), "entry updated",
		slog.Int64("entry_id", updated.ID),
		slog.String("slug", updated.Slug),
	)

	httpx.OK(c, newOwnerEntryDetail(updated))
}

// Delete soft deletes one entry.
//
// 204 on the first call and 404 on the second. Not idempotent in its status, and
// that is the honest answer: after the first call no live entry has this id, which
// is the same situation as an id that never existed. Answering 204 again would
// require either moving deleted_at, losing when the delete happened, or an extra
// read to tell the two apart, and neither buys the client anything.
func (h *Handler) Delete(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	if err := h.service.SoftDelete(c.Request.Context(), id); err != nil {
		h.writeEntryError(c, "delete entry", err)
		return
	}

	httpx.NoContent(c)
}

// Publish makes one entry public.
//
// Publishing an already published entry succeeds and changes nothing, because the
// end state is the one asked for and published_at does not move.
func (h *Handler) Publish(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	published, err := h.service.Publish(c.Request.Context(), id)
	if err != nil {
		h.writeEntryError(c, "publish entry", err)
		return
	}

	httpx.OK(c, newOwnerEntryDetail(published))
}

// Unpublish returns one entry to draft.
//
// The entry leaves the public list at once. published_at stays, so a later
// republish does not present it as new.
func (h *Handler) Unpublish(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	drafted, err := h.service.Unpublish(c.Request.Context(), id)
	if err != nil {
		h.writeEntryError(c, "unpublish entry", err)
		return
	}

	httpx.OK(c, newOwnerEntryDetail(drafted))
}

// Archive retires one entry.
//
// Not a delete and not a draft. The entry leaves the site, stays in the admin
// list, and keeps its publication date.
func (h *Handler) Archive(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	archived, err := h.service.Archive(c.Request.Context(), id)
	if err != nil {
		h.writeEntryError(c, "archive entry", err)
		return
	}

	httpx.OK(c, newOwnerEntryDetail(archived))
}

// ListAdmin returns one page of entries of any status, newest edit first.
//
// Behind the admin group's guard, and backed by a query that has no visibility
// filter. This is the only list that reports drafts.
func (h *Handler) ListAdmin(c *gin.Context) {
	page, err := intQuery(c, "page")
	if err != nil {
		httpx.Error(c, err)
		return
	}
	pageSize, err := intQuery(c, "page_size")
	if err != nil {
		httpx.Error(c, err)
		return
	}

	// An absent status means every status. A present but unknown one is refused
	// rather than matching nothing: "stauts=draft" would otherwise report an empty
	// list, which reads as "no drafts" instead of "you typed it wrong".
	var status *entry.Status
	if raw := c.Query("status"); raw != "" {
		s := entry.Status(raw)
		status = &s
	}

	result, err := h.service.ListAdmin(c.Request.Context(), status, page, pageSize)
	if err != nil {
		if errors.Is(err, entry.ErrInvalidStatus) {
			httpx.Error(c, invalidField("status",
				"must be one of draft, published, archived", err))
			return
		}

		h.logger.ErrorContext(c.Request.Context(), "list admin entries failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
		return
	}

	httpx.List(c, newAdminEntrySummaries(result.Entries), paginationMeta{
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	})
}

// GetByID returns one entry whatever its status, body included.
//
// Addressed by id, not slug: a draft is edited before its slug is settled, and an
// edit may be changing the slug itself.
func (h *Handler) GetByID(c *gin.Context) {
	id, err := entryID(c)
	if err != nil {
		httpx.Error(c, err)
		return
	}

	found, err := h.service.GetByID(c.Request.Context(), id)
	if err != nil {
		h.writeEntryError(c, "read entry by id", err)
		return
	}

	httpx.OK(c, newOwnerEntryDetail(found))
}

// resolveAuthor reads the acting account from the request context.
func (h *Handler) resolveAuthor(c *gin.Context) (int64, bool) {
	if h.author == nil {
		return 0, false
	}
	authorID, ok := h.author(c)
	if !ok || authorID == 0 {
		return 0, false
	}
	return authorID, true
}

// writeCreateError maps a create failure onto a response.
//
// A thin caller of writeEntryError. Kept as its own function only for the op
// string; a create cannot produce ErrEntryNotFound or ErrNoUpdateFields, and
// branches that cannot be reached cost nothing.
func (h *Handler) writeCreateError(c *gin.Context, err error) {
	h.writeEntryError(c, "create entry", err)
}

// writeEntryError maps a failure from any entry write or admin read onto a
// response.
//
// The mapping lives here and not in the service: which HTTP status a taken slug
// deserves is not a fact about entries. Each domain error is named explicitly, so
// a new one added to the domain falls through to 500 rather than being reported
// as a client error by a default that happened to cover it.
//
// One mapper for every operation rather than one per endpoint. The same domain
// error means the same thing to a client whichever route produced it, and a
// per-endpoint copy is where two of them start disagreeing about a status code.
// op only names the operation in the log line.
func (h *Handler) writeEntryError(c *gin.Context, op string, err error) {
	switch {
	case errors.Is(err, entry.ErrEntryNotFound):
		// 404 for a soft deleted entry as well as an id that never existed. The two
		// are the same fact from a client's side: nothing live answers to this id.
		httpx.Error(c, apperr.NotFound("no entry matches this id"))

	case errors.Is(err, entry.ErrNoUpdateFields):
		httpx.Error(c, apperr.InvalidInput("this request changes nothing").
			WithField("body", "name at least one field to change").
			WithCause(err))

	case errors.Is(err, entry.ErrSlugTaken):
		// 409, not 400. The request is well formed and would succeed with a
		// different slug, and a client can act on that: ask for another one.
		httpx.Error(c, apperr.Conflict("an entry already uses this slug").
			WithField("slug", "already taken").
			WithCause(err))

	case errors.Is(err, entry.ErrInvalidSlug):
		httpx.Error(c, invalidField("slug",
			"must be lowercase letters and digits joined by single hyphens", err))

	case errors.Is(err, entry.ErrInvalidTitle):
		httpx.Error(c, invalidField("title",
			"must be present and at most 255 characters", err))

	case errors.Is(err, entry.ErrInvalidType):
		httpx.Error(c, invalidField("type",
			"must be one of journal, book, movie, music, travel, photo", err))

	case errors.Is(err, entry.ErrInvalidStatus):
		httpx.Error(c, invalidField("status",
			"must be one of draft, published, archived", err))

	case errors.Is(err, entry.ErrInvalidVisibility):
		httpx.Error(c, invalidField("visibility",
			"must be one of public, private, unlisted", err))

	case errors.Is(err, entry.ErrInvalidMeta):
		httpx.Error(c, invalidField("meta", "must be a JSON object", err))

	case errors.Is(err, entry.ErrUnknownCategory):
		// 400 naming the field, not 404. The request named an entry that does exist;
		// what is missing is the category it asked for, and a 404 here would read as
		// "that entry is gone".
		httpx.Error(c, invalidField("category_id",
			"no category has this id", err))

	default:
		h.logger.ErrorContext(c.Request.Context(), op+" failed",
			slog.String("error", err.Error()),
		)
		httpx.Error(c, apperr.From(err))
	}
}

// resolveCategory turns the ?category= slug into an id, 0 when absent.
//
// An absent filter is not an error: the unfiltered list is the front page. A
// present-but-unknown slug is a 404, for the reason given at the call site.
//
// An empty ?category= is treated as absent rather than refused. A frontend that
// builds its query string from form state sends category= for "all categories", and
// answering 400 there would break the one case the filter exists to serve.
func (h *Handler) resolveCategory(c *gin.Context) (int64, error) {
	slug := c.Query("category")
	if slug == "" {
		return 0, nil
	}

	if h.category == nil {
		// Refused rather than ignored. Serving the unfiltered list would answer a
		// request for one category with every entry on the site, and the client would
		// have no way to tell.
		h.logger.ErrorContext(c.Request.Context(), "category filter requested but no resolver is wired")
		return 0, apperr.Internal("this filter is not available")
	}

	id, err := h.category(c, slug)
	if err != nil {
		return 0, err
	}

	return id, nil
}

// entryID reads the :id path parameter.
//
// Refused rather than defaulted to zero. An unparseable id is a client mistake,
// and a zero would reach the service and come back as a 404, which reads as "that
// entry is gone" for a request that never named one.
func entryID(c *gin.Context) (int64, error) {
	raw := c.Param("id")

	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		// 400, not 404. The path segment is not an id at all, so there is no resource
		// to be missing; saying so points a client at its own URL construction.
		return 0, apperr.InvalidInput("this entry id is not a positive whole number").
			WithField("id", "must be a positive whole number")
	}

	return id, nil
}

// invalidField builds a 400 naming the field that was refused.
//
// The field name is what makes the response useful to an editor: "invalid input"
// alone leaves a client to guess which of eleven fields to fix.
func invalidField(name, reason string, cause error) *apperr.Error {
	return apperr.InvalidInput("this entry cannot be saved as submitted").
		WithField(name, reason).
		WithCause(cause)
}

// intQuery reads an integer query parameter, treating an absent or empty one as
// zero.
//
// Zero means "not asked for", which the service reads as the default. A value
// that is present but not a number is refused instead of defaulted: it is a
// mistake in the client rather than a request for the default, and silently
// serving page 1 for page=abc would hide it.
func intQuery(c *gin.Context, name string) (int, error) {
	raw := c.Query(name)
	if raw == "" {
		return 0, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, &apperr.Error{
			Code:    apperr.CodeInvalidInput,
			Status:  http.StatusBadRequest,
			Message: "this query parameter must be a whole number",
			Fields:  map[string]string{name: "must be a whole number"},
		}
	}
	return value, nil
}
