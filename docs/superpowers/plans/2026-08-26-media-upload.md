# Aliyun OSS Media Upload Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe direct-to-Aliyun-OSS image upload, current-article reuse, cover selection, and resilient Milkdown image insertion without building a full media library.

**Architecture:** The backend signs short-lived OSS PutObject requests through a storage interface, but never receives image bytes. After upload the admin registers metadata; the service verifies the object with HeadObject and records it in PostgreSQL with an article association. The admin upload coordinator stores the File blob in IndexedDB, reports XMLHttpRequest progress, retries from a new presign, and inserts a standard Markdown image only after registration succeeds.

**Tech Stack:** Go 1.25, Alibaba Cloud OSS Go SDK V2 v1.6.0, PostgreSQL/sqlc/Gin, Vue 3, Milkdown 7.22.1 upload and image-block components, native IndexedDB, XMLHttpRequest.

**Spec:** `docs/superpowers/specs/2026-08-26-admin-writing-experience-design.md`

## Global Constraints

- Plans 1 through 3 are complete.
- Alibaba Cloud OSS is the storage provider; preserve a domain-owned storage interface.
- Image bytes go directly from the browser to OSS and never through the Go API.
- Accepted upload types are JPEG, PNG, WebP, and GIF. SVG is rejected because it can contain executable content when served inline.
- Maximum image size is 20 MiB.
- Presigned URLs expire after 10 minutes.
- Upload failure is local to the image and never blocks text autosave.
- Cover upload uses the same pipeline as body images.
- Reuse is limited to media already attached to the current article.
- First iteration does not add global search, deduplication, delete UI, orphan cleanup automation, or batch management.
- Markdown remains the only body representation. Alt text uses image alt, caption uses image title, and wide display uses a reserved URL fragment produced by a shared helper.

---

## File structure

- `backend/internal/storage`: provider-neutral signing and object verification.
- `backend/internal/storage/aliyunoss`: SDK adapter only.
- `backend/internal/media`: metadata domain and persistence.
- `backend/internal/mediahttp`: authenticated media routes.
- `admin/src/media/upload-coordinator.ts`: presign, XHR, registration, retry.
- `admin/src/media/upload-recovery.ts`: IndexedDB pending File blobs.
- `admin/src/components/writing/MediaUpload.vue`: picker/drop/paste progress UI.
- `admin/src/components/writing/ArticleMediaPicker.vue`: current-entry reuse.
- `admin/src/editor/image-display.ts`: reserved display fragment helper.
- `packages/markdown`: figure, caption, and wide-image rendering.

### Task 1: Add OSS configuration and storage abstraction

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/.env.example`
- Create: `backend/internal/storage/storage.go`
- Create: `backend/internal/storage/aliyunoss/client.go`
- Test: `backend/internal/storage/aliyunoss/client_test.go`
- Modify: `backend/go.mod`, `backend/go.sum`

**Interfaces:**
- Produces: `storage.Client.PresignPut` and `storage.Client.Head`
- Produces: `config.OSSConfig`

- [ ] **Step 1: Write failing config tests**

Cover development with OSS disabled, enabled config requiring region/bucket/credentials/public URL, invalid public URL, invalid TTL, and max bytes outside 1..104857600.

```go
setenv(t, "OSS_ENABLED", "true")
setenv(t, "OSS_REGION", "cn-hangzhou")
setenv(t, "OSS_BUCKET", "alive-media")
setenv(t, "OSS_ACCESS_KEY_ID", "test-id")
setenv(t, "OSS_ACCESS_KEY_SECRET", "test-secret")
setenv(t, "OSS_PUBLIC_BASE_URL", "https://media.example.com")
cfg, err := config.Load()
if err != nil || cfg.OSS.PresignTTL != 10*time.Minute { t.Fatalf("cfg = %+v, err = %v", cfg, err) }
```

- [ ] **Step 2: Add exact configuration fields**

```go
type OSSConfig struct {
	Enabled         bool
	Region          string
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	AccessKeySecret string
	PublicBaseURL   string
	PresignTTL      time.Duration
	MaxUploadBytes  int64
}
```

Environment names are `OSS_ENABLED`, `OSS_REGION`, `OSS_ENDPOINT`, `OSS_BUCKET`, `OSS_ACCESS_KEY_ID`, `OSS_ACCESS_KEY_SECRET`, `OSS_PUBLIC_BASE_URL`, `OSS_PRESIGN_TTL`, and `MEDIA_MAX_UPLOAD_BYTES`. Never print the access key or secret in config reports.

- [ ] **Step 3: Define the storage interface and fakeable values**

```go
type PresignedPut struct {
	URL       string
	Method    string
	Headers   map[string]string
	ExpiresAt time.Time
}

type ObjectInfo struct {
	SizeBytes int64
	MIMEType  string
	ETag      string
}

type Client interface {
	PresignPut(ctx context.Context, key, mimeType string, expires time.Duration) (PresignedPut, error)
	Head(ctx context.Context, key string) (ObjectInfo, error)
}
```

- [ ] **Step 4: Install and wrap the official V2 SDK**

Run: `cd backend && GOPROXY=https://goproxy.cn,direct go get github.com/aliyun/alibabacloud-oss-go-sdk-v2@v1.6.0`

The adapter builds the client with region, optional endpoint, and static credentials. `PresignPut` uses `oss.PutObjectRequest` with a signed `Content-Type`; `Head` uses `oss.HeadObjectRequest`. No OSS SDK type crosses the `aliyunoss` package boundary.

- [ ] **Step 5: Test request translation without OSS network access**

Inject the minimal SDK-facing interface or an HTTP test transport. Assert bucket, key, MIME header, expiration, object size, ETag, and error wrapping. Do not call real OSS in unit tests.

- [ ] **Step 6: Run tests and commit**

```bash
cd backend && go test ./internal/config ./internal/storage/... -count=1
git add backend/internal/config backend/internal/storage backend/.env.example backend/go.mod backend/go.sum
git commit -m "feat: add Aliyun OSS storage signer"
```

### Task 2: Add media records and article associations

**Files:**
- Create: `backend/migrations/000008_create_media.up.sql`
- Create: `backend/migrations/000008_create_media.down.sql`
- Create: `backend/sql/queries/media.sql`
- Regenerate: `backend/internal/postgres/sqlcgen/*.go`
- Create: `backend/internal/media/model.go`
- Create: `backend/internal/media/service.go`
- Create: `backend/internal/media/repository.go`
- Create: `backend/internal/media/mediatest/store.go`
- Test: `backend/internal/media/*_test.go`

**Interfaces:**
- Produces: `Media`, `PresignInput`, `RegisterInput`
- Produces: `Presign`, `Register`, `ListForEntry`

- [ ] **Step 1: Write failing service tests**

Cover MIME allowlist, 20 MiB boundary, unknown entry, generated key prefix, registration Head verification, size/MIME mismatch, width/height validation, duplicate key idempotency, and entry-scoped list.

```go
signed, err := service.Presign(ctx, media.PresignInput{
	AuthorID: 7, EntryID: 42, Filename: "山中.png", MIMEType: "image/png", SizeBytes: 1024,
})
if err != nil { t.Fatal(err) }
if !strings.HasPrefix(signed.ObjectKey, "media/7/42/2026/08/") { t.Fatal(signed.ObjectKey) }
```

- [ ] **Step 2: Add migration 000008**

```sql
CREATE TABLE media (
  id BIGSERIAL PRIMARY KEY,
  object_key VARCHAR(512) NOT NULL UNIQUE,
  url TEXT NOT NULL,
  mime_type VARCHAR(64) NOT NULL,
  size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
  width INT CHECK (width IS NULL OR width > 0),
  height INT CHECK (height IS NULL OR height > 0),
  checksum VARCHAR(128),
  uploaded_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE entry_media (
  entry_id BIGINT NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
  media_id BIGINT NOT NULL REFERENCES media(id) ON DELETE RESTRICT,
  sort_order INT NOT NULL DEFAULT 0,
  PRIMARY KEY (entry_id, media_id)
);
CREATE INDEX idx_entry_media_media ON entry_media (media_id, entry_id);
```

The down migration drops `entry_media` before `media`.

- [ ] **Step 3: Add sqlc queries**

Create queries to check that a live entry belongs to the author, insert media idempotently by object key, attach it to an entry with `ON CONFLICT DO NOTHING`, and list undeleted media for one live entry ordered by association sort order then media ID.

- [ ] **Step 4: Implement domain validation and key generation**

Generate keys with 16 random bytes hex-encoded and an extension derived from the validated MIME type, never from the filename:

```go
media/{authorID}/{entryID}/{yyyy}/{mm}/{random}.{ext}
```

`Presign` checks ownership before signing. `Register` calls `storage.Head`, compares exact size and normalized MIME, derives public URL from the configured base plus escaped object key, inserts the media row, and attaches it to the entry.

Before `Head`, `Register` must require the exact prefix
`media/{authorID}/{entryID}/` and reject every other object key. A signed-in
caller cannot register another entry's object or an arbitrary bucket key.

- [ ] **Step 5: Run tests and commit**

```bash
cd backend
make sqlc
go test ./internal/media -count=1
git add backend/migrations/000008_* backend/sql/queries/media.sql backend/internal/postgres/sqlcgen backend/internal/media
git commit -m "feat: record article media"
```

### Task 3: Expose authenticated media APIs

**Files:**
- Create: `backend/internal/mediahttp/dto.go`
- Create: `backend/internal/mediahttp/handler.go`
- Test: `backend/internal/mediahttp/handler_test.go`
- Modify: `backend/internal/router/router.go`
- Modify: `backend/internal/router/router_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `docs/api.md`

**Interfaces:**
- Produces: `POST /api/v1/admin/media/presign`
- Produces: `POST /api/v1/admin/media`
- Produces: `GET /api/v1/admin/entries/:id/media`

- [ ] **Step 1: Write failing route tests**

Assert all routes require auth. Test valid presign, forbidden MIME, too-large file, nonexistent entry, registration before upload, verified registration, duplicate registration returning the existing record, and current-entry list.

- [ ] **Step 2: Implement exact DTOs**

```go
type presignRequest struct {
	EntryID   int64  `json:"entry_id" binding:"required,min=1"`
	Filename  string `json:"filename" binding:"required"`
	MIMEType  string `json:"mime_type" binding:"required"`
	SizeBytes int64  `json:"size_bytes" binding:"required,min=1"`
}

type registerRequest struct {
	EntryID   int64  `json:"entry_id" binding:"required,min=1"`
	ObjectKey string `json:"object_key" binding:"required"`
	MIMEType  string `json:"mime_type" binding:"required"`
	SizeBytes int64  `json:"size_bytes" binding:"required,min=1"`
	Width     *int   `json:"width"`
	Height    *int   `json:"height"`
}
```

Presign response includes object key, upload URL, method, required headers, and ISO expiration. Media response includes id, URL, MIME, size, width, height, and uploaded time.

- [ ] **Step 3: Wire storage and media services**

When `OSS_ENABLED=false`, startup still succeeds but media routes return `UNAVAILABLE`; do not install a nil signer that panics. When enabled, construct the official adapter and media service in `cmd/server/main.go`, then require the media service in router dependencies.

- [ ] **Step 4: Run backend tests**

Run: `cd backend && go test ./internal/mediahttp ./internal/router ./cmd/server -count=1`

Expected: PASS.

- [ ] **Step 5: Document OSS bucket CORS and API contract**

Document a bucket CORS rule allowing the production admin origin and `http://localhost:5173`, method PUT, allowed header `Content-Type`, and exposed headers `ETag` and `x-oss-request-id`. Do not allow `*` origins in production.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/mediahttp backend/internal/router backend/cmd/server docs/api.md
git commit -m "feat: expose direct media upload APIs"
```

### Task 4: Build resilient browser upload coordination

**Files:**
- Create: `admin/src/api/media.ts`
- Modify: `admin/src/api/index.ts`
- Modify: `admin/src/types/api.ts`
- Create: `admin/src/media/upload-recovery.ts`
- Create: `admin/src/media/upload-coordinator.ts`
- Test: `admin/src/media/*.test.ts`
- Modify: `admin/src/test/setup.ts`

**Interfaces:**
- Produces: `UploadItem`, `UploadStatus`, `createUploadCoordinator`
- Consumes: presign, direct PUT, register, IndexedDB

- [ ] **Step 1: Upgrade IndexedDB and write failing recovery tests**

Upgrade `alive-admin` to version 2 and add `pending-uploads` keyed by upload ID. Store the `File` blob, entry ID, alt, caption, display mode, and timestamps. Test reload recovery and removal after registration.

- [ ] **Step 2: Write failing coordinator tests**

Cover type/size rejection, local preview creation, ordered state transitions, progress, PUT failure, expired presign retry, registration failure, successful cleanup, and cancellation.

```ts
expect(states).toEqual(['queued', 'signing', 'uploading', 'registering', 'complete'])
expect(progress).toEqual(expect.arrayContaining([0, 50, 100]))
```

- [ ] **Step 3: Define upload state**

```ts
export type UploadStatus = 'queued' | 'signing' | 'uploading' | 'registering' | 'failed' | 'complete'
export interface UploadItem {
  id: string
  entryId: number
  file: File
  previewURL: string
  progress: number
  status: UploadStatus
  error: string | null
  media: MediaRecord | null
}
```

- [ ] **Step 4: Implement direct PUT with XMLHttpRequest**

Use XHR because fetch does not provide upload progress. Apply only headers returned by the presign response. Treat any non-2xx response as failure. On retry request a new presign; never reuse an expired URL.

- [ ] **Step 5: Read image dimensions before signing**

Use `createImageBitmap(file)` when available and an `Image` object URL fallback. Registration sends verified local width and height; the backend still verifies size and MIME through HeadObject.

- [ ] **Step 6: Run tests and commit**

```bash
cd admin && npm test -- --run src/media
git add admin/src/api admin/src/types admin/src/media admin/src/test/setup.ts
git commit -m "feat: add resilient direct media upload"
```

### Task 5: Integrate body images, captions, width, reuse, and cover

**Files:**
- Create: `admin/src/editor/image-display.ts`
- Test: `admin/src/editor/image-display.test.ts`
- Modify: `packages/markdown/src/index.ts`
- Modify: `packages/markdown/src/index.test.ts`
- Modify: `frontend/assets/css/prose.css`
- Create: `admin/src/components/writing/MediaUpload.vue`
- Create: `admin/src/components/writing/ArticleMediaPicker.vue`
- Modify: `admin/src/components/writing/ArticleSettings.vue`
- Modify: `admin/src/components/MarkdownEditor.vue`
- Test: corresponding admin tests

**Interfaces:**
- Produces: Markdown `![alt](url "caption")`
- Produces: wide form `![alt](url#alive-wide "caption")`
- Produces: `withImageDisplay(url, 'normal' | 'wide')`

- [ ] **Step 1: Write failing display helper and renderer tests**

```ts
expect(withImageDisplay('https://media.example/a.jpg', 'wide'))
  .toBe('https://media.example/a.jpg#alive-wide')
expect(withImageDisplay('https://media.example/a.jpg#alive-wide', 'normal'))
  .toBe('https://media.example/a.jpg')
expect(renderMarkdown('![山路](https://media.example/a.jpg#alive-wide "清晨")'))
  .toContain('<figure class="image image--wide">')
```

- [ ] **Step 2: Render semantic figures in the shared package**

Add a token transform for a paragraph whose inline children consist of exactly
one image. Transform that standalone image paragraph into a `figure`; do not
emit a block-level figure from the inline image renderer because `<figure>`
inside `<p>` is invalid HTML. The image title becomes `figcaption`, alt remains
on the image, and `#alive-wide` adds `image--wide` while being removed from the
request URL emitted into `src`. Inline images inside ordinary text remain plain
`img` elements. Raw HTML remains disabled.

- [ ] **Step 3: Add frontend wide-image CSS**

Normal figures remain within `--measure`. Wide figures may expand to `--page-width` but never past viewport padding. Captions use the existing UI-font caption rules. Verify 375px has no overflow.

- [ ] **Step 4: Add Milkdown upload and image-block components**

Use `@milkdown/kit/plugin/upload` for paste and drop and the image-block component for editing image attrs. Keep a failed local placeholder in the editor UI but serialize only registered media URLs. On successful registration insert the standard Markdown image with alt, title, and display fragment.

- [ ] **Step 5: Build current-entry reuse UI**

`ArticleMediaPicker` calls the entry media list endpoint, displays thumbnails, and inserts a selected registered URL without uploading again. It does not show media from another article.

- [ ] **Step 6: Replace cover URL input**

`ArticleSettings` offers upload or current-entry selection, then autosaves `cover_url` as the registered media URL. Keep a secondary “粘贴外部 URL” action for existing externally hosted covers.

- [ ] **Step 7: Run tests and builds**

```bash
cd packages/markdown && npx vitest run
cd ../../admin && npm test -- --run src/media src/editor/image-display.test.ts src/components/writing src/components/MarkdownEditor.test.ts && npm run build
cd ../frontend && npm run typecheck && npm run build
```

- [ ] **Step 8: Commit**

```bash
git add packages/markdown admin frontend/assets/css/prose.css
git commit -m "feat: insert and reuse article images"
```

### Task 6: OSS integration and browser checkpoint

**Files:**
- Modify: `docs/progress.md`
- Modify only defects found in media-owned files

- [ ] **Step 1: Configure a non-production OSS test prefix**

Use a dedicated test bucket or `alive-test/` prefix and a restricted access key that can PutObject and HeadObject only for that location. Configure exact CORS origins. Never paste credentials into commands captured in documentation or commit them to `.env.example`.

- [ ] **Step 2: Run automated checks**

```bash
cd backend && make check
cd ../admin && npm test -- --run && npm run build
cd ../frontend && npm run typecheck && npm run build
```

- [ ] **Step 3: Verify real upload scenarios**

Test file picker, drag, clipboard paste, JPEG/PNG/WebP/GIF, rejected SVG, rejected >20 MiB, network failure during PUT, expired presign, registration failure, refresh with pending upload, retry, current-article reuse, cover selection, alt, caption, normal width, and wide width.

- [ ] **Step 4: Verify published rendering**

Publish a disposable article and check image URL, caption, intrinsic dimensions, lazy loading, normal/wide geometry, theme behavior, and 375px overflow in all three themes.

- [ ] **Step 5: Verify traffic path**

Use browser network tools to prove the binary PUT goes directly to the OSS host and the Go API receives only presign and metadata JSON requests.

- [ ] **Step 6: Update evidence and commit**

```bash
git add docs/progress.md
git commit -m "docs: verify Aliyun OSS media upload"
```
