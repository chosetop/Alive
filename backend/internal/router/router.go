// Package router is the one place that knows the full URL surface of this API.
//
// Handlers do not register themselves from init functions. Reading this file
// tells you every route that exists, and nothing else needs to be read to be
// sure the list is complete.
package router

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
	"github.com/p30huiwei/alive/backend/internal/auth"
	"github.com/p30huiwei/alive/backend/internal/authhttp"
	"github.com/p30huiwei/alive/backend/internal/config"
	"github.com/p30huiwei/alive/backend/internal/entry"
	"github.com/p30huiwei/alive/backend/internal/entryhttp"
	"github.com/p30huiwei/alive/backend/internal/health"
	"github.com/p30huiwei/alive/backend/internal/httpx"
	"github.com/p30huiwei/alive/backend/internal/middleware"
	"github.com/p30huiwei/alive/backend/internal/postgres"
	"github.com/p30huiwei/alive/backend/internal/taxonomy"
	"github.com/p30huiwei/alive/backend/internal/taxonomyhttp"
)

// Dependencies holds everything the router needs to build handlers.
//
// A struct rather than a long parameter list: the next stage adds an entry
// service, a storage client and an auth service, and each of those would
// otherwise change this signature and every caller of it.
type Dependencies struct {
	Config *config.Config
	Logger *slog.Logger
	Pool   *postgres.Pool

	// AuthService is required. The router builds the auth HTTP adapter over it
	// rather than receiving a built handler, so this file stays the only place
	// that knows which paths exist.
	AuthService *auth.Service

	// EntryService is required. Same arrangement as AuthService: the adapter is
	// built here, not passed in.
	EntryService *entry.Service

	// TaxonomyService is required. Required rather than optional even though the
	// category endpoints could be left off: the entry list's ?category= filter
	// resolves through it, and an optional service would mean that filter silently
	// answering with every entry on the site.
	TaxonomyService *taxonomy.Service
}

// New builds the fully wired HTTP handler.
//
// It panics on a missing dependency. This runs once during startup, before the
// listener opens, so a panic here is a process that refuses to boot rather than
// one that serves traffic and fails on the first login.
func New(deps Dependencies) *gin.Engine {
	if deps.Config == nil {
		panic("router: Config is required")
	}
	if deps.AuthService == nil {
		panic("router: AuthService is required")
	}
	if deps.EntryService == nil {
		panic("router: EntryService is required")
	}
	if deps.TaxonomyService == nil {
		panic("router: TaxonomyService is required")
	}

	if !deps.Config.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New, not gin.Default: Default installs its own logger and recovery,
	// which would duplicate ours and write plain text instead of JSON.
	engine := gin.New()

	// Order matters. RequestID first so the logger and recovery can name the
	// request. Recovery before route handling so it can catch their panics.
	engine.Use(
		middleware.RequestID(),
		middleware.Logger(deps.Logger),
		middleware.Recovery(deps.Logger),
		middleware.CORS(deps.Config.CORS),
	)

	// Trust no proxy headers by default. Without this, a client can spoof
	// X-Forwarded-For and control what ClientIP reports into the logs. Set this
	// to the real proxy once one is in front of the service.
	_ = engine.SetTrustedProxies(nil)

	// Off by default in gin, which makes a wrong method return 404 and hides a
	// common client mistake behind a misleading status.
	engine.HandleMethodNotAllowed = true
	engine.NoRoute(handleNoRoute)
	engine.NoMethod(handleNoMethod)

	// Infrastructure endpoints live at the root, outside any version prefix:
	// a probe URL should never change when the API version does.
	health.NewHandler(deps.Pool, deps.Logger).Register(engine)

	// The versioned API surface.
	api := engine.Group("/api/v1")

	// The limiter guards login only, and is built here rather than inside authhttp
	// so that one limiter instance backs the route for the process's whole life.
	// Building it per request would reset the counts on every attempt.
	var loginGuards []gin.HandlerFunc
	if limit := middleware.LoginRateLimit(deps.Config.RateLimit, deps.Logger); limit != nil {
		loginGuards = append(loginGuards, limit)
	}

	// POST /api/v1/auth/login   public, rate limited
	// POST /api/v1/auth/logout  public, idempotent
	// GET  /api/v1/me           requires a session
	//
	// Held in a variable because its RequireAuth guards routes in other packages
	// too. One handler means one cookie manager, so every route reads the session
	// cookie by the same name and settings.
	authHandler := authhttp.NewHandler(deps.AuthService, deps.Config.Session, deps.Logger)
	authHandler.Register(api, loginGuards...)

	// POST   /api/v1/entries              requires a session
	// PATCH  /api/v1/entries/:id          requires a session, partial update
	// DELETE /api/v1/entries/:id          requires a session, soft delete
	// POST   /api/v1/entries/:id/publish  requires a session
	// POST   /api/v1/entries/:id/unpublish
	// POST   /api/v1/entries/:id/archive
	// GET    /api/v1/entries              public, paginated
	// GET    /api/v1/entries/:slug        public
	//
	// GET /api/v1/admin/entries           requires a session, every status
	// GET /api/v1/admin/entries/:id       requires a session, every status
	//
	// The author is resolved through a function rather than by entryhttp importing
	// authhttp, so neither adapter depends on the other. This adapter is where the
	// session becomes an author id, and it is the only place that knows the create
	// route has an author at all.
	//
	// Two registrations, because the admin reads sit under a different prefix. They
	// are the only routes that can return a draft, and separating them means the path
	// says so and the guard is on the group rather than on each route.
	// The category filter is resolved through a function for the same reason as the
	// author: entryhttp does not import taxonomyhttp or taxonomy, so neither adapter
	// depends on the other and this file stays the only place that knows both exist.
	entryHandler := entryhttp.NewHandler(
		deps.EntryService,
		authorFromSession,
		categoryFromSlug(deps.TaxonomyService),
		deps.Logger,
	)
	entryHandler.Register(api, authHandler.RequireAuth())
	entryHandler.RegisterAdmin(api, authHandler.RequireAuth())

	// POST   /api/v1/categories       requires a session
	// PATCH  /api/v1/categories/:id   requires a session
	// DELETE /api/v1/categories/:id   requires a session, entries become uncategorised
	// GET    /api/v1/categories       public, with entry counts
	//
	// GET /api/v1/admin/categories      requires a session, with timestamps
	// GET /api/v1/admin/categories/:id  requires a session
	//
	// The public list is not paginated: categories are navigation, and navigation
	// that needs paging is not navigable.
	taxonomyHandler := taxonomyhttp.NewHandler(deps.TaxonomyService, deps.Logger)
	taxonomyHandler.Register(api, authHandler.RequireAuth())
	taxonomyHandler.RegisterAdmin(api, authHandler.RequireAuth())

	return engine
}

// categoryFromSlug builds the resolver entryhttp uses for its ?category= filter.
//
// The status mapping happens here rather than in entryhttp, because this is the only
// side that may recognise a taxonomy error: entryhttp does not import taxonomy, and
// the whole point of the resolver type is that it does not have to.
//
// An unknown slug is a 404 rather than an empty list. /entries?category=nope and
// /entries?category=travel-with-no-posts-yet are different situations, and a client
// that cannot tell them apart shows "no posts in this category" for a typo.
//
// Every other error falls through as-is and becomes a 500, which is what a failed
// query is. Flattening either case to an id of 0 would answer a request for one
// category with every entry on the site.
func categoryFromSlug(service *taxonomy.Service) entryhttp.CategoryResolver {
	return func(c *gin.Context, slug string) (int64, error) {
		id, err := service.ResolveSlug(c.Request.Context(), slug)
		if err != nil {
			// One case to map, not two. A malformed slug never arrives as
			// ErrInvalidSlug: the service answers it as not-found without a query,
			// because a slug that cannot exist names no category, and a reader
			// following a stale link has no use for a message about hyphens.
			if errors.Is(err, taxonomy.ErrCategoryNotFound) {
				return 0, apperr.NotFound("no category matches this slug")
			}
			return 0, err
		}
		return id, nil
	}
}

// authorFromSession reports the account a request is acting as.
//
// Reads what authhttp.RequireAuth put on the context. Only reached on a route
// that carries that middleware, and the false result is what keeps that honest if
// the guard is ever left off.
func authorFromSession(c *gin.Context) (int64, bool) {
	authenticated, ok := authhttp.Authenticated(c)
	if !ok {
		return 0, false
	}
	return authenticated.User.ID, true
}

// handleNoRoute answers an unknown path in the standard error envelope.
func handleNoRoute(c *gin.Context) {
	httpx.Error(c, apperr.NotFound("no route matches this path"))
}

// handleNoMethod answers a known path with an unsupported method.
func handleNoMethod(c *gin.Context) {
	httpx.Error(c, &apperr.Error{
		Code:    apperr.CodeInvalidInput,
		Status:  http.StatusMethodNotAllowed,
		Message: "this method is not allowed on this path",
	})
}
