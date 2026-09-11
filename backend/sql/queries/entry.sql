-- name: CreateEntry :one
INSERT INTO entries (
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING
    id,
    revision,
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: GetPublicEntryBySlug :one
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.world = sqlc.arg(world)
  AND e.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public';

-- name: GetLinkEntryBySlug :one
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.world = sqlc.arg(world)
  AND e.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility IN ('public', 'unlisted');

-- name: ListPublicEntries :many
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.world = sqlc.arg(world)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public'
  AND (sqlc.narg(category_id)::bigint IS NULL
       OR e.category_id = sqlc.narg(category_id)::bigint)
ORDER BY e.display_order DESC, COALESCE(e.happened_at, e.published_at) DESC, e.id DESC
LIMIT sqlc.arg(limit_) OFFSET sqlc.arg(offset_);

-- name: CountPublicEntries :one
SELECT count(*)
FROM entries
WHERE world = sqlc.arg(world)
  AND deleted_at IS NULL
  AND status = 'published'
  AND visibility = 'public'
  AND (sqlc.narg(category_id)::bigint IS NULL
       OR category_id = sqlc.narg(category_id)::bigint);

-- name: UpdateEntry :one
UPDATE entries
SET
    revision = revision + 1,
    title = CASE WHEN sqlc.arg(set_title)::boolean
                 THEN sqlc.arg(title)::varchar ELSE title END,
    slug = CASE WHEN sqlc.arg(set_slug)::boolean
                THEN sqlc.arg(slug)::varchar ELSE slug END,
    summary = CASE WHEN sqlc.arg(set_summary)::boolean
                   THEN sqlc.narg(summary)::text ELSE summary END,
    content_md = CASE WHEN sqlc.arg(set_content_md)::boolean
                      THEN sqlc.arg(content_md)::text ELSE content_md END,
    word_count = CASE WHEN sqlc.arg(set_content_md)::boolean
                      THEN sqlc.arg(word_count)::int ELSE word_count END,
    cover_url = CASE WHEN sqlc.arg(set_cover_url)::boolean
                     THEN sqlc.narg(cover_url)::text ELSE cover_url END,
    visibility = CASE WHEN sqlc.arg(set_visibility)::boolean
                      THEN sqlc.arg(visibility)::varchar ELSE visibility END,
    meta = CASE WHEN sqlc.arg(set_meta)::boolean
                THEN sqlc.arg(meta)::jsonb ELSE meta END,
    happened_at = CASE WHEN sqlc.arg(set_happened_at)::boolean
                       THEN sqlc.narg(happened_at)::timestamptz ELSE happened_at END,
    category_id = CASE WHEN sqlc.arg(set_category_id)::boolean
                       THEN sqlc.narg(category_id)::bigint ELSE category_id END
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: SoftDeleteEntry :one
UPDATE entries
SET deleted_at = sqlc.arg(deleted_at)
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING id;

-- name: PublishEntry :one
UPDATE entries
SET
    revision = revision + 1,
    status = 'published',
    published_at = COALESCE(published_at, sqlc.arg(published_at)),
    display_order = CASE
      WHEN status <> 'published' THEN (
        SELECT COALESCE(MAX(other.display_order), 0) + 1
        FROM entries other
        WHERE other.world = entries.world
          AND other.deleted_at IS NULL
          AND other.status = 'published'
      )
      ELSE display_order
    END
WHERE entries.id = sqlc.arg(id)
  AND entries.revision = sqlc.arg(expected_revision)
  AND entries.deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: UnpublishEntry :one
UPDATE entries
SET
    revision = revision + 1,
    status = 'draft'
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: ArchiveEntry :one
UPDATE entries
SET
    revision = revision + 1,
    status = 'archived'
WHERE id = sqlc.arg(id)
  AND revision = sqlc.arg(expected_revision)
  AND deleted_at IS NULL
RETURNING
    id,
    revision,
    author_id,
    category_id,
    world,
    kind,
    title,
    slug,
    summary,
    content_md,
    cover_url,
    status,
    visibility,
    meta,
    word_count,
    happened_at,
    published_at,
    created_at,
    updated_at;

-- name: EntrySlugExists :one
SELECT EXISTS (
    SELECT 1
    FROM entries
    WHERE world = sqlc.arg(world)
      AND slug = sqlc.arg(slug)
      AND deleted_at IS NULL
);

-- name: EntrySlugExistsExcluding :one
SELECT EXISTS (
    SELECT 1
    FROM entries
    WHERE world = sqlc.arg(world)
      AND slug = sqlc.arg(slug)
      AND id <> sqlc.arg(excluded_id)
      AND deleted_at IS NULL
);

-- name: ListAdminEntries :many
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    CASE WHEN e.world = 'saying' THEN e.content_md ELSE ''::text END AS content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.deleted_at IS NULL
  AND (sqlc.narg(world)::varchar IS NULL OR e.world = sqlc.narg(world)::varchar)
  AND (sqlc.narg(category_id)::bigint IS NULL OR e.category_id = sqlc.narg(category_id)::bigint)
  AND (sqlc.narg(status)::varchar IS NULL OR e.status = sqlc.narg(status)::varchar)
  AND (
    sqlc.narg(search)::text IS NULL
    OR e.title ILIKE '%' || sqlc.narg(search)::text || '%'
    OR e.slug ILIKE '%' || sqlc.narg(search)::text || '%'
    OR COALESCE(e.summary, '') ILIKE '%' || sqlc.narg(search)::text || '%'
  )
ORDER BY e.updated_at DESC, e.id DESC
LIMIT sqlc.arg(limit_) OFFSET sqlc.arg(offset_);

-- name: CountAdminEntries :one
SELECT count(*)
FROM entries
WHERE deleted_at IS NULL
  AND (sqlc.narg(world)::varchar IS NULL OR world = sqlc.narg(world)::varchar)
  AND (sqlc.narg(category_id)::bigint IS NULL OR category_id = sqlc.narg(category_id)::bigint)
  AND (sqlc.narg(status)::varchar IS NULL OR status = sqlc.narg(status)::varchar)
  AND (
    sqlc.narg(search)::text IS NULL
    OR title ILIKE '%' || sqlc.narg(search)::text || '%'
    OR slug ILIKE '%' || sqlc.narg(search)::text || '%'
    OR COALESCE(summary, '') ILIKE '%' || sqlc.narg(search)::text || '%'
  );

-- name: ListPublishedEntriesForOrdering :many
SELECT
    e.id,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    CASE WHEN e.world = 'saying' THEN e.content_md ELSE ''::text END AS content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.world = sqlc.arg(world)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
ORDER BY e.display_order DESC, e.id DESC;

-- name: DashboardMetrics :one
SELECT
    count(*) AS total_entries,
    count(*) FILTER (WHERE status = 'published') AS published_entries,
    COALESCE(sum(word_count), 0)::bigint AS total_words
FROM entries
WHERE deleted_at IS NULL;

-- name: GetAdminEntryByID :one
SELECT
    e.id,
    e.revision,
    e.author_id,
    e.category_id,
    c.name AS category_name,
    c.slug AS category_slug,
    e.world,
    e.kind,
    e.title,
    e.slug,
    e.summary,
    e.content_md,
    e.cover_url,
    e.status,
    e.visibility,
    e.meta,
    e.word_count,
    e.happened_at,
    e.published_at,
    e.created_at,
    e.updated_at
FROM entries e
LEFT JOIN categories c
    ON c.id = e.category_id
   AND c.world = e.world
WHERE e.id = sqlc.arg(id)
  AND e.deleted_at IS NULL;
