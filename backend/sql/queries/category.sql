-- Queries for categories.
--
-- Unlike entries, there is no visibility rule to enforce here and no soft
-- delete to filter on. A category is a name, a slug and a sort order; it carries
-- no content of its own, so the reads are the same set of rows for everyone and
-- a delete is a delete.
--
-- What is not the same for everyone is the count attached to each category. The
-- public list counts only entries a public reader could reach, so the number
-- beside a category matches what opening it will show. ListCategoriesWithCounts
-- carries that filter; ListCategories has no count at all.

-- name: CreateCategory :one
-- Insert one category.
--
-- sort_order arrives from the caller rather than defaulting here, so that the
-- domain owns what "unspecified" means.
INSERT INTO categories (
    name,
    slug,
    description,
    sort_order
) VALUES (
    $1, $2, $3, $4
)
RETURNING
    id,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at;

-- name: GetCategoryByID :one
-- Read one category by id. Used after a write and by the admin edit form.
SELECT
    id,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
-- Read one category by slug.
--
-- This is how a category page resolves its URL segment, and how a list request
-- filtered by category turns ?category=travel into an id. Resolving first means
-- an unknown slug is a 404 rather than an empty list, which are different
-- answers: one says the URL is wrong, the other says the category is empty.
SELECT
    id,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
WHERE slug = $1;

-- name: ListCategories :many
-- Every category, in display order.
--
-- sort_order first, then id as the tie-break so that categories sharing a
-- sort_order have a stable order rather than whatever the planner returns. No
-- pagination: this is a navigation structure, and one that needs paging is one
-- nobody can navigate.
SELECT
    id,
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at
FROM categories
ORDER BY sort_order, id;

-- name: ListCategoriesWithCounts :many
-- Every category with the number of entries a public reader can see in it.
--
-- LEFT JOIN, not an inner one, so a category with nothing in it still appears
-- with a count of zero. Whether to show an empty category is a display decision,
-- and hiding it here would leave the frontend unable to make it.
--
-- The count conditions sit in the JOIN clause rather than in a WHERE. In a WHERE
-- they would discard the whole category row when no entry matched, turning the
-- LEFT JOIN back into an inner one.
--
-- Same three conditions as the public entry reads: not deleted, published,
-- public. An unlisted entry is deliberately not counted, since it is absent from
-- the list the count describes.
SELECT
    c.id,
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
   AND e.deleted_at IS NULL
   AND e.status = 'published'
   AND e.visibility = 'public'
GROUP BY c.id
ORDER BY c.sort_order, c.id;

-- name: UpdateCategory :one
-- Apply a partial update to one category.
--
-- The same paired-flag shape as UpdateEntry, and for the same reason: description
-- is nullable, so "clear the description" and "leave it alone" both arrive as
-- NULL under COALESCE and would become one statement.
--
-- updated_at is left to the categories_set_updated_at trigger.
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
    name,
    slug,
    description,
    sort_order,
    created_at,
    updated_at;

-- name: DeleteCategory :one
-- Delete one category, returning its id so the caller can tell a hit from a miss.
--
-- A physical delete, unlike an entry. A category is a name and a slug, so
-- recreating one costs nothing, and a deleted_at here would add a condition to
-- every read of a table that has no other reason to need one.
--
-- Entries referencing it are not touched by this statement. The foreign key is
-- ON DELETE SET NULL, so they become uncategorised. That makes this delete
-- succeed quietly even when the category is in use: nothing afterwards records
-- which category those rows pointed at.
DELETE FROM categories
WHERE id = $1
RETURNING id;

-- name: CategorySlugExists :one
-- Whether any category already holds this slug.
--
-- Asked before an insert so the caller gets a conflict naming the field rather
-- than a constraint violation. It does not replace categories_slug_key: two
-- concurrent creates can both read false, and the constraint is what settles it.
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE slug = $1
);

-- name: CategorySlugExistsExcluding :one
-- Whether a category other than this one holds the slug.
--
-- The exclusion is what lets an edit form submit a category's own slug back
-- unchanged without being refused as a conflict with itself.
SELECT EXISTS (
    SELECT 1
    FROM categories
    WHERE slug = sqlc.arg(slug)
      AND id <> sqlc.arg(excluded_id)
);
