-- name: CreateCategory :one
INSERT INTO categories (
    world,
    name,
    slug,
    description,
    sort_order
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING
    id,
    world,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at;

-- name: GetCategoryByID :one
SELECT
    id,
    world,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT
    id,
    world,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
WHERE world = sqlc.arg(world)
  AND slug = sqlc.arg(slug);

-- name: ListCategories :many
SELECT
    id,
    world,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
WHERE world = sqlc.arg(world)
ORDER BY sort_order, id;

-- name: ListCategoriesWithCounts :many
SELECT
    c.id,
    c.world,
    c.name,
    c.slug,
    c.description,
    c.sort_order,
    c.created_at,
    c.updated_at,
    count(e.id) AS entry_count
FROM categories c
LEFT JOIN entries e
    ON e.category_id = c.id
   AND e.world = c.world
   AND e.deleted_at IS NULL
   AND e.status = 'published'
   AND e.visibility = 'public'
WHERE c.world = sqlc.arg(world)
GROUP BY c.id
ORDER BY c.sort_order, c.id;

-- name: UpdateCategory :one
UPDATE categories
SET
    name = CASE WHEN sqlc.arg(set_name)::boolean
                THEN sqlc.arg(name)::varchar ELSE name END,
    slug = CASE WHEN sqlc.arg(set_slug)::boolean
                THEN sqlc.arg(slug)::varchar ELSE slug END,
    description = CASE WHEN sqlc.arg(set_description)::boolean
                       THEN sqlc.narg(description)::text ELSE description END,
    sort_order = CASE WHEN sqlc.arg(set_sort_order)::boolean
                      THEN sqlc.arg(sort_order)::int ELSE sort_order END
WHERE id = sqlc.arg(id)
RETURNING
    id,
    world,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at;

-- name: DeleteCategory :one
DELETE FROM categories
WHERE id = $1
RETURNING id;

-- name: CategorySlugExists :one
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE world = sqlc.arg(world)
      AND slug = sqlc.arg(slug)
);

-- name: CategorySlugExistsExcluding :one
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE world = sqlc.arg(world)
      AND slug = sqlc.arg(slug)
      AND id <> sqlc.arg(excluded_id)
);
