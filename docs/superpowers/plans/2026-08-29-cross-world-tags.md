# Cross-World Tags Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add one shared tag vocabulary across all content worlds, revision-safe Entry tag editing, and a public mixed tag page that preserves each world's visual language.

**Architecture:** Tags are global taxonomy records joined to Entries through `entry_tags`; they never carry a world key. Tag replacement happens in the same PostgreSQL transaction as an Entry revision compare-and-swap so stale editors cannot overwrite newer associations. Public tag queries return a discriminated union, and Nuxt delegates every result to its registered world preview adapter.

**Tech Stack:** Go 1.25, Gin, PostgreSQL 18/sqlc, Vue 3.5, Nuxt 3.21.11, Vitest 4.1.

**Spec:** `docs/superpowers/specs/2026-08-29-content-worlds-design.md`

## Global Constraints

- Complete Foundation and Sayings first.
- Tags are global; Categories remain world-scoped. Do not add `world` to tags.
- Tag names are unique case-insensitively and slugs are globally unique.
- An Entry may have at most 20 tags. Duplicate submitted IDs are invalid rather than silently reordered.
- Entry tag writes require the current Entry revision and increment it once.
- Public tag pages include only public, published, non-deleted Entries from worlds whose status is `open`.
- Hidden or unopened worlds do not appear in tag aggregation, even when their direct URLs remain readable.
- Mixed pages must dispatch through explicit world adapters; unknown/missing adapters render a contained unavailable card and diagnostic log, never Journal.
- Tags are optional and do not block publication.
- This plan does not add tag hierarchy, aliases, colors, following, manual ordering, or tag deletion merging.

---

## File structure

- `backend/migrations/000011_create_tags.*.sql`: global tags and Entry relation.
- `backend/internal/taxonomy/tag_*.go`: tag domain, service, and repository.
- `backend/internal/taxonomyhttp/tag_*.go`: public/admin HTTP adapters.
- `admin/src/components/writing/TagPicker.vue`: shared editor picker.
- `frontend/content-worlds/registry.ts`: mixed-card adapter contract.
- `frontend/pages/tags/[slug].vue`: cross-world aggregation page.

### Task 1: Add global tag storage and domain rules

**Files:**
- Create: `backend/migrations/000011_create_tags.up.sql`
- Create: `backend/migrations/000011_create_tags.down.sql`
- Create: `backend/sql/queries/tag.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Create: `backend/internal/taxonomy/tag_model.go`
- Create: `backend/internal/taxonomy/tag_service.go`
- Create: `backend/internal/taxonomy/tag_repository.go`
- Modify: `backend/internal/taxonomy/taxonomytest/store.go`
- Test: `backend/internal/taxonomy/tag_model_test.go`
- Test: `backend/internal/taxonomy/tag_service_test.go`
- Test: `backend/internal/taxonomy/tag_repository_test.go`

**Interfaces:**
- Produces: `Tag`, `CreateTagInput`, `UpdateTagInput`, `TagStore`
- Produces: global create/list/update/delete operations
- Produces: migration `000011`

- [ ] **Step 1: Write failing validation and integration tests**

Cover trimmed names, 1..64-rune names, generated explicit slugs, case-insensitive duplicate names, global duplicate slugs, usage counts, deletion of an unused tag, and refusal to delete a used tag.

- [ ] **Step 2: Add migration `000011`**

```sql
CREATE EXTENSION IF NOT EXISTS citext;
CREATE TABLE tags (
  id BIGSERIAL PRIMARY KEY,
  name CITEXT NOT NULL UNIQUE,
  slug VARCHAR(160) NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE entry_tags (
  entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  tag_id BIGINT NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (entry_id, tag_id)
);
CREATE INDEX idx_entry_tags_tag_entry ON entry_tags (tag_id, entry_id);
```

The down migration drops `entry_tags` then `tags`; it does not drop `citext`, which is already used elsewhere.

- [ ] **Step 3: Implement domain and repository**

Use existing slug normalization rules, but reject an empty result. `ListTags(query, limit)` sorts exact prefix matches first, then name; cap limit at 50. `DeleteTag` maps the foreign-key violation to `ErrTagInUse`. Repository usage counts exclude soft-deleted Entries.

- [ ] **Step 4: Apply to the test database and verify**

```bash
cd backend
make test-db-create
make sqlc
TEST_DATABASE_URL='postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable' \
  go test ./internal/taxonomy -run 'Tag' -count=1
```

- [ ] **Step 5: Commit**

```bash
git add backend/migrations/000011_* backend/sql/queries/tag.sql backend/internal/postgres/sqlcgen backend/internal/taxonomy
git commit -m "feat: add global content tags"
```

### Task 2: Make Entry tag replacement revision-safe

**Files:**
- Modify: `backend/sql/queries/entry.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/entry/model.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entry/entrytest/store.go`
- Test: `backend/internal/entry/service_test.go`
- Test: `backend/internal/entry/repository_test.go`
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: `backend/internal/entryhttp/handler_test.go`

**Interfaces:**
- Produces: `Entry.Tags []taxonomy.Tag`
- Produces: `ReplaceTags(ctx, entryID, expectedRevision, tagIDs)`
- Produces: `PUT /api/v1/admin/entries/:id/tags`

- [ ] **Step 1: Write failing service and concurrency tests**

Cover empty replacement, 20-tag boundary, 21-tag rejection, duplicate/unknown IDs, stable name ordering, stale revision, rollback after an invalid ID, exactly one revision increment, and no content-field mutation.

- [ ] **Step 2: Define the request and response**

```go
type replaceEntryTagsRequest struct {
	TagIDs  []int64 `json:"tag_ids" binding:"max=20,dive,gt=0"`
	Revision int64  `json:"revision" binding:"required,min=1"`
}
```

Return the updated Entry representation with its new revision and tags. A stale revision returns the existing 409 conflict envelope.

- [ ] **Step 3: Implement one transaction**

Lock/update the Entry with `WHERE id=$1 AND revision=$2 AND deleted_at IS NULL`, validate all submitted tag IDs, delete old joins, insert new joins, increment revision once, and read the completed Entry. Any failure rolls back both joins and revision. Sort tags by case-folded name for response stability.

- [ ] **Step 4: Include tags in every Entry read**

Admin Entry reads and Journal/Saying public detail/list responses include `tags: []`; never return null. Avoid N+1 reads by aggregating tag JSON or issuing one page-level batch query keyed by Entry IDs.

- [ ] **Step 5: Verify and commit**

```bash
cd backend
make sqlc
go test ./internal/entry ./internal/entryhttp -run 'Tag|Revision' -count=1
git add backend/sql/queries/entry.sql backend/internal/postgres/sqlcgen backend/internal/entry backend/internal/entryhttp
git commit -m "feat: attach tags with revision safety"
```

### Task 3: Expose public and Admin Tag APIs

**Files:**
- Create: `backend/internal/taxonomyhttp/tag_dto.go`
- Create: `backend/internal/taxonomyhttp/tag_handler.go`
- Test: `backend/internal/taxonomyhttp/tag_handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `GET /api/v1/tags/:slug/entries`
- Produces: `GET|POST /api/v1/admin/tags`
- Produces: `PATCH|DELETE /api/v1/admin/tags/:id`
- Consumes: Entry world and `site_worlds` status

- [ ] **Step 1: Write failing route tests**

Assert Admin authentication, validation envelopes, global search, duplicate conflicts, used-tag delete conflict, open-world filtering, fixed world rank, pagination stability, and unknown-world adapter data preservation.

- [ ] **Step 2: Define the mixed public item**

```go
type taggedEntryItem struct {
	World contentworld.Key `json:"world"`
	Kind  string           `json:"kind"`
	Slug  string           `json:"slug"`
	Title string           `json:"title,omitempty"`
	Excerpt string         `json:"excerpt,omitempty"`
	CoverURL string        `json:"cover_url,omitempty"`
	Saying *sayingPreview  `json:"saying,omitempty"`
}
```

Return `tag`, `items`, and page metadata. Items sort by fixed world rank, then private publication time descending, then ID descending. Do not expose timestamps for Saying items.

- [ ] **Step 3: Filter lifecycle in SQL/service**

Join `site_worlds` and require `status='open'`. This is intentionally stricter than direct hidden-world reads. An unknown world key from corrupt storage returns an explicit internal error and log entry rather than being dropped or rendered as Journal.

- [ ] **Step 4: Wire routes and document contracts**

Keep Tag CRUD authenticated. Public tag detail is read-only and uses the global slug. Document conflict and empty-page behavior.

- [ ] **Step 5: Verify and commit**

```bash
cd backend
go test ./internal/taxonomyhttp ./internal/router ./cmd/server -run 'Tag' -count=1
git add backend/internal/taxonomyhttp backend/internal/router backend/cmd/server docs/api.md
git commit -m "feat: expose cross-world tag APIs"
```

### Task 4: Add the shared Admin Tag picker

**Files:**
- Modify: `admin/src/types/api.ts`
- Create: `admin/src/api/tags.ts`
- Modify: `admin/src/api/index.ts`
- Create: `admin/src/components/writing/TagPicker.vue`
- Test: `admin/src/components/writing/TagPicker.test.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/views/SayingEditor.vue`
- Test: affected editor tests

**Interfaces:**
- Produces: searchable global picker and inline tag creation
- Consumes: revision-aware Entry tag replacement

- [ ] **Step 1: Write failing interaction tests**

Cover debounced search, keyboard selection, inline creation, case-insensitive duplicate handling, removal, 20-tag limit, stale revision reload, failed save rollback, and identical behavior in Journal and Saying editors.

- [ ] **Step 2: Implement the shared control**

Render selected tags as removable chips and results as an accessible listbox. Search all tags without a world parameter. Create only after explicit confirmation. Do not save on highlight; save on select/remove through the revision endpoint and replace the editor's current revision with the response revision.

- [ ] **Step 3: Handle conflicts without losing text**

On 409, keep local text/draft state intact, reload Entry tags and revision, then ask the author to repeat only the tag change. Do not automatically replay a stale replacement.

- [ ] **Step 4: Verify and commit**

```bash
cd admin
npm test -- --run src/components/writing/TagPicker.test.ts src/views/EntryEditor.test.ts src/views/SayingEditor.test.ts
npm run build
git add admin/src
git commit -m "feat: share tags across content editors"
```

### Task 5: Build the mixed public Tag page

**Files:**
- Modify: `frontend/types/entry.ts`
- Create: `frontend/types/tag.ts`
- Modify: `frontend/types/index.ts`
- Create: `frontend/composables/useTagsApi.ts`
- Modify: `frontend/content-worlds/registry.ts`
- Test: `frontend/content-worlds/registry.test.ts`
- Create: `frontend/components/tags/TaggedEntryCard.vue`
- Create: `frontend/components/tags/UnavailableWorldCard.vue`
- Test: corresponding component tests
- Create: `frontend/pages/tags/[slug].vue`
- Modify: Journal and Saying detail/list tag rendering
- Test: affected page tests

**Interfaces:**
- Extends: `PublicWorldAdapter.renderTaggedItem`
- Produces: `/tags/<slug>`
- Consumes: mixed public Tag API

- [ ] **Step 1: Write failing dispatch tests**

Assert Journal and Saying adapters receive only their own shapes, fixed server ordering is preserved, unknown adapters show one contained error card, Saying cards show no time, and tag links are global from every world.

- [ ] **Step 2: Extend the registry contract**

Add a required `renderTaggedItem` component to each installed world adapter. The generic wrapper chooses by `item.world`; it does not inspect optional fields to guess a world. Log missing adapters once per key and render `UnavailableWorldCard`.

- [ ] **Step 3: Build the page**

Render tag name, optional usage count, server-ordered mixed cards, pagination, empty state, and per-page error state. Preserve each adapter's typography and URL helper. Do not group by Category or synthesize one global chronological timeline.

- [ ] **Step 4: Add tag links to world surfaces**

Journal and Saying list/detail components render global links to `/tags/<slug>`. Tag absence consumes no visual space.

- [ ] **Step 5: Verify and commit**

```bash
cd frontend
npx vitest run
npm run typecheck
npm run build
git add frontend
git commit -m "feat: add cross-world tag discovery"
```

At 1280×900 and 375×812 verify mixed cards, keyboard navigation, Saying time omission, unavailable adapter containment, pagination, and no horizontal overflow.

### Task 6: Full Tags acceptance

**Files:**
- Modify: `docs/api.md`
- Modify: `docs/architecture.md`
- Modify: `docs/progress.md`
- Modify only defects found in Tag-owned files

- [ ] **Step 1: Run all changed-project gates**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npx vitest run && npm run typecheck && npm run build
```

- [ ] **Step 2: Verify cross-world and lifecycle behavior**

Attach one tag to Journal and Saying Entries, confirm one mixed page, hide Sayings and confirm it leaves the tag page without changing its direct link, reopen it, and verify duplicate tag names cannot split by case.

- [ ] **Step 3: Verify concurrent editing**

Open one Entry in two Admin tabs, change tags in both, save the stale tab second, and confirm its 409 does not overwrite either newer tags or body text.

- [ ] **Step 4: Update evidence and commit**

```bash
git add docs/api.md docs/architecture.md docs/progress.md
git commit -m "docs: verify cross-world tags"
```
