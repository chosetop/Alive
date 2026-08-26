# Immersive Writing Workspace Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the form-first admin editor with a Codex-style article directory, centered writing canvas, contextual editing controls, settings drawer, preview, and publish check.

**Architecture:** The authenticated admin shell becomes a writing workspace for entry routes while category management keeps the existing utility layout. Reka UI supplies unstyled accessible primitives; Alive owns a small visual component layer. Milkdown remains the Markdown engine and gains slash, tooltip, and link components. A shared Markdown package keeps admin preview and Nuxt rendering behavior identical.

**Tech Stack:** Vue 3.5.41, Vue Router 5.2, Pinia 4, Milkdown 7.22.1, Reka UI 2.10.4, markdown-it 14.1.0, Vitest 4.1.0, Go/Gin/PostgreSQL for article search.

**Spec:** `docs/superpowers/specs/2026-08-26-admin-writing-experience-design.md`

## Global Constraints

- Plan 1 is complete and all owner mutations use `revision`.
- The article directory lists articles, not headings inside the current article.
- The directory is collapsible; the canvas recenters when collapsed.
- No Dashboard or long metadata form appears in the normal writing path.
- Metadata lives in a right-side settings drawer; publish uses a distinct right-side check panel.
- The editor stores Markdown only and does not leak ProseMirror values into business state.
- UI primitives are unstyled Reka UI wrappers; do not add a full visual UI kit.
- Desktop and iPad are full writing surfaces. At 375px, directory and side panels become full-screen drawers.
- Do not add scheduling, collaboration, media upload, or site theme persistence in this plan.

---

## File structure

- `admin/src/layouts/AdminLayout.vue`: select utility or writing shell from route metadata.
- `admin/src/layouts/WritingLayout.vue`: directory, header, side panels, and workspace outlet.
- `admin/src/components/writing/ArticleDirectory.vue`: recent/status groups and search.
- `admin/src/components/writing/WorkspaceHeader.vue`: save state and primary actions.
- `admin/src/components/writing/ArticleSettings.vue`: metadata form only.
- `admin/src/components/writing/PublishPanel.vue`: blocking checks, reminders, preview, transition.
- `admin/src/components/writing/EntryPreview.vue`: shared Markdown render and viewport switch.
- `admin/src/components/ui/*`: thin Reka-based primitives.
- `admin/src/editor/slash-menu.ts`, `selection-toolbar.ts`, `editor-commands.ts`: Milkdown-specific behavior.
- `packages/markdown`: one renderer consumed by frontend and admin.
- `backend/sql/queries/entry.sql` and entry service/HTTP files: article directory search.

### Task 1: Add server-side article directory search

**Files:**
- Modify: `backend/sql/queries/entry.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: existing entry service, repository, and handler tests
- Modify: `admin/src/types/api.ts`
- Modify: `admin/src/api/entries.ts`

**Interfaces:**
- Produces: `GET /api/v1/admin/entries?q=<text>&status=<status>`
- Produces: `EntryListQuery.q?: string`
- Produces: `ListAdmin(ctx, status, query, page, pageSize)`

- [ ] **Step 1: Write failing search tests**

Seed entries with search terms in title, slug, and summary. Assert case-insensitive matches, status intersection, whitespace-only query behaving as absent, and pagination total counting the filtered set.

```go
result, err := service.ListAdmin(ctx, nil, "Mountain", 1, 20)
if err != nil { t.Fatal(err) }
if got := titles(result.Entries); !slices.Equal(got, []string{"山中 Mountain"}) {
	t.Fatalf("titles = %v", got)
}
```

- [ ] **Step 2: Run and verify failure**

Run: `cd backend && go test ./internal/entry ./internal/entryhttp -run 'Search|ListAdmin' -count=1`

Expected: FAIL because list methods do not accept a query.

- [ ] **Step 3: Add SQL search to both admin list and count**

Add one nullable `search` argument with the same predicate in both queries:

```sql
AND (
  sqlc.narg(search)::text IS NULL
  OR e.title ILIKE '%' || sqlc.narg(search)::text || '%'
  OR e.slug ILIKE '%' || sqlc.narg(search)::text || '%'
  OR COALESCE(e.summary, '') ILIKE '%' || sqlc.narg(search)::text || '%'
)
```

Trim the HTTP `q`; map an empty result to `nil` before calling the store. Do not search Markdown bodies in this iteration.

- [ ] **Step 4: Regenerate sqlc and implement service/HTTP signatures**

Run `cd backend && make sqlc`. Change the service signature to:

```go
func (s *Service) ListAdmin(ctx context.Context, status *Status, query string, page, pageSize int) (Page, error)
```

Add `q` to admin TypeScript request types and API query serialization.

- [ ] **Step 5: Run backend tests and admin build**

Run:

```bash
cd backend && go test ./internal/entry ./internal/entryhttp -count=1
cd ../admin && npm run build
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/sql/queries/entry.sql backend/internal/postgres/sqlcgen backend/internal/entry backend/internal/entryhttp admin/src/api/entries.ts admin/src/types/api.ts
git commit -m "feat: search the admin article directory"
```

### Task 2: Create shared Markdown rendering

**Files:**
- Create: `packages/markdown/package.json`
- Create: `packages/markdown/package-lock.json`
- Create: `packages/markdown/src/index.ts`
- Create: `packages/markdown/src/index.test.ts`
- Modify: `frontend/package.json`, `frontend/package-lock.json`
- Modify: `frontend/utils/markdown.ts`
- Modify: `admin/package.json`, `admin/package-lock.json`
- Create: `admin/src/editor/render-preview.ts`

**Interfaces:**
- Produces: `@alive/markdown` exports `renderMarkdown` and `markdownToText`
- Produces: admin and frontend render the same fixtures byte-for-byte

- [ ] **Step 1: Write package tests from current frontend behavior**

Cover raw HTML escaping, heading demotion, table wrapper, external-link attributes, image attributes, and plain-text truncation.

```ts
expect(renderMarkdown('# Heading')).toContain('<h2>Heading</h2>')
expect(renderMarkdown('<script>alert(1)</script>')).not.toContain('<script>')
expect(renderMarkdown('| a | b |\n|---|---|\n|1|2|')).toContain('<div class="table-scroll">')
```

- [ ] **Step 2: Create the local package**

`packages/markdown/package.json` must export source without a build step:

```json
{
  "name": "@alive/markdown",
  "private": true,
  "type": "module",
  "exports": {".": "./src/index.ts"},
  "dependencies": {"markdown-it": "14.1.0"},
  "devDependencies": {"@types/markdown-it": "14.1.2", "vitest": "4.1.0"}
}
```

Move the current implementation from `frontend/utils/markdown.ts` into the package, preserving behavior exactly. Keep the frontend file as a compatibility re-export:

```ts
export { markdownToText, renderMarkdown } from '@alive/markdown'
```

Run `cd packages/markdown && npm install` to create the package lock before installing it into the two apps.

- [ ] **Step 3: Add file dependencies to both apps**

Run:

```bash
cd frontend && npm install '@alive/markdown@file:../packages/markdown'
cd ../admin && npm install '@alive/markdown@file:../packages/markdown' markdown-it@14.1.0
```

- [ ] **Step 4: Add the admin preview adapter**

`admin/src/editor/render-preview.ts` re-exports `renderMarkdown` and has no second Markdown configuration.

- [ ] **Step 5: Run package, frontend, and admin verification**

Run:

```bash
cd packages/markdown && npx vitest run
cd ../../frontend && npm run typecheck && npm run build
cd ../admin && npm run build
```

Expected: PASS with unchanged frontend output.

- [ ] **Step 6: Commit**

```bash
git add packages/markdown frontend/package* frontend/utils/markdown.ts admin/package* admin/src/editor/render-preview.ts
git commit -m "refactor: share markdown rendering"
```

### Task 3: Build the Alive UI primitive layer

**Files:**
- Modify: `admin/package.json`, `admin/package-lock.json`
- Create: `admin/src/components/ui/UiButton.vue`
- Create: `admin/src/components/ui/UiIconButton.vue`
- Create: `admin/src/components/ui/UiDialog.vue`
- Create: `admin/src/components/ui/UiPopover.vue`
- Create: `admin/src/components/ui/UiMenu.vue`
- Create: `admin/src/components/ui/UiToastRegion.vue`
- Create: `admin/src/components/ui/index.ts`
- Create: `admin/src/components/ui/ui.css`
- Test: `admin/src/components/ui/*.test.ts`

**Interfaces:**
- Produces: stable Alive wrappers; business components do not import `reka-ui` directly

- [ ] **Step 1: Install Reka UI**

Run: `cd admin && npm install reka-ui@2.10.4`

- [ ] **Step 2: Write failing accessibility behavior tests**

Test dialog Escape close and focus return, menu arrow-key navigation, popover outside-click close, disabled button semantics, and toast live-region text.

```ts
const trigger = wrapper.get('[data-test="trigger"]')
await trigger.trigger('click')
await wrapper.get('[role="dialog"]').trigger('keydown', { key: 'Escape' })
expect(document.activeElement).toBe(trigger.element)
```

- [ ] **Step 3: Implement thin wrappers**

Each wrapper exposes Alive props and slots, forwards required ARIA attributes, and maps Reka state attributes to CSS. Do not expose Reka implementation types across the business-component boundary.

Use one button size in each action slot. Variants are `primary`, `secondary`, `quiet`, and `danger`; typography remains identical across variants.

- [ ] **Step 4: Add shared interaction CSS**

Use only semantic tokens from `admin/src/style.css`. Add fixed focus rings, 120ms hover transitions, no decorative scale animation, and reduced-motion overrides.

- [ ] **Step 5: Run tests and build**

Run: `cd admin && npm test -- --run src/components/ui && npm run build`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/package* admin/src/components/ui
git commit -m "feat: add accessible Alive UI primitives"
```

### Task 4: Build the writing shell and article directory

**Files:**
- Modify: `admin/src/router/index.ts`
- Modify: `admin/src/layouts/AdminLayout.vue`
- Create: `admin/src/layouts/WritingLayout.vue`
- Create: `admin/src/components/writing/ArticleDirectory.vue`
- Create: `admin/src/components/writing/WorkspaceHeader.vue`
- Create: `admin/src/stores/writing.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Test: corresponding component/store tests

**Interfaces:**
- Consumes: Plan 1 save coordinator and new `q` API
- Produces: route meta `writingWorkspace: true`
- Produces: writing store state `directoryOpen`, `activeEntryId`, `searchQuery`

- [ ] **Step 1: Write failing shell and directory tests**

Cover recent list loading, 250ms search debounce, status groups, unsynced markers from recovery records, article switching after `flush`, new blank draft creation, directory collapse, and mobile drawer close after selection.

- [ ] **Step 2: Add route metadata and nested writing routes**

Extend route meta:

```ts
interface RouteMeta {
  requiresAuth?: boolean
  guestOnly?: boolean
  writingWorkspace?: boolean
}
```

Keep one top-level authenticated `AdminLayout` route for `/dashboard`, `/entries`, and `/categories`. Add a second top-level authenticated `WritingLayout` route at `/entries` with children `new` and `:id`. Vue Router resolves the exact `/entries` library record and the more specific writing children without nesting the writing shell inside the utility shell. Both top-level records use the same `requiresAuth` guard; do not duplicate authentication logic inside either layout.

- [ ] **Step 3: Implement the writing store**

The Pinia store owns directory visibility and active ID only. It must not own Milkdown content or autosave internals.

- [ ] **Step 4: Implement the directory**

Load recent items with `page_size=20`; load status groups on demand; debounce search by 250ms; use title fallback `无标题草稿`. New article calls `createEntry({})` and routes to the returned ID.

- [ ] **Step 5: Implement workspace geometry**

Desktop grid uses a 14rem directory and flexible canvas. When collapsed, the directory column becomes zero and the canvas recenters. At 48rem and below, the directory is a modal drawer. Preserve a fixed-height header status slot.

- [ ] **Step 6: Run tests and build**

Run: `cd admin && npm test -- --run src/components/writing src/layouts src/stores/writing.test.ts && npm run build`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/router admin/src/layouts admin/src/components/writing/ArticleDirectory.vue admin/src/components/writing/WorkspaceHeader.vue admin/src/stores admin/src/views/EntryEditor.vue
git commit -m "feat: add immersive writing shell"
```

### Task 5: Move metadata and publication into side panels

**Files:**
- Create: `admin/src/components/writing/ArticleSettings.vue`
- Create: `admin/src/components/writing/PublishPanel.vue`
- Create: `admin/src/components/writing/EntryPreview.vue`
- Create: `admin/src/editor/publish-checks.ts`
- Test: corresponding tests
- Modify: `admin/src/views/EntryEditor.vue`

**Interfaces:**
- Produces: `getPublishChecks(entry): { blockers: PublishCheck[]; reminders: PublishCheck[] }`
- Consumes: revision-aware transitions from Plan 1

- [ ] **Step 1: Write failing pure publish-check tests**

```ts
expect(getPublishChecks(emptyDraft).blockers.map((x) => x.field)).toEqual([
  'title', 'slug', 'content_md',
])
expect(getPublishChecks({...complete, summary: '', cover_url: ''}).reminders.map((x) => x.field))
  .toEqual(['summary', 'category_id', 'cover_url', 'happened_at'])
```

- [ ] **Step 2: Write failing component tests**

Assert settings fields autosave independently, focus returns after drawer close, publish is disabled with blockers, reminders can be acknowledged, final copy reflects visibility, preview switches desktop/mobile width, and successful publish preserves editor selection.

- [ ] **Step 3: Implement ArticleSettings**

Render slug, type, category, summary, cover URL placeholder, happened time, visibility, archive, and delete. Each field emits a partial update to the save coordinator. Keep delete behind explicit confirmation.

- [ ] **Step 4: Implement EntryPreview**

Render local unsaved title and Markdown through `@alive/markdown`. Apply `.prose` styles shared by copying structural rules into an admin preview stylesheet now; Plan 3 moves semantic colors and type into `@alive/theme`.

- [ ] **Step 5: Implement PublishPanel**

Before opening, await `flush`. Show blockers and reminders. On confirm, call `publishEntry(id, revision)`, update the coordinator revision, keep the route and editor mounted, and announce success in the toast region.

- [ ] **Step 6: Run tests and build**

Run: `cd admin && npm test -- --run src/components/writing src/editor/publish-checks.test.ts && npm run build`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/components/writing admin/src/editor/publish-checks* admin/src/views/EntryEditor.vue
git commit -m "feat: add article settings and publish check"
```

### Task 6: Add contextual Milkdown controls

**Files:**
- Create: `admin/src/editor/editor-commands.ts`
- Create: `admin/src/editor/slash-menu.ts`
- Create: `admin/src/editor/selection-toolbar.ts`
- Modify: `admin/src/components/MarkdownEditor.vue`
- Test: `admin/src/editor/*.test.ts`, `admin/src/components/MarkdownEditor.test.ts`

**Interfaces:**
- Produces: `EditorCommand` registry shared by slash and selection UI
- Produces: `MarkdownEditor` emits only `update(markdown)` and `ready(controller)`

- [ ] **Step 1: Define and test the command registry**

```ts
export interface EditorCommand {
  id: 'heading-2' | 'heading-3' | 'bullet-list' | 'ordered-list' | 'quote' | 'code' | 'divider'
  label: string
  keywords: string[]
  run(ctx: Ctx): boolean
}
```

Test filtering by Chinese label and English keyword, command execution, and removal of the typed slash text.

- [ ] **Step 2: Add slash-plugin behavior**

Use `slashFactory` and `SlashProvider` from `@milkdown/kit/plugin/slash`. Show only in a paragraph when text before the caret begins with `/`; hide inside code blocks. Render the menu with Vue and the Alive `UiMenu` wrapper, with ArrowUp/ArrowDown, Enter, and Esc behavior.

- [ ] **Step 3: Add selection tooltip behavior**

Use the Milkdown tooltip factory for bold, emphasis, strike, inline code, and link. Do not render it for an empty selection. Use the same fixed 14px typography for every action.

- [ ] **Step 4: Add link component and shortcut**

Use Milkdown's link tooltip component. Bind `Cmd/Ctrl + K`; if text is selected, enter add-link mode, otherwise edit the link under the caret. Escape closes and restores the editor selection.

- [ ] **Step 5: Test the mounted editor**

Test slash opening, keyboard selection, tooltip visibility, link shortcut, undo history, Markdown output, and destruction on route change. Do not snapshot the entire ProseMirror DOM.

- [ ] **Step 6: Run tests and build**

Run: `cd admin && npm test -- --run src/editor src/components/MarkdownEditor.test.ts && npm run build`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/editor admin/src/components/MarkdownEditor.vue admin/src/components/MarkdownEditor.test.ts
git commit -m "feat: add contextual writing controls"
```

### Task 7: Responsive, accessibility, and browser checkpoint

**Files:**
- Modify: admin writing components and styles only as evidence requires
- Modify: `docs/progress.md`

- [ ] **Step 1: Run automated checks**

Run:

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npm run typecheck && npm run build
```

- [ ] **Step 2: Verify keyboard-only use**

Using a real browser, create and switch articles, search, open settings, edit a link, open the slash menu, preview, publish, close every overlay with Esc, and verify focus returns to the trigger.

- [ ] **Step 3: Verify responsive surfaces**

Capture desktop, iPad landscape, iPad portrait, and 375px views. Check long Chinese titles, English words, empty title fallback, large code blocks, tables, and the full visibility labels.

- [ ] **Step 4: Verify the calm-writing constraints**

Confirm no persistent formatting toolbar, no form fields in the main flow, no layout jitter between save states, no adjacent destructive and primary actions, and no horizontal overflow at 375px.

- [ ] **Step 5: Update evidence and commit**

```bash
git add admin docs/progress.md
git commit -m "fix: polish the immersive writing workspace"
```
