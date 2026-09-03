# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Alive is a single-owner personal content site: a Go API, a Nuxt 3 SSR public site, a Vue 3 SPA admin, and two shared local packages. No registration, no RBAC — auth only ever answers "is this the site owner".

Docs are in Chinese and are the design of record:

- `docs/architecture.md` — design intent and the ten binding decisions
- `docs/api.md` — current implemented fields, status codes, edge behaviour. **On conflict with architecture.md, this one wins** (it was verified against a running service).
- `docs/progress.md` — stage-by-stage build status
- `docs/superpowers/{plans,specs,ledgers}/` — per-feature plans and designs

## Commands

Backend (`backend/`, `make help` lists all):

```bash
make run                  # go run ./cmd/server
make check                # fmt + vet + test  (run before finishing backend work)
make test                 # go test -race ./...   (DB tests skip)
make test-db-create       # create + migrate alive_test, once
make test-integration     # same tests with TEST_DATABASE_URL set
make migrate-create name=add_x
make migrate-up
make sqlc                 # regenerate internal/postgres/sqlcgen from sql/queries
make user-create name=owner
```

Single Go test: `go test -race ./internal/entry -run TestName`. Integration variant needs `TEST_DATABASE_URL="postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable"` in front.

Frontend (`frontend/`, Nuxt, port 3000):

```bash
npm run dev
npm run lint            # eslint; lint:fix, format also exist
npm run typecheck
npx vitest run          # no test script in package.json
npx vitest run utils/entry-navigation.test.ts
```

Admin (`admin/`, Vite, port 5173):

```bash
npm run dev
npm run build           # vue-tsc -b && vite build — this is the typecheck
npm run test:run
npm run test:run -- src/views/Entries.test.ts
```

Shared packages: `cd packages/markdown && npm test` (and `packages/theme`).

Whole stack in Docker: `cp .env.docker.example .env`, set `ALIVE_DB_PASSWORD`, then `docker compose up -d --build`. Gateway on `127.0.0.1:8081` serves the frontend at `/`, the admin build at `/admin`, and proxies `/api`. See `docs/docker-quick-start.md`.

## Backend architecture

Go 1.25, Gin, pgx/v5, sqlc. Module `github.com/p30huiwei/alive/backend`.

Every domain follows the same four-layer split, and the naming tells you which layer a file is:

```
internal/<domain>/           model.go  repository.go  service.go   <domain>test/store.go
internal/<domain>http/       handler.go  dto.go
```

- `model.go` — domain types, validation, sentinel errors (`ErrCategoryNotFound`, …)
- `repository.go` — **the only file in the package that imports pgx**. Wraps `sqlcgen`, translates driver errors into the model's sentinels. Layers above see domain types only.
- `service.go` — business rules. Declares a `Store` interface *next to the code that calls it*, so a method exists on that interface only because the service uses it. `<domain>test/store.go` is the fake used to test the service without a database.
- `<domain>http` — HTTP adapter: DTOs, status mapping. Adapters never import each other.

Domains: `auth`, `entry`, `taxonomy` (categories + tags), `contentworld`, `media`, `site`. Plus `apperr`, `httpx`, `middleware`, `config`, `postgres`, `storage` (+ `storage/aliyunoss`), `dbtest`, `health`.

`internal/router/router.go` is the single registry of every URL — handlers do not self-register from `init`. Read that one file to know the full surface; add new routes there. It panics on a missing dependency at startup rather than serving broken traffic. Cross-domain wiring goes through function values passed in from the router (`authorFromSession`, `categoryFromSlug`) so no two adapters depend on each other.

### Data model

One `entries` table with `type` + a JSONB `meta` column — book/film/travel/etc. get **no** separate tables. Classification is data, not schema. `jsonb` is mapped to `json.RawMessage` in `sqlc.yaml` so the owning domain decodes its own shape.

Two orthogonal axes, deliberately not merged: `Status` (`draft`/`published`/`archived`) and `Visibility` (`public`/`private`/`unlisted`). Public list reads require published+public; link reads also accept `unlisted`.

Markdown is stored as source only — no `content_html` in the database; the frontend renders it. Images live in object storage (Aliyun OSS) with only the key in the database.

Mutations on entries, site settings, and worlds use revision CAS: the client sends the expected revision, a mismatch is a conflict.

Schema lives in `migrations/` (golang-migrate, `brew install golang-migrate`), and sqlc reads the schema from those same files, so there is one definition. Queries in `sql/queries/*.sql` → generated into `internal/postgres/sqlcgen/`. Never hand-edit `sqlcgen`.

### Testing

`go test ./...` must pass on a machine with no PostgreSQL: `dbtest.DSN` skips when `TEST_DATABASE_URL` is unset. Integration tests refuse any database whose name does not end in `_test`. Packages run in parallel, so a test **names its rows with a unique prefix and deletes only those** — never truncate a shared table (that bug cost five failures in eight runs).

## Content worlds

The organising concept across all three apps. A world (`journal`, `saying`, `video`) is a closed registry with per-world status (`unopened`/`open`/`hidden`), nav label, sort order, view mode (`stream`/`wall`/`focus`), and media capability.

Three registries must stay in step when a world changes:

- `backend/internal/contentworld/model.go` — keys, statuses, view modes, capabilities
- `frontend/content-worlds/registry.ts` — public paths, time policy, schema.org type
- `admin/src/content-worlds/registry.ts` — admin-side definitions

## Frontend (`frontend/`)

Nuxt 3 SSR (not 4 — pinned to what the installed Node runs; see `frontend/README.md`), Pinia, native `$fetch`, no UI framework and no utility CSS by design.

- `components` is configured with `pathPrefix: false`. Domain folders exist but component names stay short (`SayingStream`, not `SayingsSayingStream`) — the prefixed names silently render empty world pages after hydration.
- `utils/api-base.ts` resolves the API origin per environment; `NUXT_API_INTERNAL_BASE` is SSR-only and never reaches the browser bundle.
- `assets/css/prose.css` is global on purpose: scoped styles cannot reach `v-html` output.
- The ChillKai webfont is proxied through a Nitro route rule because its CDN sends no CORS header and `@font-face` fetches are always CORS-mode.
- `server/routes/sitemap.xml.ts` — sitemap and feed sit at the root, no version prefix.

## Admin (`admin/`)

Vue 3 + Vite SPA, Pinia, vue-router, Milkdown editor, reka-ui, GSAP.

Two shells share the `/entries` prefix: `AdminLayout` (utility chrome) and `WritingLayout` (immersive writing canvas), distinguished by `meta.writingWorkspace`. **The `AdminLayout` record must stay declared before the `WritingLayout` record** — the paths score identically and Vue Router breaks the tie by declaration order; swapping them renders an empty canvas where the article library belongs. `src/router/routes.test.ts` pins this.

`src/api/client.ts` refuses to load without `VITE_API_BASE_URL`, by design.

## Local dev config

Copy each `.env.example` to `.env`. Running the three processes separately (rather than via Docker) needs, on the backend: `CORS_ALLOWED_ORIGINS` listing the dev origins, and `SESSION_COOKIE_SECURE=false` — `http://localhost` never stores a `Secure` cookie, so login appears to succeed while no session is ever set. Production is same-origin behind the gateway: no CORS, `SameSite=Strict`.

`VITE_API_BASE_URL` / `NUXT_PUBLIC_API_BASE` take an origin with no trailing slash and **no** `/api/v1` suffix — the clients add the version prefix.

## Conventions

Comments in this codebase explain *why*, often at length, and frequently record a bug that a given line prevents. Match that: when a choice is non-obvious or was forced by a real failure, say so in the comment. Don't add comments that restate the code.

