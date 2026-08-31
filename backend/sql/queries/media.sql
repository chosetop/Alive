-- name: CreateMedia :one
INSERT INTO media (author_id, object_key, url, mime_type, byte_size)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, author_id, object_key, url, mime_type, byte_size, created_at;

-- name: GetMediaByID :one
SELECT id, author_id, object_key, url, mime_type, byte_size, created_at
FROM media WHERE id = $1;

-- name: ListMediaForEntry :many
SELECT m.id, m.author_id, m.object_key, m.url, m.mime_type, m.byte_size, m.created_at
FROM media m
JOIN entry_media em ON em.media_id = m.id
WHERE em.entry_id = $1
ORDER BY em.created_at, m.id;
