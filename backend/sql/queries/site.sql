-- name: GetSiteSettings :one
SELECT default_theme, revision, updated_at
FROM site_settings
WHERE id = 1;

-- name: UpdateSiteTheme :one
UPDATE site_settings
SET default_theme = sqlc.arg(default_theme),
    revision = revision + 1
WHERE id = 1
  AND revision = sqlc.arg(expected_revision)
RETURNING default_theme, revision, updated_at;
