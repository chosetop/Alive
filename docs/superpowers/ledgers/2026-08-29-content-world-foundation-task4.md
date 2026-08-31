# Content World Foundation Task 4 Ledger

## 2026-08-29 Conflict Scan

- Existing worktree is already isolated on branch `content-worlds-foundation`. Unrelated dirty state is limited to `frontend/package-lock.json`, which is out of scope and must remain untouched.
- Backend world lifecycle APIs already exist at `/api/v1/worlds` and `/api/v1/admin/worlds`, but the admin frontend has no world settings client, registry, or `/worlds` route yet.
- `EntryEditor.vue` still creates new drafts with `createEntry({})`, keeps `form.type`, patches `type`, and loads categories without a world filter. That conflicts with Task 4's world-first creation and world-scoped categories.
- The current writing routes send `/entries/new` straight into `EntryEditor.vue`; there is no selection screen or `/entries/new/:world` validation path yet.
- `PublishPanel.vue` only emits a generic publish event. It does not branch on `WORLD_NOT_OPEN`, does not reuse world setting revisions, and cannot offer the two explicit continuations the plan requires.
- `ArticleDirectory.vue` currently lists recent entries and status groups without a world filter. Once more than one world is open, that would stop matching the plan's fixed `全部 | 日志` filter.
- `Categories.vue` and `CategoryForm.vue` assume categories are global. They neither require a world at creation time nor preserve a fixed world while editing.
- The current Admin entry/category API types still model the legacy top-level `type` contract and do not expose `WorldKey`, `WorldSetting`, or world-aware list/create payloads.

## Rulings

- Task 4 keeps the immersive writing shell intact. `/entries/new/:world` routes Journal into the existing `EntryEditor.vue`; no Milkdown, autosave, recovery, revision, or layout restructuring is allowed.
- Only registered worlds with a non-null `editorRouteName` are creatable from the admin in this task. That means the visible create path is Journal-only for now, while the registry remains extensible for later Saying and Video editors.
- `createEntry` must send `{ world: 'journal' }` for new Journal drafts. Legacy `type` selection is removed from admin create and patch flows rather than mirrored alongside world.
- Category list/create requests are world-scoped immediately. The Admin category screen uses an explicit selected world, defaults to Journal, and does not allow changing a category's world while editing.
- The article directory gets a fixed world filter with exactly `全部 | 日志` in this task. `全部` means no world query; `日志` means `world=journal`.
- World settings saves must use the row revision from the latest load. On a 409 conflict, the row reloads, the stale edit is discarded, and the UI requires another explicit save/publish confirmation.
- The current backend reports a blocked public publish as `409 CONFLICT` with `fields.world`, not a dedicated `WORLD_NOT_OPEN` code. Task 4 therefore detects that concrete response shape in the shared publish flow instead of changing backend contracts.
- The blocked-world branch is handled only during an explicit publish attempt. Draft saves, opening the panel, or loading the editor must never mutate world status.

## 2026-08-29 Minimal Closure

- Implemented the minimum usable Admin world foundation: `admin` world registry/types, world settings API client, `/worlds`, `/entries/new`, `/entries/new/:world`, `NewEntry.vue`, `Worlds.vue`, Journal-first create payloads, and Journal-scoped category wiring for Entry editor/directory/categories/entries.
- Removed the legacy admin type selector and stopped emitting `type` in entry patch flows. Admin entry/category types now model `world`/`kind` and world-scoped requests.
- Verified the smallest coherent surface before commit:
  - `cd admin && npm test -- --run src/content-worlds/registry.test.ts src/views/NewEntry.test.ts src/views/Worlds.test.ts src/router/routes.test.ts src/api/entries.test.ts src/views/Categories.test.ts src/components/writing/ArticleDirectory.test.ts src/components/writing/ArticleSettings.test.ts src/views/EntryEditor.test.ts src/views/Entries.test.ts src/components/EntryRow.test.ts src/editor/save-coordinator.test.ts`
  - `cd admin && npm run build`
- Remaining Task 4 work not included in this minimal closure: shared blocked-world publish recovery UI/composable, publish-panel continuation actions, and the fixed `全部 | 日志` directory world filter control.
