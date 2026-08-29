# Videos World Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add owner-published Videos with one primary uploaded video, cover-led browsing, a focused authoring flow, and accessible playback.

**Architecture:** Videos remain Entries with `world=video`. The existing Aliyun OSS media subsystem is extended to accept pre-encoded MP4 objects, while `entry_media.role` identifies exactly one primary video per Entry. Go validates Video meta and publication readiness. Vue composes direct upload, title, cover, and short Markdown description. Nuxt uses poster-led cards and the native HTML video player.

**Tech Stack:** Go 1.25, Gin, PostgreSQL 18/sqlc, Alibaba Cloud OSS Go SDK V2, Vue 3.5, Nuxt 3.21.11, native HTML5 video, Vitest 4.1.

**Spec:** `docs/superpowers/specs/2026-08-29-content-worlds-design.md`

## Global Constraints

- Complete Foundation, Sayings, Tags, and `2026-08-26-media-upload.md` first.
- Media migration is `000012`; this plan owns `000013_video_media_role`.
- “影像” means videos published or organized by the owner, not watched films.
- One Entry has exactly one primary video at publication. Playlists, episodes, and multiple angles are out of scope.
- Accept pre-encoded `video/mp4` only, maximum 500 MiB. Do not add transcoding, adaptive bitrate, HLS/DASH, background jobs, or server-proxied bytes.
- The player never autoplays. Use `controls`, `playsinline`, `preload="metadata"`, and a poster.
- Cover is required for first publication. This plan does not implement automatic frame extraction.
- Duration, captured time, and place are optional. Never infer captured time from upload time.
- Description is short Markdown and uses the existing safe renderer.
- Video upload failure must not block text autosave.
- The Video world is not opened silently; lifecycle rules from Foundation apply.

---

## File structure

- `backend/migrations/000013_video_media_role.*.sql`: media role constraint.
- `backend/internal/media`: video MIME, role attachment, and authorization.
- `backend/internal/entry/video.go`: Video meta and publication policy.
- `backend/internal/entryhttp`: `/videos` public adapters.
- `admin/src/views/VideoEditor.vue`: upload-first authoring.
- `frontend/components/videos`: cards, list, player, and detail.
- `frontend/pages/videos`: list, Category, and detail routes.

### Task 1: Extend media storage for primary MP4 video

**Files:**
- Create: `backend/migrations/000013_video_media_role.up.sql`
- Create: `backend/migrations/000013_video_media_role.down.sql`
- Modify: `backend/sql/queries/media.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/media/model.go`
- Modify: `backend/internal/media/service.go`
- Modify: `backend/internal/media/repository.go`
- Modify: `backend/internal/media/mediatest/store.go`
- Test: `backend/internal/media/model_test.go`
- Test: `backend/internal/media/service_test.go`
- Test: `backend/internal/media/repository_test.go`
- Modify: `backend/internal/mediahttp/dto.go`
- Test: `backend/internal/mediahttp/handler_test.go`
- Modify: `backend/internal/config/config.go`
- Modify: `backend/.env.example`

**Interfaces:**
- Produces: `media.RoleBodyImage`, `media.RoleCover`, `media.RolePrimaryVideo`
- Produces: `Attach(ctx, entryID, mediaID, role, expectedRevision)`
- Extends: presign/register to `video/mp4` with a 500 MiB video limit

- [ ] **Step 1: Write failing media tests**

Cover MP4 boundary sizes, image limits remaining 20 MiB, rejected WebM/MOV/MPEG, HeadObject MIME/size verification, non-Video primary attachment rejection, second-primary replacement, stale Entry revision, and rollback.

- [ ] **Step 2: Add migration `000013`**

```sql
ALTER TABLE entry_media
  ADD COLUMN role VARCHAR(32) NOT NULL DEFAULT 'body_image',
  ADD CONSTRAINT entry_media_role_check
    CHECK (role IN ('body_image', 'cover', 'primary_video'));
CREATE UNIQUE INDEX uk_entry_media_primary_video
  ON entry_media (entry_id) WHERE role = 'primary_video';
CREATE UNIQUE INDEX uk_entry_media_cover
  ON entry_media (entry_id) WHERE role = 'cover';
```

The down migration drops both partial indexes, the check, and the role column. Existing image associations become `body_image`.

- [ ] **Step 3: Split upload policy by media class**

Keep current image allowlist and 20 MiB maximum. Add exact `video/mp4` with `VIDEO_MAX_UPLOAD_BYTES`, default 524288000, validated within 1..1073741824. Object keys use `video/<author>/<entry>/<yyyy>/<mm>/<random>.mp4`. Never trust the filename extension; sign and register against MIME and verify HeadObject.

- [ ] **Step 4: Attach roles with Entry revision CAS**

`Attach` verifies ownership, Entry world, media MIME, and role compatibility. Replacing a primary video or cover happens transactionally: remove the prior row for that role, insert the selected media relation, increment Entry revision once, and return updated media/revision. Images cannot become primary video; MP4 cannot become body image or cover.

Extend video registration with `role: "primary_video"` and `revision`; after HeadObject verification it calls the same attachment transaction and returns the new Entry revision. Image registration remains compatible with the Media plan and attaches as `body_image`; choosing a Video cover explicitly calls `Attach(..., RoleCover, revision)`.

- [ ] **Step 5: Verify and commit**

```bash
cd backend
make test-db-create
make sqlc
TEST_DATABASE_URL='postgres://alive:alive@127.0.0.1:5432/alive_test?sslmode=disable' \
  go test ./internal/media ./internal/mediahttp -run 'Video|Role|Upload' -count=1
git add backend/migrations/000013_* backend/sql/queries/media.sql backend/internal/postgres/sqlcgen backend/internal/media backend/internal/mediahttp backend/internal/config backend/.env.example
git commit -m "feat: support primary video media"
```

### Task 2: Add Video domain and public APIs

**Files:**
- Create: `backend/internal/entry/video.go`
- Test: `backend/internal/entry/video_test.go`
- Modify: `backend/internal/entry/model.go`
- Modify: `backend/internal/entry/service.go`
- Test: `backend/internal/entry/service_test.go`
- Modify: `backend/sql/queries/entry.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Modify: `backend/internal/entry/repository.go`
- Test: `backend/internal/entry/repository_test.go`
- Modify: `backend/internal/entryhttp/dto.go`
- Modify: `backend/internal/entryhttp/handler.go`
- Test: `backend/internal/entryhttp/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `VideoMeta` and `ValidateVideoForPublish`
- Produces: `GET /api/v1/videos`, `GET /api/v1/videos/:slug`
- Produces: Video list/detail responses with primary media and tags

- [ ] **Step 1: Write failing domain and API tests**

Cover required title/slug/cover/primary MP4, optional description, duration bounds, captured time, place length, Category scope, public/unlisted visibility, hidden direct reads, lifecycle publish gate, and no film-specific metadata.

- [ ] **Step 2: Define exact Video meta**

```go
type VideoMeta struct {
	DurationSeconds *int    `json:"duration_seconds,omitempty"`
	CapturedAt      *string `json:"captured_at,omitempty"`
	Place           string  `json:"place,omitempty"`
}
```

Reject unknown fields. Duration is 1..86400 seconds when present. `captured_at` is RFC3339 with an offset. Place is at most 200 runes. Store description in `content_md`; title, slug, cover, Category, and tags use shared fields.

- [ ] **Step 3: Dispatch Video publication checks**

Publication requires non-empty title, valid slug, registered cover image attached with role `cover`, and registered `video/mp4` attached as `primary_video`. The service reads readiness in one repository call after the revision check. Unlisted publication is allowed while unopened; public publication follows the world gate.

- [ ] **Step 4: Add world-pinned queries and DTOs**

`/videos` returns slug, title, cover, duration, place, Category, tags, and an optional excerpt; it does not return the MP4 URL. `/videos/:slug` returns those fields plus description and primary media URL/MIME. List is public-only; detail accepts public or unlisted. Category filtering resolves only Video Categories.

- [ ] **Step 5: Regenerate, verify, document, and commit**

```bash
cd backend
make sqlc
go test ./internal/entry ./internal/entryhttp ./internal/router -run 'Video' -count=1
git add backend/internal/entry/video.go backend/internal/entry/video_test.go backend/internal/entry backend/sql/queries/entry.sql backend/internal/postgres/sqlcgen backend/internal/entryhttp backend/internal/router docs/api.md
git commit -m "feat: expose the videos world"
```

### Task 3: Build upload-first Video authoring

**Files:**
- Modify: `admin/src/content-worlds/registry.ts`
- Test: `admin/src/content-worlds/registry.test.ts`
- Modify: `admin/src/types/api.ts`
- Modify: `admin/src/api/media.ts`
- Create: `admin/src/components/video/PrimaryVideoUpload.vue`
- Test: `admin/src/components/video/PrimaryVideoUpload.test.ts`
- Create: `admin/src/views/VideoEditor.vue`
- Test: `admin/src/views/VideoEditor.test.ts`
- Modify: `admin/src/router/index.ts`
- Modify: `admin/src/views/NewEntry.vue`
- Modify: `admin/src/components/writing/ArticleDirectory.vue`
- Test: affected route, creation, and directory tests

**Interfaces:**
- Produces: Admin route `/entries/new/video`
- Produces: primary video upload/replace and Video publish checks
- Consumes: OSS upload coordinator, cover pipeline, revision CAS

- [ ] **Step 1: Write failing authoring tests**

Assert creation sends `{ world: 'video' }`, upload is first focus, only MP4 is accepted, progress/retry survive text autosave, title/cover/primary video gate publication, optional meta clears correctly, replacement updates revision, and leaving the route warns only for genuinely pending local changes/uploads.

- [ ] **Step 2: Extend the upload coordinator without duplicating it**

Parameterize the existing coordinator with `mediaClass: 'image' | 'video'`, accepted MIME, and maximum bytes. Preserve its IndexedDB recovery state and direct PUT behavior. Store media class in pending upload records and bump the IndexedDB schema version once.

- [ ] **Step 3: Register and build the dedicated editor**

Set `editorRouteName: 'video-editor-new'` and `createLabel: '发影像'`. Layout order is primary video, title, cover, short description, then optional duration/captured time/place and settings. Reuse shared Category, Tag, visibility, autosave, recovery, and publish primitives; do not embed Video controls in Journal's editor.

- [ ] **Step 4: Add directory behavior**

Append `影像` after `片语`. Rows show cover thumbnail, title, draft/published state, and upload-in-progress state. Category requests use `world=video`.

- [ ] **Step 5: Verify and commit**

```bash
cd admin
npm test -- --run src/media src/components/video src/views/VideoEditor.test.ts src/content-worlds src/components/writing/ArticleDirectory.test.ts
npm run build
git add admin/src
git commit -m "feat: add video authoring"
```

### Task 4: Build Video browsing and playback

**Files:**
- Create: `frontend/types/video.ts`
- Modify: `frontend/types/index.ts`
- Create: `frontend/composables/useVideosApi.ts`
- Create: `frontend/components/videos/VideoCard.vue`
- Create: `frontend/components/videos/VideoList.vue`
- Create: `frontend/components/videos/VideoPlayer.vue`
- Create: `frontend/components/videos/VideoDetail.vue`
- Test: corresponding component tests
- Create: `frontend/pages/videos/index.vue`
- Create: `frontend/pages/videos/[slug].vue`
- Create: `frontend/pages/videos/categories/[slug].vue`
- Modify: `frontend/content-worlds/registry.ts`
- Test: `frontend/content-worlds/registry.test.ts`
- Create: `frontend/components/home/VideosHomePreview.vue`
- Test: `frontend/components/home/VideosHomePreview.test.ts`
- Modify: `frontend/content-worlds/sitemap.ts`
- Test: `frontend/content-worlds/sitemap.test.ts`
- Modify: `frontend/assets/css/main.css`

**Interfaces:**
- Produces: `/videos`, `/videos/<slug>`, `/videos/categories/<slug>`
- Produces: Video mixed-tag adapter and homepage preview
- Consumes: public Video API

- [ ] **Step 1: Write failing card and player tests**

Assert cover alt text, duration formatting, Category/tag links, no MP4 download on list cards, native player attributes, poster, keyboard operability, missing-media failure state, reduced motion, and a two-to-three-card home preview.

- [ ] **Step 2: Build poster-led list and Category pages**

Use responsive cards or horizontal rows with cover, title, optional duration/place, and a clear link. Keep MP4 URLs out of list markup. Category page uses the same list adapter and world-scoped URL.

- [ ] **Step 3: Build accessible playback**

Render `<video controls playsinline preload="metadata" :poster="coverURL">` with one MP4 source and fallback text/link. Never autoplay or loop by default. Provide a visible failure message and direct media link if native playback errors. Reserve poster aspect ratio to avoid layout shift.

- [ ] **Step 4: Build detail and structured data**

Player is primary; title, short description, optional captured time/place, Category, and tags are secondary. Add canonical URL and `VideoObject` JSON-LD with content URL, thumbnail, name, optional duration, and upload date retained only in metadata where required—not as a fabricated captured date.

- [ ] **Step 5: Register Video adapters**

Register root/detail/Category helpers, homepage preview, and mixed-tag card. Preview requests three items and renders two or three cover cards after Sayings in fixed order.

Append a Video sitemap source for `/videos`, open Video Category paths, and public detail paths. The shared generator invokes it only while Video is open and never emits MP4 object URLs.

- [ ] **Step 6: Verify and commit**

```bash
cd frontend
npx vitest run
npm run typecheck
npm run build
git add frontend
git commit -m "feat: add video browsing and playback"
```

At 1280×900 and 375×812 verify list, detail, Category, tag card, home preview, player keyboard controls, failed playback, poster layout, captions capability, and no overflow.

### Task 5: OSS and lifecycle acceptance

**Files:**
- Modify: `docs/api.md`
- Modify: `docs/architecture.md`
- Modify: `docs/progress.md`
- Modify only defects found in Video-owned files

- [ ] **Step 1: Run all changed-project gates**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npx vitest run && npm run typecheck && npm run build
```

- [ ] **Step 2: Verify real direct uploads**

Using a non-production OSS prefix, upload a small MP4, reject WebM/MOV and over-limit MP4, interrupt/retry a PUT, refresh during pending upload, replace the primary video, and prove binary bytes go from browser to OSS rather than through Go.

- [ ] **Step 3: Verify publication and lifecycle**

Confirm missing title/video/cover block publication with field errors. Publish unlisted while unopened, explicitly open on public publish, hide and reopen the world, and verify direct URLs and Entry publication states remain unchanged.

- [ ] **Step 4: Verify public behavior**

Check list pages never request MP4, detail uses metadata preload only, poster displays before play, keyboard controls work, failure fallback is readable, homepage order remains Journal/Sayings/Videos, and tag pages use the Video adapter.

- [ ] **Step 5: Update evidence and commit**

```bash
git add docs/api.md docs/architecture.md docs/progress.md
git commit -m "docs: verify the videos world"
```
