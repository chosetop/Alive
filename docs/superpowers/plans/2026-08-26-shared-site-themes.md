# Shared Site Themes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one semantic theme definition drive the admin preview and frontend SSR, with an explicit site default and visitor cookie override.

**Architecture:** A local `@alive/theme` package owns theme names, labels, precedence logic, and CSS semantic tokens. A singleton backend site-settings record stores the default theme with optimistic revision. The admin previews any theme locally and changes the site default only through an explicit save; Nuxt resolves visitor cookie over the fetched site default during SSR.

**Tech Stack:** PostgreSQL/sqlc/Gin, Vue 3, Pinia, Nuxt 3.21.11 SSR, local npm package, CSS custom properties and `data-theme` attributes.

**Spec:** `docs/superpowers/specs/2026-08-26-admin-writing-experience-design.md`

## Global Constraints

- Plans 1 and 2 are complete.
- Initial theme keys are exactly `ink`, `lamp`, and `codex-lavender`.
- Theme selection in admin is preview-only until “设为站点默认” succeeds.
- Visitor cookie overrides site default; absence of a visitor cookie means follow the site default.
- Frontend SSR must emit the correct `data-theme` in the first HTML response; no hydration color flash.
- Open frontend pages do not receive a live theme push.
- Components read semantic tokens only; no theme-specific hex value appears in a component stylesheet.
- Codex Lavender uses the approved B prose color: deep grape title and lower-saturation lavender-purple body.

---

## File structure

- `packages/theme`: shared manifest, resolver, and semantic CSS.
- `backend/migrations/000007_create_site_settings.*.sql`: singleton persisted default.
- `backend/internal/site`: domain, service, repository.
- `backend/internal/sitehttp`: public read and authenticated update.
- `admin/src/stores/theme.ts`: preview/default/save state.
- `admin/src/components/writing/ThemePicker.vue`: preview grid and explicit save.
- `frontend/composables/useSiteSettings.ts`: SSR-fetched default.
- `frontend/composables/useTheme.ts`: visitor-cookie precedence.
- `frontend/components/ThemeToggle.vue`: three themes plus “跟随站点”.

### Task 1: Create the shared theme package

**Files:**
- Create: `packages/theme/package.json`
- Create: `packages/theme/package-lock.json`
- Create: `packages/theme/src/index.ts`
- Create: `packages/theme/src/theme.test.ts`
- Create: `packages/theme/src/themes.css`
- Modify: `admin/package.json`, `admin/package-lock.json`
- Modify: `frontend/package.json`, `frontend/package-lock.json`

**Interfaces:**
- Produces: `ThemeName`, `THEMES`, `isThemeName`, `resolveTheme`
- Produces: `@alive/theme/themes.css`

- [x] **Step 1: Write failing manifest and precedence tests**

```ts
expect(THEMES.map((x) => x.name)).toEqual(['ink', 'lamp', 'codex-lavender'])
expect(resolveTheme({ visitor: 'lamp', siteDefault: 'ink' })).toBe('lamp')
expect(resolveTheme({ visitor: null, siteDefault: 'codex-lavender' })).toBe('codex-lavender')
expect(resolveTheme({ visitor: 'bogus', siteDefault: 'ink' })).toBe('ink')
```

- [x] **Step 2: Create the package manifest**

```json
{
  "name": "@alive/theme",
  "private": true,
  "type": "module",
  "exports": {
    ".": "./src/index.ts",
    "./themes.css": "./src/themes.css"
  },
  "devDependencies": {"vitest": "4.1.0"}
}
```

Every theme also defines `--font-prose`, `--font-heading`, `--font-ui`, and
`--font-mono`. Paper and lamp keep ChillKai for prose and headings; Codex
Lavender also keeps ChillKai but changes its ink colors. Interface text remains
the system sans stack in all themes. Add the single `@font-face` declaration to
the shared CSS, pointing at `/fonts/chillkai.woff2`.

Use this exact TypeScript contract:

```ts
export const THEMES = [
  { name: 'ink', label: '纸墨', colorScheme: 'light' },
  { name: 'lamp', label: '灯下', colorScheme: 'dark' },
  { name: 'codex-lavender', label: 'Codex Lavender', colorScheme: 'light' },
] as const
export type ThemeName = (typeof THEMES)[number]['name']
export function isThemeName(value: unknown): value is ThemeName
export function resolveTheme(input: { visitor: unknown; siteDefault: unknown }): ThemeName
```

Invalid site defaults fall back to `ink`; an invalid visitor value is treated as absent.

- [x] **Step 3: Move semantic palettes into shared CSS**

Define every semantic token under all three selectors. The Codex Lavender core must use:

```css
[data-theme='codex-lavender'] {
  color-scheme: light;
  --c-paper: #fcfbfd;
  --c-surface: #f5f4f7;
  --c-surface-sunken: #ebe7f0;
  --c-ink: #68399d;
  --c-prose: #7652a2;
  --c-ink-muted: #8d6bb4;
  --c-ink-faint: #765b94;
  --c-line: #ece8ef;
  --c-line-strong: #dcd3e5;
  --c-accent: #9a6bdc;
  --c-accent-hover: #8454c9;
}
```

Verify text token contrast on its intended surface before accepting the values. If a sampled value fails WCAG AA, darken that semantic text token while preserving hue; do not change the approved prose family to neutral gray.

- [x] **Step 4: Install the local package in both apps**

Run:

```bash
cd packages/theme && npm install
cd admin && npm install '@alive/theme@file:../packages/theme'
cd ../frontend && npm install '@alive/theme@file:../packages/theme'
```

- [x] **Step 5: Run package tests**

Run: `cd packages/theme && npx vitest run`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add packages/theme admin/package* frontend/package*
git commit -m "feat: add shared Alive theme package"
```

### Task 2: Persist the site default theme

**Files:**
- Create: `backend/migrations/000007_create_site_settings.up.sql`
- Create: `backend/migrations/000007_create_site_settings.down.sql`
- Create: `backend/sql/queries/site.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Create: `backend/internal/site/model.go`
- Create: `backend/internal/site/service.go`
- Create: `backend/internal/site/repository.go`
- Create: `backend/internal/site/sitetest/store.go`
- Test: `backend/internal/site/*_test.go`

**Interfaces:**
- Produces: singleton `SiteSettings{DefaultTheme string, Revision int64, UpdatedAt time.Time}`
- Produces: `Get(ctx)` and `UpdateTheme(ctx, theme, expectedRevision)`

- [x] **Step 1: Write failing service tests**

Cover default read, three accepted keys, unknown key rejection, stale revision, and revision increment.

```go
updated, err := service.UpdateTheme(ctx, "codex-lavender", 1)
if err != nil || updated.Revision != 2 || updated.DefaultTheme != "codex-lavender" {
	t.Fatalf("update = (%+v, %v)", updated, err)
}
```

- [x] **Step 2: Add migration 000007**

```sql
CREATE TABLE site_settings (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  default_theme VARCHAR(64) NOT NULL DEFAULT 'ink',
  revision BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT site_settings_theme_check
    CHECK (default_theme IN ('ink', 'lamp', 'codex-lavender'))
);
INSERT INTO site_settings (id) VALUES (1);
CREATE TRIGGER site_settings_set_updated_at
  BEFORE UPDATE ON site_settings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
```

The down migration drops the table only.

- [x] **Step 3: Add sqlc queries**

```sql
-- name: GetSiteSettings :one
SELECT default_theme, revision, updated_at FROM site_settings WHERE id = 1;

-- name: UpdateSiteTheme :one
UPDATE site_settings
SET default_theme = sqlc.arg(default_theme), revision = revision + 1
WHERE id = 1 AND revision = sqlc.arg(expected_revision)
RETURNING default_theme, revision, updated_at;
```

- [x] **Step 4: Implement domain and repository**

Define `ErrInvalidTheme` and `ErrVersionConflict` in the `site` package. The repository translates `pgx.ErrNoRows` on update into `ErrVersionConflict`; the singleton row is created by migration and absence is an internal error.

- [x] **Step 5: Run tests**

Run:

```bash
cd backend
make sqlc
go test ./internal/site -count=1
```

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add backend/migrations/000007_* backend/sql/queries/site.sql backend/internal/postgres/sqlcgen backend/internal/site
git commit -m "feat: persist the site default theme"
```

### Task 3: Add public and admin site-settings APIs

**Files:**
- Create: `backend/internal/sitehttp/dto.go`
- Create: `backend/internal/sitehttp/handler.go`
- Test: `backend/internal/sitehttp/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `GET /api/v1/site`
- Produces: `PATCH /api/v1/admin/site` body `{default_theme, revision}`

- [x] **Step 1: Write failing handler and router tests**

Assert public GET needs no session, admin PATCH returns 401 without a session, valid PATCH increments revision, invalid theme returns `fields.default_theme`, and stale revision returns 409 with `fields.revision`.

- [x] **Step 2: Implement DTOs and handlers**

Use one response:

```go
type siteSettingsResponse struct {
	DefaultTheme string    `json:"default_theme"`
	Revision     int64     `json:"revision"`
	UpdatedAt    time.Time `json:"updated_at"`
}
```

The public response includes revision so the admin can reuse GET without a second shape. PATCH accepts no other site fields.

- [x] **Step 3: Wire the service**

Add `SiteService *site.Service` to router dependencies, require it at startup, register public GET on `/site`, and authenticated PATCH on `/admin/site`. Construct the service in `cmd/server/main.go`.

- [x] **Step 4: Run backend tests**

Run: `cd backend && go test ./internal/sitehttp ./internal/router ./cmd/server -count=1`

Expected: PASS.

- [x] **Step 5: Update API docs and commit**

```bash
git add backend/internal/sitehttp backend/internal/router backend/cmd/server docs/api.md
git commit -m "feat: expose site theme settings"
```

### Task 4: Apply shared themes and admin preview state

**Files:**
- Modify: `admin/src/style.css`
- Modify: `admin/src/main.ts`
- Modify: `admin/vite.config.ts`
- Create: `admin/src/api/site.ts`
- Modify: `admin/src/api/index.ts`
- Create: `admin/src/stores/theme.ts`
- Create: `admin/src/components/writing/ThemePicker.vue`
- Modify: `admin/src/layouts/WritingLayout.vue`
- Test: `admin/src/stores/theme.test.ts`
- Test: `admin/src/components/writing/ThemePicker.test.ts`

**Interfaces:**
- Produces: theme store with `siteDefault`, `preview`, `isDirty`, `saveDefault`, `cancelPreview`

- [x] **Step 1: Write failing theme-store tests**

```ts
store.hydrate({ default_theme: 'ink', revision: 1 })
store.previewTheme('codex-lavender')
expect(document.documentElement.dataset.theme).toBe('codex-lavender')
expect(api.patchSite).not.toHaveBeenCalled()
await store.saveDefault()
expect(api.patchSite).toHaveBeenCalledWith({ default_theme: 'codex-lavender', revision: 1 })
```

Also test save failure retaining preview while site default remains unchanged.

- [x] **Step 2: Import shared CSS once**

Add `import '@alive/theme/themes.css'` in `admin/src/main.ts`. Remove palette literals from `admin/src/style.css`; keep spacing, geometry, typography scale, and admin-only semantic aliases there.

Add a development proxy for `/fonts/chillkai.woff2` in `admin/vite.config.ts`
using the same verified CDN source and cache behavior already documented in
`frontend/nuxt.config.ts`. In production, keep the absolute `/fonts/...` URL so
the existing site-level font route serves both `/` and `/admin`.

- [x] **Step 3: Implement API and store**

Create `getSiteSettings` and `updateSiteSettings`. The Pinia store writes `data-theme` for preview, stores the last server revision, and changes `siteDefault` only after PATCH succeeds.

- [x] **Step 4: Implement ThemePicker**

Open from the directory footer. Render the three theme names and representative swatches from CSS custom properties. Clicking previews. Render “设为站点默认” only when dirty, and “取消预览” to return to the current site default.

- [x] **Step 5: Run admin tests and build**

Run: `cd admin && npm test -- --run src/stores/theme.test.ts src/components/writing/ThemePicker.test.ts && npm run build`

Expected: PASS.

- [x] **Step 6: Commit**

```bash
git add admin/src admin/package*
git commit -m "feat: preview and save admin themes"
```

### Task 5: Resolve visitor theme over site default during SSR

**Files:**
- Create: `frontend/composables/useSiteSettings.ts`
- Modify: `frontend/composables/useTheme.ts`
- Modify: `frontend/components/ThemeToggle.vue`
- Modify: `frontend/layouts/default.vue`
- Modify: `frontend/assets/css/tokens.css`
- Modify: `frontend/nuxt.config.ts`
- Create: `frontend/composables/useTheme.test.ts`

**Interfaces:**
- Consumes: `GET /api/v1/site`, `@alive/theme`
- Produces: visitor cookie absent means site default; explicit visitor choice overrides it

- [x] **Step 1: Write failing precedence tests**

Cover SSR no-cookie plus site `codex-lavender`, visitor `lamp` plus site `ink`, invalid cookie, “跟随站点” clearing the cookie, and API failure falling back to `ink`.

- [x] **Step 2: Fetch settings once per SSR render**

```ts
export function useSiteSettings() {
  const api = useApi()
  return useAsyncData('site-settings', () => api.get<SiteSettings>('/site'), {
    default: () => ({ default_theme: 'ink', revision: 1, updated_at: '' }),
  })
}
```

- [x] **Step 3: Rewrite useTheme around the shared resolver**

`useTheme(siteDefault)` returns `theme`, `visitorTheme`, `setVisitorTheme`, and `applyTheme`. `setVisitorTheme(null)` deletes the cookie. `applyTheme` always writes the resolved valid theme into SSR `htmlAttrs`.

- [x] **Step 4: Replace the cycling button with an accessible menu**

The menu has four choices: 跟随站点, 纸墨, 灯下, Codex Lavender. Mark the resolved current theme and separately indicate when the choice follows the site.

- [x] **Step 5: Import shared CSS and remove duplicated palettes**

Add `@alive/theme/themes.css` before main/prose styles in `nuxt.config.ts`. Keep layout scale and component rules in frontend CSS, but remove theme color definitions now owned by the package.

Change prose and editor body rules to read `--c-prose`; headings continue to
read `--c-ink`. This is what lets Codex Lavender use the approved B prose purple
without making every UI label the same color.

- [x] **Step 6: Run frontend tests, typecheck, and SSR build**

Run:

```bash
cd frontend
npx vitest run composables/useTheme.test.ts
npm run typecheck
npm run build
```

Expected: PASS.

- [x] **Step 7: Commit**

```bash
git add frontend packages/theme
git commit -m "feat: apply site themes during SSR"
```

### Task 6: Theme visual and contrast checkpoint

**Files:**
- Modify: `packages/theme/src/themes.css` only for token corrections
- Modify: `docs/progress.md`

- [x] **Step 1: Run all changed-project checks**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npm run typecheck && npm run build
```

- [x] **Step 2: Verify the two-step save behavior**

Preview each theme in admin and confirm the frontend remains unchanged. Save Codex Lavender as default and confirm a fresh browser with no theme cookie receives it in SSR HTML. Set a visitor preference to lamp and confirm it survives site-default changes. Clear the preference and confirm it follows the site again.

- [x] **Step 3: Capture visual evidence**

For paper, lamp, and Codex Lavender, capture admin desktop, admin 375px, frontend article desktop, and frontend article 375px. Check adjacent surfaces, selected article state, focus ring, body prose, links, code blocks, quotes, table borders, image captions, and error text.

- [x] **Step 4: Measure contrast**

Record contrast ratios for body text, muted text, faint readable text, accent links, focus indicators, and danger text on their actual surfaces. All readable text must meet WCAG AA for its rendered size.

- [x] **Step 5: Update evidence and commit**

```bash
git add packages/theme/src/themes.css docs/progress.md
git commit -m "fix: verify shared theme contrast"
```
