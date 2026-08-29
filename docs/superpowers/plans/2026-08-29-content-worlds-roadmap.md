# Alive Content Worlds Implementation Roadmap

> **For agentic workers:** Execute the linked plans in order. Each plan has its own test and review gate. Do not start a later plan while an earlier plan is failing.

**Goal:** Deliver the approved Journal, Sayings, and Videos worlds through independently shippable stages without weakening the existing immersive journal editor.

**Spec:** `docs/superpowers/specs/2026-08-29-content-worlds-design.md`

## Execution order

1. [Content world foundation and Journal cutover](2026-08-29-content-world-foundation.md)
2. [Sayings world](2026-08-29-sayings-world.md)
3. [Cross-world tags](2026-08-29-cross-world-tags.md)
4. [Aliyun OSS media upload](2026-08-26-media-upload.md)
5. [Videos world](2026-08-29-videos-world.md)

## Cross-plan contracts

- Foundation produces `contentworld.Key`, immutable `Entry.World`, optional `Entry.Kind`, world-scoped categories, `site_worlds`, `/api/v1/worlds`, `/api/v1/admin/worlds`, Journal public APIs, and the frontend/admin registries.
- Sayings consumes the registries and produces generated stable short IDs, Saying validation and meta, `/api/v1/sayings`, the minimal authoring surface, three visitor view modes, and the Sayings home preview.
- Tags consumes immutable Entry worlds and produces one cross-world tag vocabulary, revision-aware Entry tag replacement, mixed public tag pages, and tag pickers shared by every editor.
- Media upload is the existing direct-to-OSS image foundation. In this roadmap its migration number is `000012`, after content foundation `000010` and tags `000011`.
- Videos consumes content worlds, tags, and media. It extends media with an Entry-owned primary MP4 role, then produces `/api/v1/videos`, Video authoring, playback, category pages, and the Videos home preview.
- Every world registers its own admin editor, homepage preview, public list, detail route, publish policy, and empty state. No component switches on an unknown string and silently renders Journal.

## Phase independence

- After Plan 1, Alive is a complete Journal-only site at `/journal`, with a mixed-home shell that currently contains only Journal.
- After Plan 2, Sayings can be opened and used without Tags, OSS, or Videos.
- After Plan 3, Journal and Sayings gain cross-world discovery; Videos is not required.
- After Plan 4, image upload and current-entry image reuse work independently of Videos.
- After Plan 5, Videos can be opened. If it is never opened, every earlier phase remains useful.

## Global verification gate

Run only the projects changed by the completed plan:

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npx vitest run && npm run typecheck && npm run build
```

Use Node 22.17.0 or newer for Admin tests because the current jsdom/undici dependency chain does not start workers under Node 20.19.4. Before every commit or push, re-read `git status --short --branch -uall` and preserve unrelated work.

For plans that change rendering, complete a browser checkpoint at 1280×900 and 375×812. Verify keyboard operation, touch targets, focus restoration, reduced motion, and horizontal overflow.

## Spec coverage

| Design requirement | Owning plan |
|---|---|
| World registry, immutable world, optional kind | Foundation |
| World-scoped categories and fixed public namespaces | Foundation |
| unopened/open/hidden lifecycle and fixed navigation order | Foundation |
| Lifecycle-aware sitemap with world-specific sources | Foundation, Sayings, Videos |
| Journal immersive editor and `/journal` cutover | Foundation |
| Mixed homepage registry and Journal preview | Foundation |
| Titleless Sayings, hidden time, stable short IDs | Sayings |
| Stream, wall, and focus Sayings views | Sayings |
| Site default versus visitor local view preference | Sayings |
| Cross-world tags and mixed tag pages | Tags |
| Direct OSS upload, image reuse, cover selection | Media upload |
| One primary video, cover, duration, place, player | Videos |
| Unknown world fails explicitly | Foundation and every world plan |
| No arbitrary world creation or cross-world conversion | Foundation |
| No legacy URL or test-content migration | Foundation |
