-- name: ListOpenWorlds :many
SELECT world, status, nav_label, sort_order, default_view, revision, updated_at
FROM site_worlds
WHERE status = 'open'
ORDER BY sort_order, world;

-- name: ListAllWorlds :many
SELECT world, status, nav_label, sort_order, default_view, revision, updated_at
FROM site_worlds
ORDER BY sort_order, world;

-- name: GetWorld :one
SELECT world, status, nav_label, sort_order, default_view, revision, updated_at
FROM site_worlds
WHERE world = $1;

-- name: UpdateWorld :one
UPDATE site_worlds
SET
  status = CASE
    WHEN sqlc.arg(set_status)::bool THEN sqlc.arg(status)
    ELSE status
  END,
  nav_label = CASE
    WHEN sqlc.arg(set_nav_label)::bool THEN sqlc.arg(nav_label)
    ELSE nav_label
  END,
  default_view = CASE
    WHEN sqlc.arg(set_default_view)::bool THEN sqlc.arg(default_view)
    ELSE default_view
  END,
  revision = revision + 1
WHERE world = sqlc.arg(world)
  AND revision = sqlc.arg(expected_revision)
RETURNING world, status, nav_label, sort_order, default_view, revision, updated_at;
