# Content World Foundation and Journal Cutover Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the content-world foundation, scope categories to worlds, expose world lifecycle settings, and move the existing public Journal experience to `/journal` without changing its writing behavior.

**Architecture:** PostgreSQL stores an immutable `world` key on Entries and Categories, an optional `kind`, and one revisioned settings row per supported world. Go owns the closed world registry and lifecycle rules. Public reads become world-specific while authenticated Entry writes remain shared. Vue and Nuxt use explicit registries so later worlds add adapters without modifying Journal internals.

**Tech Stack:** Go 1.25, Gin, PostgreSQL 18/sqlc, Vue 3.5, Vue Router 5.2, Pinia 4, Nuxt 3.21.11, Vitest 4.1.

**Spec:** `docs/superpowers/specs/2026-08-29-content-worlds-design.md`

## Global Constraints

- Supported world keys are exactly `journal`, `saying`, and `video` in this iteration.
- `world` is chosen at Entry and Category creation and cannot be patched later. Migration `000010` keeps a database default only as a temporary compatibility seam for the pre-cutover sqlc writers; Task 3 removes that seam after all callers send `world` explicitly.
- `kind` is present in storage and responses but is `""` for the first three worlds.
- Category slugs are unique within one world, not globally.
- Entry slugs are unique within one world, not globally.
- Existing rows are test content. The migration sets every existing Entry and Category to `journal`; it does not preserve old type semantics or old public URLs.
- Journal keeps the current Milkdown editor, autosave, recovery, revision CAS, and publish checks.
- Public routes do not redirect old root-slug or `/categories/:slug` URLs.
- Unknown world keys return validation or not-found errors and never render as Journal.
- Do not add Tags, Sayings UI, Video media, scheduling, arbitrary world creation, or cross-world conversion in this plan.
- Run Admin verification with Node 22.17.0 or newer.

---

## File structure

- `backend/internal/contentworld`: closed registry, lifecycle settings domain, persistence, and test fake.
- `backend/internal/contentworldhttp`: public/admin world settings adapters.
- `backend/migrations/000010_content_world_foundation.*.sql`: Entry, Category, and site-world schema cutover.
- `backend/sql/queries/world.sql`: world settings reads and revision-aware updates.
- `backend/internal/entry`: immutable world/kind and world-aware public queries.
- `backend/internal/taxonomy`: world-scoped Category contracts.
- `admin/src/content-worlds/registry.ts`: editor route and copy registry.
- `admin/src/views/Worlds.vue`: lifecycle and site-label settings.
- `frontend/content-worlds/registry.ts`: public route, label, and home-preview registry.
- `frontend/pages/journal`: Journal list, Category, and detail pages.
- `frontend/components/home`: mixed-home shell and Journal preview.

### Task 1: Add the world registry and database constraints

**Files:**
- Create: `backend/migrations/000010_content_world_foundation.up.sql`
- Create: `backend/migrations/000010_content_world_foundation.down.sql`
- Create: `backend/internal/contentworld/model.go`
- Create: `backend/internal/contentworld/model_test.go`
- Modify: `backend/internal/postgres/sqlcgen/entry_integration_test.go`

**Interfaces:**
- Produces: `contentworld.Key`, `contentworld.Status`, `contentworld.ViewMode`, `contentworld.Definition`
- Produces: `contentworld.Lookup(key Key) (Definition, bool)` and `contentworld.Keys() []Key`
- Produces: immutable `entries.world`, `entries.kind`, immutable `categories.world`, and `site_worlds`

- [ ] **Step 1: Write failing registry and migration tests**

Add table-driven tests for the three supported definitions and database tests for scoped slug uniqueness, the composite Category world foreign key, and immutable worlds:

```go
func TestRegistryDefinesSupportedWorlds(t *testing.T) {
	want := []contentworld.Key{contentworld.Journal, contentworld.Saying, contentworld.Video}
	if got := contentworld.Keys(); !slices.Equal(got, want) { t.Fatalf("keys = %v", got) }
}

func TestEntryCannotUseCategoryFromAnotherWorld(t *testing.T) {
	// Insert a saying category, then attempt a journal Entry with that category id.
	// Assert SQLSTATE 23503.
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/contentworld ./internal/postgres/sqlcgen -run 'Registry|World|CategoryFromAnotherWorld' -count=1`

Expected: FAIL because the package, columns, and constraints do not exist.

- [ ] **Step 3: Implement the closed registry**

Use exact domain values:

```go
type Key string
const (
	Journal Key = "journal"
	Saying  Key = "saying"
	Video   Key = "video"
)

type Status string
const (
	Unopened Status = "unopened"
	Open     Status = "open"
	Hidden   Status = "hidden"
)

type ViewMode string
const (
	ViewNone   ViewMode = ""
	ViewStream ViewMode = "stream"
	ViewWall   ViewMode = "wall"
	ViewFocus  ViewMode = "focus"
)

type MediaCapability string
const (
	MediaNone MediaCapability = "none"
	MediaMarkdownImages MediaCapability = "markdown_images"
	MediaPrimaryVideo MediaCapability = "primary_video"
)

type Definition struct {
	Key Key
	DefaultLabel string
	SortOrder int
	AllowedViews []ViewMode
	CategoryEnabled bool
	MediaCapability MediaCapability
}
```

`Lookup` returns false for unknown keys. `Keys` returns a copy in Journal, Saying, Video order.

- [ ] **Step 4: Add migration `000010`**

The up migration performs this exact sequence:

```sql
UPDATE entries SET type = 'journal';
ALTER TABLE entries ADD COLUMN world VARCHAR(32) NOT NULL DEFAULT 'journal';
ALTER TABLE entries ADD CONSTRAINT entries_world_check
  CHECK (world IN ('journal', 'saying', 'video'));
ALTER TABLE entries ADD COLUMN kind VARCHAR(32) NOT NULL DEFAULT '';

DROP INDEX uk_entries_slug;
CREATE UNIQUE INDEX uk_entries_world_slug
  ON entries (world, slug) WHERE deleted_at IS NULL AND slug <> '';
CREATE INDEX idx_entries_world
  ON entries (world, published_at DESC)
  WHERE deleted_at IS NULL AND status = 'published';

CREATE INDEX idx_entries_world_public_timeline
  ON entries (world, COALESCE(happened_at, published_at) DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';
CREATE INDEX idx_entries_world_category_public
  ON entries (world, category_id, COALESCE(happened_at, published_at) DESC, id DESC)
  WHERE deleted_at IS NULL
    AND status = 'published'
    AND visibility = 'public';

ALTER TABLE categories ADD COLUMN world VARCHAR(32) NOT NULL DEFAULT 'journal';
ALTER TABLE categories ADD CONSTRAINT categories_world_check
  CHECK (world IN ('journal', 'saying', 'video'));
ALTER TABLE categories DROP CONSTRAINT categories_slug_key;
ALTER TABLE categories ADD CONSTRAINT categories_world_slug_key UNIQUE (world, slug);
ALTER TABLE categories ADD CONSTRAINT categories_id_world_key UNIQUE (id, world);

ALTER TABLE entries DROP CONSTRAINT entries_category_id_fkey;
ALTER TABLE entries ADD CONSTRAINT entries_category_world_fkey
  FOREIGN KEY (category_id, world) REFERENCES categories (id, world)
  ON DELETE SET NULL (category_id);
```

Add a shared `prevent_world_change()` trigger function and attach it to `entries` and `categories`. Create `site_worlds` with `world`, `status`, `nav_label`, `sort_order`, `default_view`, `revision`, and `updated_at`. Seed Journal as `open`, Saying and Video as `unopened`; use labels `日志`, `片语`, `影像`, sort orders 10/20/30, and Saying default view `stream`.

The down migration first aborts with a clear exception when duplicate live Entry slugs or duplicate Category slugs exist across worlds. Otherwise it drops the immutable triggers and world-aware indexes/foreign key, drops `world` and `kind` while retaining the legacy `type` column and indexes, restores global slug uniqueness and the original Category foreign key, then drops Category `world`, `site_worlds`, and the trigger function. It does not reconstruct discarded test type semantics.

- [ ] **Step 5: Apply the migration to the test database and rerun tests**

Run:

```bash
cd backend
make test-db-create
TEST_DATABASE_URL='postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable' \
  go test ./internal/contentworld ./internal/postgres/sqlcgen -run 'Registry|World|CategoryFromAnotherWorld' -count=1
```

Expected: PASS. `make test-db-create` creates `alive_test` when absent and applies all pending migrations; do not point this test at the development database.

- [ ] **Step 6: Commit**

```bash
git add backend/migrations/000010_* backend/internal/contentworld backend/internal/postgres/sqlcgen/entry_integration_test.go
git commit -m "feat: add the content world foundation"
```

### Task 2: Persist and expose world lifecycle settings

**Files:**
- Create: `backend/sql/queries/world.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Create: `backend/internal/contentworld/service.go`
- Create: `backend/internal/contentworld/repository.go`
- Create: `backend/internal/contentworld/contentworldtest/store.go`
- Test: `backend/internal/contentworld/service_test.go`
- Test: `backend/internal/contentworld/repository_test.go`
- Create: `backend/internal/contentworldhttp/dto.go`
- Create: `backend/internal/contentworldhttp/handler.go`
- Test: `backend/internal/contentworldhttp/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Produces: `contentworld.Setting`
- Produces: `Service.ListOpen`, `Service.ListAdmin`, `Service.Update`
- Produces: `GET /api/v1/worlds`, `GET /api/v1/admin/worlds`, `PATCH /api/v1/admin/worlds/:key`
- Produces: `Service.AllowsPublicPublish(ctx, key) (bool, error)`

- [ ] **Step 1: Write failing service and handler tests**

Cover fixed order, public omission of unopened/hidden worlds, revision conflicts, invalid status, invalid labels, unsupported view modes, hidden worlds allowing public publication, and unknown keys:

```go
updated, err := service.Update(ctx, contentworld.Saying, contentworld.UpdateInput{
	ExpectedRevision: 1,
	Status: ptr(contentworld.Open),
	NavLabel: ptr("片语"),
	DefaultView: ptr(contentworld.ViewWall),
})
if err != nil || updated.Revision != 2 { t.Fatalf("updated = %+v, err = %v", updated, err) }
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/contentworld ./internal/contentworldhttp ./internal/router -run 'World|Lifecycle|View' -count=1`

Expected: FAIL because services and routes do not exist.

- [ ] **Step 3: Add sqlc queries and repository**

Implement `ListOpenWorlds`, `ListAllWorlds`, `GetWorld`, and revision-aware `UpdateWorld`. `ListOpenWorlds` filters `status = 'open'` and sorts by `sort_order, world`. `UpdateWorld` increments revision and returns no row on stale revision.

Map no row from an update to `ErrVersionConflict`; map unknown keys on reads to `ErrWorldNotFound`.

- [ ] **Step 4: Implement lifecycle validation**

Use:

```go
type Setting struct {
	World Key
	Status Status
	NavLabel string
	SortOrder int
	DefaultView ViewMode
	Revision int64
	UpdatedAt time.Time
}

type UpdateInput struct {
	ExpectedRevision int64
	Status *Status
	NavLabel *string
	DefaultView *ViewMode
}
```

Reject empty or over-64-rune labels. `default_view` must be one of the selected world's `AllowedViews`; Journal and Video accept only `""`. `AllowsPublicPublish` returns true for `open` and `hidden`, false for `unopened`.

- [ ] **Step 5: Add exact HTTP contracts and wiring**

Public response fields: `world`, `nav_label`, `sort_order`, `default_view`. Admin additionally returns `status`, `revision`, `updated_at`.

PATCH body:

```go
type updateWorldRequest struct {
	Status *string `json:"status"`
	NavLabel *string `json:"nav_label"`
	DefaultView *string `json:"default_view"`
	Revision int64 `json:"revision" binding:"required,min=1"`
}
```

Register public and authenticated routes, add `WorldService` to router dependencies, and construct it in `cmd/server/main.go`.

- [ ] **Step 6: Run backend verification and commit**

```bash
cd backend
make sqlc
go test ./internal/contentworld ./internal/contentworldhttp ./internal/router ./cmd/server -count=1
git add backend/sql/queries/world.sql backend/internal/postgres/sqlcgen backend/internal/contentworld backend/internal/contentworldhttp backend/internal/router backend/cmd/server
git commit -m "feat: manage content world lifecycle"
```

### Task 3: Make Entries and Categories world-aware

**Files:**
- Modify: `backend/sql/queries/entry.sql`
- Modify: `backend/sql/queries/category.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/entry/model.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entry/entrytest/store.go`
- Test: `backend/internal/entry/model_test.go`
- Test: `backend/internal/entry/service_test.go`
- Test: `backend/internal/entry/repository_test.go`
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: `backend/internal/entryhttp/handler_test.go`
- Modify: `backend/internal/taxonomy/model.go`
- Modify: `backend/internal/taxonomy/service.go`
- Modify: `backend/internal/taxonomy/repository.go`
- Modify: `backend/internal/taxonomy/taxonomytest/store.go`
- Modify: `backend/internal/taxonomyhttp/dto.go`
- Modify: `backend/internal/taxonomyhttp/handler.go`
- Test: existing taxonomy and router tests
- Modify: `backend/internal/router/router.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `Entry.World contentworld.Key`, `Entry.Kind string`
- Produces: `Category.World contentworld.Key`
- Produces: `GET /api/v1/journals`, `GET /api/v1/journals/:slug`
- Produces: world filter on Admin Entries and Category endpoints
- Consumes: `contentworld.Service.AllowsPublicPublish`

- [ ] **Step 1: Write failing world behavior tests**

Cover required world on create, immutable world on PATCH, kind default, same slug in different worlds, scoped public reads, Category duplicate slugs across worlds, Category mismatch returning `fields.category_id`, unopened public publish refusal, hidden publish allowance, and unlisted publish in an unopened world.

```go
_, err := service.Create(ctx, entry.CreateInput{AuthorID: 1, World: contentworld.Saying})
if err != nil { t.Fatal(err) }

_, err = service.Publish(ctx, unopenedPublicID, 1)
if !errors.Is(err, entry.ErrWorldNotOpen) { t.Fatalf("err = %v", err) }
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/entry ./internal/entryhttp ./internal/taxonomy ./internal/taxonomyhttp ./internal/router -run 'World|Journal|Category' -count=1`

Expected: FAIL on missing fields, routes, and constraints.

- [ ] **Step 3: Replace Entry Type with immutable World**

Remove `entry.Type` and `validTypes`. Add `World contentworld.Key` and `Kind string` to Entry, CreateInput, repository params, SQL reads, and every response. Require a valid world on create. Remove world from `UpdateInput`; keep `world` in the HTTP PATCH DTO only so the handler can return `INVALID_INPUT` with `fields.world = "cannot be changed after creation"` instead of ignoring it.

Keep `kind` read-only and empty until a later world defines values.

- [ ] **Step 4: Scope repository queries and publication**

Change public repository signatures to:

```go
ListPublic(ctx context.Context, world contentworld.Key, categoryID int64, limit, offset int) ([]Entry, int64, error)
GetPublicByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error)
GetLinkByWorldSlug(ctx context.Context, world contentworld.Key, slug string) (Entry, error)
ListAdmin(ctx context.Context, world *contentworld.Key, categoryID int64, status *Status, search *string, limit, offset int) ([]Entry, int64, error)
```

Before publishing a public Entry, call `AllowsPublicPublish`. Unlisted publication skips that gate. Private content remains owner-only and does not open a world.

- [ ] **Step 5: Scope Categories**

Add `World` to Category, create input, responses, queries, slug-existence checks, lists, and counts. Category world is required on create and absent from PATCH. Change slug resolution to `ResolveSlug(ctx, world, slug)`.

Public and admin Category lists require `?world=<key>`; an unknown world is 400. Counts join only Entries with `e.world = c.world`.

- [ ] **Step 6: Register Journal public routes**

Keep authenticated writes under `/entries`. Remove public GET registration for `/entries` and `/entries/:slug`; register `/journals` and `/journals/:slug`, both pinned to `contentworld.Journal`. Admin list accepts optional `world` and intersects it with status, query, and Category.

- [ ] **Step 7: Regenerate, verify, document, and commit**

```bash
cd backend
make sqlc
go test ./internal/entry ./internal/entryhttp ./internal/taxonomy ./internal/taxonomyhttp ./internal/router -count=1
git add backend/sql/queries backend/internal/postgres/sqlcgen backend/internal/entry backend/internal/entryhttp backend/internal/taxonomy backend/internal/taxonomyhttp backend/internal/router docs/api.md
git commit -m "feat: scope entries and categories to worlds"
```

### Task 4: Add Admin world settings and world-first creation

**Files:**
- Create: `admin/src/content-worlds/registry.ts`
- Test: `admin/src/content-worlds/registry.test.ts`
- Modify: `admin/src/types/api.ts`
- Create: `admin/src/api/worlds.ts`
- Modify: `admin/src/api/index.ts`
- Modify: `admin/src/api/entries.ts`
- Modify: `admin/src/api/categories.ts`
- Create: `admin/src/views/Worlds.vue`
- Test: `admin/src/views/Worlds.test.ts`
- Create: `admin/src/views/NewEntry.vue`
- Test: `admin/src/views/NewEntry.test.ts`
- Modify: `admin/src/router/index.ts`
- Modify: `admin/src/router/routes.test.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/views/EntryEditor.test.ts`
- Create: `admin/src/composables/useWorldPublishFlow.ts`
- Test: `admin/src/composables/useWorldPublishFlow.test.ts`
- Modify: `admin/src/components/writing/PublishPanel.vue`
- Modify: `admin/src/components/writing/PublishPanel.test.ts`
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Modify: `admin/src/components/writing/ArticleDirectory.test.ts`
- Modify: `admin/src/components/CategoryForm.vue`
- Modify: `admin/src/views/Categories.vue`
- Modify: corresponding tests

**Interfaces:**
- Produces: `WorldKey`, `WorldSetting`, `AdminWorldSetting`
- Produces: `ADMIN_WORLD_REGISTRY`
- Produces: routes `/entries/new`, `/entries/new/:world`, `/worlds`
- Consumes: world-aware Entry, Category, and world settings APIs

- [ ] **Step 1: Write failing registry, route, and settings tests**

Assert unknown worlds are rejected, only registered editors can be selected, Journal creation sends `{ world: 'journal' }`, Categories send and filter by world, and stale world settings surface a reload message.

```ts
expect(resolveAdminWorld('journal')).toMatchObject({ label: '日志', createLabel: '写日志' })
expect(resolveAdminWorld('unknown')).toBeNull()
expect(entriesApi.createEntry).toHaveBeenCalledWith({ world: 'journal' })
```

- [ ] **Step 2: Define exact Admin contracts**

```ts
export type WorldKey = 'journal' | 'saying' | 'video'
export type WorldStatus = 'unopened' | 'open' | 'hidden'
export type SayingViewMode = 'stream' | 'wall' | 'focus'

export interface AdminWorldDefinition {
  key: WorldKey
  label: string
  createLabel: string
  editorRouteName: string | null
  publishPolicy: 'journal' | 'saying' | 'video'
  categoryEnabled: boolean
  mediaCapability: 'markdown-images' | 'none' | 'primary-video'
}
```

Foundation registers Journal with `editorRouteName: 'entry-new-world'`; Saying and Video remain null until their plans add editors. Unknown keys return null.

- [ ] **Step 3: Build world settings management**

`Worlds.vue` lists the three supported settings in fixed order and offers unopened/open/hidden transitions, nav-label edit, and Saying default-view selection. Use each row's revision in PATCH; on 409 reload the current row before allowing another save.

- [ ] **Step 4: Add world-first creation and preserve Journal editor**

`/entries/new` renders `NewEntry.vue`. `/entries/new/:world` validates the registry and routes Journal to the existing `EntryEditor.vue`. Change new-draft creation to `createEntry({ world: 'journal' })`. Remove Type controls from Article settings and Entry patch construction. Do not rename or restructure Milkdown, autosave, recovery, or revision code.

Add a shared world-aware publish flow. When public publication returns `WORLD_NOT_OPEN`, the panel offers exactly two explicit continuations: change visibility to unlisted and publish, or PATCH that world's current setting to `open` and retry publication. Show `发布这条内容，并开放「<label>」` for the second action. Reuse the world setting revision; on a 409 reload it and require another confirmation. Never open a world on panel open or on draft save. Journal follows the same code path but normally skips this branch because it is initially open.

- [ ] **Step 5: Scope directory and Categories**

Add `world?: WorldKey` to Admin Entry list queries. The directory filter begins with `全部 | 日志`; later plans append worlds. Category management requires a selected world for create/list and does not offer world changes while editing.

- [ ] **Step 6: Run Admin verification and commit**

```bash
cd admin
npm test -- --run src/content-worlds src/views/Worlds.test.ts src/views/NewEntry.test.ts src/views/EntryEditor.test.ts src/components/writing/ArticleDirectory.test.ts src/views/Categories.test.ts
npm run build
git add admin/src/content-worlds admin/src/types admin/src/api admin/src/views admin/src/router admin/src/components
git commit -m "feat: choose and manage content worlds"
```

### Task 5: Cut the public site over to Journal and mixed-home registries

**Files:**
- Create: `frontend/content-worlds/registry.ts`
- Test: `frontend/content-worlds/registry.test.ts`
- Modify: `frontend/types/entry.ts`
- Modify: `frontend/types/category.ts`
- Create: `frontend/types/world.ts`
- Modify: `frontend/types/index.ts`
- Create: `frontend/composables/useWorlds.ts`
- Modify: `frontend/composables/useEntriesApi.ts`
- Modify: `frontend/composables/useCategoriesApi.ts`
- Modify: `frontend/composables/useSiteCategories.ts`
- Create: `frontend/components/home/MixedHome.vue`
- Create: `frontend/components/home/JournalHomePreview.vue`
- Test: corresponding component tests
- Create: `frontend/content-worlds/sitemap.ts`
- Test: `frontend/content-worlds/sitemap.test.ts`
- Create: `frontend/server/routes/sitemap.xml.ts`
- Test: `frontend/server/routes/sitemap.xml.test.ts`
- Move: `frontend/pages/[slug].vue` to `frontend/pages/journal/[slug].vue`
- Move: `frontend/pages/categories/[slug].vue` to `frontend/pages/journal/categories/[slug].vue`
- Create: `frontend/pages/journal/index.vue`
- Modify: `frontend/pages/index.vue`
- Modify: `frontend/layouts/default.vue`
- Modify: `frontend/components/EntryRow.vue`
- Modify: `frontend/utils/entry-navigation.ts`
- Remove: `frontend/composables/useEntryType.ts`
- Test: affected frontend tests

**Interfaces:**
- Produces: `PUBLIC_WORLD_REGISTRY: Partial<Record<WorldKey, PublicWorldAdapter>>`
- Produces: `useWorlds()` and `/journal` public routes
- Produces: mixed homepage with a Journal preview slot
- Produces: lifecycle-aware `/sitemap.xml`

- [ ] **Step 1: Write failing routing and registry tests**

Assert Journal URLs, Category URLs, canonical paths, unknown-world omission, fixed open-world order, and no root-slug route:

```ts
expect(journalEntryPath('hello')).toBe('/journal/hello')
expect(journalCategoryPath('travel')).toBe('/journal/categories/travel')
expect(resolvePublicWorld('saying')).toBeNull()
```

- [ ] **Step 2: Add public world types and registry**

```ts
export interface PublicWorldAdapter {
  key: WorldKey
  rootPath: string
  entryPath: (slug: string) => string
  categoryPath: (slug: string) => string
  homePreview: Component
  listView: Component
  detailView: Component
  emptyState: Component
  timePolicy: 'timeline' | 'hidden' | 'metadata'
  seoType: 'Article' | 'SocialMediaPosting' | 'VideoObject'
}
```

Register only Journal in this plan. Journal declares its timeline time policy, Article schema, empty state, and dedicated list/detail components. `resolvePublicWorld` returns null for Saying and Video until their components exist. Responsive behavior and unavailable-state handling remain component contracts verified by each world's browser checkpoint; they are not guessed from a fallback adapter.

- [ ] **Step 3: Update API clients and types**

Replace Entry `type` with `world` and `kind`. Journal list/detail call `/journals` and `/journals/:slug`. Category calls include `world=journal`. Add `useWorlds` over public `/worlds` with an empty-list fallback so a world-settings outage does not invent navigation.

- [ ] **Step 4: Move Journal pages without compatibility routes**

Move the existing timeline and detail behavior under `/journal`. Update canonical, neighbors, Entry links, Category links, and structured data. Delete the old root dynamic page and old Category page. Remove `useEntryType`; Journal rendering is direct and no longer switches on legacy types. Direct Journal routes fetch Journal data regardless of whether Journal appears in `GET /worlds`, so temporarily hidden worlds remain reachable.

- [ ] **Step 5: Build mixed homepage and fixed navigation**

`MixedHome` iterates open world settings in API order, looks up a registered preview, and renders an explicit unavailable block when an open world has no installed adapter. It never falls back to Journal. `JournalHomePreview` fetches page 1 with `page_size=1` and links to `/journal`.

The masthead renders open world links. Footer Category navigation is scoped to the active world; the mixed homepage does not show one global Category list.

- [ ] **Step 6: Add lifecycle-aware sitemap generation**

Define a server-safe sitemap source registry separate from the Vue component registry. Register Journal list pagination, detail paths, and Category paths. `/sitemap.xml` first reads public `/worlds`, invokes only installed sources whose status is open, and always includes `/`. Omit unlisted/private Entries, unopened/hidden worlds, old root paths, and unknown adapters. Escape XML values, emit absolute URLs from configured public site URL, and return a contained 503 rather than a partial sitemap when the world settings request fails. Sayings and Videos append sources in their own plans.

- [ ] **Step 7: Run Frontend verification and browser checkpoint**

```bash
cd frontend
npx vitest run
npm run typecheck
npm run build
```

At 1280×900 and 375×812 verify `/`, `/journal`, one Journal detail, and one Journal Category. Confirm no old root-slug route resolves, no horizontal overflow appears, and Journal typography is unchanged.

- [ ] **Step 8: Commit**

```bash
git add frontend
git commit -m "feat: move public journals into their world"
```

### Task 6: Full foundation verification and documentation

**Files:**
- Modify: `docs/api.md`
- Modify: `docs/architecture.md`
- Modify: `docs/progress.md`
- Modify only defects found in foundation-owned files

**Interfaces:**
- Consumes: all preceding tasks
- Produces: a release-ready Journal-only content-world foundation

- [ ] **Step 1: Run all changed-project gates**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npx vitest run && npm run typecheck && npm run build
```

- [ ] **Step 2: Verify lifecycle and failure paths manually**

Create a Journal draft, publish it, hide Journal, confirm navigation/home removal while direct URLs remain readable, reopen it, create same-slug Categories in Journal and Saying, and confirm cross-world Category assignment is refused. Request an unknown world and confirm it never returns Journal content.

- [ ] **Step 3: Verify the existing writing contract**

Edit a Journal through autosave, reload recovery, manual save, publish checks, route switching, delete behavior, and revision conflict. These are regression gates, not optional polish.

- [ ] **Step 4: Update docs and commit**

Record exact API contracts, migration 000010, browser viewports, command results, and any non-blocking warnings.

```bash
git add docs/api.md docs/architecture.md docs/progress.md
git commit -m "docs: verify the content world foundation"
```
