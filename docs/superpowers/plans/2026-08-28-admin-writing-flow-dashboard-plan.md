# Admin Writing Flow and Dashboard Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Improve the admin writing flow, add searchable/category-filtered content browsing, and replace the Dashboard placeholder with real writing metrics.

**Architecture:** Keep the existing authenticated routes and entry API. Extend the admin list query with a category filter only if the current backend contract does not already provide it; keep search and category state in the content page URL. Add a small authenticated dashboard statistics response derived from the same entry repository so metrics are complete rather than page-sized estimates. After soft deletion, select the adjacent live entry from the directory ordering and route directly to its editor.

**Tech Stack:** Vue 3, TypeScript, Pinia, Vue Router, Vitest, Go, Gin, PostgreSQL, sqlc.

**Spec:** Approved in chat on 2026-08-28; requirements are recorded in the current task conversation.

## Global Constraints

- Preserve the shared iOS visual language and the 「薰衣草」 theme.
- Deleting an entry must not navigate to Dashboard or the content list.
- Delete must continue to require explicit confirmation and must not delete another entry when no adjacent entry exists.
- Search and category filters must compose and survive refresh through URL query parameters.
- Dashboard metrics must come from authenticated backend data, never hard-coded numbers.
- Use Node 22.17.0 for frontend checks.

---

### Task 1: Define the backend contracts for category filtering and dashboard metrics

**Files:**
- Modify: `backend/sql/queries/entry.sql`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/postgres/sqlcgen/entry.sql.go` and related generated files via the project sqlc command
- Test: `backend/internal/entryhttp/handler_test.go`, `backend/internal/entry/service_test.go`

**Interfaces:**
- `GET /api/v1/admin/entries?page=&page_size=&status=&q=&category=` accepts a category id or stable category slug according to the existing API convention; malformed filters return the existing validation error shape.
- `GET /api/v1/admin/dashboard` returns `{ data: { total_entries, published_entries, total_words } }` for the authenticated owner.

- [ ] Write failing handler/service tests for category-filtered admin lists and complete dashboard counts.
- [ ] Run the focused Go tests and verify they fail because the new contract is absent.
- [ ] Implement the smallest repository queries and HTTP DTOs, regenerate sqlc output with the repository command, and preserve existing pagination semantics.
- [ ] Run focused tests, then `make check`.
- [ ] Commit with `feat: add admin content filters and dashboard metrics`.

### Task 2: Add content search and category filtering to the admin page

**Files:**
- Modify: `admin/src/types/api.ts`
- Modify: `admin/src/api/entries.ts`
- Create or modify: `admin/src/api/dashboard.ts` only if shared response typing requires it
- Modify: `admin/src/views/Entries.vue`
- Test: `admin/src/views/Entries.test.ts`, `admin/src/api/entries.test.ts`

**Interfaces:**
- `Entries.vue` owns `q`, `category`, and existing `status` filters; it serializes them to the URL and reloads page 1 when a filter changes.
- Category options use the existing authenticated category list API and include an explicit “全部分类” option.

- [ ] Add failing tests for URL-restored search/category state, combined filters, clear filters, and empty results.
- [ ] Run the focused Vitest tests and verify the expected failures.
- [ ] Implement debounced search, category select, query synchronization, loading/error/empty states, and accessible labels using existing UI primitives.
- [ ] Run the focused tests and then the full admin test suite.
- [ ] Commit with `feat: add searchable admin content library`.

### Task 3: Make deletion continue directly into the adjacent writing entry

**Files:**
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/components/writing/WorkspaceHeader.vue`
- Test: `admin/src/views/EntryEditor.test.ts`, `admin/src/components/writing/ArticleDirectory.test.ts`, `admin/src/components/writing/WorkspaceHeader.test.ts`

**Interfaces:**
- The directory exposes the current live ordered entry ids to the writing shell, or the editor receives an equivalent adjacent-entry resolver without duplicating API ownership.
- `handleDelete` selects next item, then previous item, then stays on an empty writing route when the deleted entry was alone.

- [ ] Add failing tests for next-entry selection, last-entry fallback to previous, sole-entry empty state, and no-home redirect.
- [ ] Run focused tests and confirm the old `router.replace({ name: 'entries' })` behavior fails the new assertions.
- [ ] Implement deletion continuation while preserving flush, coordinator disposal, confirmation, and error behavior.
- [ ] Make the Alive mark in `WorkspaceHeader` a keyboard-accessible Dashboard link.
- [ ] Run writing-flow tests and full admin tests.
- [ ] Commit with `feat: continue writing after entry deletion`.

### Task 4: Replace Dashboard placeholder with real writing metrics

**Files:**
- Create or modify: `admin/src/api/dashboard.ts`
- Modify: `admin/src/views/Dashboard.vue`
- Test: `admin/src/views/Dashboard.test.ts`, `admin/src/api/dashboard.test.ts`

- [ ] Add failing tests for loading, successful metric rendering, API failure, and removal of the old placeholder copy.
- [ ] Run focused tests and verify they fail before implementation.
- [ ] Fetch the authenticated dashboard response, render total words, total entries, and published entries as iOS-style metric cards, and keep useful links to content/categories.
- [ ] Run focused and full admin tests.
- [ ] Commit with `feat: show dashboard writing metrics`.

### Task 5: Final verification and progress record

**Files:**
- Modify: `docs/progress.md`

- [ ] Run admin full tests and production build under Node 22.17.0.
- [ ] Run theme tests, backend `make check`, and `git diff --check`.
- [ ] Verify desktop and 375px layouts in the local browser without mutating existing content.
- [ ] Record implementation, verification counts, and any environment caveat in `docs/progress.md`.
- [ ] Commit the progress record and perform a final read-only diff review.
