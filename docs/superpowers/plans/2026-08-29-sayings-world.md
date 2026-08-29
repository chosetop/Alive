# Sayings World Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add titleless Sayings authoring, stable short permanent links, three time-neutral browsing modes, and a fixed-position homepage preview.

**Architecture:** Sayings remain Entries with `world=saying`. Go owns Saying-specific meta validation, publication rules, short-ID allocation, and world-scoped public queries. Vue adds a focused editor through the Admin registry. Nuxt renders one result set through stream, wall, or focus adapters; the site default comes from `site_worlds`, while a versioned visitor preference remains local.

**Tech Stack:** Go 1.25, Gin, PostgreSQL 18/sqlc, Vue 3.5, Nuxt 3.21.11, Vitest 4.1, native Web Storage and History APIs.

**Spec:** `docs/superpowers/specs/2026-08-29-content-worlds-design.md`

## Global Constraints

- Complete `2026-08-29-content-world-foundation.md` first.
- Do not add a title input, cover, visible timestamp, reading time, recommendation, or Journal chrome to public Sayings.
- A Saying may contain multiple lines and limited inline Markdown, but no headings, tables, fenced code, lists, blockquotes, images, or raw HTML.
- Warn above 300 CJK characters without rejecting the draft only for length.
- Permanent IDs are random, stable, URL-safe, and independent of database IDs and dates.
- All three views consume the same API records and permanent links.
- View preference is not in the URL. Persist only an explicit visitor choice; invalid values fall back to the current site default.
- Homepage preview has its own fixed layout and ignores visitor view preference.
- Categories are world-scoped. Tags belong to the later Tags plan.

---

## File structure

- `backend/internal/entry/saying.go`: meta, body policy, publish validation, short-ID generation.
- `backend/sql/queries/entry.sql`: Saying list/detail/neighbor queries.
- `backend/internal/entryhttp`: `/sayings` adapters.
- `admin/src/views/SayingEditor.vue`: minimal authoring surface.
- `frontend/composables/useSayingView.ts`: preference and return-anchor rules.
- `frontend/components/sayings`: stream, wall, focus, actions, and detail UI.
- `frontend/pages/sayings`: list, Category, and permanent-link pages.

### Task 1: Add the Saying domain and stable short IDs

**Files:**
- Create: `backend/internal/entry/saying.go`
- Test: `backend/internal/entry/saying_test.go`
- Modify: `backend/internal/entry/model.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entry/entrytest/store.go`
- Test: `backend/internal/entry/model_test.go`
- Test: `backend/internal/entry/service_test.go`

**Interfaces:**
- Produces: `SayingMeta`, `ValidateSayingDraft`, `ValidateSayingForPublish`
- Produces: `ShortIDGenerator` and `GenerateSayingShortID`
- Consumes: `Entry.World`, `Entry.Meta`, `Entry.Slug`

- [ ] **Step 1: Write failing domain tests**

Cover empty publish body, optional source/author, unknown meta keys, forbidden block syntax, multiline inline content, 300-character warning boundary, exact 10-character IDs, collision retry, and retry exhaustion.

```go
func TestSayingShortIDUsesURLSafeAlphabet(t *testing.T) {
	id, err := entry.GenerateSayingShortID(bytes.NewReader(bytes.Repeat([]byte{1}, 16)))
	if err != nil || len(id) != 10 { t.Fatalf("id = %q, err = %v", id, err) }
	if !regexp.MustCompile(`^[23456789abcdefghjkmnpqrstuvwxyz]{10}$`).MatchString(id) { t.Fatal(id) }
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `cd backend && go test ./internal/entry -run 'Saying|ShortID' -count=1`

Expected: FAIL because the Saying policy and generator do not exist.

- [ ] **Step 3: Define the exact meta and body policy**

```go
type SayingMeta struct {
	Source string `json:"source,omitempty"`
	Author string `json:"author,omitempty"`
}
type SayingDraftAssessment struct { LongFormWarning bool }
```

Decode with `json.Decoder.DisallowUnknownFields`. Trim source and author and cap each at 200 runes. Accept paragraphs plus emphasis, strong, strike, code span, and links. Reject Markdown AST nodes for headings, lists, blockquotes, fenced/indented code, tables, images, thematic breaks, and raw HTML. Publish requires non-empty rendered text.

- [ ] **Step 4: Generate and allocate short IDs**

Use `crypto/rand`, alphabet `23456789abcdefghjkmnpqrstuvwxyz`, length 10, and rejection sampling. Add `ShortIDGenerator func() (string, error)` as a Service option. On Saying creation with an empty slug, generate and call the world-scoped slug-existence check; retry at most five collisions, then return `ErrShortIDExhausted`. Ordinary PATCH cannot change a Saying slug.

- [ ] **Step 5: Dispatch validation by world**

Keep Journal rules unchanged. Saying drafts allow blank title and generated slug; `ValidateForPublish` dispatches to Saying rules. Unknown worlds return `ErrUnknownWorld`, never Journal validation.

- [ ] **Step 6: Verify and commit**

```bash
cd backend
go test ./internal/entry -run 'Saying|ShortID|Publish' -count=1
git add backend/internal/entry
git commit -m "feat: define saying content rules"
```

### Task 2: Expose Saying public APIs

**Files:**
- Modify: `backend/sql/queries/entry.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entry/service.go`
- Test: `backend/internal/entry/repository_test.go`
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: `backend/internal/entryhttp/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `GET /api/v1/sayings`
- Produces: `GET /api/v1/sayings/:shortID`
- Produces: `SayingListItem`, `SayingDetail`, and adjacent links

- [ ] **Step 1: Write failing repository and handler tests**

Test public-only membership, unlisted detail access, Category filter, stable newest-first ordering, invalid IDs, omitted time/title/cover fields, previous/next links, and isolation from other worlds.

- [ ] **Step 2: Define exact public responses**

```go
type sayingListItem struct {
	ShortID string          `json:"short_id"`
	Content string          `json:"content_md"`
	Source  string          `json:"source,omitempty"`
	Author  string          `json:"author,omitempty"`
	Category *categoryBrief `json:"category,omitempty"`
}
type sayingDetail struct {
	sayingListItem
	Previous *sayingLink `json:"previous,omitempty"`
	Next     *sayingLink `json:"next,omitempty"`
}
```

Do not serialize title, summary, cover, word count, happened/published/created/updated time. Tags are appended later without changing these omissions.

- [ ] **Step 3: Implement world-pinned queries**

List only public, published, non-deleted Saying rows. Internally sort by `COALESCE(published_at, created_at) DESC, id DESC`, but never expose those values. Detail accepts public or unlisted rows. Neighbor queries use the same private ordering keys.

- [ ] **Step 4: Register exact routes**

Register `/sayings` and `/sayings/:shortID`. Validate the path against the 10-character alphabet before querying. Category filtering uses `category=<slug>` and resolves only within `world=saying`.

- [ ] **Step 5: Regenerate, verify, document, and commit**

```bash
cd backend
make sqlc
go test ./internal/entry ./internal/entryhttp ./internal/router -run 'Saying' -count=1
git add backend/sql/queries/entry.sql backend/internal/postgres/sqlcgen backend/internal/entry backend/internal/entryhttp backend/internal/router docs/api.md
git commit -m "feat: expose public sayings"
```

### Task 3: Add the focused Admin Saying editor

**Files:**
- Modify: `admin/src/content-worlds/registry.ts`
- Test: `admin/src/content-worlds/registry.test.ts`
- Modify: `admin/src/types/api.ts`
- Modify: `admin/src/api/entries.ts`
- Create: `admin/src/views/SayingEditor.vue`
- Test: `admin/src/views/SayingEditor.test.ts`
- Modify: `admin/src/router/index.ts`
- Modify: `admin/src/views/NewEntry.vue`
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Test: affected route, creation, and directory tests

**Interfaces:**
- Produces: Admin route `/entries/new/saying`
- Produces: Saying create/update payloads and publish checks
- Consumes: existing autosave, recovery, and revision-conflict primitives

- [ ] **Step 1: Write failing authoring tests**

Assert creation sends `{ world: 'saying' }`, focus lands in the body, title/cover controls are absent, source/author/category persist, the long-form warning is non-blocking, forbidden blocks show a field error, publish does not require title, and route changes flush pending saves.

- [ ] **Step 2: Register Saying explicitly**

Set `editorRouteName: 'saying-editor-new'`, `createLabel: '记片语'`, and directory label `片语`. Unknown worlds remain rejected. Add a dedicated route; do not branch the Journal template on world.

- [ ] **Step 3: Build the editor surface**

Use one autosizing body field with inline-format actions only. Put source, original author, Saying Category, visibility, and publish actions in the existing settings layer. Display “这段话已经接近一篇日志” above 300 CJK characters. Continue using revision CAS, offline recovery, deletion, and publication actions through shared composables.

- [ ] **Step 4: Scope directory behavior**

Append `片语` after `日志` in the fixed filter. Saying rows use the first non-empty body line as excerpt and never synthesize a time-based title. Category requests use only `world=saying`.

- [ ] **Step 5: Verify and commit**

```bash
cd admin
npm test -- --run src/content-worlds src/views/SayingEditor.test.ts src/views/NewEntry.test.ts src/components/writing/ArticleDirectory.test.ts
npm run build
git add admin/src
git commit -m "feat: add focused saying authoring"
```

### Task 4: Add view preference and return-position state

**Files:**
- Create: `frontend/types/saying.ts`
- Modify: `frontend/types/index.ts`
- Create: `frontend/composables/useSayingsApi.ts`
- Create: `frontend/composables/useSayingView.ts`
- Test: `frontend/composables/useSayingView.test.ts`
- Create: `frontend/components/sayings/SayingViewPicker.vue`
- Test: `frontend/components/sayings/SayingViewPicker.test.ts`

**Interfaces:**
- Produces: `SayingView = 'stream' | 'wall' | 'focus'`
- Produces: `resolveSayingView`, `chooseSayingView`, `rememberSayingAnchor`, `consumeSayingAnchor`

- [ ] **Step 1: Write failing preference tests**

Cover site default, explicit local override, invalid/version-old storage, storage exceptions, explicit mode changes, and one-time restoration of a stored short ID.

- [ ] **Step 2: Implement exact precedence**

Use key `alive:sayings-view:v1`. Resolution is valid explicit local choice, then valid `site_worlds.default_view`, then `stream`. Do not write site default into storage.

- [ ] **Step 3: Preserve the current Saying**

Use `history.state.aliveSayingAnchor = { shortId, offsetTop }` before opening detail or changing mode. On return, wait for the target item, focus it with `preventScroll`, restore its recorded offset, then remove the anchor. If unavailable, focus the list heading.

- [ ] **Step 4: Build an accessible picker**

Use a radiogroup or menu with visible selected state. After mode change, focus the same Saying in the new layout. Honor reduced motion and avoid forced smooth scrolling.

- [ ] **Step 5: Verify and commit**

```bash
cd frontend
npx vitest run composables/useSayingView.test.ts components/sayings/SayingViewPicker.test.ts
git add frontend/types frontend/composables frontend/components/sayings
git commit -m "feat: preserve saying view preferences"
```

### Task 5: Build public list, detail, Category, and home preview

**Files:**
- Modify: `frontend/content-worlds/registry.ts`
- Test: `frontend/content-worlds/registry.test.ts`
- Create: `frontend/components/sayings/SayingItem.vue`
- Create: `frontend/components/sayings/SayingActions.vue`
- Create: `frontend/components/sayings/SayingStream.vue`
- Create: `frontend/components/sayings/SayingWall.vue`
- Create: `frontend/components/sayings/SayingFocus.vue`
- Create: `frontend/components/sayings/SayingDetail.vue`
- Test: corresponding component tests
- Create: `frontend/components/home/SayingsHomePreview.vue`
- Test: `frontend/components/home/SayingsHomePreview.test.ts`
- Create: `frontend/pages/sayings/index.vue`
- Create: `frontend/pages/sayings/[shortId].vue`
- Create: `frontend/pages/sayings/categories/[slug].vue`
- Modify: `frontend/content-worlds/sitemap.ts`
- Test: `frontend/content-worlds/sitemap.test.ts`
- Modify: `frontend/assets/css/main.css`

**Interfaces:**
- Produces: `/sayings`, `/sayings/<short-id>`, `/sayings/categories/<slug>`
- Produces: fixed homepage preview position after Journal
- Consumes: public Saying API and `useSayingView`

- [ ] **Step 1: Write failing rendering and navigation tests**

Assert selectable body text, no timestamp labels or datetime attributes, mobile wall fallback, keyboard and button focus navigation, accessible actions menu, copy-link behavior, minimal detail omissions, Category paths, and a one-to-three-item home preview unaffected by stored mode.

- [ ] **Step 2: Build shared item semantics**

`SayingItem` renders sanitized limited Markdown and a focusable wrapper with `data-saying-id`. The body is not a link. `SayingActions` has an accessible `···` button with Open, Copy link, and Share; use Web Share when available and copy as fallback.

- [ ] **Step 3: Implement the three adapters**

Stream is one readable column. Wall is two tracks on desktop and one on narrow screens. Focus renders exactly one item with focusable previous/next buttons plus Left/Right keys; ignore shortcuts while an input, textarea, select, or contenteditable owns focus.

- [ ] **Step 4: Build the minimal permanent page**

Render body, optional source/author, Category, actions, and previous/next links. Do not mount Journal metadata or recommendation components. Use canonical `/sayings/<short-id>` and JSON-LD without a fabricated headline.

- [ ] **Step 5: Register public world and homepage preview**

Register Saying only after routes/components exist. Preview requests three public items, renders one to three in a fixed compact stack, and has one link to `/sayings`. It does not read local storage.

Append a Saying sitemap source that emits `/sayings`, open Saying Category paths, and public permanent links. It must not emit unlisted links or add date-derived path segments/visible metadata.

- [ ] **Step 6: Verify responsive behavior and commit**

```bash
cd frontend
npx vitest run
npm run typecheck
npm run build
git add frontend
git commit -m "feat: add time-neutral saying browsing"
```

At 1280×900 and 375×812 verify all modes, detail return, Category, keyboard/touch controls, text selection, no visible time, no overflow, and reduced motion.

### Task 6: Full Sayings acceptance

**Files:**
- Modify: `docs/api.md`
- Modify: `docs/architecture.md`
- Modify: `docs/progress.md`
- Modify only defects found in Saying-owned files

- [ ] **Step 1: Run all changed-project gates**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npx vitest run && npm run typecheck && npm run build
```

- [ ] **Step 2: Exercise lifecycle boundaries**

Create a Saying while unopened, publish it as unlisted and open its permanent link, confirm list absence, then explicitly open the world and publish publicly. Hide it and confirm navigation/home removal while direct URLs remain readable.

- [ ] **Step 3: Exercise preference and history behavior**

Change the site default without a local choice, set a visitor choice, change the site default again, invalidate storage, refresh every mode, and return from detail. Confirm site and visitor values never overwrite each other.

- [ ] **Step 4: Update evidence and commit**

```bash
git add docs/api.md docs/architecture.md docs/progress.md
git commit -m "docs: verify the sayings world"
```
