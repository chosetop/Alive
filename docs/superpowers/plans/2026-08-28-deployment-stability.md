# Deployment and Stability Closeout Implementation Plan

**Goal:** Prepare the current Alive writing and reading workflow for ordinary sharing with friends. The site remains a normal personal website: no SEO work, visitor accounts, invite codes, site password, or special access-validation layer.

**Architecture:** Keep the existing public frontend, authenticated admin, Go API, and PostgreSQL boundaries. Make production deployment explicit and repeatable, close the backend issues already identified before deployment, document backup/restore operations, and finish with a real release smoke test. Do not introduce media storage, tags, or a new visitor identity system in this plan.

## Scope and non-goals

- In scope: deployment topology, production configuration, trusted proxy handling, session absolute expiry, health checks, logs, database backup/restore verification, and release smoke testing.
- In scope: ordinary public article sharing using the existing visibility semantics.
- Out of scope: SEO, `sitemap.xml`, `feed.xml`, visitor login, invitation links, site-wide password, media upload/OSS, tags, batch content management, and visual redesign.
- Keep the existing `public`, `unlisted`, and `private` behavior; do not invent additional access rules.

## Task 1: Capture the current release baseline

- Read `docs/progress.md`, `docs/architecture.md`, and `docs/api.md`.
- Confirm the current `main` checkout, clean/known worktree state, service ports, migration version, and the retained example articles.
- Record the exact commands and expected URLs used by the release smoke test.
- Do not modify user data or rotate credentials as part of the baseline.

## Task 2: Close deployment-blocking backend hardening

- Add an absolute session expiry while preserving the existing sliding expiry behavior within that upper bound.
- Configure `trusted proxy` from explicit production configuration so rate limiting uses the real client IP only when requests come from configured proxy addresses.
- Preserve safe defaults for local development and fail clearly on invalid production configuration.
- Add focused tests for absolute expiry, renewal at the boundary, proxy configuration, and rate-limit client IP behavior.

## Task 3: Document the simple production topology

- Document the intended reverse-proxy arrangement: public frontend, `/admin` static assets, API routing, HTTPS termination, and PostgreSQL on a private network.
- Document required environment variables without storing passwords, AccessKeys, or tokens.
- Document startup ordering, `/health`, `/health/ready`, migration execution, log locations, and rollback expectations.
- Keep the local three-service workflow documented and working.

## Task 4: Add and verify the database recovery runbook

- Document a PostgreSQL backup command, retention recommendation, restore-to-isolated-database procedure, and post-restore migration/version check.
- Run a restore verification against an isolated database or temporary local database; never overwrite the development database.
- Record the result in `docs/progress.md`.

## Task 5: Release smoke test and documentation checkpoint

- Verify login, admin article editing/autosave, publishing, frontend reading, ordinary public sharing, and the existing private/unlisted visibility behavior.
- Verify service health, API error handling, and no horizontal overflow at desktop and 375px widths for changed rendering.
- Run the project gate for changed projects:

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npm run typecheck && npm run build
```

- Update `docs/progress.md` with commands, versions, results, and any known non-blocking warnings.

## Acceptance criteria

- A fresh operator can understand how to start and deploy Alive without guessing the service topology.
- A proxy deployment does not collapse all clients into one rate-limit bucket.
- A stolen session has a finite lifetime even if it is continuously renewed.
- A database backup can be restored and checked without touching the development database.
- Friends can open ordinary published article links without a new login or verification flow.
- OSS remains deferred to Plan 6 and is not required for this release.
