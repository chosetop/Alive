# Admin iOS Visual Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the Alive admin interface as a distinctive iOS-inspired writing desk while preserving the existing routes, APIs, editor behavior, and the 薰衣草 theme.

**Architecture:** The shared `@alive/theme` package remains the source of theme names and semantic CSS variables, with one additional complete theme, 夜航, added only if its contrast and use-case tests pass. Admin-level aliases and UI primitives provide the visual language; layouts and views consume those primitives without owning theme-specific colors or icon-library details.

**Tech Stack:** Vue 3, Vue Router 5, Pinia 4, Vite 8, Vitest 4, `@alive/theme`, CSS custom properties, Reka UI, optional `lucide-vue-next`.

**Spec:** `docs/superpowers/specs/2026-08-27-admin-ios-visual-redesign-design.md`

## Global Constraints

- Preserve the existing authentication, route names, API contracts, autosave coordinator, publish checks, recovery flow, and editor behavior.
- Keep `codex-lavender` as the stable theme key and display name `薰衣草`; do not break its frontend rendering or visitor preference behavior.
- New theme variables must be semantic and emitted through `@alive/theme`; no business component stylesheet may contain theme-specific hex values.
- Add at most one new theme in this plan: `night-ink` with display name `夜航`; omit it only if contrast or compatibility tests cannot pass without weakening the design.
- Use icons only for existing actions, through an admin wrapper; every icon-only control has an accessible name and a minimum 44px coarse-pointer target.
- Keep the Alive 光标 as the only new brand signature; do not add decorative gradients, invented statistics, or unrelated interactions.
- Respect `prefers-reduced-motion: reduce`, preserve visible keyboard focus, and verify 1280px and 375px layouts.
- Follow TDD for behavior changes: write the smallest failing test before implementation and run the focused test before moving on.

---

## File structure

- `packages/theme/src/index.ts`: theme manifest and type-safe theme names.
- `packages/theme/src/themes.css`: complete semantic palettes, fonts, and new surface/status variables for every theme.
- `packages/theme/src/theme.test.ts`: manifest, label, resolver, and semantic-token contract tests.
- `backend/internal/site/model.go`, `backend/internal/site/service_test.go`, `backend/internal/sitehttp/handler.go`, `backend/internal/sitehttp/handler_test.go`: accept the new theme as a validated site default without changing the endpoint contract.
- `backend/migrations/000008_allow_night_ink.*.sql`: update the persisted site-theme check constraint for the new theme.
- `admin/src/style.css`: admin geometry aliases, global typography, glass surfaces, focus, and motion rules.
- `admin/src/components/ui/ui.css`: iOS-like primitives and interaction states.
- `admin/src/components/ui/UiIcon.vue`, `admin/src/components/ui/index.ts`: the only business-facing icon boundary if the current primitives need it.
- `admin/src/layouts/AdminLayout.vue`: utility shell, desktop rail, mobile navigation, top context bar, and user controls.
- `admin/src/layouts/WritingLayout.vue`, `admin/src/components/writing/WorkspaceHeader.vue`, `admin/src/components/writing/ArticleDirectory.vue`, `admin/src/components/writing/ThemePicker.vue`: writing shell visual integration.
- `admin/src/views/Dashboard.vue`, `admin/src/views/Entries.vue`, `admin/src/components/EntryRow.vue`, `admin/src/views/Categories.vue`, `admin/src/components/CategoryForm.vue`: management-page visual integration.
- `admin/src/components/ui/ui.test.ts`, `admin/src/layouts/*.test.ts`, `admin/src/views/*.test.ts`, and focused component tests: semantic and regression coverage.
- `docs/progress.md`: actual implementation, verification, and browser evidence.

## Interfaces

- `@alive/theme` exports `ThemeName`, `THEMES`, `isThemeName`, and `resolveTheme`; `night-ink` is valid only if the backend validator and CSS manifest are updated together.
- `UiIcon` accepts `{ name: IconName; label?: string; size?: 16 | 20 | 24 }` and renders decorative icons as `aria-hidden="true"`; icon-only callers must pass `label`.
- Existing `UiButton`, `UiIconButton`, `UiMenu`, `UiDialog`, `UiPopover`, and store APIs remain source-compatible.

### Task 1: Extend semantic themes and validation

**Files:**
- Modify: `packages/theme/src/index.ts`
- Modify: `packages/theme/src/themes.css`
- Modify: `packages/theme/src/theme.test.ts`
- Modify: `backend/internal/site/model.go`
- Modify: `backend/internal/site/service_test.go`
- Modify: `backend/internal/sitehttp/handler.go`
- Modify: `backend/internal/sitehttp/handler_test.go`
- Create: `backend/migrations/000008_allow_night_ink.up.sql`
- Create: `backend/migrations/000008_allow_night_ink.down.sql`

**Interfaces:**
- Produces: `ThemeName` including `night-ink`, `THEMES` entry `{ name: 'night-ink', label: '夜航', colorScheme: 'dark' }`, and semantic variables `--c-glass`, `--c-glass-border`, `--c-focus`, `--c-success`, and `--c-success-surface`.
- Preserves: `codex-lavender` and its label `薰衣草`.

- [ ] **Step 1: Write failing theme and backend validation tests**

```ts
expect(THEMES).toContainEqual({ name: 'codex-lavender', label: '薰衣草', colorScheme: 'light' })
expect(THEMES).toContainEqual({ name: 'night-ink', label: '夜航', colorScheme: 'dark' })
expect(isThemeName('night-ink')).toBe(true)
expect(resolveTheme({ visitor: 'night-ink', siteDefault: 'ink' })).toBe('night-ink')
```

```go
for _, theme := range []string{"ink", "lamp", "codex-lavender", "night-ink"} {
	if _, err := service.UpdateTheme(context.Background(), theme, 1); err != nil {
		t.Fatalf("UpdateTheme(%q): %v", theme, err)
	}
}
```

- [ ] **Step 2: Run focused tests and verify the new cases fail**

Run: `cd packages/theme && npm test -- --run src/theme.test.ts && cd ../../backend && go test ./internal/site ./internal/sitehttp`

Expected: FAIL because `night-ink` is absent from the manifest and backend allow-list.

- [ ] **Step 3: Add the manifest entry, semantic palettes, and database constraint migration**

Add `night-ink` to `THEMES`, keep `薰衣草` unchanged, and define every semantic variable under `ink`, `lamp`, `codex-lavender`, and `night-ink`. The new theme must include a deep blue-black paper, translucent slate glass, warm readable ink, a violet-blue Alive accent, green success state, danger state, focus ring, and overlay. Update backend validation and the invalid-theme message to list the supported values. Add migration `000008_allow_night_ink` that drops and recreates `site_settings_theme_check` with all four supported keys; do not rewrite migration `000007`.

- [ ] **Step 4: Run focused tests and contrast checks**

Run: `cd packages/theme && npm test -- --run src/theme.test.ts && cd ../../backend && go test ./internal/site ./internal/sitehttp`

Expected: PASS; tests must also assert that each theme has all required semantic selectors/tokens and that `薰衣草` remains the exact display label.

- [ ] **Step 5: Commit the theme contract**

```bash
git add packages/theme backend/internal/site backend/internal/sitehttp
git commit -m "feat: extend admin theme tokens"
```

### Task 2: Build the iOS visual token and icon boundary

**Files:**
- Modify: `admin/src/style.css`
- Modify: `admin/src/components/ui/ui.css`
- Create: `admin/src/components/ui/UiIcon.vue`
- Modify: `admin/src/components/ui/index.ts`
- Modify: `admin/src/components/ui/ui.test.ts`
- Modify: `admin/package.json`, `admin/package-lock.json` only if an icon package is needed

**Interfaces:**
- Consumes: semantic variables from Task 1 and existing UI primitive contracts.
- Produces: `UiIcon` with a constrained icon-name union and shared iOS-like control states; existing primitives stay import-compatible.

- [ ] **Step 1: Write failing primitive tests**

```ts
it('keeps icon-only controls named and exposes a coarse pointer target', () => {
  const wrapper = mount(UiIconButton, {
    props: { label: '搜索文章' },
    slots: { default: '<UiIcon name="search" />' },
  })
  expect(wrapper.attributes('aria-label')).toBe('搜索文章')
  expect(readFile('src/components/ui/ui.css')).toContain('44px')
})
```

Also assert that shared CSS contains the glass, focus, radius, shadow, motion, and `prefers-reduced-motion` rules, without asserting component-specific layout selectors.

- [ ] **Step 2: Run focused tests to verify the new contract fails**

Run: `cd admin && npm test -- --run src/components/ui/ui.test.ts`

Expected: FAIL because `UiIcon` and the new target/semantic rules do not yet exist.

- [ ] **Step 3: Implement the visual primitives**

Add the iOS-style token aliases, translucent surfaces, 12–18px radius scale, quiet layered shadows, 44px coarse-pointer sizing, and focus states. Use one icon implementation: prefer `lucide-vue-next` if the current package lock can install it without changing unrelated dependencies; otherwise use a small inline SVG registry inside `UiIcon.vue`. Do not allow business components to import the library directly.

- [ ] **Step 4: Run tests and build**

Run: `cd admin && npm test -- --run src/components/ui/ui.test.ts && npm run build`

Expected: PASS and a successful Vite production build.

- [ ] **Step 5: Commit the primitive layer**

```bash
git add admin/src/style.css admin/src/components/ui admin/package.json admin/package-lock.json
git commit -m "feat: add admin iOS visual primitives"
```

### Task 3: Rebuild the utility shell and navigation

**Files:**
- Modify: `admin/src/layouts/AdminLayout.vue`
- Modify: `admin/src/layouts/AdminLayout.test.ts` if present, otherwise create `admin/src/layouts/AdminLayout.test.ts`
- Modify: `admin/src/router/routes.test.ts` only for preserved route assertions

**Interfaces:**
- Consumes: Task 2 primitives and existing `auth`/router stores.
- Produces: desktop sidebar, mobile navigation trigger/drawer, contextual top bar, active-state Alive 光标, and named icon controls without changing route names.

- [ ] **Step 1: Write failing shell tests**

```ts
it('renders the active navigation state with a non-color cue', () => {
  const wrapper = mountAdminLayout('/entries')
  const active = wrapper.get('a[aria-current="page"]')
  expect(active.attributes('data-active')).toBe('true')
  expect(active.find('[data-alive-cursor]').exists()).toBe(true)
})

it('uses a mobile navigation trigger with an accessible label', () => {
  const wrapper = mountAdminLayout('/entries')
  expect(wrapper.get('[data-mobile-nav]').attributes('aria-label')).toBe('打开主导航')
})
```

- [ ] **Step 2: Run focused shell tests to verify failure**

Run: `cd admin && npm test -- --run src/layouts/AdminLayout.test.ts`

Expected: FAIL because the current shell has no mobile trigger, context bar, or Alive cursor marker.

- [ ] **Step 3: Implement the shell**

Keep the current nav item destinations and logout behavior. Add the top context label from the current route, preserve `aria-label="主导航"`, use `aria-current="page"`, render the Alive cursor on active items, and move the desktop sidebar into a mobile drawer/compact navigation below the existing responsive breakpoint. Use existing `UiDialog`/`UiIconButton` contracts for the mobile path.

- [ ] **Step 4: Run shell, router, and auth regressions**

Run: `cd admin && npm test -- --run src/layouts/AdminLayout.test.ts src/router/routes.test.ts src/App.test.ts`

Expected: PASS with all existing route and auth expectations unchanged.

- [ ] **Step 5: Commit the shell**

```bash
git add admin/src/layouts/AdminLayout.vue admin/src/layouts/AdminLayout.test.ts admin/src/router/routes.test.ts
git commit -m "feat: redesign admin navigation shell"
```

### Task 4: Restyle management pages and shared writing controls

**Files:**
- Modify: `admin/src/views/Dashboard.vue`
- Modify: `admin/src/views/Entries.vue`
- Modify: `admin/src/components/EntryRow.vue`
- Modify: `admin/src/views/Categories.vue`
- Modify: `admin/src/components/CategoryForm.vue`
- Modify: `admin/src/components/writing/ThemePicker.vue`
- Modify: corresponding existing tests

**Interfaces:**
- Consumes: Task 2 primitives, Task 3 shell, existing API/store behavior.
- Produces: consistent page header, glass action cards, status chips, tab filters, error/empty states, and theme picker with 薰衣草 plus any valid new theme.

- [ ] **Step 1: Write failing semantic markup tests**

```ts
it('keeps content actions and status readable as semantic controls', () => {
  const wrapper = mountEntries()
  expect(wrapper.get('[data-page-header]').exists()).toBe(true)
  expect(wrapper.get('[role="tablist"]').attributes('aria-label')).toBe('按状态筛选')
  expect(wrapper.get('[data-primary-action]').text()).toBe('写一篇')
})

it('shows the preserved 薰衣草 label in the theme picker', () => {
  const wrapper = mountThemePicker()
  expect(wrapper.text()).toContain('薰衣草')
})
```

- [ ] **Step 2: Run focused page tests to verify failure**

Run: `cd admin && npm test -- --run src/views/Entries.test.ts src/views/Categories.test.ts src/components/writing/ThemePicker.test.ts`

Expected: FAIL only for the new data hooks/semantic layout assertions; existing API behavior tests remain passing.

- [ ] **Step 3: Implement the page visual layer**

Convert page headers to a consistent title/context/action structure, use the semantic surface and status tokens, preserve all current Chinese copy and API calls, add icon affordances only where they clarify an existing action, and ensure the theme picker renders all manifest entries. Keep empty and error messages as real state messages rather than decorative dashboard content.

- [ ] **Step 4: Run page tests and build**

Run: `cd admin && npm test -- --run src/views src/components/EntryRow.test.ts src/components/writing/ThemePicker.test.ts && npm run build`

Expected: PASS and successful production build.

- [ ] **Step 5: Commit management-page styling**

```bash
git add admin/src/views admin/src/components/EntryRow.vue admin/src/components/CategoryForm.vue admin/src/components/writing/ThemePicker.vue
git commit -m "feat: restyle admin management pages"
```

### Task 5: Integrate the writing desk visual language

**Files:**
- Modify: `admin/src/layouts/WritingLayout.vue`
- Modify: `admin/src/components/writing/WorkspaceHeader.vue`
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Modify: `admin/src/components/writing/ArticleSettings.vue`
- Modify: `admin/src/components/writing/PublishPanel.vue`
- Modify: `admin/src/components/MarkdownEditor.vue`
- Modify: corresponding existing layout/component tests

**Interfaces:**
- Consumes: Task 2 primitives and Task 3 theme-aware shell rules.
- Preserves: directory resize bounds, mobile drawer behavior, save-status slot, autosave flush gate, publish/delete semantics, and editor Markdown output.

- [ ] **Step 1: Write failing visual-contract tests**

```ts
it('keeps the writing workspace stable while exposing the current save state', () => {
  const wrapper = mountWritingLayout()
  expect(wrapper.get('[data-writing-workspace]').attributes('data-visual-mode')).toBe('writing-desk')
  expect(wrapper.get('[data-save-status]').attributes('aria-live')).toBe('polite')
  expect(wrapper.get('[data-directory-resize]').attributes('role')).toBe('separator')
})
```

Add source/layout assertions for 220px–420px directory bounds, 375px no-overflow rules, and reduced-motion handling while retaining the existing behavioral tests.

- [ ] **Step 2: Run focused writing tests to verify failure**

Run: `cd admin && npm test -- --run src/layouts/WritingLayout.test.ts src/components/writing src/views/EntryEditor.layout.test.ts`

Expected: FAIL only for the new visual hooks; existing editor behavior remains green.

- [ ] **Step 3: Implement the writing desk styling**

Apply the paper/desk background, translucent directory and header surfaces, Alive cursor save-state marker, quiet editor chrome, consistent action hierarchy, and mobile drawer presentation. Do not add preview, heading-folding, media upload, scheduling, collaboration, or new editor commands.

- [ ] **Step 4: Run writing regressions and build**

Run: `cd admin && npm test -- --run src/layouts/WritingLayout.test.ts src/components/writing src/components/MarkdownEditor.test.ts src/views/EntryEditor.test.ts src/views/EntryEditor.layout.test.ts && npm run build`

Expected: PASS; existing autosave, conflict, publish, delete, directory, and editor tests remain unchanged or are updated only for presentation selectors.

- [ ] **Step 5: Commit the writing desk**

```bash
git add admin/src/layouts/WritingLayout.vue admin/src/components/writing admin/src/components/MarkdownEditor.vue
git commit -m "feat: apply writing desk visual language"
```

### Task 6: Browser QA, accessibility verification, and progress record

**Files:**
- Modify: `docs/progress.md`
- Modify: tests only when a verified browser issue exposes a missing contract from Tasks 1–5

**Interfaces:**
- Consumes: all completed implementation tasks and the existing local startup workflow.
- Produces: reproducible verification evidence and a current progress checkpoint; no new runtime interface.

- [ ] **Step 1: Run the complete admin verification suite**

Run:

```bash
cd admin
npm test -- --run
npm run build
git diff --check
```

Expected: all admin tests pass, build succeeds, and no whitespace errors are reported.

- [ ] **Step 2: Run backend and shared-theme regression checks**

Run:

```bash
cd backend && make check
cd ../packages/theme && npm test -- --run
```

Expected: backend checks and theme tests pass, including `薰衣草` and `夜航` validation if the new theme was kept.

- [ ] **Step 3: Inspect the real UI at desktop width**

Start the existing local services using the documented project workflow, open `http://127.0.0.1:5173`, and verify login, Dashboard, 内容, 分类, theme picker, and 写作. Confirm the active navigation marker, save status, glass surfaces, icon labels, focus ring, theme switching, and no console error overlay.

- [ ] **Step 4: Inspect the real UI at 375px**

Verify the mobile navigation path, content filters, category form, delete confirmation, writing drawer, publish controls, keyboard focus where available, and absence of horizontal overflow. Verify that touch targets are at least 44px for coarse pointers.

- [ ] **Step 5: Record evidence in progress.md**

Add the actual commands, test counts, build result, browser URLs/viewport sizes, theme names verified, and any known pre-existing environment limitation. Do not record credentials, cookies, or tokens. Update the active-plan summary so Plan 4 is marked complete only after all checks pass; otherwise record the exact remaining task.

- [ ] **Step 6: Commit the verification record**

```bash
git add docs/progress.md
git commit -m "docs: record admin visual redesign verification"
```

## Completion gate

The plan is complete only when Tasks 1–6 are checked, `薰衣草` remains selectable and functional, the new theme (if retained) is accepted by backend and frontend contracts, the admin full test suite and build pass, and real-browser checks cover both 1280px-class desktop and 375px mobile layouts.
