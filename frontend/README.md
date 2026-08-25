# Alive — Frontend

Public-facing site for Alive, a personal content record. Nuxt 3 with SSR.

This is stage one: project skeleton, API layer, auth infrastructure. The reader-facing
pages (entry list, entry detail, categories, Markdown rendering) come later.

## Stack

|               |                                      |
| ------------- | ------------------------------------ |
| Framework     | Nuxt 3 (SSR)                         |
| Language      | TypeScript, strict                   |
| State         | Pinia                                |
| Routing       | Nuxt file-based routing (Vue Router) |
| HTTP          | native `$fetch` — no Axios           |
| Lint / format | ESLint (`@nuxt/eslint`) + Prettier   |

No UI framework and no utility CSS by design; dependencies stay minimal until
there is a concrete need.

### Why Nuxt 3 and not 4

Nuxt 4 requires Node `^22.19.0 || ^24.11.0 || >=26`. This project is on Node
20.19.4, which Nuxt 3.21 supports exactly (`^20.19.0 || >=22.12.0`). Nothing in
stage one needs a Nuxt 4 feature, so the version that runs on the installed
runtime won.

Upgrading is a Node upgrade first. After that, Nuxt 4 moves the source root to
`app/` — `pages/`, `layouts/`, `composables/`, `stores/`, `plugins/`,
`components/`, `utils/`, `app.vue` and `error.vue` all move inside it, while
`nuxt.config.ts`, `public/` and `server/` stay put. The project tsconfig also
changes from extending `.nuxt/tsconfig.json` to referencing four generated
configs.

## Requirements

Node `^20.19.0 || >=22.12.0`, npm. Built and verified on Node 20.19.4 / npm 10.8.2.

## Install

```bash
npm install
```

### Two notes on installing

**Re-resolving from scratch needs npm 11.** npm 10.8.2 fails on a fresh
dependency resolve with `Cannot read properties of null (reading 'edgesOut')`,
a bug in its peer resolver. Installing from the committed lockfile works fine on
npm 10; only regenerating the lockfile needs a newer npm:

```bash
npx npm@11 install     # only when the lockfile must be rebuilt
```

**The `overrides` block is load-bearing.** Both entries work around packages
that claim Node 20 support but depend on packages requiring Node 22+:

| Pin                              | Reason                                                                                                                                                                                             |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `eslint-flat-config-utils@3.0.2` | 3.1.0+ calls `Object.groupBy` (Node 21+), which makes ESLint crash on startup. This is also why `@nuxt/eslint` is held at 1.15.2 — its `^3.0.1` range admits 3.0.2, while 1.16+ requires `^3.2.0`. |
| `eslint-plugin-regexp@3.1.1`     | 3.2.0 declares Node `^20.19.0` but depends on `jsdoc-type-pratt-parser@^9`, which needs Node 22+.                                                                                                  |

Both become unnecessary on Node 22+, at which point they can be dropped and
`@nuxt/eslint` can go back to latest.

One install warning is expected and harmless: `rollup-plugin-visualizer` wants
Node ≥22. It is a lazy import reached only by `nuxt analyze`, so dev and build
never load it. Pinning it to a 6.x release would break `nitropack`'s declared
`^7.0.1`.

## Develop

```bash
npm run dev      # http://localhost:3000
```

## Build

```bash
npm run build    # production build into .output/
npm run preview  # serve the build locally
```

## Checks

```bash
npm run lint       # ESLint
npm run typecheck  # vue-tsc via nuxt typecheck
npm run format     # Prettier, writes in place
```

## Environment

The backend origin is never hardcoded. It is read through `runtimeConfig`, so the
same build runs against any environment.

```bash
cp .env.example .env
```

| Variable               | Default                 | Notes                                         |
| ---------------------- | ----------------------- | --------------------------------------------- |
| `NUXT_PUBLIC_API_BASE` | `http://localhost:8080` | Origin only. No trailing slash, no `/api/v1`. |

Read it via `useRuntimeConfig().public.apiBase`, never `process.env`. In practice
you rarely need it directly — go through the API composables instead.

## Backend

Base path `/api/v1`, appended by the API client. With the default origin, a call
to `api.get('/entries')` requests `http://localhost:8080/api/v1/entries`.

`docs/api.md` in the repository root is the authority on backend behaviour. It
documents what is implemented and verified against a running service, not what is
planned. Where this code and that document disagree, the document is right.

### Authentication

The session is an **HttpOnly cookie**, not a Bearer token. Consequences:

- Every request sends `credentials: 'include'`. `useApi` does this for you.
- During SSR there is no cookie jar, so the incoming `cookie` header is forwarded
  explicitly. Without that, a signed-in visitor renders as signed out and flips
  after hydration.
- Nothing is stored in `localStorage` or `sessionStorage`. The token is not
  readable from JavaScript and never appears in a response body. The auth store
  holds the current user only.
- The backend must list this origin in `CORS_ALLOWED_ORIGINS`; `*` is not valid
  for a credentialed API.

Running over plain HTTP locally, the backend needs `SESSION_COOKIE_SECURE=false`,
otherwise the cookie is never sent and login appears to silently fail.

## Layout

```text
frontend/
├── assets/css/         base stylesheet
├── composables/
│   ├── useApi.ts            HTTP client: base URL, cookies, error normalising
│   ├── useEntriesApi.ts     entry endpoints
│   └── useCategoriesApi.ts  category endpoints
├── layouts/default.vue
├── pages/index.vue
├── plugins/            session resolution (server + client fallback)
├── stores/auth.ts
├── types/              API contract types
├── utils/api-error.ts  ApiError
├── error.vue           404 and uncaught render errors
└── nuxt.config.ts
```

## Working with the API layer

Call the endpoint composables; don't reach for `$fetch` in a component. That
keeps cookie handling, the version prefix and error normalising in one place.

```ts
const entries = useEntriesApi()
const { data, meta } = await entries.list({ page: 1, category: 'travel' })
```

Failures reject with `ApiError`, carrying `code`, `status`, `fields` and
`request_id`.

```ts
import { ApiError } from '~/utils/api-error'

try {
  await auth.login({ username, password })
} catch (error) {
  if (error instanceof ApiError && error.isInvalidCredentials) {
    // wrong username or password
  }
}
```

Three things to keep in mind, each of which is easy to get wrong without an
immediate symptom:

**Branch on `code`, never on `message`.** Messages are for humans and may change
between releases; codes are stable once published.

**`INVALID_INPUT` is not the same as HTTP 400.** It also covers 405 (unsupported
method). Use `error.status` when the difference matters — `isValidationError` and
`isMethodNotAllowed` already do.

**On PATCH, an empty value clears a field and `null` does not.** Omitted or
`null` means "leave alone"; `''` or `0` means "clear". So never expand
`undefined` into `null`, and never send a full field set just to make a body look
complete. An empty body, or one containing only nulls, is a 400.

Publishing has dedicated endpoints. `PATCH` rejects `status` outright, so use
`publish()`, `unpublish()` and `archive()`.

Addressing differs by audience and is not interchangeable: **public reads take a
slug, writes and admin reads take an id.** A reader identifies an entry by its
URL, while an editor may be changing the slug itself.
