# Admin World Workspaces Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the admin world settings table with three medium-specific world desks and make Journal, Saying, and Video open inside one consistent, world-aware writing workspace.

**Architecture:** Keep `@alive/theme`, the existing Entry lifecycle, and `/entries/:id` URLs. Add presentation and editor-dispatch contracts to the closed admin world registry, extract world cards/settings into focused components, make the writing directory explicitly world-scoped, and let a host choose the correct editor for existing Entries. GSAP is isolated behind one composable and only coordinates the world-entry and content-swap transitions.

**Tech Stack:** Vue 3 Composition API, Vue Router, Pinia, TypeScript, Vitest, Vue Test Utils, GSAP 3.15, existing `@alive/theme` tokens.

**Spec:** `docs/superpowers/specs/2026-08-31-admin-world-workspaces-visual-design.md`

## Global Constraints

- Preserve existing backend and public APIs; the admin Entry detail URL remains `/entries/:id`.
- Keep the fixed world order `journal`, `saying`, `video`.
- Use existing semantic theme tokens; do not add a fixed color theme per world.
- Desktop writing directory remains resizable; below `48rem` it remains a drawer.
- Touch targets on narrow screens must be at least `44px` in either rendered dimension.
- Empty workspaces remain client-only until the first meaningful edit; entering a world must not create an Entry.
- World state, save state, errors, and selected state must be readable as text, never color or animation alone.
- GSAP may animate only `x`, `y`, `scale`, and `autoAlpha`; reduced-motion removes position, scale, and stagger.
- Every mounted GSAP context must be reverted on unmount.
- Preserve autosave flush-before-navigation, recovery, conflict, publish checks, Categories, Tags, and media behavior.

---

## File Structure

### Create

- `admin/src/components/worlds/WorldDeskCard.vue` — one world entrance, material preview, recent content, counts, and separate settings action.
- `admin/src/components/worlds/WorldSettingsPanel.vue` — revision-safe world settings drawer and dirty-state ownership.
- `admin/src/components/worlds/WorldDeskCard.test.ts` — card semantics and independent actions.
- `admin/src/components/worlds/WorldSettingsPanel.test.ts` — supported fields, dirty save, success, and 409 reload.
- `admin/src/views/WorldEditorHost.vue` — fetches an existing Entry once and dispatches its registered editor.
- `admin/src/views/WorldEditorHost.test.ts` — Journal/Saying/Video/unknown-world dispatch.
- `admin/src/components/writing/JournalCanvas.vue` — Journal-specific central manuscript surface.
- `admin/src/components/writing/VideoCanvas.vue` — Video-specific viewfinder, title, summary, and media events.
- `admin/src/components/writing/JournalCanvas.test.ts` — manuscript semantics and update events.
- `admin/src/components/writing/VideoCanvas.test.ts` — media-first layout and update events.
- `admin/src/composables/useWorldTransition.ts` — scoped GSAP timelines and reduced-motion behavior.
- `admin/src/composables/useWorldTransition.test.ts` — timeline construction, cleanup, and reduced motion.
- `admin/src/views/SayingEditor.test.ts` — shared-shell behavior, client-first creation, flush, and permalink.

### Modify

- `admin/src/content-worlds/registry.ts` — presentation, editor-kind, directory copy, and empty-state contracts.
- `admin/src/content-worlds/registry.test.ts` — pin all new contracts.
- `admin/src/views/Worlds.vue` — orchestration, snapshots, card layout, recent-entry navigation, and settings selection.
- `admin/src/views/Worlds.test.ts` — snapshot isolation, enter behavior, empty route, and settings panel.
- `admin/src/stores/writing.ts` — active world and world-reset semantics.
- `admin/src/stores/writing.test.ts` — active-world state reset.
- `admin/src/components/writing/ArticleDirectory.vue` — world-scoped queries, labels, create route, and footer.
- `admin/src/components/writing/ArticleDirectory.test.ts` — query and copy isolation for all worlds.
- `admin/src/layouts/WritingLayout.vue` — mount/key the directory only after the active world is known.
- `admin/src/layouts/WritingLayout.test.ts` — no Journal flash and world-aware drawer label.
- `admin/src/router/index.ts` — mount `WorldEditorHost` at the existing edit route.
- `admin/src/router/routes.test.ts` — preserve route URL, shell, props, and auth metadata.
- `admin/src/views/EntryEditor.vue` — accept a preloaded Entry, use registered world context, and render Journal/Video canvases.
- `admin/src/views/EntryEditor.test.ts` — preloaded-entry behavior and no duplicate GET.
- `admin/src/views/EntryEditor.layout.test.ts` — world-specific canvas selection.
- `admin/src/views/SayingEditor.vue` — shared header/directory contract, save status, flush gate, and approved note layout.
- `admin/src/components/writing/WorkspaceHeader.vue` — world breadcrumb and narrow action menu.
- `admin/src/components/writing/WorkspaceHeader.test.ts` — world label, stable save region, and mobile actions.
- `admin/src/style.css` — shared workspace geometry aliases only where multiple components consume them.

---

### Task 1: Lock the Admin World Presentation Contract

**Files:**
- Modify: `admin/src/content-worlds/registry.ts`
- Modify: `admin/src/content-worlds/registry.test.ts`

**Interfaces:**
- Consumes: existing `WorldKey`, editor route names, publish policies, and media capabilities.
- Produces: `AdminWorldDefinition.editorKind`, `material`, `description`, `directoryNoun`, `emptyCopy`, and `searchPlaceholder`.

- [ ] **Step 1: Write the failing registry assertions**

```ts
expect(resolveAdminWorld('journal')).toMatchObject({
  editorKind: 'long-form',
  material: 'manuscript',
  description: '长文、图片与时间留下的痕迹',
  directoryNoun: '日志',
  emptyCopy: '还没有日志。写下第一篇。',
  searchPlaceholder: '搜索日志',
})
expect(resolveAdminWorld('saying')).toMatchObject({
  editorKind: 'saying',
  material: 'note',
  directoryNoun: '片语',
})
expect(resolveAdminWorld('video')).toMatchObject({
  editorKind: 'long-form',
  material: 'viewfinder',
  directoryNoun: '影像',
})
```

- [ ] **Step 2: Run the registry test and verify the contract is absent**

Run: `cd admin && npm test -- --run src/content-worlds/registry.test.ts`

Expected: FAIL because `editorKind`, `material`, and copy fields are undefined.

- [ ] **Step 3: Add exact union types and values**

```ts
export type AdminEditorKind = 'long-form' | 'saying'
export type WorldMaterial = 'manuscript' | 'note' | 'viewfinder'

export interface AdminWorldDefinition {
  // existing fields stay unchanged
  editorKind: AdminEditorKind
  material: WorldMaterial
  description: string
  directoryNoun: string
  emptyCopy: string
  searchPlaceholder: string
}
```

Populate all six new fields for all three registry entries. Use the copy pinned by the spec; do not compute it from `world.key` in components.

- [ ] **Step 4: Run the focused tests**

Run: `cd admin && npm test -- --run src/content-worlds/registry.test.ts src/views/NewEntry.test.ts`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add admin/src/content-worlds/registry.ts admin/src/content-worlds/registry.test.ts
git commit -m "feat(admin): define world workspace presentation contracts"
```

---

### Task 2: Replace the World Settings Rows with Three World Desks

**Files:**
- Create: `admin/src/components/worlds/WorldDeskCard.vue`
- Create: `admin/src/components/worlds/WorldDeskCard.test.ts`
- Create: `admin/src/components/worlds/WorldSettingsPanel.vue`
- Create: `admin/src/components/worlds/WorldSettingsPanel.test.ts`
- Modify: `admin/src/views/Worlds.vue`
- Modify: `admin/src/views/Worlds.test.ts`

**Interfaces:**
- Consumes: `AdminWorldDefinition`, `AdminWorldSetting`, `EntryListItem`, `worldsApi`, `entriesApi.listEntriesAdmin`, and `categoriesApi.listCategoriesAdmin`.
- Produces: `WorldDeskSnapshot`, card events `enter`/`settings`, and settings-panel event `saved`.

- [ ] **Step 1: Write failing card and settings-panel tests**

```ts
const wrapper = mount(WorldDeskCard, {
  props: { definition: journal, setting, recentEntry, entryCount: 12, categoryCount: 4 },
})
expect(wrapper.get('[data-world-desk="journal"]').text()).toContain('一场缓慢的夏雨')
expect(wrapper.get('[data-world-status]').text()).toBe('已开放')
await wrapper.get('[data-world-enter]').trigger('click')
expect(wrapper.emitted('enter')).toEqual([['journal']])
await wrapper.get('[data-world-settings]').trigger('click')
expect(wrapper.emitted('settings')).toEqual([['journal']])
```

For `WorldSettingsPanel`, assert that Journal and Video do not render the Saying-only default-view selector, that save stays disabled until dirty, and that a 409 reloads and emits no stale `saved` payload.

- [ ] **Step 2: Run the component tests to verify missing files fail**

Run: `cd admin && npm test -- --run src/components/worlds/WorldDeskCard.test.ts src/components/worlds/WorldSettingsPanel.test.ts`

Expected: FAIL with unresolved component imports.

- [ ] **Step 3: Implement the card and settings contracts**

```ts
export interface WorldDeskSnapshot {
  setting: AdminWorldSetting
  recentEntry: EntryListItem | null
  entryCount: number
  categoryCount: number
  error: string | null
}
```

`WorldDeskCard` must use a non-interactive outer `<article>`. Give the primary enter action a stretched-link hit area and keep the settings `<button>` above it in stacking order; never nest one interactive element inside another.

`WorldSettingsPanel` owns its local draft and calls:

```ts
await worldsApi.updateWorld(setting.world, {
  revision: setting.revision,
  status: draft.status,
  nav_label: draft.navLabel,
  default_view: draft.defaultView,
})
```

On 409, reload settings through an injected `reload` callback and show `设置已被其他保存更新，请重新确认。`.

- [ ] **Step 4: Rewrite `Worlds.vue` as orchestration only**

Load settings once. For each registered world, independently request:

```ts
entriesApi.listEntriesAdmin({ world: definition.key, page_size: 1 })
categoriesApi.listCategoriesAdmin({ world: definition.key })
```

Use `Promise.allSettled` per world so one failed snapshot does not remove the other desks. Enter behavior is exact:

```ts
if (snapshot.recentEntry) {
  await router.push({ name: 'entry-edit', params: { id: String(snapshot.recentEntry.id) } })
} else {
  await router.push({ name: definition.editorRouteName!, params: { world: definition.key } })
}
```

The empty route must remain client-first; do not call `createEntry` here.

- [ ] **Step 5: Extend `Worlds.test.ts`**

Add assertions for fixed desk order, `{ world, page_size: 1 }` queries, navigation to recent id, navigation to empty editor, independent snapshot failure, opening the correct settings panel, and refreshing the saved card.

- [ ] **Step 6: Run the focused suite**

Run: `cd admin && npm test -- --run src/views/Worlds.test.ts src/components/worlds`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/views/Worlds.vue admin/src/views/Worlds.test.ts admin/src/components/worlds
git commit -m "feat(admin): turn content worlds into creative desks"
```

---

### Task 3: Make the Writing Directory Explicitly World-Scoped

**Files:**
- Modify: `admin/src/stores/writing.ts`
- Modify: `admin/src/stores/writing.test.ts`
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Modify: `admin/src/components/writing/ArticleDirectory.test.ts`
- Modify: `admin/src/layouts/WritingLayout.vue`
- Modify: `admin/src/layouts/WritingLayout.test.ts`

**Interfaces:**
- Consumes: `WorldKey`, `resolveAdminWorld`, registry copy, current flush gate, and existing directory query behavior.
- Produces: `writing.activeWorld`, `setActiveWorld(world)`, and required `ArticleDirectory.world` prop.

- [ ] **Step 1: Write failing store and directory tests**

```ts
const store = useWritingStore()
store.setSearchQuery('雨')
store.setDirectoryEntries([journalItem])
store.setActiveWorld('saying')
expect(store.activeWorld).toBe('saying')
expect(store.searchQuery).toBe('')
expect(store.directoryEntries).toEqual([])
```

Mount `ArticleDirectory` with `world="saying"` and assert every recent, search, and status request includes `world: 'saying'`. Assert the create button reads `写片语` and routes to `saying-editor-new` without calling `createEntry`.

- [ ] **Step 2: Run tests and verify the world prop/state is missing**

Run: `cd admin && npm test -- --run src/stores/writing.test.ts src/components/writing/ArticleDirectory.test.ts src/layouts/WritingLayout.test.ts`

Expected: FAIL on missing `setActiveWorld` and missing `world` prop.

- [ ] **Step 3: Add active-world reset semantics**

```ts
const activeWorld = ref<WorldKey | null>(null)

function setActiveWorld(world: WorldKey | null): void {
  if (activeWorld.value === world) return
  activeWorld.value = world
  activeEntryId.value = null
  searchQuery.value = ''
  directoryEntries.value = []
}
```

Return both symbols from the store.

- [ ] **Step 4: Scope the directory**

Require `world: WorldKey`. Include `world: props.world` in `loadRecent`, `runSearch`, and `toggleGroup`. Replace hard-coded article copy with the registry definition. Change create behavior to flush, then navigate to the registered client-first new route; never call `entriesApi.createEntry` from the directory.

Use the current world in accessible names: `aria-label="日志目录"`, `aria-label="片语目录"`, or `aria-label="影像目录"`.

- [ ] **Step 5: Gate and key the directory in `WritingLayout`**

```vue
<ArticleDirectory
  v-if="writing.activeWorld"
  :key="writing.activeWorld"
  :world="writing.activeWorld"
  :drawer="isNarrow"
/>
```

Both desktop and drawer mount points must use the same active-world gate. The drawer title comes from `resolveAdminWorld(writing.activeWorld)?.directoryNoun` and must not briefly say “文章目录”.

- [ ] **Step 6: Run focused tests**

Run: `cd admin && npm test -- --run src/stores/writing.test.ts src/components/writing/ArticleDirectory.test.ts src/layouts/WritingLayout.test.ts`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/stores/writing.ts admin/src/stores/writing.test.ts admin/src/components/writing/ArticleDirectory.vue admin/src/components/writing/ArticleDirectory.test.ts admin/src/layouts/WritingLayout.vue admin/src/layouts/WritingLayout.test.ts
git commit -m "feat(admin): scope the writing directory to its world"
```

---

### Task 4: Dispatch Existing Entries to the Correct World Editor

**Files:**
- Create: `admin/src/views/WorldEditorHost.vue`
- Create: `admin/src/views/WorldEditorHost.test.ts`
- Modify: `admin/src/router/index.ts`
- Modify: `admin/src/router/routes.test.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/views/EntryEditor.test.ts`
- Modify: `admin/src/views/SayingEditor.vue`

**Interfaces:**
- Consumes: `entriesApi.getEntry(id)`, `AdminWorldDefinition.editorKind`, `writing.setActiveWorld`, `EntryEditor.initialEntry`, and `SayingEditor.initialEntry`.
- Produces: one GET per existing edit navigation and a concrete editor selection for every known world.

- [ ] **Step 1: Write failing host tests**

```ts
api.getEntry.mockResolvedValue(entry({ world: 'saying' }))
const wrapper = mount(WorldEditorHost, { props: { id: '41' } })
await flushPromises()
expect(wrapper.findComponent(SayingEditor).exists()).toBe(true)
expect(wrapper.findComponent(EntryEditor).exists()).toBe(false)
expect(useWritingStore().activeWorld).toBe('saying')
```

Repeat for Journal and Video (`EntryEditor` with the correct preloaded Entry). For `world: 'unknown'`, assert a role-alert error and no editor.

- [ ] **Step 2: Run host and route tests to verify failure**

Run: `cd admin && npm test -- --run src/views/WorldEditorHost.test.ts src/router/routes.test.ts`

Expected: FAIL because the host does not exist and `entry-edit` still mounts `EntryEditor` directly.

- [ ] **Step 3: Implement the host with one fetch**

```ts
const entry = ref<EntryDetail | null>(null)
const definition = computed(() => resolveAdminWorld(entry.value?.world))

let loadGeneration = 0
async function load(id: string): Promise<void> {
  const generation = ++loadGeneration
  const loaded = await entriesApi.getEntry(Number(id))
  if (generation !== loadGeneration) return
  const world = resolveAdminWorld(loaded.world)
  if (!world) throw new Error(`Unsupported world: ${loaded.world}`)
  writing.setActiveWorld(world.key)
  entry.value = loaded
}
watch(() => props.id, (id) => void load(id), { immediate: true })
```

The generation guard keeps a slower prior id request from replacing a newer directory selection. Render `SayingEditor` for `editorKind === 'saying'`; render `EntryEditor` for `editorKind === 'long-form'`. Pass `:initial-entry="entry"` to either, and also pass `:id="String(entry.id)"` to `EntryEditor` so its existing delete, recovery, and route-flush paths keep the real id.

- [ ] **Step 4: Teach editors to consume the preloaded result**

Add `initialEntry?: EntryDetail` to both editor props. In existing-entry mode, apply it and bind the coordinator without calling `getEntry` again. New routes continue through their current client-first branch.

The host owns the loading/error state for existing Entries. The editor owns save, conflict, publish, and deletion states after mount.

- [ ] **Step 5: Replace only the route component**

Keep path, name, `props: true`, writing-shell parent, and auth metadata unchanged:

```ts
{
  path: ':id',
  name: 'entry-edit',
  component: () => import('../views/WorldEditorHost.vue'),
  props: true,
}
```

- [ ] **Step 6: Run routing and editor tests**

Run: `cd admin && npm test -- --run src/views/WorldEditorHost.test.ts src/router/routes.test.ts src/views/EntryEditor.test.ts`

Expected: PASS, including an assertion that an existing Entry produces exactly one `getEntry` call.

- [ ] **Step 7: Commit**

```bash
git add admin/src/views/WorldEditorHost.vue admin/src/views/WorldEditorHost.test.ts admin/src/router/index.ts admin/src/router/routes.test.ts admin/src/views/EntryEditor.vue admin/src/views/EntryEditor.test.ts admin/src/views/SayingEditor.vue
git commit -m "feat(admin): dispatch entries to world-specific editors"
```

---

### Task 5: Give Every Editor One Shared World Header

**Files:**
- Modify: `admin/src/components/writing/WorkspaceHeader.vue`
- Modify: `admin/src/components/writing/WorkspaceHeader.test.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/views/SayingEditor.vue`

**Interfaces:**
- Consumes: `worldLabel`, save/Entry status, current directory state, and existing header events.
- Produces: stable breadcrumb `世界 / <world>`, one save region, and narrow-screen overflow events.

- [ ] **Step 1: Write failing header tests**

```ts
const wrapper = mount(WorkspaceHeader, {
  props: { worldLabel: '片语', saveStatus: 'saved', entryStatus: 'draft' },
})
expect(wrapper.get('[data-world-context]').text()).toBe('世界 / 片语')
expect(wrapper.findAll('[data-save-status]')).toHaveLength(1)
```

At a narrow layout attribute or prop, assert settings/archive/delete are in the existing menu component while Publish remains visible.

- [ ] **Step 2: Run the header suite and verify missing prop/markup**

Run: `cd admin && npm test -- --run src/components/writing/WorkspaceHeader.test.ts`

Expected: FAIL on `data-world-context`.

- [ ] **Step 3: Add the breadcrumb without destabilizing save state**

Add required `worldLabel: string`. The left track contains directory toggle plus:

```vue
<span data-world-context class="world-context">
  <RouterLink :to="{ name: 'worlds' }">世界</RouterLink>
  <span aria-hidden="true">/</span>
  <strong>{{ worldLabel }}</strong>
</span>
```

Keep the current fixed-height `aria-live="polite"` save region. Do not duplicate it in either editor.

- [ ] **Step 4: Pass registry labels from both editors**

`EntryEditor` uses `worldDefinition.label`; `SayingEditor` uses the resolved Saying definition. Both keep the existing publish/settings/delete event contract.

- [ ] **Step 5: Run header and editor integration tests**

Run: `cd admin && npm test -- --run src/components/writing/WorkspaceHeader.test.ts src/views/EntryEditor.test.ts src/views/SayingEditor.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/src/components/writing/WorkspaceHeader.vue admin/src/components/writing/WorkspaceHeader.test.ts admin/src/views/EntryEditor.vue admin/src/views/SayingEditor.vue admin/src/views/SayingEditor.test.ts
git commit -m "feat(admin): unify world editor headers"
```

---

### Task 6: Split Journal and Video into Medium-Specific Canvases

**Files:**
- Create: `admin/src/components/writing/JournalCanvas.vue`
- Create: `admin/src/components/writing/JournalCanvas.test.ts`
- Create: `admin/src/components/writing/VideoCanvas.vue`
- Create: `admin/src/components/writing/VideoCanvas.test.ts`
- Modify: `admin/src/views/EntryEditor.vue`
- Modify: `admin/src/views/EntryEditor.layout.test.ts`

**Interfaces:**
- Consumes: current form fields, `MarkdownEditor`, `VideoUpload`, disabled state, current Entry id/revision, and existing update handlers.
- Produces: `update:title`, `update:summary`, `update:content`, and `revision` events while `EntryEditor` remains the save/session owner.

- [ ] **Step 1: Write failing canvas tests**

Journal assertions:

```ts
expect(wrapper.get('[data-journal-canvas]').element).toBeTruthy()
expect(wrapper.get('[data-journal-title]').attributes('aria-label')).toBe('日志标题')
await wrapper.get('[data-journal-title]').setValue('一场缓慢的夏雨')
expect(wrapper.emitted('update:title')).toEqual([['一场缓慢的夏雨']])
```

Video assertions:

```ts
expect(wrapper.get('[data-video-viewfinder]').element).toBeTruthy()
expect(wrapper.get('[data-video-viewfinder]').attributes('data-aspect')).toBe('16:9')
expect(wrapper.get('[data-video-title]').element).toBeTruthy()
expect(wrapper.get('[data-video-summary]').element).toBeTruthy()
```

For an unsaved Video Entry, assert actionable client-first copy and no `VideoUpload` mount.

- [ ] **Step 2: Run tests and verify the canvases are missing**

Run: `cd admin && npm test -- --run src/components/writing/JournalCanvas.test.ts src/components/writing/VideoCanvas.test.ts`

Expected: FAIL with unresolved imports.

- [ ] **Step 3: Build `JournalCanvas`**

Use an actual text input/textarea for the title and the existing `MarkdownEditor` for content. The canvas provides manuscript layout, page-edge line, date/context text, and typography only. It emits values upward; it must not call APIs or own revisions.

- [ ] **Step 4: Build `VideoCanvas`**

The first element after the shared header is a `16:9` viewfinder. When saved, mount `VideoUpload` inside it and forward `revision`. Place title and summary below. Keep Categories, Tags, visibility, cover, and destructive actions in `ArticleSettings`.

- [ ] **Step 5: Dispatch canvases inside `EntryEditor`**

```vue
<JournalCanvas
  v-if="worldDefinition?.material === 'manuscript'"
  :title="form.title"
  :content="form.contentMd"
  :disabled="controlsDisabled"
  @update:title="handleTitleUpdate"
  @update:content="handleContentUpdate"
/>
<VideoCanvas
  v-else-if="worldDefinition?.material === 'viewfinder'"
  :entry="original"
  :title="form.title"
  :summary="form.summary"
  :disabled="controlsDisabled"
  @update:title="handleTitleUpdate"
  @update:summary="handleSummaryUpdate"
  @revision="onMediaRevision"
/>
```

Remove the old Video slot from the generic template after these tests pass.

- [ ] **Step 6: Run canvas and editor regression tests**

Run: `cd admin && npm test -- --run src/components/writing/JournalCanvas.test.ts src/components/writing/VideoCanvas.test.ts src/views/EntryEditor.layout.test.ts src/views/EntryEditor.test.ts`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/components/writing/JournalCanvas.vue admin/src/components/writing/JournalCanvas.test.ts admin/src/components/writing/VideoCanvas.vue admin/src/components/writing/VideoCanvas.test.ts admin/src/views/EntryEditor.vue admin/src/views/EntryEditor.layout.test.ts
git commit -m "feat(admin): add journal and video workspace canvases"
```

---

### Task 7: Rebuild Saying as the Shared Note Workspace

**Files:**
- Modify: `admin/src/views/SayingEditor.vue`
- Create: `admin/src/views/SayingEditor.test.ts`

**Interfaces:**
- Consumes: `WorkspaceHeader`, `writingFlushKey`, `writing.setActiveWorld`, current Saying CRUD methods, `TagPicker`, and registry copy.
- Produces: one central note, collapsed supplementary details, header save status, and flush-before-directory-navigation.

- [ ] **Step 1: Write behavior-first Saying tests**

Cover these exact cases:

```ts
expect(wrapper.get('[data-saying-note]').element).toBeTruthy()
expect(wrapper.find('[data-saying-title]').exists()).toBe(false)
expect(wrapper.get('[data-world-context]').text()).toContain('片语')
expect(api.createEntry).not.toHaveBeenCalled()
await wrapper.get('[data-saying-body]').setValue('风经过窗边时，房间忽然安静。')
expect(api.createEntry).toHaveBeenCalledWith({ world: 'saying', visibility: 'public' })
```

Also assert supplementary fields are collapsed by default, `>300` characters shows the Journal suggestion, the injected flush gate saves before resolving, and a published permalink can be copied.

- [ ] **Step 2: Run the test and verify the old standalone layout fails**

Run: `cd admin && npm test -- --run src/views/SayingEditor.test.ts`

Expected: FAIL because the existing editor has no shared header, note marker, or flush registration.

- [ ] **Step 3: Register the world and flush gate**

On mount call `writing.setActiveWorld('saying')`, register `save` as the current `writingFlushKey`, and clear only if the gate still belongs to this instance on unmount.

Map local states to `SaveStatus`: untouched/last successful save → `saved`, edit queued → `pending`, request active → `saving`, request failure → `error`.

Use one `650ms` debounce for ordinary Saying edits. The input handler marks `pending`, ensures the client-first Entry exists, and schedules `save`; the injected flush gate clears the timer and awaits `save` immediately. Clear the timer on unmount. Tests use fake timers to prove multiple keystrokes produce one PATCH and a directory switch flushes before navigation.

- [ ] **Step 4: Implement the approved note composition**

Use one textarea/content control inside `[data-saying-note]`. Move source, author, visibility, and Tags behind a button with `aria-expanded`. Remove bottom Save/Publish duplication; explicit Publish stays in `WorkspaceHeader`, while autosave/flush owns draft persistence.

Do not add a yellow background, pin, tape, or fixed handwritten rotation.

- [ ] **Step 5: Run Saying, directory, and host tests**

Run: `cd admin && npm test -- --run src/views/SayingEditor.test.ts src/components/writing/ArticleDirectory.test.ts src/views/WorldEditorHost.test.ts`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add admin/src/views/SayingEditor.vue admin/src/views/SayingEditor.test.ts
git commit -m "feat(admin): bring sayings into the shared writing workspace"
```

---

### Task 8: Add Scoped GSAP World Transitions

**Files:**
- Create: `admin/src/composables/useWorldTransition.ts`
- Create: `admin/src/composables/useWorldTransition.test.ts`
- Modify: `admin/src/views/Worlds.vue`
- Modify: `admin/src/layouts/WritingLayout.vue`
- Modify: `admin/src/views/WorldEditorHost.vue`

**Interfaces:**
- Consumes: scoped root element, Router navigation callback, `gsap.context`, `gsap.matchMedia`, and mounted desk/canvas refs.
- Produces: `enterWorld(card, siblings, navigate)`, `enterWorkspace(rail, canvas)`, `swapCanvas(surface, replace)`, and `dispose()`.

- [ ] **Step 1: Write failing composable tests with a mocked GSAP module**

Assert the normal path constructs one timeline with labels and only approved properties. Assert reduced motion calls `navigate` without `x`, `y`, `scale`, or stagger. Assert `dispose()` calls both `context.revert()` and kills the active timeline.

```ts
expect(timeline.to).toHaveBeenCalledWith(siblings, expect.objectContaining({ autoAlpha: 0, y: 10 }), 'leave')
expect(timeline.to).toHaveBeenCalledWith(card, expect.objectContaining({ scale: 1.018 }), 'leave')
expect(context.revert).toHaveBeenCalledOnce()
```

- [ ] **Step 2: Run the composable test and verify it is missing**

Run: `cd admin && npm test -- --run src/composables/useWorldTransition.test.ts`

Expected: FAIL with unresolved import.

- [ ] **Step 3: Implement lifecycle-safe timelines**

Use `gsap.context(callback, root)` for selector scoping. Use labels `leave`, `navigate`, and `enter`. Card leave uses `power3.inOut`; rail and canvas enter use `power2.out`. Store one active timeline and kill it before starting another.

Reduced motion resolves navigation synchronously and sets final visibility without motion.

- [ ] **Step 4: Wire world entry and workspace entrance**

`Worlds.vue` awaits `enterWorld(...)`; the navigation callback performs the existing recent/empty route decision. `WritingLayout`/`WorldEditorHost` calls `enterWorkspace` only after the active world and editor are mounted.

Wrap the dynamic editor in one stable host surface. When `props.id` changes, `WorldEditorHost` fetches the next Entry first, then calls:

```ts
await transition.swapCanvas(editorSurface.value, async () => {
  entry.value = nextEntry
  writing.setActiveWorld(nextEntry.world)
  await nextTick()
})
```

`swapCanvas` fades the stable surface out in `140–160ms`, awaits `replace`, then fades it in with `y: 8` over `220–260ms`. Error or flush-veto paths do not start leave animation.

- [ ] **Step 5: Verify no route or component leaks**

Extend tests to unmount during a running transition, navigate between two Entry ids, and enable `prefers-reduced-motion`. Assert no stale inline `transform`, `opacity`, or `visibility` remains after cleanup.

- [ ] **Step 6: Run motion and shell suites**

Run: `cd admin && npm test -- --run src/composables/useWorldTransition.test.ts src/views/Worlds.test.ts src/layouts/WritingLayout.test.ts src/views/WorldEditorHost.test.ts`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add admin/src/composables/useWorldTransition.ts admin/src/composables/useWorldTransition.test.ts admin/src/views/Worlds.vue admin/src/layouts/WritingLayout.vue admin/src/views/WorldEditorHost.vue
git commit -m "feat(admin): choreograph world workspace transitions"
```

---

### Task 9: Responsive, Accessibility, and Full Regression Acceptance

**Files:**
- Modify: `admin/src/views/Worlds.vue`
- Modify: `admin/src/components/worlds/WorldDeskCard.vue`
- Modify: `admin/src/components/writing/WorkspaceHeader.vue`
- Modify: `admin/src/components/writing/JournalCanvas.vue`
- Modify: `admin/src/components/writing/VideoCanvas.vue`
- Modify: `admin/src/views/SayingEditor.vue`
- Modify: `admin/src/style.css`
- Modify: `admin/src/views/Worlds.test.ts`
- Modify: `admin/src/components/worlds/WorldDeskCard.test.ts`
- Modify: `admin/src/components/writing/WorkspaceHeader.test.ts`
- Modify: `admin/src/components/writing/JournalCanvas.test.ts`
- Modify: `admin/src/components/writing/VideoCanvas.test.ts`
- Modify: `admin/src/views/SayingEditor.test.ts`
- Modify: `admin/src/layouts/WritingLayout.test.ts`
- Modify: `admin/src/composables/useWorldTransition.test.ts`

**Interfaces:**
- Consumes: all world-workspace components and existing `48rem` drawer breakpoint.
- Produces: desktop asymmetric desks, vertical mobile desks, one mobile directory, visible focus, and complete reduced-motion acceptance.

- [ ] **Step 1: Add source/DOM assertions for responsive and accessible contracts**

Pin these behaviors:

- desktop world grid uses Journal spanning two rows;
- at `48rem` or below the world cards are one column;
- card enter and settings controls are separate focusable elements;
- mobile header keeps Publish visible and gives other actions a named menu;
- Saying removes float shadow on narrow screens;
- Video viewfinder remains `aspect-ratio: 16 / 9`;
- all three empty states include their exact action-oriented copy;
- reduced-motion CSS and GSAP paths are both present.

- [ ] **Step 2: Run the affected suites and observe any missing contracts**

Run: `cd admin && npm test -- --run src/views/Worlds.test.ts src/components/worlds src/components/writing src/layouts/WritingLayout.test.ts src/views/SayingEditor.test.ts src/views/WorldEditorHost.test.ts`

Expected: FAIL only for responsive/a11y details not yet applied.

- [ ] **Step 3: Apply responsive styles without new fixed colors**

Use existing spacing, radius, surface, ink, line, and focus variables. Keep shared aliases in `style.css`; keep medium-specific styles inside their components. Do not move component-only selectors to the global stylesheet.

- [ ] **Step 4: Run the complete admin test suite**

Run: `cd admin && npm test -- --run`

Expected: all tests PASS with no unhandled promise rejection or Vue warning.

- [ ] **Step 5: Run type-check and production build**

Run: `cd admin && npm run build`

Expected: `vue-tsc -b` and Vite build both exit 0.

- [ ] **Step 6: Perform browser acceptance at desktop and 375px**

Verify, with real authenticated data:

1. `/worlds` displays Journal, Saying, Video in fixed order and one failed snapshot does not remove the others.
2. Entering each non-empty world opens its most recently edited Entry in the correct editor.
3. Entering an empty world creates no server record until meaningful input.
4. Directory search/status queries never return another world's content.
5. Journal, Saying, and Video show manuscript, note, and viewfinder canvases respectively.
6. Pending edits flush before switching Entries or returning to Worlds.
7. Settings save and 409 recovery work from the right-side panel.
8. Desktop directory resize/collapse works; 375px uses one named drawer and no horizontal overflow.
9. Keyboard focus reaches card enter, card settings, directory rows, header actions, and panel close in logical order.
10. Reduced-motion mode removes card/canvas travel while navigation and state changes remain complete.

- [ ] **Step 7: Commit final polish**

```bash
git add admin/src/views/Worlds.vue admin/src/views/Worlds.test.ts admin/src/components/worlds/WorldDeskCard.vue admin/src/components/worlds/WorldDeskCard.test.ts admin/src/components/writing/WorkspaceHeader.vue admin/src/components/writing/WorkspaceHeader.test.ts admin/src/components/writing/JournalCanvas.vue admin/src/components/writing/JournalCanvas.test.ts admin/src/components/writing/VideoCanvas.vue admin/src/components/writing/VideoCanvas.test.ts admin/src/views/SayingEditor.vue admin/src/views/SayingEditor.test.ts admin/src/layouts/WritingLayout.test.ts admin/src/composables/useWorldTransition.test.ts admin/src/style.css
git commit -m "feat(admin): finish responsive world workspaces"
```

- [ ] **Step 8: Record final evidence**

Run:

```bash
git status --short
git log -9 --oneline
```

Expected: only pre-existing user-owned untracked files remain; the nine task commits are visible in order. Do not add `.superpowers/brainstorm/` or the repository-root `node_modules/`.
