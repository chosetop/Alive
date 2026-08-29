# Content World Foundation Task 3 Ledger

## 2026-08-29 Conflict Scan

- Existing `entry` domain still treats top-level content shape as legacy `type`, including validation, DTOs, fake store behavior, and route copy.
- Existing public entry routes are still mounted at `/api/v1/entries` and `/api/v1/entries/:slug`; no `/api/v1/journals` surface exists yet.
- Existing `taxonomy` domain keeps globally unique category slugs and does not require or carry a `world`.
- Existing category resolution for entry filters is global (`ResolveSlug(ctx, slug)`), which would silently cross worlds once duplicate slugs are allowed.
- Existing publish flow has no lifecycle gate; any valid public entry can publish regardless of the world's `open` or `unopened` state.
- Existing worktree is already isolated on branch `content-worlds-foundation`. Unrelated dirty state is limited to `frontend/package-lock.json`, which is out of scope and must remain untouched.

## Rulings

- Task 3 removes legacy `entry.Type` from active behavior and replaces it with immutable `Entry.World` plus read-only `Entry.Kind`.
- Unknown or missing worlds are input errors for entry/category create, admin list filters, and category list filters. They must never fall back to Journal.
- World-scoped category resolution is required before duplicate slugs across worlds are safe. `ResolveSlug` therefore takes `(world, slug)`.
- Public Journal reads move to `/api/v1/journals` and `/api/v1/journals/:slug`. The old public `GET /entries*` routes are removed; authenticated write routes remain under `/api/v1/entries`.
- Public publish is blocked when `visibility=public` and the selected world is `unopened`; `visibility=unlisted` is exempt and still publishable.
- Hidden worlds are publishable for direct access; the gate is `open || hidden`.

## 2026-08-29 Finish Pass

- Reproduced the inherited failure: `taxonomyhttp` tests referenced `contentworld` without importing it.
- Added the missing test import, then verified the targeted world/category/journal tests compiled and passed.
- Regenerated sqlc and ran gofmt after inspecting the query/router/service boundaries.
- Aligned the Entry repository/service/fake public read names to the plan's explicit world-slug contract: `GetPublicByWorldSlug` and `GetLinkByWorldSlug`.
- Fixed the entry fake store to enforce slug uniqueness by `(world, slug)` on both create and update. The previous fake still rejected duplicate slugs globally, which disagreed with migration `000010` and could hide cross-world behavior.
- Updated an old test seed to carry `World: contentworld.Journal` explicitly instead of relying on an empty-world fixture.
- Cleared stale comments that still referred to public `/entries?category=...` after the Journal route cutover.
- Verification:
  - `cd backend && go test ./internal/entry ./internal/entryhttp ./internal/taxonomy ./internal/taxonomyhttp ./internal/router -count=1`
  - `cd backend && TEST_DATABASE_URL='postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable' go test ./internal/postgres/sqlcgen -run 'World|CategoryFromAnotherWorld|Entry|Category' -count=1`
  - `cd backend && go test ./... -count=1`
