# Draft Autosave Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow incomplete server-side drafts and protect every edit with revision-aware autosave plus IndexedDB recovery.

**Architecture:** PostgreSQL remains the source of truth and gains an integer `revision` used for optimistic concurrency. Drafts may contain empty title, slug, or body; publication validates the complete projected entry. The admin writes every edit to IndexedDB before a debounced API save, and a framework-neutral coordinator serializes requests and exposes an explicit state machine.

**Tech Stack:** Go 1.25.4, PostgreSQL 18, sqlc, Gin, Vue 3.5, TypeScript 6, Vitest 4.1.0, Vue Test Utils 2.4.6, jsdom 30.0.1, native IndexedDB.

**Spec:** `docs/superpowers/specs/2026-08-26-admin-writing-experience-design.md`

## Global Constraints

- Draft title, slug, and Markdown body may be empty; published entries may not be incomplete.
- `content_md` remains the only stored body representation.
- Empty draft slugs do not participate in uniqueness; a non-empty live slug remains unique.
- Owner responses include `revision`; public response shapes do not.
- A stale write returns HTTP 409 and never overwrites a newer revision.
- Local recovery is written before an API save begins.
- Existing `admin/src/App.vue` contains a user-owned MilkdownProvider change; preserve it.
- Do not add a manual save button. `Cmd/Ctrl + S` calls the same flush path as preview, publish, and article switching.

---

## File structure

**Backend**

- `backend/migrations/000006_entry_drafts_revision.up.sql`: relax draft fields, add revision and published completeness constraints.
- `backend/migrations/000006_entry_drafts_revision.down.sql`: refuse rollback when incomplete drafts exist, then restore the previous schema.
- `backend/sql/queries/entry.sql`: return and compare revision on owner writes.
- `backend/internal/entry/model.go`: entry revision and publish validation.
- `backend/internal/entry/service.go`: draft creation, projected-field validation, revision-aware mutations.
- `backend/internal/entry/repository.go`: translate stale writes and named constraints.
- `backend/internal/entryhttp/dto.go`: revision request and response fields.
- Existing entry tests: domain, service, repository, HTTP, generated integration tests.

**Admin**

- `admin/vitest.config.ts`: Vue unit-test environment.
- `admin/src/test/setup.ts`: IndexedDB and DOM cleanup.
- `admin/src/editor/recovery-store.ts`: IndexedDB persistence only.
- `admin/src/editor/save-coordinator.ts`: framework-neutral save state machine.
- `admin/src/editor/useEntryAutosave.ts`: Vue lifecycle adapter.
- `admin/src/editor/*.test.ts`: deterministic recovery and save tests.
- `admin/src/types/api.ts`, `admin/src/api/entries.ts`, `admin/src/api/patch.ts`: revision-aware API contract.
- `admin/src/views/EntryEditor.vue`: temporary integration into the current editor until Plan 2 replaces the shell.

### Task 1: Add incomplete draft and revision schema

**Files:**
- Create: `backend/migrations/000006_entry_drafts_revision.up.sql`
- Create: `backend/migrations/000006_entry_drafts_revision.down.sql`
- Modify: `backend/sql/queries/entry.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Test: `backend/internal/postgres/sqlcgen/entry_integration_test.go`

**Interfaces:**
- Produces: `entries.revision BIGINT NOT NULL DEFAULT 1`
- Produces: every owner write returns `revision`
- Produces: `UpdateEntry` accepts `expected_revision`

- [ ] **Step 1: Write a failing integration test for multiple empty drafts and stale writes**

Add a test that inserts two draft rows with `title=''`, `slug=''`, and `content_md=''`, then updates the first row twice with the same expected revision. Assert the first update returns revision 2 and the second returns `pgx.ErrNoRows`.

```go
first := createEntry(t, q, sqlcgen.CreateEntryParams{AuthorID: userID, Status: "draft"})
_ = createEntry(t, q, sqlcgen.CreateEntryParams{AuthorID: userID, Status: "draft"})

updated, err := q.UpdateEntry(ctx, updateParams(first.ID, first.Revision, "first title"))
if err != nil || updated.Revision != 2 {
	t.Fatalf("first update = (%+v, %v), want revision 2", updated, err)
}
_, err = q.UpdateEntry(ctx, updateParams(first.ID, first.Revision, "stale title"))
if !errors.Is(err, pgx.ErrNoRows) {
	t.Fatalf("stale update error = %v, want pgx.ErrNoRows", err)
}
```

- [ ] **Step 2: Run the integration test and verify it fails**

Run: `cd backend && TEST_DATABASE_URL=postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable go test ./internal/postgres/sqlcgen -run TestEntryIncompleteDraftAndRevision -count=1`

Expected: FAIL because `Revision` and `ExpectedRevision` do not exist and the old constraints reject an empty slug.

- [ ] **Step 3: Add migration 000006**

The up migration must implement these exact rules:

```sql
ALTER TABLE entries ALTER COLUMN title SET DEFAULT '';
ALTER TABLE entries ALTER COLUMN slug SET DEFAULT '';
ALTER TABLE entries ADD COLUMN revision BIGINT NOT NULL DEFAULT 1;

DROP INDEX uk_entries_slug;
ALTER TABLE entries DROP CONSTRAINT entries_slug_format_check;
ALTER TABLE entries ADD CONSTRAINT entries_slug_format_check
  CHECK (slug = '' OR slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$');
ALTER TABLE entries ADD CONSTRAINT entries_published_title_check
  CHECK (status <> 'published' OR title <> '');
ALTER TABLE entries ADD CONSTRAINT entries_published_slug_check
  CHECK (status <> 'published' OR slug <> '');
ALTER TABLE entries ADD CONSTRAINT entries_published_content_check
  CHECK (status <> 'published' OR btrim(content_md) <> '');
CREATE UNIQUE INDEX uk_entries_slug
  ON entries (slug)
  WHERE deleted_at IS NULL AND slug <> '';
```

The down migration must start with a `DO` block that raises an exception if any live row has an empty title or slug. It then drops the new checks and revision, restores the original slug check, removes the defaults, and recreates the old unique index.

- [ ] **Step 4: Make every owner query return and compare revision**

Add `revision` to `CreateEntry`, `UpdateEntry`, `PublishEntry`, `UnpublishEntry`, `ArchiveEntry`, and admin detail selections. The update and three transitions must include:

```sql
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
```

Every mutation sets `revision = revision + 1`. Public list and public detail queries must not expose revision.

- [ ] **Step 5: Regenerate sqlc and apply the test migration**

Run:

```bash
cd backend
make sqlc
migrate -path migrations -database 'postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable' up
```

Expected: generated params include `ExpectedRevision int64`; owner rows include `Revision int64`.

- [ ] **Step 6: Run the integration test**

Run: `cd backend && TEST_DATABASE_URL=postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable go test ./internal/postgres/sqlcgen -run TestEntryIncompleteDraftAndRevision -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/migrations/000006_* backend/sql/queries/entry.sql backend/internal/postgres/sqlcgen
git commit -m "feat: allow revisioned incomplete drafts"
```

### Task 2: Enforce draft and publication rules in the entry domain

**Files:**
- Modify: `backend/internal/entry/model.go`
- Modify: `backend/internal/entry/service.go`
- Modify: `backend/internal/entry/repository.go`
- Modify: `backend/internal/entry/entrytest/store.go`
- Test: `backend/internal/entry/model_test.go`
- Test: `backend/internal/entry/service_test.go`
- Test: `backend/internal/entry/repository_test.go`

**Interfaces:**
- Produces: `ErrVersionConflict`
- Produces: `ValidateForPublish(Entry) error`
- Produces: `UpdateInput.ExpectedRevision int64`
- Produces: `Publish(ctx, id, expectedRevision)`, `Unpublish(...)`, `Archive(...)`

- [ ] **Step 1: Write failing domain tests**

Cover these exact cases:

```go
func TestValidateForPublish(t *testing.T) {
	valid := entry.Entry{Title: "Title", Slug: "title", ContentMD: "body", Visibility: entry.VisibilityPublic}
	if err := entry.ValidateForPublish(valid); err != nil { t.Fatal(err) }

	cases := []struct{name string; mutate func(*entry.Entry); want error}{
		{"title", func(e *entry.Entry){ e.Title = "" }, entry.ErrInvalidTitle},
		{"slug", func(e *entry.Entry){ e.Slug = "" }, entry.ErrInvalidSlug},
		{"body", func(e *entry.Entry){ e.ContentMD = "  " }, entry.ErrEmptyContent},
	}
	// Run each case and assert errors.Is.
}
```

Add service tests proving `Create` accepts an all-empty draft, `Update` accepts empty title and slug for a draft, `Publish` rejects incomplete content before calling the store, and a stale store result becomes `ErrVersionConflict`.

- [ ] **Step 2: Run the focused tests and verify they fail**

Run: `cd backend && go test ./internal/entry -run 'TestValidateForPublish|TestCreateIncompleteDraft|TestPublishIncomplete|TestUpdateVersionConflict' -count=1`

Expected: FAIL with missing errors, fields, and signatures.

- [ ] **Step 3: Add revision and validation types**

Add to `Entry`:

```go
Revision int64
```

Add sentinels:

```go
ErrEmptyContent    = errors.New("entry: empty content")
ErrVersionConflict = errors.New("entry: version conflict")
```

Add `ValidateForPublish` that calls `ValidateTitle`, `ValidateSlug`, checks `strings.TrimSpace(ContentMD)`, and validates visibility. Add separate draft validators that allow empty title and slug but still enforce maximum lengths and slug format when non-empty.

- [ ] **Step 4: Change create and update service rules**

`Create` always stores `StatusDraft`; remove the pre-create slug query when `Slug == ""`. `UpdateInput` gains:

```go
ExpectedRevision int64
```

Reject `ExpectedRevision < 1`. Skip slug-conflict lookup for an empty slug. Pass the expected revision into repository params.

- [ ] **Step 5: Make transitions revision-aware**

Change service signatures to:

```go
func (s *Service) Publish(ctx context.Context, id, expectedRevision int64) (Entry, error)
func (s *Service) Unpublish(ctx context.Context, id, expectedRevision int64) (Entry, error)
func (s *Service) Archive(ctx context.Context, id, expectedRevision int64) (Entry, error)
```

`Publish` loads the current entry, checks its revision, calls `ValidateForPublish`, then performs the transition. Repository methods translate a no-row mutation into `ErrEntryNotFound` if the entry no longer exists and `ErrVersionConflict` if it exists at another revision.

- [ ] **Step 6: Map named database constraints**

Extend repository constraint mapping:

```go
case "entries_published_title_check": return ErrInvalidTitle
case "entries_published_slug_check": return ErrInvalidSlug
case "entries_published_content_check": return ErrEmptyContent
```

Keep `uk_entries_slug` mapped to `ErrSlugTaken`.

- [ ] **Step 7: Run domain and repository tests**

Run: `cd backend && go test ./internal/entry -count=1`

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/entry
git commit -m "feat: validate drafts when publishing"
```

### Task 3: Expose the revision-aware HTTP contract

**Files:**
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: `backend/internal/entryhttp/handler_test.go`
- Modify: `docs/api.md`
- Modify: `admin/src/types/api.ts`
- Modify: `admin/src/api/entries.ts`
- Modify: `admin/src/api/patch.ts`

**Interfaces:**
- Produces: `EntryDetail.revision: number`
- Produces: `EntryUpdateRequest.revision: number`
- Produces: transition body `{ revision: number }`
- Produces: HTTP 409 with `fields.revision`

- [ ] **Step 1: Write failing HTTP tests**

Add tests for empty `POST /entries`, missing revision on PATCH, stale revision on PATCH, incomplete publish, and stale publish. The stale response must be:

```json
{
  "error": {
    "code": "CONFLICT",
    "message": "entry changed since it was loaded",
    "fields": {"revision": "请重新载入或保留当前内容为恢复草稿"}
  }
}
```

- [ ] **Step 2: Run the HTTP tests and verify they fail**

Run: `cd backend && go test ./internal/entryhttp -run 'TestCreateEmptyDraft|TestUpdateRequiresRevision|TestUpdateStaleRevision|TestPublishIncomplete|TestPublishStaleRevision' -count=1`

Expected: FAIL against the old required fields and transition signatures.

- [ ] **Step 3: Change request and response DTOs**

Remove required bindings from create title, slug, and content. Remove create `status`. Add to update and owner response:

```go
Revision int64 `json:"revision" binding:"required,min=1"`
```

Use one transition request type:

```go
type transitionEntryRequest struct {
	Revision int64 `json:"revision" binding:"required,min=1"`
}
```

- [ ] **Step 4: Map publish and revision errors**

Map `ErrEmptyContent` to `INVALID_INPUT` with `fields.content_md`; map `ErrVersionConflict` to the exact 409 body from Step 1. Do not expose revision on public DTOs.

- [ ] **Step 5: Update admin contract types**

Use these exact interfaces:

```ts
export interface EntryDetail extends Omit<EntryListItem, 'category'> {
  revision: number
  content_md: string
  category_id: number
  category: EntryCategoryRef | null
}

export interface EntryCreateRequest {
  type?: EntryType
  visibility?: EntryVisibility
}

export interface EntryUpdateRequest {
  revision: number
  title?: string
  slug?: string
  // existing optional fields continue here
}

export type EntryPatchFields = Omit<EntryUpdateRequest, 'revision'>
```

Update the three transition functions to send `{ revision }`.

Change `buildEntryPatch(original, form)` so its result begins with
`{ revision: original.revision }`. Change `isEmptyPatch` so it ignores the
required `revision` key and reports empty only when no editable field differs.

- [ ] **Step 6: Run HTTP tests and admin build**

Run:

```bash
cd backend && go test ./internal/entryhttp -count=1
cd ../admin && npm run build
```

Expected: PASS.

- [ ] **Step 7: Update API documentation and commit**

Document incomplete drafts, required mutation revision, 409 conflict, and publish-stage validation.

```bash
git add backend/internal/entryhttp admin/src/api admin/src/types docs/api.md
git commit -m "feat: expose revision-aware entry writes"
```

### Task 4: Add the admin test harness and IndexedDB recovery store

**Files:**
- Modify: `admin/package.json`
- Modify: `admin/package-lock.json`
- Create: `admin/vitest.config.ts`
- Create: `admin/src/test/setup.ts`
- Create: `admin/src/editor/recovery-store.ts`
- Test: `admin/src/editor/recovery-store.test.ts`

**Interfaces:**
- Produces: `RecoveryRecord`
- Produces: `EntryRecoveryStore.get/put/remove/list`

- [ ] **Step 1: Install pinned test dependencies**

Run:

```bash
cd admin
npm install -D vitest@4.1.0 @vue/test-utils@2.4.6 jsdom@30.0.1 fake-indexeddb@6.2.5
```

Add scripts:

```json
"test": "vitest",
"test:run": "vitest run"
```

- [ ] **Step 2: Write failing recovery-store tests**

Test put/get, newer local record replacement, list of unsynced records, and remove after successful synchronization.

```ts
const record: RecoveryRecord = {
  entryId: 42,
  revision: 3,
  fields: { title: '山中', content_md: '正文' },
  savedLocallyAt: '2026-08-26T12:00:00.000Z',
  syncState: 'pending',
}
await store.put(record)
expect(await store.get(42)).toEqual(record)
```

- [ ] **Step 3: Run the test and verify it fails**

Run: `cd admin && npm test -- --run src/editor/recovery-store.test.ts`

Expected: FAIL because the store does not exist.

- [ ] **Step 4: Implement the native IndexedDB wrapper**

Use database `alive-admin`, version 1, object store `entry-recovery`, and `entryId` as the key path. Export:

```ts
export interface RecoveryRecord {
  entryId: number
  revision: number
  fields: EntryPatchFields
  savedLocallyAt: string
  syncState: 'pending' | 'conflict'
}

export class EntryRecoveryStore {
  get(entryId: number): Promise<RecoveryRecord | null>
  put(record: RecoveryRecord): Promise<void>
  remove(entryId: number): Promise<void>
  list(): Promise<RecoveryRecord[]>
}
```

Wrap requests and transactions in promises; reject on `request.onerror`, `transaction.onerror`, and `transaction.onabort`.

- [ ] **Step 5: Run recovery tests**

Run: `cd admin && npm test -- --run src/editor/recovery-store.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/package.json admin/package-lock.json admin/vitest.config.ts admin/src/test admin/src/editor
git commit -m "test: add admin recovery store coverage"
```

### Task 5: Build the deterministic save coordinator

**Files:**
- Create: `admin/src/editor/save-coordinator.ts`
- Create: `admin/src/editor/useEntryAutosave.ts`
- Test: `admin/src/editor/save-coordinator.test.ts`
- Test: `admin/src/editor/useEntryAutosave.test.ts`

**Interfaces:**
- Consumes: `EntryRecoveryStore`, `entriesApi.updateEntry`
- Produces: `SaveSnapshot`, `createSaveCoordinator`, `useEntryAutosave`

- [ ] **Step 1: Write failing state-machine tests with fake timers**

Cover debounce, local-write-before-network, one in-flight request, changes queued during save, manual flush, offline, retry, and 409 conflict.

```ts
const coordinator = createSaveCoordinator({
  entryId: 42,
  initialRevision: 1,
  waitMs: 1000,
  recoveryStore,
  save: vi.fn().mockResolvedValue({...entry, revision: 2}),
})
coordinator.update({ title: '新的标题' })
await vi.advanceTimersByTimeAsync(999)
expect(save).not.toHaveBeenCalled()
await vi.advanceTimersByTimeAsync(1)
expect(recoveryStore.put).toHaveBeenCalledBefore(save)
```

- [ ] **Step 2: Run and verify failure**

Run: `cd admin && npm test -- --run src/editor/save-coordinator.test.ts`

Expected: FAIL because the coordinator does not exist.

- [ ] **Step 3: Implement the coordinator API**

```ts
export type SaveStatus = 'saved' | 'pending' | 'saving' | 'offline' | 'error' | 'conflict'

export interface SaveSnapshot {
  status: SaveStatus
  revision: number
  pendingFields: EntryPatchFields | null
  error: ApiClientError | null
}

export interface SaveCoordinator {
  update(fields: EntryPatchFields): void
  flush(): Promise<EntryDetail | null>
  retry(): Promise<EntryDetail | null>
  subscribe(listener: (snapshot: SaveSnapshot) => void): () => void
  dispose(): Promise<void>
}
```

Merge pending field patches, never run two network saves concurrently, and loop once more after an in-flight save when new fields arrived. On network failure keep recovery state `pending`; on 409 set it to `conflict` and stop automatic retries.

- [ ] **Step 4: Add Vue lifecycle adapter tests and implementation**

`useEntryAutosave` exposes refs for status, revision, error, and `flush`. It subscribes on mount and calls `dispose` on unmount. It does not know about routes or render UI.

- [ ] **Step 5: Run editor tests**

Run: `cd admin && npm test -- --run src/editor`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/src/editor
git commit -m "feat: add revision-aware autosave coordinator"
```

### Task 6: Integrate autosave into the current editor safely

**Files:**
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/components/MarkdownEditor.vue`
- Test: `admin/src/views/EntryEditor.test.ts`

**Interfaces:**
- Consumes: `useEntryAutosave`
- Produces: current editor automatically saves every field and flushes on shortcuts and transitions

- [ ] **Step 1: Write failing component tests**

Mount the edit view with mocked entry/category APIs. Assert a title input schedules an update, Milkdown output schedules `content_md`, `Cmd+S` flushes, publish flushes before transition, and a 409 renders conflict actions without replacing the editor content.

- [ ] **Step 2: Run and verify failure**

Run: `cd admin && npm test -- --run src/views/EntryEditor.test.ts`

Expected: FAIL because the page still uses the manual save function.

- [ ] **Step 3: Replace manual save wiring**

For existing entries, create one coordinator after load. For a new route, call `createEntry({})`, replace to the edit route, then attach the coordinator. Every form field calls `autosave.update` with only the changed field. Remove the Save button and render the six save states in one fixed-height status slot.

- [ ] **Step 4: Add shortcut and transition flushing**

Add a window keydown handler that prevents default only for `Cmd/Ctrl + S` and awaits `flush`. `publish`, `unpublish`, `archive`, route switching, and preview must await `flush` before their next request.

- [ ] **Step 5: Preserve local content on failures**

Do not assign API response field values back into the mounted Milkdown editor. Update only the stored revision and server snapshot. Conflict actions read the `RecoveryRecord`; “另存为恢复草稿” calls `createEntry({})` and patches the recovered fields into that new ID.

- [ ] **Step 6: Run component tests and build**

Run:

```bash
cd admin
npm test -- --run src/views/EntryEditor.test.ts
npm run build
```

Expected: PASS.

- [ ] **Step 7: Run full backend and admin verification**

Run:

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
```

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add admin/src/views/EntryEditor.vue admin/src/components/MarkdownEditor.vue admin/src/views/EntryEditor.test.ts
git commit -m "feat: autosave entry drafts"
```

### Task 7: Browser verification checkpoint

**Files:**
- Modify only if verification finds a defect in files owned by this plan.

- [ ] **Step 1: Start the three services**

Run backend on 8080, admin on 5173, and frontend on 3000 using the documented development account.

- [ ] **Step 2: Verify the destructive scenarios with disposable data**

Create two blank drafts, type while offline, reload, reconnect, edit one draft in two tabs, resolve the conflict by creating a recovery draft, publish a completed draft, and verify an incomplete draft cannot publish.

- [ ] **Step 3: Record evidence**

Update `docs/progress.md` with commands, response statuses, database migration version, and browser scenarios actually verified. Do not record credentials.

- [ ] **Step 4: Commit evidence**

```bash
git add docs/progress.md
git commit -m "docs: verify revisioned autosave flow"
```
