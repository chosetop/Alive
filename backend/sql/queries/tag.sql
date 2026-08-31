-- name: CreateTag :one
INSERT INTO tags (
    name,
    slug
) VALUES (
    $1, $2
)
RETURNING
    id,
    name,
    slug,
    created_at,
    updated_at;

-- name: GetTagByID :one
SELECT
    id,
    name,
    slug,
    created_at,
    updated_at
FROM tags
WHERE id = $1;

-- name: GetTagBySlug :one
SELECT
    id,
    name,
    slug,
    created_at,
    updated_at
FROM tags
WHERE slug = sqlc.arg(slug);

-- name: ListTags :many
SELECT
    t.id,
    t.name,
    t.slug,
    t.created_at,
    t.updated_at,
    count(e.id) AS usage_count
FROM tags t
LEFT JOIN entry_tags et
    ON et.tag_id = t.id
LEFT JOIN entries e
    ON e.id = et.entry_id
   AND e.deleted_at IS NULL
WHERE sqlc.arg(query) = ''
   OR lower(t.name) LIKE '%' || lower(sqlc.arg(query)) || '%'
   OR lower(t.slug) LIKE '%' || lower(sqlc.arg(query)) || '%'
GROUP BY t.id
ORDER BY
    CASE
        WHEN sqlc.arg(query) = '' THEN 1
        WHEN lower(t.name) LIKE lower(sqlc.arg(query)) || '%'
          OR lower(t.slug) LIKE lower(sqlc.arg(query)) || '%' THEN 0
        ELSE 1
    END,
    lower(t.name),
    t.id
LIMIT sqlc.arg(limit_);

-- name: UpdateTag :one
UPDATE tags
SET
    name = CASE WHEN sqlc.arg(set_name)::boolean
                THEN sqlc.arg(name)::citext ELSE name END,
    slug = CASE WHEN sqlc.arg(set_slug)::boolean
                THEN sqlc.arg(slug)::varchar ELSE slug END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    name,
    slug,
    created_at,
    updated_at;

-- name: DeleteTag :one
DELETE FROM tags
WHERE id = $1
RETURNING id;

-- name: TagNameExists :one
SELECT EXISTS (
    SELECT 1
    FROM tags
    WHERE name = sqlc.arg(name)
);

-- name: TagSlugExists :one
SELECT EXISTS (
    SELECT 1
    FROM tags
    WHERE slug = sqlc.arg(slug)
);

-- name: ListTagsByEntryID :many
SELECT
    t.id,
    t.name,
    t.slug,
    t.created_at,
    t.updated_at
FROM tags t
JOIN entry_tags et
    ON et.tag_id = t.id
WHERE et.entry_id = sqlc.arg(entry_id)
ORDER BY lower(t.name), t.id;

-- name: ListPublicEntriesByTag :many
SELECT
    e.world,
    e.kind,
    e.slug,
    e.title,
    e.summary,
    e.cover_url
FROM entries e
JOIN entry_tags et ON et.entry_id = e.id
JOIN tags t ON t.id = et.tag_id
JOIN site_worlds sw ON sw.world = e.world AND sw.status = 'open'
WHERE t.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public'
ORDER BY COALESCE(e.happened_at, e.published_at) DESC, e.id DESC
LIMIT sqlc.arg(limit_) OFFSET sqlc.arg(offset_);

-- name: CountPublicEntriesByTag :one
SELECT count(*)
FROM entries e
JOIN entry_tags et ON et.entry_id = e.id
JOIN tags t ON t.id = et.tag_id
JOIN site_worlds sw ON sw.world = e.world AND sw.status = 'open'
WHERE t.slug = sqlc.arg(slug)
  AND e.deleted_at IS NULL
  AND e.status = 'published'
  AND e.visibility = 'public';
