-- name: CreateMedia :one
INSERT INTO media (author_id, object_key, mime_type, byte_size)
VALUES ($1, $2, $3, $4)
RETURNING id, author_id, object_key, mime_type, byte_size, created_at;

-- name: GetMediaByID :one
SELECT id, author_id, object_key, mime_type, byte_size, created_at
FROM media WHERE id = $1;
